package consumer

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	rmq "github.com/rabbitmq/amqp091-go"
)

const (
	reconnectDelay = 3 * time.Second
	rechannelDelay = 2 * time.Second
)

var (
	AlreadyClosedError = errors.New("consumer connection is already closed")
)

type ConsumerConfig struct {
	Addr        string
	ConsumerTag string
	RollupIds   []uint32
}

type BlockData struct {
	RollupId uint32
	Block    types.Block
}

type Consumer struct {
	consumerTag string
	blockstream chan BlockData

	rollupIds []uint32

	isReady           bool
	contextCancelFunc context.CancelFunc
	connection        *rmq.Connection
	onConnClosed      <-chan *rmq.Error
	channel           *rmq.Channel
	onChanClosed      <-chan *rmq.Error

	logger logging.Logger
}

func NewConsumer(config ConsumerConfig, logger logging.Logger) Consumer {
	ctx, cancel := context.WithCancel(context.Background())

	consumer := Consumer{
		consumerTag:       config.ConsumerTag,
		rollupIds:         config.RollupIds,
		blockstream:       make(chan BlockData),
		contextCancelFunc: cancel,
		logger:            logger,
	}

	go consumer.Reconnect(config.Addr, ctx)

	return consumer
}

func (consumer *Consumer) Reconnect(addr string, ctx context.Context) {
	for {
		consumer.logger.Info("Reconnecting...")

		consumer.isReady = false
		conn, err := consumer.connect(addr)
		if err != nil {
			consumer.logger.Warn("Connection setup failed", "err", err)

			select {
			case <-ctx.Done():
				consumer.logger.Info("Consumer context canceled")
				return
			case <-time.After(reconnectDelay):
			}

			continue
		}

		if done := consumer.ResetChannel(conn, ctx); done {
			return
		}

		consumer.logger.Info("Connected")

		select {
		case <-ctx.Done():
			consumer.logger.Info("Consumer context canceled")
			// deref cancel smth?
			break

		case err := <-consumer.onConnClosed:
			if !err.Recover {
				consumer.logger.Error("Can't recover connection", "err", err)
				break
			}

			consumer.logger.Warn("Recovering connection, closed with:", "err", err)

		case err := <-consumer.onChanClosed:
			if !err.Recover {
				consumer.logger.Error("Can't recover connection", "err", err)
				break
			}

			consumer.logger.Warn("Reconnecting channel, closed with:", "err", err)
		}
	}
}

func (consumer *Consumer) connect(addr string) (*rmq.Connection, error) {
	conn, err := rmq.Dial(addr)
	if err != nil {
		return nil, err
	}

	consumer.changeConnection(conn)
	return conn, nil
}

func (consumer *Consumer) changeConnection(conn *rmq.Connection) {
	consumer.connection = conn

	closeNotifier := make(chan *rmq.Error)
	consumer.onConnClosed = conn.NotifyClose(closeNotifier)
}

func (consumer *Consumer) ResetChannel(conn *rmq.Connection, ctx context.Context) bool {
	for {
		consumer.isReady = false

		err := consumer.setupChannel(conn, ctx)
		if err != nil {
			consumer.logger.Warn("Channel setup failed", "err", err)

			select {
			case <-ctx.Done():
				consumer.logger.Info("Consumer context canceled")
				return true

			case rmqError := <-consumer.onConnClosed:
				if rmqError.Recover {
					consumer.logger.Error("Can't recover connection", "err", err)
					return true
				}

				consumer.logger.Warn("Recovering connection, closed with:", "err", err)
				return false
			case <-time.After(rechannelDelay):
			}

			continue
		}

		return false
	}
}

func (consumer *Consumer) setupChannel(conn *rmq.Connection, ctx context.Context) error {
	channel, err := conn.Channel()
	if err != nil {
		return err
	}

	for _, rollupId := range consumer.rollupIds {
		queue, err := channel.QueueDeclare(consumer.getQueueName(rollupId), true, false, false, false, nil)
		if err != nil {
			return err
		}

		deliveries, err := channel.Consume(
			queue.Name,
			consumer.consumerTag,
			false,
			false,
			false,
			false,
			nil,
		)

		if err != nil {
			return err
		}

		go consumer.listen(rollupId, deliveries, ctx)
	}

	consumer.changeChannel(channel)
	consumer.isReady = true
	return nil
}

func (consumer *Consumer) changeChannel(channel *rmq.Channel) {
	consumer.channel = channel

	closeNotifer := make(chan *rmq.Error)
	consumer.onChanClosed = channel.NotifyClose(closeNotifer)
}

func (consumer *Consumer) getQueueName(rollupId uint32) string {
	return strconv.FormatUint(uint64(rollupId), 10)
}

func (consumer *Consumer) listen(rollupId uint32, stream <-chan rmq.Delivery, ctx context.Context) {
	for {
		select {
		case d, ok := <-stream:
			if !ok {
				consumer.logger.Info("Deliveries channel close", "rollupId", rollupId)
				break
			}

			consumer.logger.Info("New delivery", "rollupId", rollupId)

			var block types.Block
			if err := rlp.DecodeBytes(d.Body, &block); err != nil {
				consumer.logger.Warn("Invalid block", "rollupId", rollupId, "err", err)
				continue
			}

			consumer.blockstream <- BlockData{RollupId: rollupId, Block: block}
			d.Ack(true)

		case <-ctx.Done():
			consumer.logger.Info("Consumer context canceled")
			// TODO: some closing and canceling here
			break
		}
	}
}

func (consumer *Consumer) Close(ctx context.Context) error {
	if !consumer.isReady {
		return AlreadyClosedError
	}

	// shut down goroutines
	consumer.contextCancelFunc()

	err := consumer.channel.Close()
	if err != nil {
		return err
	}

	err = consumer.connection.Close()
	if err != nil {
		return err
	}

	consumer.isReady = false
	return nil
}

func (consumer Consumer) GetBlockStream() <-chan BlockData {
	return consumer.blockstream
}
