package operator

import (
	"context"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"

	opsetupdatereg "github.com/NethermindEth/near-sffl/contracts/bindings/SFFLOperatorSetUpdateRegistry"
	registryrollup "github.com/NethermindEth/near-sffl/contracts/bindings/SFFLRegistryRollup"
	taskmanager "github.com/NethermindEth/near-sffl/contracts/bindings/SFFLTaskManager"
	"github.com/NethermindEth/near-sffl/core"
	"github.com/NethermindEth/near-sffl/core/chainio"
	coretypes "github.com/NethermindEth/near-sffl/core/types"
	"github.com/NethermindEth/near-sffl/metrics"
	"github.com/NethermindEth/near-sffl/operator/attestor"
	"github.com/NethermindEth/near-sffl/types"

	"github.com/Layr-Labs/eigensdk-go/chainio/clients"
	sdkelcontracts "github.com/Layr-Labs/eigensdk-go/chainio/clients/elcontracts"
	"github.com/Layr-Labs/eigensdk-go/chainio/clients/eth"
	"github.com/Layr-Labs/eigensdk-go/chainio/txmgr"
	"github.com/Layr-Labs/eigensdk-go/crypto/bls"
	sdkecdsa "github.com/Layr-Labs/eigensdk-go/crypto/ecdsa"
	"github.com/Layr-Labs/eigensdk-go/logging"
	sdklogging "github.com/Layr-Labs/eigensdk-go/logging"
	sdkmetrics "github.com/Layr-Labs/eigensdk-go/metrics"
	"github.com/Layr-Labs/eigensdk-go/metrics/collectors/economic"
	rpccalls "github.com/Layr-Labs/eigensdk-go/metrics/collectors/rpc_calls"
	"github.com/Layr-Labs/eigensdk-go/nodeapi"
	"github.com/Layr-Labs/eigensdk-go/signerv2"
	sdktypes "github.com/Layr-Labs/eigensdk-go/types"
)

const AVS_NAME = "super-fast-finality-layer"
const SEM_VER = "0.0.1"

type Operator struct {
	config    types.NodeConfig
	logger    logging.Logger
	ethClient eth.EthClient
	// TODO(samlaf): remove both avsWriter and eigenlayerWrite from operator
	// they are only used for registration, so we should make a special registration package
	// this way, auditing this operator code makes it obvious that operators don't need to
	// write to the chain during the course of their normal operations
	// writing to the chain should be done via the cli only
	metricsReg       *prometheus.Registry
	metrics          metrics.Metrics
	nodeApi          *nodeapi.NodeApi
	avsWriter        *chainio.AvsWriter
	avsReader        chainio.AvsReaderer
	avsSubscriber    chainio.AvsSubscriberer
	eigenlayerReader sdkelcontracts.ELReader
	eigenlayerWriter sdkelcontracts.ELWriter
	blsKeypair       *bls.KeyPair
	operatorId       bls.OperatorId
	operatorAddr     common.Address

	// receive new tasks in this chan (typically from listening to onchain event)
	checkpointTaskCreatedChan chan *taskmanager.ContractSFFLTaskManagerCheckpointTaskCreated
	// receive operator set updates in this chan
	// TODO: agree on operatorSetUpdateC vs operatorSetUpdateChan
	operatorSetUpdateChan chan *opsetupdatereg.ContractSFFLOperatorSetUpdateRegistryOperatorSetUpdatedAtBlock

	// ip address of aggregator
	aggregatorServerIpPortAddr string
	// rpc client to send signed task responses to aggregator
	aggregatorRpcClient AggregatorRpcClienter
	// needed when opting in to avs (allow this service manager contract to slash operator)
	sfflServiceManagerAddr common.Address
	// NEAR DA indexer consumer
	attestor attestor.Attestorer
}

