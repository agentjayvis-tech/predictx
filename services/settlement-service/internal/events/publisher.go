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

// SettlementCompletedEvent is published on settlement.completed.
type SettlementCompletedEvent struct {
	Event        string    `json:"event"`
	SettlementID string    `json:"settlement_id"`
	MarketID     string    `json:"market_id"`
	WinnerCount  int       `json:"winner_count"`
	NetPoolMinor int64     `json:"net_pool_minor"`
	Timestamp    time.Time `json:"timestamp"`
}

// SettlementVoidedEvent is published on settlement.voided (market refunded).
type SettlementVoidedEvent struct {
	Event     string    `json:"event"`
	MarketID  string    `json:"market_id"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// Publisher publishes settlement lifecycle events to Kafka.
type Publisher struct {
	writers map[string]*kafka.Writer
	brokers []string
	log     *zap.Logger
	mu      sync.Mutex
}

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

// PublishSettlementCompleted emits settlement.completed after successful payout.
func (p *Publisher) PublishSettlementCompleted(ctx context.Context, settlementID, marketID uuid.UUID, winnerCount int, netPoolMinor int64) {
	evt := SettlementCompletedEvent{
		Event:        "settlement_completed",
		SettlementID: settlementID.String(),
		MarketID:     marketID.String(),
		WinnerCount:  winnerCount,
		NetPoolMinor: netPoolMinor,
		Timestamp:    time.Now().UTC(),
	}
	topic := "settlement.completed"
	if err := p.publish(ctx, topic, []byte(marketID.String()), evt); err != nil {
		p.log.Warn("events: settlement.completed failed",
			zap.String("settlement_id", settlementID.String()),
			zap.Error(err),
		)
	}
}

// PublishSettlementVoided emits settlement.voided after a market refund.
func (p *Publisher) PublishSettlementVoided(ctx context.Context, marketID uuid.UUID, reason string) {
	evt := SettlementVoidedEvent{
		Event:     "settlement_voided",
		MarketID:  marketID.String(),
		Reason:    reason,
		Timestamp: time.Now().UTC(),
	}
	topic := "settlement.voided"
	if err := p.publish(ctx, topic, []byte(marketID.String()), evt); err != nil {
		p.log.Warn("events: settlement.voided failed",
			zap.String("market_id", marketID.String()),
			zap.Error(err),
		)
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
