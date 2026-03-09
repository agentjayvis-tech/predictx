package events

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// MarketResolvedPayload is the event published by Resolution Service on markets.resolved.
type MarketResolvedPayload struct {
	Event        string `json:"event"`
	MarketID     string `json:"market_id"`
	ResolutionID string `json:"resolution_id"`
	Outcome      string `json:"outcome"` // "YES" | "NO" | "VOID"
	Source       string `json:"source"`
	Timestamp    string `json:"timestamp"`
}

// MarketResolver is the interface the consumer needs from the service layer.
type MarketResolver interface {
	MarkResolved(ctx context.Context, marketID uuid.UUID) error
	VoidMarket(ctx context.Context, marketID uuid.UUID, reason string) error
}

// Consumer subscribes to markets.resolved and updates market statuses.
type Consumer struct {
	reader  *kafka.Reader
	svc     MarketResolver
	log     *zap.Logger
}

// NewConsumer creates a new Kafka consumer for the markets.resolved topic.
func NewConsumer(brokers, topic, groupID string, svc MarketResolver, log *zap.Logger) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  strings.Split(brokers, ","),
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 1e6,
	})
	return &Consumer{reader: r, svc: svc, log: log}
}

// Run starts the consume loop. Blocking — call in a goroutine.
// Exits when ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) {
	c.log.Info("kafka consumer started", zap.String("topic", c.reader.Config().Topic))
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.log.Info("kafka consumer stopping")
				return
			}
			c.log.Error("kafka: read message error", zap.Error(err))
			continue
		}

		var payload MarketResolvedPayload
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			c.log.Warn("kafka: unmarshal markets.resolved", zap.Error(err))
			continue
		}

		marketID, err := uuid.Parse(payload.MarketID)
		if err != nil {
			c.log.Warn("kafka: invalid market_id", zap.String("raw", payload.MarketID))
			continue
		}

		if payload.Outcome == "VOID" {
			if err := c.svc.VoidMarket(ctx, marketID, "resolution_void"); err != nil {
				c.log.Error("consumer: void market failed",
					zap.String("market_id", marketID.String()), zap.Error(err))
			} else {
				c.log.Info("consumer: market voided via resolution",
					zap.String("market_id", marketID.String()))
			}
		} else {
			if err := c.svc.MarkResolved(ctx, marketID); err != nil {
				c.log.Error("consumer: mark resolved failed",
					zap.String("market_id", marketID.String()), zap.Error(err))
			} else {
				c.log.Info("consumer: market resolved",
					zap.String("market_id", marketID.String()),
					zap.String("outcome", payload.Outcome))
			}
		}
	}
}

// Close shuts down the Kafka reader.
func (c *Consumer) Close() error {
	return c.reader.Close()
}
