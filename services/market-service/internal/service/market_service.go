package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/predictx/market-service/internal/cache"
	"github.com/predictx/market-service/internal/domain"
	"github.com/predictx/market-service/internal/events"
	"github.com/predictx/market-service/internal/repository"
	"go.uber.org/zap"
)

// MarketService implements the core market lifecycle business logic.
type MarketService struct {
	repo      *repository.MarketRepo
	cache     *cache.MarketCache
	publisher *events.Publisher
	cfg       serviceConfig
	log       *zap.Logger
}

type serviceConfig struct {
	topicCreated string
	topicVoided  string
}

func NewMarketService(
	repo *repository.MarketRepo,
	cache *cache.MarketCache,
	publisher *events.Publisher,
	topicCreated, topicVoided string,
	log *zap.Logger,
) *MarketService {
	return &MarketService{
		repo:      repo,
		cache:     cache,
		publisher: publisher,
		cfg:       serviceConfig{topicCreated: topicCreated, topicVoided: topicVoided},
		log:       log,
	}
}

// CreateMarket creates a new market.
// Admin markets are immediately activated (draft → active) and emit market.created.
// User-proposed markets start in draft status.
func (s *MarketService) CreateMarket(ctx context.Context, req domain.CreateMarketRequest) (*domain.Market, error) {
	if !domain.SupportedCategories[req.Category] {
		return nil, domain.ErrInvalidCategory
	}
	if req.ClosesAt.Before(time.Now()) {
		return nil, domain.ErrClosesAtInPast
	}

	status := domain.StatusDraft
	if req.CreatorType == domain.CreatorAdmin {
		status = domain.StatusActive
	}

	m := &domain.Market{
		ID:                 uuid.New(),
		Title:              req.Title,
		Question:           req.Question,
		ResolutionCriteria: req.ResolutionCriteria,
		Category:           req.Category,
		Status:             status,
		CreatorID:          req.CreatorID,
		CreatorType:        req.CreatorType,
		PoolAmountMinor:    0,
		Currency:           "COINS",
		ClosesAt:           req.ClosesAt,
		ResolvesAt:         req.ResolvesAt,
		Metadata:           req.Metadata,
	}

	created, err := s.repo.CreateMarket(ctx, m)
	if err != nil {
		return nil, err
	}

	if created.Status == domain.StatusActive {
		go s.publisher.PublishMarketCreated(ctx, s.cfg.topicCreated,
			created.ID, string(created.Category), created.ClosesAt.Format(time.RFC3339))
	}

	return created, nil
}

// GetMarket returns a market by ID, using the cache for reads.
func (s *MarketService) GetMarket(ctx context.Context, marketID uuid.UUID) (*domain.Market, error) {
	if raw, ok := s.cache.Get(ctx, marketID); ok {
		var m domain.Market
		if err := json.Unmarshal(raw, &m); err == nil {
			return &m, nil
		}
	}

	m, err := s.repo.GetMarket(ctx, marketID)
	if err != nil {
		return nil, err
	}

	if raw, err := json.Marshal(m); err == nil {
		s.cache.Set(ctx, marketID, raw)
	}
	return m, nil
}

// ListMarkets returns markets with optional filters.
func (s *MarketService) ListMarkets(ctx context.Context, f domain.ListFilters) ([]*domain.Market, error) {
	return s.repo.ListMarkets(ctx, f)
}

// ListResolvable returns markets eligible for resolution (active + past closes_at).
func (s *MarketService) ListResolvable(ctx context.Context) ([]*domain.Market, error) {
	return s.repo.ListResolvable(ctx)
}

// UpdateStatus performs a validated status transition.
func (s *MarketService) UpdateStatus(ctx context.Context, marketID uuid.UUID, newStatus domain.MarketStatus) error {
	m, err := s.repo.GetMarket(ctx, marketID)
	if err != nil {
		return err
	}

	if !domain.IsValidTransition(m.Status, newStatus) {
		return domain.ErrInvalidTransition
	}

	if err := s.repo.UpdateStatus(ctx, marketID, newStatus, m.Status); err != nil {
		return err
	}

	s.cache.Invalidate(ctx, marketID)

	if newStatus == domain.StatusActive {
		updated, _ := s.repo.GetMarket(ctx, marketID)
		if updated != nil {
			go s.publisher.PublishMarketCreated(ctx, s.cfg.topicCreated,
				marketID, string(updated.Category), updated.ClosesAt.Format(time.RFC3339))
		}
	}

	if newStatus == domain.StatusVoided {
		go s.publisher.PublishMarketVoided(ctx, s.cfg.topicVoided, marketID, "admin_action")
	}

	return nil
}

// MarkResolved transitions a market to resolved (called by Kafka consumer on markets.resolved).
func (s *MarketService) MarkResolved(ctx context.Context, marketID uuid.UUID) error {
	m, err := s.repo.GetMarket(ctx, marketID)
	if err != nil {
		return err
	}

	// Idempotent: already resolved/voided
	if m.Status == domain.StatusResolved || m.Status == domain.StatusVoided {
		return nil
	}

	if !domain.IsValidTransition(m.Status, domain.StatusResolved) {
		return domain.ErrInvalidTransition
	}

	if err := s.repo.UpdateStatus(ctx, marketID, domain.StatusResolved, m.Status); err != nil {
		return err
	}

	s.cache.Invalidate(ctx, marketID)
	return nil
}

// VoidMarket cancels a market regardless of current status (called by Kafka consumer or admin).
func (s *MarketService) VoidMarket(ctx context.Context, marketID uuid.UUID, reason string) error {
	m, err := s.repo.GetMarket(ctx, marketID)
	if err != nil {
		return err
	}

	// Idempotent
	if m.Status == domain.StatusVoided {
		return nil
	}

	if !domain.IsValidTransition(m.Status, domain.StatusVoided) {
		return domain.ErrInvalidTransition
	}

	if err := s.repo.UpdateStatus(ctx, marketID, domain.StatusVoided, m.Status); err != nil {
		return err
	}

	s.cache.Invalidate(ctx, marketID)
	go s.publisher.PublishMarketVoided(ctx, s.cfg.topicVoided, marketID, reason)
	return nil
}
