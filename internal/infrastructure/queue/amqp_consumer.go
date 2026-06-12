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

type ConsumerConfig struct {
	URL      string
	Exchange string
	Queue    string // e.g. "app.worker"
	Tag      string // consumer tag
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

// New creates a new AMQPConsumer and declares the queue + binding.
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

	// Declare durable queue
	_, err = ch.QueueDeclare(
		cfg.Queue, // name
		true,      // durable
		false,     // autoDelete
		false,     // exclusive
		false,     // noWait
		nil,       // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange for each handler's event type
	for eventName := range handlers {
		err = ch.QueueBind(
			cfg.Queue,         // queue
			string(eventName), // routingKey
			cfg.Exchange,      // exchange
			false,             // noWait
			nil,               // args
		)
		if err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("failed to bind queue for event %s: %w", eventName, err)
		}
	}

	consumer := &AMQPConsumer{
		conn:     conn,
		channel:  ch,
		exchange: cfg.Exchange,
		queue:    cfg.Queue,
		tag:      cfg.Tag,
		handlers: handlers,
		stopCh:   make(chan struct{}),
	}

	return consumer, nil
}

// Consume starts listening for messages in a goroutine.
// It returns immediately; the consumer runs in the background.
func (c *AMQPConsumer) Consume() error {
	deliveryCh, err := c.channel.Consume(
		c.queue, // queue
		c.tag,   // consumer
		false,   // autoAck (we'll ack manually)
		false,   // exclusive
		false,   // noLocal
		false,   // noWait
		nil,     // args
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

// handleMessage processes a single message.
func (c *AMQPConsumer) handleMessage(delivery amqp091.Delivery) {
	startTime := time.Now()
	eventName := event.Name(delivery.RoutingKey)

	handler, exists := c.handlers[eventName]
	if !exists {
		logger.Log.WithFields(map[string]interface{}{
			"event":       eventName,
			"duration_ms": time.Since(startTime).Milliseconds(),
		}).Warn("No handler registered for event")
		delivery.Nack(false, false) // Do not requeue
		return
	}

	// Create context with timeout for handler execution
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := handler.Handle(ctx, delivery.Body)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{
			"event":       eventName,
			"duration_ms": duration,
		}).Error("Failed to handle event")
		delivery.Nack(false, false) // Do not requeue on error
		return
	}

	logger.Log.WithFields(map[string]interface{}{
		"event":       eventName,
		"duration_ms": duration,
	}).Info("Event processed successfully")

	delivery.Ack(false) // Single acknowledgment
}

// Shutdown gracefully stops the consumer.
func (c *AMQPConsumer) Shutdown(ctx context.Context) error {
	var shutdownErr error

	c.once.Do(func() {
		// Signal the message loop to stop
		close(c.stopCh)

		// Cancel consumption from the queue
		if err := c.channel.Cancel(c.tag, false); err != nil {
			shutdownErr = fmt.Errorf("failed to cancel consumer: %w", err)
			return
		}

		// Wait for the message processing loop to finish or context to expire
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

		// Close channel and connection
		if err := c.channel.Close(); err != nil {
			logger.Log.WithError(err).Warn("Failed to close AMQP channel")
		}
		if err := c.conn.Close(); err != nil {
			logger.Log.WithError(err).Warn("Failed to close AMQP connection")
		}
	})

	return shutdownErr
}
