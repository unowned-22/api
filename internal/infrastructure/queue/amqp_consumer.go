package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/logger"
)

// ConsumerConfig holds all configuration for the AMQP consumer.
//
// Dead-letter exchange (DLX) setup:
// When a handler returns an error, the message is Nack'd with requeue=false.
// RabbitMQ will forward it to DeadLetterExchange / DeadLetterRoutingKey instead
// of silently dropping it. Both the DLX and the DLQ are declared by NewConsumer
// so no manual broker configuration is required.
//
// Environment variables (set in config.go):
//
//	RABBITMQ_DLX              — exchange name, default "app.dlx"
//	RABBITMQ_DLX_ROUTING_KEY  — DLQ name / routing key, default "app.worker.dead"
type ConsumerConfig struct {
	URL      string
	Exchange string
	Queue    string // e.g. "app.worker"
	Tag      string // consumer tag

	// DeadLetterExchange is the name of the exchange that receives messages
	// rejected by handlers. Must be a durable direct exchange.
	DeadLetterExchange string // e.g. "app.dlx"

	// DeadLetterRoutingKey is the routing key used when publishing to the DLX.
	// A queue with this name is declared and bound to the DLX automatically.
	DeadLetterRoutingKey string // e.g. "app.worker.dead"
}

// AMQPConsumer listens to RabbitMQ for events and dispatches them to registered handlers.
type AMQPConsumer struct {
	conn     *amqp091.Connection
	channel  *amqp091.Channel
	exchange string
	queue    string
	tag      string
	handlers map[event.Name]event.Handler

	// Lifecycle management
	deliveryCh <-chan amqp091.Delivery
	stopCh     chan struct{}
	wg         sync.WaitGroup
	once       sync.Once
}

// NewConsumer creates a new AMQPConsumer, declares the main queue with a
// dead-letter exchange, declares the DLX and DLQ, and binds all handler
// routing keys to the main exchange.
func NewConsumer(cfg ConsumerConfig, handlers map[event.Name]event.Handler) (*AMQPConsumer, error) {
	if len(handlers) == 0 {
		return nil, fmt.Errorf("at least one handler must be provided")
	}

	conn, err := amqp091.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// --- Dead-letter exchange & queue ---
	// Declare the DLX first so that the main queue can reference it.
	if cfg.DeadLetterExchange != "" {
		if err := ch.ExchangeDeclare(
			cfg.DeadLetterExchange,
			"direct",
			true,  // durable
			false, // autoDelete
			false, // internal
			false, // noWait
			nil,
		); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("failed to declare dead-letter exchange %q: %w", cfg.DeadLetterExchange, err)
		}

		// Declare the DLQ. Its name doubles as the routing key to keep things simple.
		if _, err := ch.QueueDeclare(
			cfg.DeadLetterRoutingKey, // name
			true,                     // durable
			false,                    // autoDelete
			false,                    // exclusive
			false,                    // noWait
			nil,
		); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("failed to declare dead-letter queue %q: %w", cfg.DeadLetterRoutingKey, err)
		}

		if err := ch.QueueBind(
			cfg.DeadLetterRoutingKey, // queue
			cfg.DeadLetterRoutingKey, // routing key
			cfg.DeadLetterExchange,   // exchange
			false,
			nil,
		); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("failed to bind dead-letter queue: %w", err)
		}
	}

	// --- Main queue ---
	// x-dead-letter-exchange routes rejected/expired messages to the DLX.
	mainQueueArgs := amqp091.Table{}
	if cfg.DeadLetterExchange != "" {
		mainQueueArgs["x-dead-letter-exchange"] = cfg.DeadLetterExchange
		mainQueueArgs["x-dead-letter-routing-key"] = cfg.DeadLetterRoutingKey
	}

	if _, err := ch.QueueDeclare(
		cfg.Queue,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		mainQueueArgs,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange for each handler's event type.
	for eventName := range handlers {
		if err := ch.QueueBind(
			cfg.Queue,         // queue
			string(eventName), // routingKey
			cfg.Exchange,      // exchange
			false,             // noWait
			nil,
		); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("failed to bind queue for event %q: %w", eventName, err)
		}
	}

	return &AMQPConsumer{
		conn:     conn,
		channel:  ch,
		exchange: cfg.Exchange,
		queue:    cfg.Queue,
		tag:      cfg.Tag,
		handlers: handlers,
		stopCh:   make(chan struct{}),
	}, nil
}

// Consume starts consuming messages from the queue in a background goroutine.
func (c *AMQPConsumer) Consume() error {
	deliveryCh, err := c.channel.Consume(
		c.queue,
		c.tag,
		false, // autoAck — we Ack/Nack manually
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	c.deliveryCh = deliveryCh
	c.wg.Add(1)
	go c.processMessages()
	return nil
}

// processMessages is the main message processing loop.
func (c *AMQPConsumer) processMessages() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopCh:
			logger.Log.Info("Consumer message loop stopping")
			return
		case delivery := <-c.deliveryCh:
			if delivery.Body == nil {
				// Channel closed
				return
			}
			c.handleMessage(delivery)
		}
	}
}

// handleMessage processes a single delivery.
//
// On success: Ack.
// On handler error: Nack with requeue=false. If a dead-letter exchange is
// configured on the queue, RabbitMQ forwards the message there automatically.
// On unknown event: Nack with requeue=false (no handler will ever exist).
func (c *AMQPConsumer) handleMessage(delivery amqp091.Delivery) {
	startTime := time.Now()
	eventName := event.Name(delivery.RoutingKey)

	handler, exists := c.handlers[eventName]
	if !exists {
		logger.Log.WithFields(map[string]interface{}{
			"event":       eventName,
			"duration_ms": time.Since(startTime).Milliseconds(),
		}).Warn("No handler registered for event")
		// requeue=false: an unregistered event will never be handled.
		// The DLX picks it up for investigation.
		delivery.Nack(false, false)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := handler.Handle(ctx, delivery.Body)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{
			"event":       eventName,
			"duration_ms": duration,
		}).Error("Failed to handle event")
		// requeue=false: the DLX (if configured) takes the message.
		// This avoids a tight retry loop inside RabbitMQ; retry policy is
		// handled at the DLQ level (operator decision) or via the outbox
		// retry_count mechanism on the publishing side.
		delivery.Nack(false, false)
		return
	}

	logger.Log.WithFields(map[string]interface{}{
		"event":       eventName,
		"duration_ms": duration,
	}).Info("Event processed successfully")

	delivery.Ack(false)
}

// Shutdown gracefully stops the consumer.
func (c *AMQPConsumer) Shutdown(ctx context.Context) error {
	var shutdownErr error

	c.once.Do(func() {
		close(c.stopCh)

		if err := c.channel.Cancel(c.tag, false); err != nil {
			shutdownErr = fmt.Errorf("failed to cancel consumer: %w", err)
			return
		}

		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logger.Log.Info("Consumer loop finished gracefully")
		case <-ctx.Done():
			logger.Log.Warn("Consumer shutdown timeout exceeded")
			shutdownErr = fmt.Errorf("consumer shutdown timeout")
		}

		if err := c.channel.Close(); err != nil {
			logger.Log.WithError(err).Warn("Failed to close AMQP channel")
		}
		if err := c.conn.Close(); err != nil {
			logger.Log.WithError(err).Warn("Failed to close AMQP connection")
		}
	})

	return shutdownErr
}
