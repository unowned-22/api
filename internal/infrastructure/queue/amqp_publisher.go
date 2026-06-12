package queue

import (
	"context"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
	"github.com/unowned-22/api/internal/domain/event"
)

type Config struct {
	URL      string
	Exchange string
}

type AMQPPublisher struct {
	conn     *amqp091.Connection
	channel  *amqp091.Channel
	exchange string
}

// New creates and returns a new AMQPPublisher.
func New(cfg Config) (*AMQPPublisher, error) {
	conn, err := amqp091.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare the topic exchange (idempotent)
	err = ch.ExchangeDeclare(
		cfg.Exchange, // name
		"topic",      // kind
		true,         // durable
		false,        // auto-delete
		false,        // internal
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return &AMQPPublisher{
		conn:     conn,
		channel:  ch,
		exchange: cfg.Exchange,
	}, nil
}

// Publish sends an event to the exchange with routing key = event name.
func (p *AMQPPublisher) Publish(ctx context.Context, evt event.Event) error {
	err := p.channel.PublishWithContext(
		ctx,
		p.exchange,
		string(evt.Name), // routing key
		false,            // mandatory
		false,            // immediate
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent,
			Body:         evt.Payload,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}
	return nil
}

// Close gracefully closes the channel and connection.
func (p *AMQPPublisher) Close() error {
	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}
	return nil
}

// Compile-time check
var _ event.Publisher = (*AMQPPublisher)(nil)