func createEthClients(config types.NodeConfig, registry *prometheus.Registry, logger sdklogging.Logger) (eth.EthClient, eth.EthClient, error) {
	if config.EnableMetrics {
		rpcCallsCollector := rpccalls.NewCollector(AVS_NAME, registry)
		ethRpcClient, err := eth.NewInstrumentedClient(config.EthRpcUrl, rpcCallsCollector)
		if err != nil {
			logger.Errorf("Cannot create http ethclient", "err", err)
			return nil, nil, err
		}
		ethWsClient, err := eth.NewInstrumentedClient(config.EthWsUrl, rpcCallsCollector)
		if err != nil {
			logger.Errorf("Cannot create ws ethclient", "err", err)
			return nil, nil, err
		}

		return ethRpcClient, ethWsClient, nil
	}

	ethRpcClient, err := eth.NewClient(config.EthRpcUrl)
	if err != nil {
		logger.Errorf("Cannot create http ethclient", "err", err)
		return nil, nil, err
	}
	ethWsClient, err := eth.NewClient(config.EthWsUrl)
	if err != nil {
		logger.Errorf("Cannot create ws ethclient", "err", err)
		return nil, nil, err
	}

	return ethRpcClient, ethWsClient, nil
}

func createLogger(config *types.NodeConfig) (sdklogging.Logger, error) {
	var logLevel logging.LogLevel
	if config.Production {
		logLevel = sdklogging.Production
	} else {
		logLevel = sdklogging.Development
	}

	return sdklogging.NewZapLogger(logLevel)
}

