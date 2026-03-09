package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/predictx/wallet-service/internal/domain"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Publisher publishes wallet events to Kafka.
type Publisher struct {
	writer *kafka.Writer
	log    *zap.Logger
	mu     sync.Mutex
}

// PaymentCompletedEvent is the payload for the payments.completed topic.
type PaymentCompletedEvent struct {
	Event          string    `json:"event"`
	TransactionID  string    `json:"transaction_id"`
	UserID         string    `json:"user_id"`
	Currency       string    `json:"currency"`
	AmountMinor    int64     `json:"amount_minor"`
	TxnType        string    `json:"txn_type"`
	BalanceAfterMinor int64  `json:"balance_after_minor"`
	Timestamp      time.Time `json:"timestamp"`
}

// NewPublisher creates a Kafka publisher for the given broker addresses and topic.
func NewPublisher(brokers, topic string, log *zap.Logger) *Publisher {
	w := &kafka.Writer{
		Addr:         kafka.TCP(strings.Split(brokers, ",")...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false, // synchronous for financial event durability
	}
	return &Publisher{writer: w, log: log}
}

// PublishPaymentCompleted emits a payment.completed event for deposits and payouts.
func (p *Publisher) PublishPaymentCompleted(ctx context.Context, txn *domain.Transaction, newBalance int64) {
	payload := PaymentCompletedEvent{
		Event:             "payment.completed",
		TransactionID:     txn.ID.String(),
		UserID:            txn.UserID.String(),
		Currency:          string(txn.Currency),
		AmountMinor:       txn.AmountMinor,
		TxnType:           string(txn.TxnType),
		BalanceAfterMinor: newBalance,
		Timestamp:         time.Now().UTC(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		p.log.Error("events: marshal payment.completed", zap.Error(err))
		return
	}

	msg := kafka.Message{
		Key:   []byte(txn.UserID.String()),
		Value: data,
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		// Non-fatal: log and continue. Event can be replayed from DB audit log.
		p.log.Warn("events: failed to publish payment.completed",
			zap.String("txn_id", txn.ID.String()),
			zap.Error(err),
		)
		return
	}

	p.log.Info("events: published payment.completed",
		zap.String("txn_id", txn.ID.String()),
		zap.String("currency", string(txn.Currency)),
		zap.Int64("amount_minor", txn.AmountMinor),
	)
}

// Close shuts down the Kafka writer gracefully.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("events: close writer: %w", err)
	}
	return nil
}
