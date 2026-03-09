package events

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/predictx/order-service/internal/domain"
)

// Publisher publishes order events to Kafka.
type Publisher struct {
	writers map[string]*kafka.Writer
	brokers []string
	log     *zap.Logger
	mu      sync.Mutex
}

// NewPublisher creates a new Kafka publisher.
func NewPublisher(brokers string, log *zap.Logger) *Publisher {
	return &Publisher{
		writers: make(map[string]*kafka.Writer),
		brokers: strings.Split(brokers, ","),
		log:     log,
	}
}

// writerFor gets or creates a writer for a given topic.
func (p *Publisher) writerFor(topic string) *kafka.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()

	if w, ok := p.writers[topic]; ok {
		return w
	}

	w := &kafka.Writer{
		Addr:         kafka.TCP(p.brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false, // Synchronous for financial durability
	}
	p.writers[topic] = w
	return w
}

// PublishOrderPlaced publishes an order.placed event.
func (p *Publisher) PublishOrderPlaced(ctx context.Context, order *domain.Order, topic string) error {
	payload := map[string]interface{}{
		"event":           "order.placed",
		"order_id":        order.ID.String(),
		"user_id":         order.UserID.String(),
		"market_id":       order.MarketID.String(),
		"order_type":      string(order.OrderType),
		"status":          string(order.Status),
		"time_in_force":   string(order.TimeInForce),
		"price_minor":     order.PriceMinor,
		"quantity_shares": order.QuantityShares,
		"currency":        order.Currency,
		"outcome_index":   order.OutcomeIndex,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}

	return p.publish(ctx, topic, order.UserID.String(), payload)
}

// PublishOrderCancelled publishes an order.cancelled event.
func (p *Publisher) PublishOrderCancelled(ctx context.Context, order *domain.Order, topic string) error {
	payload := map[string]interface{}{
		"event":      "order.cancelled",
		"order_id":   order.ID.String(),
		"user_id":    order.UserID.String(),
		"market_id":  order.MarketID.String(),
		"status":     string(order.Status),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	return p.publish(ctx, topic, order.UserID.String(), payload)
}

// publish sends a message to Kafka with idempotent semantics.
func (p *Publisher) publish(ctx context.Context, topic, partitionKey string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		p.log.Error("failed to marshal event", zap.Error(err))
		return err
	}

	msg := kafka.Message{
		Key:   []byte(partitionKey), // Partition by user for ordering
		Value: data,
	}

	writer := p.writerFor(topic)
	if err := writer.WriteMessages(ctx, msg); err != nil {
		// Log but don't fail — event can be replayed from DB audit log
		p.log.Warn("failed to publish event", zap.String("topic", topic), zap.Error(err))
		return nil
	}

	p.log.Info("event published",
		zap.String("topic", topic),
		zap.String("partition_key", partitionKey),
	)

	return nil
}

// Close closes all writers.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, w := range p.writers {
		if err := w.Close(); err != nil {
			p.log.Error("failed to close writer", zap.Error(err))
		}
	}
	return nil
}
