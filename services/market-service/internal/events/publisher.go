package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Publisher publishes market lifecycle events to Kafka.
type Publisher struct {
	writers map[string]*kafka.Writer
	brokers []string
	log     *zap.Logger
	mu      sync.Mutex
}

// MarketCreatedEvent is the payload for market.created.
type MarketCreatedEvent struct {
	Event      string    `json:"event"`
	MarketID   string    `json:"market_id"`
	Category   string    `json:"category"`
	ClosesAt   time.Time `json:"closes_at"`
	Timestamp  time.Time `json:"timestamp"`
}

// MarketVoidedEvent is the payload for market.voided.
type MarketVoidedEvent struct {
	Event     string    `json:"event"`
	MarketID  string    `json:"market_id"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// NewPublisher creates a Kafka publisher for the given broker addresses.
func NewPublisher(brokers string, log *zap.Logger) *Publisher {
	return &Publisher{
		writers: make(map[string]*kafka.Writer),
		brokers: strings.Split(brokers, ","),
		log:     log,
	}
}

func (p *Publisher) writerFor(topic string) *kafka.Writer {
	if w, ok := p.writers[topic]; ok {
		return w
	}
	w := &kafka.Writer{
		Addr:         kafka.TCP(p.brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}
	p.writers[topic] = w
	return w
}

func (p *Publisher) publish(ctx context.Context, topic string, key []byte, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("events: marshal: %w", err)
	}

	p.mu.Lock()
	w := p.writerFor(topic)
	p.mu.Unlock()

	if err := w.WriteMessages(ctx, kafka.Message{Key: key, Value: data}); err != nil {
		p.log.Warn("events: publish failed",
			zap.String("topic", topic),
			zap.Error(err),
		)
		return err
	}

	p.log.Info("events: published", zap.String("topic", topic), zap.String("key", string(key)))
	return nil
}

// PublishMarketCreated emits market.created when a market goes active.
func (p *Publisher) PublishMarketCreated(ctx context.Context, topic string, marketID uuid.UUID, category, closesAt string) {
	evt := MarketCreatedEvent{
		Event:    "market_created",
		MarketID: marketID.String(),
		Category: category,
		ClosesAt: parseTime(closesAt),
		Timestamp: time.Now().UTC(),
	}
	if err := p.publish(ctx, topic, []byte(marketID.String()), evt); err != nil {
		p.log.Warn("events: market.created failed", zap.String("market_id", marketID.String()), zap.Error(err))
	}
}

// PublishMarketVoided emits market.voided when a market is cancelled.
func (p *Publisher) PublishMarketVoided(ctx context.Context, topic string, marketID uuid.UUID, reason string) {
	evt := MarketVoidedEvent{
		Event:     "market_voided",
		MarketID:  marketID.String(),
		Reason:    reason,
		Timestamp: time.Now().UTC(),
	}
	if err := p.publish(ctx, topic, []byte(marketID.String()), evt); err != nil {
		p.log.Warn("events: market.voided failed", zap.String("market_id", marketID.String()), zap.Error(err))
	}
}

// Close shuts down all Kafka writers.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var lastErr error
	for _, w := range p.writers {
		if err := w.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
