package operator

import (
	"errors"
	"fmt"
	"net/rpc"
	"sync"
	"time"

	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/NethermindEth/near-sffl/core"
	"github.com/NethermindEth/near-sffl/core/types/messages"
)

type AggregatorRpcClienter interface {
	core.Metricable

	SendSignedCheckpointTaskResponseToAggregator(signedCheckpointTaskResponse *messages.SignedCheckpointTaskResponse)
	SendSignedStateRootUpdateToAggregator(signedStateRootUpdateMessage *messages.SignedStateRootUpdateMessage)
	SendSignedOperatorSetUpdateToAggregator(signedOperatorSetUpdateMessage *messages.SignedOperatorSetUpdateMessage)
	GetAggregatedCheckpointMessages(fromTimestamp, toTimestamp uint64) (*messages.CheckpointMessages, error)
}

const (
	ResendInterval = 2 * time.Second
)

type RpcMessage = interface{}

type AggregatorRpcClient struct {
	rpcClientLock        sync.RWMutex
	rpcClient            *rpc.Client
	aggregatorIpPortAddr string

	unsentMessagesLock sync.Mutex
	unsentMessages     []RpcMessage
	resendTicker       *time.Ticker

	logger   logging.Logger
	listener RpcClientEventListener
}

var _ core.Metricable = (*AggregatorRpcClient)(nil)

func NewAggregatorRpcClient(aggregatorIpPortAddr string, logger logging.Logger) (*AggregatorRpcClient, error) {
	resendTicker := time.NewTicker(ResendInterval)

	client := &AggregatorRpcClient{
		// set to nil so that we can create an rpc client even if the aggregator is not running
		rpcClient:            nil,
		logger:               logger,
		aggregatorIpPortAddr: aggregatorIpPortAddr,
		unsentMessages:       make([]RpcMessage, 0),
		resendTicker:         resendTicker,
		listener:             &SelectiveRpcClientListener{},
	}

	go client.onTick()
	return client, nil
}

func (c *AggregatorRpcClient) WithMetrics(registry *prometheus.Registry) error {
	listener, err := MakeRpcClientMetrics(registry)
	if err != nil {
		return err
	}

	c.listener = listener
	return nil
}

func (c *AggregatorRpcClient) dialAggregatorRpcClient() error {
	c.rpcClientLock.Lock()
	defer c.rpcClientLock.Unlock()

	if c.rpcClient != nil {
		return nil
	}

	c.logger.Info("rpc client is nil. Dialing aggregator rpc client")

	client, err := rpc.DialHTTP("tcp", c.aggregatorIpPortAddr)
	if err != nil {
		c.logger.Error("Error dialing aggregator rpc client", "err", err)
		return err
	}

	c.rpcClient = client

	return nil
}

func (c *AggregatorRpcClient) InitializeClientIfNotExist() error {
	c.rpcClientLock.RLock()
	if c.rpcClient != nil {
		c.rpcClientLock.RUnlock()
		return nil
	}
	c.rpcClientLock.RUnlock()

	return c.dialAggregatorRpcClient()
}

func (c *AggregatorRpcClient) handleRpcError(err error) error {
	if err == rpc.ErrShutdown {
		go c.handleRpcShutdown()
	}

	return nil
}

func (c *AggregatorRpcClient) handleRpcShutdown() {
	c.rpcClientLock.Lock()
	defer c.rpcClientLock.Unlock()

	if c.rpcClient != nil {
		c.logger.Info("Closing RPC client due to shutdown")

		err := c.rpcClient.Close()
		if err != nil {
			c.logger.Error("Error closing RPC client", "err", err)
		}

		c.rpcClient = nil
	}
}

func (c *AggregatorRpcClient) onTick() {
	for {
		<-c.resendTicker.C

		err := c.InitializeClientIfNotExist()
		if err != nil {
			c.logger.Error("Error initializing client", "err", err)
			continue
		}

		c.unsentMessagesLock.Lock()
		if len(c.unsentMessages) == 0 {
			c.unsentMessagesLock.Unlock()
			continue
		}
		c.unsentMessagesLock.Unlock()

		c.tryResendFromDeque()
	}
}

// Expected to be called with initialized client.
func (c *AggregatorRpcClient) tryResendFromDeque() {
	c.unsentMessagesLock.Lock()
	defer c.unsentMessagesLock.Unlock()

	if len(c.unsentMessages) != 0 {
		c.logger.Info("Resending messages from queue")
	}

	errorPos := 0
	for i := 0; i < len(c.unsentMessages); i++ {
		message := c.unsentMessages[i]

		// Assumes client exists
		var err error
		var reply bool

		switch message := message.(type) {
		case *messages.SignedCheckpointTaskResponse:
			// TODO(edwin): handle error
			err = c.rpcClient.Call("Aggregator.ProcessSignedCheckpointTaskResponse", message, &reply)

		case *messages.SignedStateRootUpdateMessage:
			err = c.rpcClient.Call("Aggregator.ProcessSignedStateRootUpdateMessage", message, &reply)

		case *messages.SignedOperatorSetUpdateMessage:
			err = c.rpcClient.Call("Aggregator.ProcessSignedOperatorSetUpdateMessage", message, &reply)

		default:
			panic("unreachable")
		}

		if err != nil {
			c.logger.Error("Couldn't resend message", "err", err)

			if i == 0 {
				c.logger.Error("Couldn't resend first message, most likely a connection error")
				return
			}

			c.unsentMessages[errorPos] = message
			errorPos++
		}
	}

	c.unsentMessages = c.unsentMessages[:errorPos]
}

