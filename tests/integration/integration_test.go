package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Layr-Labs/eigensdk-go/chainio/clients"
	"github.com/Layr-Labs/eigensdk-go/chainio/clients/eth"
	"github.com/Layr-Labs/eigensdk-go/chainio/txmgr"
	sdklogging "github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/Layr-Labs/eigensdk-go/signerv2"
	sdkutils "github.com/Layr-Labs/eigensdk-go/utils"
	"github.com/NethermindEth/near-sffl/aggregator"
	"github.com/NethermindEth/near-sffl/core/chainio"
	"github.com/NethermindEth/near-sffl/core/config"
	"github.com/NethermindEth/near-sffl/operator"
	"github.com/NethermindEth/near-sffl/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type IntegrationClients struct {
	Sdkclients clients.Clients
}

func TestIntegration(t *testing.T) {
	log.Println("This test takes ~50 seconds to run...")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	/* Start the anvil chain */
	anvilC := startAnvilTestContainer(ctx, "8545")
	// Not sure why but deferring anvilC.Terminate() causes a panic when the test finishes...
	// so letting it terminate silently for now
	anvilEndpoint, err := anvilC.Endpoint(ctx, "")
	if err != nil {
		t.Error(err)
	}

	anvilRollupC := startAnvilTestContainer(ctx, "8547")
	anvilRollupEndpoint, err := anvilRollupC.Endpoint(ctx, "")
	if err != nil {
		t.Error(err)
	}

	time.Sleep(4 * time.Second)

	var aggConfigRaw config.ConfigRaw
	aggConfigFilePath := "../../config-files/aggregator.yaml"
	sdkutils.ReadYamlConfig(aggConfigFilePath, &aggConfigRaw)
	aggConfigRaw.EthRpcUrl = "http://" + anvilEndpoint
	aggConfigRaw.EthWsUrl = "ws://" + anvilEndpoint

	var sfflDeploymentRaw config.SFFLDeploymentRaw
	sfflDeploymentFilePath := "../../contracts/evm/script/output/31337/sffl_avs_deployment_output.json"
	sdkutils.ReadJsonConfig(sfflDeploymentFilePath, &sfflDeploymentRaw)

	logger, err := sdklogging.NewZapLogger(aggConfigRaw.Environment)
	if err != nil {
		t.Fatalf("Failed to create logger: %s", err.Error())
	}
	ethRpcClient, err := eth.NewClient(aggConfigRaw.EthRpcUrl)
	if err != nil {
		t.Fatalf("Failed to create eth client: %s", err.Error())
	}
	ethWsClient, err := eth.NewClient(aggConfigRaw.EthWsUrl)
	if err != nil {
		t.Fatalf("Failed to create eth client: %s", err.Error())
	}

	aggregatorEcdsaPrivateKeyString := "0x2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6"
	if aggregatorEcdsaPrivateKeyString[:2] == "0x" {
		aggregatorEcdsaPrivateKeyString = aggregatorEcdsaPrivateKeyString[2:]
	}
	aggregatorEcdsaPrivateKey, err := crypto.HexToECDSA(aggregatorEcdsaPrivateKeyString)
	if err != nil {
		t.Fatalf("Cannot parse ecdsa private key: %s", err.Error())
	}
	aggregatorAddr, err := sdkutils.EcdsaPrivateKeyToAddress(aggregatorEcdsaPrivateKey)
	if err != nil {
		t.Fatalf("Cannot get operator address: %s", err.Error())
	}

	chainId, err := ethRpcClient.ChainID(context.Background())
	if err != nil {
		t.Fatalf("Cannot get chainId: %s", err.Error())
	}

	privateKeySigner, _, err := signerv2.SignerFromConfig(signerv2.Config{PrivateKey: aggregatorEcdsaPrivateKey}, chainId)
	if err != nil {
		t.Fatalf("Cannot create signer: %s", err.Error())
	}
	txMgr := txmgr.NewSimpleTxManager(ethRpcClient, logger, privateKeySigner, aggregatorAddr)

	config := &config.Config{
		EcdsaPrivateKey:                aggregatorEcdsaPrivateKey,
		Logger:                         logger,
		EthHttpRpcUrl:                  aggConfigRaw.EthRpcUrl,
		EthHttpClient:                  ethRpcClient,
		EthWsRpcUrl:                    aggConfigRaw.EthWsUrl,
		EthWsClient:                    ethWsClient,
		OperatorStateRetrieverAddr:     common.HexToAddress(sfflDeploymentRaw.Addresses.OperatorStateRetrieverAddr),
		SFFLRegistryCoordinatorAddr:    common.HexToAddress(sfflDeploymentRaw.Addresses.RegistryCoordinatorAddr),
		AggregatorServerIpPortAddr:     aggConfigRaw.AggregatorServerIpPortAddr,
		AggregatorRestServerIpPortAddr: aggConfigRaw.AggregatorRestServerIpPortAddr,
		AggregatorDatabasePath:         aggConfigRaw.AggregatorDatabasePath,
		RegisterOperatorOnStartup:      aggConfigRaw.RegisterOperatorOnStartup,
		TxMgr:                          txMgr,
		AggregatorAddress:              aggregatorAddr,
	}

	/* Prepare the config file for operator */
	nodeConfig := types.NodeConfig{}
	nodeConfigFilePath := "../../config-files/operator.anvil.yaml"
	err = sdkutils.ReadYamlConfig(nodeConfigFilePath, &nodeConfig)
	if err != nil {
		t.Fatalf("Failed to read yaml config: %s", err.Error())
	}
	/* Register operator*/
	// log.Println("registering operator for integration tests")
	// we need to do this dynamically and can't just hardcode a registered operator into the anvil
	// state because the anvil state dump doesn't also dump the receipts tree so we lose events,
	// and the aggregator thus can't get the operator's pubkey
	// operatorRegistrationCmd := exec.Command("bash", "./operator-registration.sh")
	// err = operatorRegistrationCmd.Run()
	// if err != nil {
	// 	t.Fatalf("Failed to register operator: %s", err.Error())
	// }

	/* start operator */
	// the passwords are set to empty strings
	log.Println("starting operator for integration tests")
	os.Setenv("OPERATOR_BLS_KEY_PASSWORD", "")
	os.Setenv("OPERATOR_ECDSA_KEY_PASSWORD", "")
	nodeConfig.BlsPrivateKeyStorePath = "../keys/test.bls.key.json"
	nodeConfig.EcdsaPrivateKeyStorePath = "../keys/test.ecdsa.key.json"
	nodeConfig.RegisterOperatorOnStartup = true
	nodeConfig.EthRpcUrl = "http://" + anvilEndpoint
	nodeConfig.EthWsUrl = "ws://" + anvilEndpoint
	for id, _ := range nodeConfig.RollupIdsToRpcUrls {
		nodeConfig.RollupIdsToRpcUrls[id] = "ws://" + anvilRollupEndpoint
	}

	operator, err := operator.NewOperatorFromConfig(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create operator: %s", err.Error())
	}
	go operator.Start(ctx)
	log.Println("Started operator. Sleeping 15 seconds to give it time to register...")
	time.Sleep(15 * time.Second)

	/* start aggregator */
	log.Println("starting aggregator for integration tests")
	agg, err := aggregator.NewAggregator(config)
	if err != nil {
		t.Fatalf("Failed to create aggregator: %s", err.Error())
	}
	go agg.Start(ctx)
	log.Println("Started aggregator. Sleeping 20 seconds to give operator time to answer task 1...")
	time.Sleep(20 * time.Second)

	// get avsRegistry client to interact with the chain
	avsReader, err := chainio.BuildAvsReaderFromConfig(config)
	if err != nil {
		t.Fatalf("Cannot create AVS Reader: %s", err.Error())
	}

	// check if the task is recorded in the contract for task index 1
	taskHash, err := avsReader.AvsServiceBindings.TaskManager.AllCheckpointTaskHashes(&bind.CallOpts{}, 1)
	if err != nil {
		t.Fatalf("Cannot get task hash: %s", err.Error())
	}
	if taskHash == [32]byte{} {
		t.Fatalf("Task hash is empty")
	}

	// check if the task response is recorded in the contract for task index 1
	taskResponseHash, err := avsReader.AvsServiceBindings.TaskManager.AllCheckpointTaskResponses(&bind.CallOpts{}, 1)
	log.Printf("taskResponseHash: %v", taskResponseHash)
	if err != nil {
		t.Fatalf("Cannot get task response hash: %s", err.Error())
	}
	if taskResponseHash == [32]byte{} {
		t.Fatalf("Task response hash is empty")
	}

}