// TODO(samlaf): config is a mess right now, since the chainio client constructors
//
//	take the config in core (which is shared with aggregator and challenger)
func NewOperatorFromConfig(c types.NodeConfig) (*Operator, error) {
	logger, err := createLogger(&c)
	if err != nil {
		return nil, err
	}

	reg := prometheus.NewRegistry()
	eigenMetrics := sdkmetrics.NewEigenMetrics(AVS_NAME, c.EigenMetricsIpPortAddress, reg, logger)
	avsAndEigenMetrics := metrics.NewAvsAndEigenMetrics(AVS_NAME, eigenMetrics, reg)

	// Setup Node Api
	nodeApi := nodeapi.NewNodeApi(AVS_NAME, SEM_VER, c.NodeApiIpPortAddress, logger)

	ethRpcClient, ethWsClient, err := createEthClients(c, reg, logger)
	if err != nil {
		return nil, err
	}

	blsKeyPassword, ok := os.LookupEnv("OPERATOR_BLS_KEY_PASSWORD")
	if !ok {
		logger.Warnf("OPERATOR_BLS_KEY_PASSWORD env var not set. using empty string")
	}

	blsKeyPair, err := bls.ReadPrivateKeyFromFile(c.BlsPrivateKeyStorePath, blsKeyPassword)
	if err != nil {
		logger.Errorf("Cannot parse bls private key", "err", err)
		return nil, err
	}
	// TODO(samlaf): should we add the chainId to the config instead?
	// this way we can prevent creating a signer that signs on mainnet by mistake
	// if the config says chainId=5, then we can only create a goerli signer
	chainId, err := ethRpcClient.ChainID(context.Background())
	if err != nil {
		logger.Error("Cannot get chainId", "err", err)
		return nil, err
	}

	ecdsaKeyPassword, ok := os.LookupEnv("OPERATOR_ECDSA_KEY_PASSWORD")
	if !ok {
		logger.Warnf("OPERATOR_ECDSA_KEY_PASSWORD env var not set. using empty string")
	}

	signerV2, _, err := signerv2.SignerFromConfig(signerv2.Config{
		KeystorePath: c.EcdsaPrivateKeyStorePath,
		Password:     ecdsaKeyPassword,
	}, chainId)
	if err != nil {
		panic(err)
	}

	chainioConfig := clients.BuildAllConfig{
		EthHttpUrl:                 c.EthRpcUrl,
		EthWsUrl:                   c.EthWsUrl,
		RegistryCoordinatorAddr:    c.AVSRegistryCoordinatorAddress,
		OperatorStateRetrieverAddr: c.OperatorStateRetrieverAddress,
		AvsName:                    AVS_NAME,
		PromMetricsIpPortAddress:   c.EigenMetricsIpPortAddress,
	}
	sdkClients, err := clients.BuildAll(chainioConfig, common.HexToAddress(c.OperatorAddress), signerV2, logger)
	if err != nil {
		panic(err)
	}

	txMgr := txmgr.NewSimpleTxManager(ethRpcClient, logger, signerV2, common.HexToAddress(c.OperatorAddress))
	avsWriter, err := chainio.BuildAvsWriter(
		txMgr, common.HexToAddress(c.AVSRegistryCoordinatorAddress),
		common.HexToAddress(c.OperatorStateRetrieverAddress), ethRpcClient, logger,
	)
	if err != nil {
		logger.Error("Cannot create AvsWriter", "err", err)
		return nil, err
	}

	avsReader, err := chainio.BuildAvsReader(
		common.HexToAddress(c.AVSRegistryCoordinatorAddress),
		common.HexToAddress(c.OperatorStateRetrieverAddress),
		ethRpcClient, logger)
	if err != nil {
		logger.Error("Cannot create AvsReader", "err", err)
		return nil, err
	}

	avsSubscriber, err := chainio.BuildAvsSubscriber(common.HexToAddress(c.AVSRegistryCoordinatorAddress),
		common.HexToAddress(c.OperatorStateRetrieverAddress), ethWsClient, logger,
	)
	if err != nil {
		logger.Error("Cannot create AvsSubscriber", "err", err)
		return nil, err
	}

	// We must register the economic metrics separately because they are exported metrics (from jsonrpc or subgraph calls)
	// and not instrumented metrics: see https://prometheus.io/docs/instrumenting/writing_clientlibs/#overall-structure
	quorumNames := map[sdktypes.QuorumNum]string{
		0: "quorum0",
	}
	economicMetricsCollector := economic.NewCollector(
		sdkClients.ElChainReader, sdkClients.AvsRegistryChainReader,
		AVS_NAME, logger, common.HexToAddress(c.OperatorAddress), quorumNames)
	reg.MustRegister(economicMetricsCollector)

	aggregatorRpcClient, err := NewAggregatorRpcClient(c.AggregatorServerIpPortAddress, logger, avsAndEigenMetrics)
	if err != nil {
		logger.Error("Cannot create AggregatorRpcClient. Is aggregator running?", "err", err)
		return nil, err
	}

	operator := &Operator{
		config:                     c,
		logger:                     logger,
		metricsReg:                 reg,
		metrics:                    avsAndEigenMetrics,
		nodeApi:                    nodeApi,
		ethClient:                  ethRpcClient,
		avsWriter:                  avsWriter,
		avsReader:                  avsReader,
		avsSubscriber:              avsSubscriber,
		eigenlayerReader:           sdkClients.ElChainReader,
		eigenlayerWriter:           sdkClients.ElChainWriter,
		blsKeypair:                 blsKeyPair,
		operatorAddr:               common.HexToAddress(c.OperatorAddress),
		aggregatorServerIpPortAddr: c.AggregatorServerIpPortAddress,
		aggregatorRpcClient:        aggregatorRpcClient,
		checkpointTaskCreatedChan:  make(chan *taskmanager.ContractSFFLTaskManagerCheckpointTaskCreated),
		operatorSetUpdateChan:      make(chan *opsetupdatereg.ContractSFFLOperatorSetUpdateRegistryOperatorSetUpdatedAtBlock),
		sfflServiceManagerAddr:     common.HexToAddress(c.AVSRegistryCoordinatorAddress),
		operatorId:                 [32]byte{0}, // this is set below
	}

	if c.RegisterOperatorOnStartup {
		operatorEcdsaPrivateKey, err := sdkecdsa.ReadKey(
			c.EcdsaPrivateKeyStorePath,
			ecdsaKeyPassword,
		)
		if err != nil {
			return nil, err
		}
		operator.registerOperatorOnStartup(operatorEcdsaPrivateKey, common.HexToAddress(c.TokenStrategyAddr))
	}

	// OperatorId is set in contract during registration so we get it after registering operator.
	operatorId, err := sdkClients.AvsRegistryChainReader.GetOperatorId(&bind.CallOpts{}, operator.operatorAddr)
	if err != nil {
		logger.Error("Cannot get operator id", "err", err)
		return nil, err
	}
	operator.operatorId = operatorId
	logger.Info("Operator info",
		"operatorId", operatorId,
		"operatorAddr", c.OperatorAddress,
		"operatorG1Pubkey", operator.blsKeypair.GetPubKeyG1(),
		"operatorG2Pubkey", operator.blsKeypair.GetPubKeyG2(),
	)

	attestor, err := attestor.NewAttestor(&c, blsKeyPair, operatorId, logger)
	if err != nil {
		return nil, err
	}
	operator.attestor = attestor

	return operator, nil
}