func (c *AggregatorRpcClient) sendOperatorMessage(sendCb func() error, message RpcMessage) {
	c.rpcClientLock.RLock()
	defer c.rpcClientLock.RUnlock()

	appendProtected := func() {
		c.unsentMessagesLock.Lock()
		c.unsentMessages = append(c.unsentMessages, message)
		c.unsentMessagesLock.Unlock()
	}

	if c.rpcClient == nil {
		appendProtected()
		return
	}

	c.logger.Info("Sending request to aggregator")
	err := sendCb()
	if err != nil {
		c.handleRpcError(err)
		appendProtected()
		return
	}
}

func (c *AggregatorRpcClient) sendRequest(sendCb func() error) error {
	c.rpcClientLock.RLock()
	defer c.rpcClientLock.RUnlock()

	if c.rpcClient == nil {
		return errors.New("rpc client is nil")
	}

	c.logger.Info("Sending request to aggregator")

	err := sendCb()
	if err != nil {
		c.handleRpcError(err)
		return err
	}

	c.logger.Info("Request successfully sent to aggregator")

	return nil
}

func (c *AggregatorRpcClient) SendSignedCheckpointTaskResponseToAggregator(signedCheckpointTaskResponse *messages.SignedCheckpointTaskResponse) {
	c.logger.Info("Sending signed task response header to aggregator", "signedCheckpointTaskResponse", fmt.Sprintf("%#v", signedCheckpointTaskResponse))

	c.sendOperatorMessage(func() error {
		var reply bool
		err := c.rpcClient.Call("Aggregator.ProcessSignedCheckpointTaskResponse", signedCheckpointTaskResponse, &reply)
		if err != nil {
			c.logger.Info("Received error from aggregator", "err", err)
			return err
		}

		c.logger.Info("Signed task response header accepted by aggregator.", "reply", reply)
		c.listener.OnMessagesReceived()
		return nil
	}, signedCheckpointTaskResponse)
}

func (c *AggregatorRpcClient) SendSignedStateRootUpdateToAggregator(signedStateRootUpdateMessage *messages.SignedStateRootUpdateMessage) {
	c.logger.Info("Sending signed state root update message to aggregator", "signedStateRootUpdateMessage", fmt.Sprintf("%#v", signedStateRootUpdateMessage))

	c.sendOperatorMessage(func() error {
		var reply bool
		err := c.rpcClient.Call("Aggregator.ProcessSignedStateRootUpdateMessage", signedStateRootUpdateMessage, &reply)
		if err != nil {
			c.logger.Info("Received error from aggregator", "err", err)
			return err
		}

		c.logger.Info("Signed state root update message accepted by aggregator.", "reply", reply)
		c.listener.OnMessagesReceived()
		return nil
	}, signedStateRootUpdateMessage)
}

func (c *AggregatorRpcClient) SendSignedOperatorSetUpdateToAggregator(signedOperatorSetUpdateMessage *messages.SignedOperatorSetUpdateMessage) {
	c.logger.Info("Sending operator set update message to aggregator", "signedOperatorSetUpdateMessage", fmt.Sprintf("%#v", signedOperatorSetUpdateMessage))

	c.sendOperatorMessage(func() error {
		var reply bool
		err := c.rpcClient.Call("Aggregator.ProcessSignedOperatorSetUpdateMessage", signedOperatorSetUpdateMessage, &reply)
		if err != nil {
			c.logger.Info("Received error from aggregator", "err", err)
			return err
		}

		c.logger.Info("Signed operator set update message accepted by aggregator.", "reply", reply)
		c.listener.OnMessagesReceived()
		return nil
	}, signedOperatorSetUpdateMessage)
}

func (c *AggregatorRpcClient) GetAggregatedCheckpointMessages(fromTimestamp, toTimestamp uint64) (*messages.CheckpointMessages, error) {
	c.logger.Info("Getting checkpoint messages from aggregator")

	var checkpointMessages messages.CheckpointMessages

	type Args struct {
		FromTimestamp, ToTimestamp uint64
	}

	err := c.sendRequest(func() error {
		err := c.rpcClient.Call("Aggregator.GetAggregatedCheckpointMessages", &Args{fromTimestamp, toTimestamp}, &checkpointMessages)
		if err != nil {
			c.logger.Info("Received error from aggregator", "err", err)
			return err
		}

		c.logger.Info("Checkpoint messages fetched from aggregator")
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &checkpointMessages, nil
}