// TODO(samlaf): have to advance chain to a block where the task is answered
func startAnvilTestContainer(ctx context.Context, exposedPort string) testcontainers.Container {
	integrationDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	req := testcontainers.ContainerRequest{
		Image: "ghcr.io/foundry-rs/foundry:latest",
		Mounts: testcontainers.ContainerMounts{
			testcontainers.ContainerMount{
				Source: testcontainers.GenericBindMountSource{
					HostPath: filepath.Join(integrationDir, "../anvil/data/avs-and-eigenlayer-deployed-anvil-state.json"),
				},
				Target: "/root/.anvil/state.json",
			},
		},
		Entrypoint:   []string{"anvil"},
		Cmd:          []string{"--host", "0.0.0.0", "--load-state", "/root/.anvil/state.json", "--port", exposedPort},
		ExposedPorts: []string{exposedPort + "/tcp"},
		WaitingFor:   wait.ForLog("Listening on"),
	}
	anvilC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		panic(err)
	}

	advanceChain(anvilC)

	return anvilC
}

func advanceChain(anvilC testcontainers.Container) {
	anvilEndpoint, err := anvilC.Endpoint(context.Background(), "")
	if err != nil {
		panic(err)
	}
	rpcUrl := "http://" + anvilEndpoint
	privateKey := "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(
			`forge script script/utils/Utils.sol --sig "advanceChainByNBlocks(uint256)" 100 --rpc-url %s --private-key %s --broadcast`,
			rpcUrl, privateKey),
	)
	cmd.Dir = "../../contracts/evm"

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

	if err != nil {
		fmt.Println(stderr.String())
		panic(err)
	}
	fmt.Println(stdout.String())
}