func (o *Operator) Start(ctx context.Context) error {
	operatorIsRegistered, err := o.avsReader.IsOperatorRegistered(&bind.CallOpts{}, o.operatorAddr)
	if err != nil {
		o.logger.Error("Error checking if operator is registered", "err", err)
		return err
	}
	if !operatorIsRegistered {
		// We bubble the error all the way up instead of using logger.Fatal because logger.Fatal prints a huge stack trace
		// that hides the actual error message. This error msg is more explicit and doesn't require showing a stack trace to the user.
		return fmt.Errorf("operator is not registered. Registering operator using the operator-cli before starting operator")
	}

	// TODO: hmm maybe remove start from attestor?
	if err := o.attestor.Start(ctx); err != nil {
		return err
	}

	o.logger.Infof("Starting operator.")

	if o.config.EnableNodeApi {
		o.nodeApi.Start()
	}

	var metricsErrChan <-chan error
	if o.config.EnableMetrics {
		metricsErrChan = o.metrics.Start(ctx, o.metricsReg)
	} else {
		metricsErrChan = make(chan error, 1)
	}

	// TODO(samlaf): wrap this call with increase in avs-node-spec metric
	newTasksSub := o.avsSubscriber.SubscribeToNewTasks(o.checkpointTaskCreatedChan)
	operatorSetUpdateSub := o.avsSubscriber.SubscribeToOperatorSetUpdates(o.operatorSetUpdateChan)

	signedRootsC := o.attestor.GetSignedRootC()
	for {
		select {
		case <-ctx.Done():
			return o.Close()
		case err := <-metricsErrChan:
			// TODO(samlaf); we should also register the service as unhealthy in the node api
			// https://eigen.nethermind.io/docs/spec/api/
			o.logger.Fatal("Error in metrics server", "err", err)
		case err := <-newTasksSub.Err():
			o.logger.Error("Error in websocket subscription", "err", err)
			// TODO(samlaf): write unit tests to check if this fixed the issues we were seeing
			newTasksSub.Unsubscribe()
			// TODO(samlaf): wrap this call with increase in avs-node-spec metric
			newTasksSub = o.avsSubscriber.SubscribeToNewTasks(o.checkpointTaskCreatedChan)
		case checkpointTaskCreatedLog := <-o.checkpointTaskCreatedChan:
			o.metrics.IncNumTasksReceived()
			taskResponse := o.ProcessCheckpointTaskCreatedLog(checkpointTaskCreatedLog)
			signedCheckpointTaskResponse, err := o.SignTaskResponse(taskResponse)
			if err != nil {
				continue
			}

			go o.aggregatorRpcClient.SendSignedCheckpointTaskResponseToAggregator(signedCheckpointTaskResponse)
		case err := <-operatorSetUpdateSub.Err():
			o.logger.Error("Error in websocket subscription", "err", err)
			operatorSetUpdateSub.Unsubscribe()
			operatorSetUpdateSub = o.avsSubscriber.SubscribeToOperatorSetUpdates(o.operatorSetUpdateChan)
		case operatorSetUpdate := <-o.operatorSetUpdateChan:
			go o.handleOperatorSetUpdate(ctx, operatorSetUpdate)

		case signedStateRootUpdateMessage := <-signedRootsC:
			go o.aggregatorRpcClient.SendSignedStateRootUpdateToAggregator(&signedStateRootUpdateMessage)

			continue
		}
	}
}

func (o *Operator) Close() error {
	if err := o.attestor.Close(); err != nil {
		return err
	}

	return nil
}

