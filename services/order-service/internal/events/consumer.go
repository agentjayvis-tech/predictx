package events

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/predictx/order-service/internal/service"
)

// Consumer reads order-related events from Kafka topics.
type Consumer struct {
	readers map[string]*kafka.Reader
	svc     *service.OrderService
	log     *zap.Logger
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(brokers, voidedTopic, voidedGroupID string, svc *service.OrderService, log *zap.Logger) *Consumer {
	brokerList := strings.Split(brokers, ",")
	return &Consumer{
		readers: map[string]*kafka.Reader{
			"market_voided": kafka.NewReader(kafka.ReaderConfig{
				Brokers:  brokerList,
				Topic:    voidedTopic,
				GroupID:  voidedGroupID,
				MinBytes: 1,
				MaxBytes: 1e6,
			}),
		},
		svc: svc,
		log: log,
	}
}

// RunMarketVoidedConsumer starts the market.voided event consumer loop.
func (c *Consumer) RunMarketVoidedConsumer(ctx context.Context) {
	reader := c.readers["market_voided"]
	defer reader.Close()

	c.log.Info("market.voided consumer started")

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.log.Info("market.voided consumer shutting down")
				return
			}
			c.log.Error("failed to read message", zap.Error(err))
			continue
		}

		var payload struct {
			Event    string `json:"event"`
			MarketID string `json:"market_id"`
			Reason   string `json:"reason"`
		}

		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			c.log.Warn("failed to unmarshal market.voided event", zap.Error(err))
			continue
		}

		marketID, err := uuid.Parse(payload.MarketID)
		if err != nil {
			c.log.Warn("invalid market_id in event", zap.String("raw", payload.MarketID), zap.Error(err))
			continue
		}

		c.handleMarketVoided(ctx, marketID)
	}
}

// handleMarketVoided cancels all pending orders on a voided market.
func (c *Consumer) handleMarketVoided(ctx context.Context, marketID uuid.UUID) {
	orders, err := c.svc.ListPendingOrders(ctx, marketID)
	if err != nil {
		c.log.Error("failed to list pending orders",
			zap.String("market_id", marketID.String()),
			zap.Error(err))
		return
	}

	if len(orders) == 0 {
		c.log.Info("no pending orders to cancel",
			zap.String("market_id", marketID.String()))
		return
	}

	c.log.Info("cancelling pending orders for voided market",
		zap.String("market_id", marketID.String()),
		zap.Int("count", len(orders)))

	for _, order := range orders {
		idempotencyKey := "market_voided_" + marketID.String() + "_" + order.ID.String()
		if _, err := c.svc.CancelOrder(ctx, order.ID, order.UserID, idempotencyKey); err != nil {
			c.log.Error("failed to cancel order",
				zap.String("order_id", order.ID.String()),
				zap.String("market_id", marketID.String()),
				zap.Error(err))
		}
	}
}

// Close closes all readers.
func (c *Consumer) Close() error {
	for topic, reader := range c.readers {
		if err := reader.Close(); err != nil {
			c.log.Error("failed to close reader", zap.String("topic", topic), zap.Error(err))
		}
	}
	return nil
}
