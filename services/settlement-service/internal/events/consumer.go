package events

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// ─── Inbound event payloads ───────────────────────────────────────────────────

// OrderMatchedPayload is emitted by the Order Service / Matching Engine
// when an order is successfully matched. Settlement Service uses it to track positions.
type OrderMatchedPayload struct {
	Event          string    `json:"event"`
	OrderID        string    `json:"order_id"`
	UserID         string    `json:"user_id"`
	MarketID       string    `json:"market_id"`
	OutcomeIndex   int       `json:"outcome_index"`
	StakeMinor     int64     `json:"stake_minor"`
	Currency       string    `json:"currency"`
	Timestamp      time.Time `json:"timestamp"`
}

// MarketResolvedPayload is emitted by the Resolution Service.
type MarketResolvedPayload struct {
	Event         string `json:"event"`
	MarketID      string `json:"market_id"`
	ResolutionID  string `json:"resolution_id"`
	Outcome       string `json:"outcome"`       // "YES" | "NO" | "VOID" or numeric index string
	WinningIndex  int    `json:"winning_index"` // canonical winning outcome index
	Source        string `json:"source"`
	Timestamp     string `json:"timestamp"`
}

// MarketVoidedPayload is emitted by the Market Service.
type MarketVoidedPayload struct {
	Event     string `json:"event"`
	MarketID  string `json:"market_id"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp"`
}

// ─── Service interface ────────────────────────────────────────────────────────

// SettlementHandler is the interface the consumer delegates to.
// RecordPosition returns error only — the consumer does not use the position object.
type SettlementHandler interface {
	RecordPositionFromEvent(ctx context.Context, userID, marketID uuid.UUID, outcomeIndex int, stakeMinor int64, currency string) error
	SettleMarket(ctx context.Context, marketID uuid.UUID, winningOutcome int, resolutionID string) error
	RefundMarket(ctx context.Context, marketID uuid.UUID, reason string) error
}

// ─── Consumer ─────────────────────────────────────────────────────────────────

// Consumer subscribes to three Kafka topics and delegates to the service layer.
type Consumer struct {
	orderMatchedReader   *kafka.Reader
	marketsResolvedReader *kafka.Reader
	marketVoidedReader   *kafka.Reader
	svc                  SettlementHandler
	log                  *zap.Logger
}

type ConsumerConfig struct {
	Brokers               string
	TopicOrderMatched     string
	TopicMarketsResolved  string
	TopicMarketVoided     string
	GroupID               string
}

func NewConsumer(cfg ConsumerConfig, svc SettlementHandler, log *zap.Logger) *Consumer {
	brokers := strings.Split(cfg.Brokers, ",")

	newReader := func(topic string) *kafka.Reader {
		return kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    topic,
			GroupID:  cfg.GroupID,
			MinBytes: 1,
			MaxBytes: 1e6,
		})
	}

	return &Consumer{
		orderMatchedReader:    newReader(cfg.TopicOrderMatched),
		marketsResolvedReader: newReader(cfg.TopicMarketsResolved),
		marketVoidedReader:    newReader(cfg.TopicMarketVoided),
		svc:                   svc,
		log:                   log,
	}
}

// Run starts all three consume loops in separate goroutines.
// Blocking until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) {
	go c.consumeOrderMatched(ctx)
	go c.consumeMarketsResolved(ctx)
	go c.consumeMarketVoided(ctx)
	<-ctx.Done()
}

func (c *Consumer) consumeOrderMatched(ctx context.Context) {
	c.log.Info("kafka consumer started", zap.String("topic", c.orderMatchedReader.Config().Topic))
	for {
		msg, err := c.orderMatchedReader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error("order.matched: read error", zap.Error(err))
			continue
		}

		var payload OrderMatchedPayload
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			c.log.Warn("order.matched: unmarshal error", zap.Error(err))
			continue
		}

		userID, err := uuid.Parse(payload.UserID)
		if err != nil {
			c.log.Warn("order.matched: invalid user_id", zap.String("raw", payload.UserID))
			continue
		}
		marketID, err := uuid.Parse(payload.MarketID)
		if err != nil {
			c.log.Warn("order.matched: invalid market_id", zap.String("raw", payload.MarketID))
			continue
		}

		currency := payload.Currency
		if currency == "" {
			currency = "COINS"
		}

		if err := c.svc.RecordPositionFromEvent(ctx, userID, marketID, payload.OutcomeIndex, payload.StakeMinor, currency); err != nil {
			c.log.Error("order.matched: record position failed",
				zap.String("order_id", payload.OrderID),
				zap.Error(err),
			)
		} else {
			c.log.Info("position recorded",
				zap.String("user_id", payload.UserID),
				zap.String("market_id", payload.MarketID),
				zap.Int("outcome", payload.OutcomeIndex),
				zap.Int64("stake", payload.StakeMinor),
			)
		}
	}
}

func (c *Consumer) consumeMarketsResolved(ctx context.Context) {
	c.log.Info("kafka consumer started", zap.String("topic", c.marketsResolvedReader.Config().Topic))
	for {
		msg, err := c.marketsResolvedReader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error("markets.resolved: read error", zap.Error(err))
			continue
		}

		var payload MarketResolvedPayload
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			c.log.Warn("markets.resolved: unmarshal error", zap.Error(err))
			continue
		}

		marketID, err := uuid.Parse(payload.MarketID)
		if err != nil {
			c.log.Warn("markets.resolved: invalid market_id", zap.String("raw", payload.MarketID))
			continue
		}

		if payload.Outcome == "VOID" {
			if err := c.svc.RefundMarket(ctx, marketID, "resolution_void"); err != nil {
				c.log.Error("markets.resolved: refund failed",
					zap.String("market_id", marketID.String()), zap.Error(err))
			}
			continue
		}

		if err := c.svc.SettleMarket(ctx, marketID, payload.WinningIndex, payload.ResolutionID); err != nil {
			c.log.Error("markets.resolved: settle failed",
				zap.String("market_id", marketID.String()),
				zap.Int("winning_index", payload.WinningIndex),
				zap.Error(err),
			)
		} else {
			c.log.Info("market settled",
				zap.String("market_id", marketID.String()),
				zap.Int("winning_index", payload.WinningIndex),
			)
		}
	}
}

func (c *Consumer) consumeMarketVoided(ctx context.Context) {
	c.log.Info("kafka consumer started", zap.String("topic", c.marketVoidedReader.Config().Topic))
	for {
		msg, err := c.marketVoidedReader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error("market.voided: read error", zap.Error(err))
			continue
		}

		var payload MarketVoidedPayload
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			c.log.Warn("market.voided: unmarshal error", zap.Error(err))
			continue
		}

		marketID, err := uuid.Parse(payload.MarketID)
		if err != nil {
			c.log.Warn("market.voided: invalid market_id", zap.String("raw", payload.MarketID))
			continue
		}

		reason := payload.Reason
		if reason == "" {
			reason = "market_voided"
		}

		if err := c.svc.RefundMarket(ctx, marketID, reason); err != nil {
			c.log.Error("market.voided: refund failed",
				zap.String("market_id", marketID.String()), zap.Error(err))
		} else {
			c.log.Info("market voided and refunded",
				zap.String("market_id", marketID.String()),
				zap.String("reason", reason),
			)
		}
	}
}

// Close shuts down all Kafka readers.
func (c *Consumer) Close() error {
	c.orderMatchedReader.Close()   //nolint:errcheck
	c.marketsResolvedReader.Close() //nolint:errcheck
	c.marketVoidedReader.Close()   //nolint:errcheck
	return nil
}