// Takes a CheckpointTaskCreatedLog struct as input and returns a TaskResponseHeader struct.
// The TaskResponseHeader struct is the struct that is signed and sent to the contract as a task response.
func (o *Operator) ProcessCheckpointTaskCreatedLog(checkpointTaskCreatedLog *taskmanager.ContractSFFLTaskManagerCheckpointTaskCreated) *taskmanager.CheckpointTaskResponse {
	o.logger.Debug("Received new task", "task", checkpointTaskCreatedLog)
	o.logger.Info("Received new task",
		"fromTimestamp", checkpointTaskCreatedLog.Task.FromTimestamp,
		"toTimestamp", checkpointTaskCreatedLog.Task.ToTimestamp,
		"taskIndex", checkpointTaskCreatedLog.TaskIndex,
		"taskCreatedBlock", checkpointTaskCreatedLog.Task.TaskCreatedBlock,
		"quorumNumbers", checkpointTaskCreatedLog.Task.QuorumNumbers,
		"quorumThreshold", checkpointTaskCreatedLog.Task.QuorumThreshold,
	)

	// TODO: build SMT based on stored message agreements and update the test

	taskResponse := &taskmanager.CheckpointTaskResponse{
		ReferenceTaskIndex:     checkpointTaskCreatedLog.TaskIndex,
		StateRootUpdatesRoot:   [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		OperatorSetUpdatesRoot: [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	return taskResponse
}

func (o *Operator) SignTaskResponse(taskResponse *taskmanager.CheckpointTaskResponse) (*coretypes.SignedCheckpointTaskResponse, error) {
	taskResponseHash, err := core.GetCheckpointTaskResponseDigest(taskResponse)
	if err != nil {
		o.logger.Error("Error getting task response header hash. skipping task (this is not expected and should be investigated)", "err", err)
		return nil, err
	}
	blsSignature := o.blsKeypair.SignMessage(taskResponseHash)
	signedCheckpointTaskResponse := &coretypes.SignedCheckpointTaskResponse{
		TaskResponse: *taskResponse,
		BlsSignature: *blsSignature,
		OperatorId:   o.operatorId,
	}
	o.logger.Debug("Signed task response", "signedCheckpointTaskResponse", signedCheckpointTaskResponse)
	return signedCheckpointTaskResponse, nil
}

func (o *Operator) handleOperatorSetUpdate(ctx context.Context, data *opsetupdatereg.ContractSFFLOperatorSetUpdateRegistryOperatorSetUpdatedAtBlock) error {
	operatorSetDelta, err := o.avsReader.GetOperatorSetUpdateDelta(ctx, data.Id)
	if err != nil {
		o.logger.Errorf("Couldn't get Operator set update delta: %v for block: %v", err, data.Id)
		return err
	}

	operators := make([]registryrollup.OperatorsOperator, len(operatorSetDelta))
	for i := 0; i < len(operatorSetDelta); i++ {
		operators[i] = registryrollup.OperatorsOperator{
			Pubkey: registryrollup.BN254G1Point{
				X: operatorSetDelta[i].Pubkey.X,
				Y: operatorSetDelta[i].Pubkey.Y,
			},
			Weight: operatorSetDelta[i].Weight,
		}
	}

	message := registryrollup.OperatorSetUpdateMessage{
		Id:        data.Id,
		Timestamp: data.Timestamp,
		Operators: operators,
	}

	signedMessage, err := SignOperatorSetUpdate(message, o.blsKeypair, o.operatorId)
	if err != nil {
		o.logger.Error("Couldn't sign operator set update message", "err", err)
		return err
	}
	o.logger.Debug("Signed operator set update response", "signedMessage", signedMessage)

	o.aggregatorRpcClient.SendSignedOperatorSetUpdateToAggregator(signedMessage)
	return nil
}

func SignOperatorSetUpdate(message registryrollup.OperatorSetUpdateMessage, blsKeyPair *bls.KeyPair, operatorId bls.OperatorId) (*coretypes.SignedOperatorSetUpdateMessage, error) {
	messageHash, err := core.GetOperatorSetUpdateMessageDigest(&message)
	if err != nil {
		return nil, err
	}
	signature := blsKeyPair.SignMessage(messageHash)
	signedOperatorSetUpdate := coretypes.SignedOperatorSetUpdateMessage{
		Message:      message,
		OperatorId:   operatorId,
		BlsSignature: *signature,
	}

	return &signedOperatorSetUpdate, nil
}
