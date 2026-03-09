package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/predictx/order-service/internal/api/grpc/orderpb"
	grpcapi "github.com/predictx/order-service/internal/api/grpc"
	httpapi "github.com/predictx/order-service/internal/api/http"
	"github.com/predictx/order-service/internal/cache"
	"github.com/predictx/order-service/internal/config"
	"github.com/predictx/order-service/internal/domain"
	"github.com/predictx/order-service/internal/events"
	"github.com/predictx/order-service/internal/repository"
	"github.com/predictx/order-service/internal/service"
)

func main() {
	cfg := config.Load()
	log := buildLogger(cfg.LogLevel)
	defer log.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Info("order-service starting",
		zap.String("port", cfg.Port),
		zap.String("grpc_port", cfg.GRPCPort),
	)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 1. Database setup
	// ═══════════════════════════════════════════════════════════════════════════════
	pool, err := repository.NewPool(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns, cfg.DatabaseMinConns)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err := repository.RunMigrations(cfg.DatabaseURL, "./migrations", log); err != nil {
		log.Fatal("migrations failed", zap.Error(err))
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// 2. Redis setup
	// ═══════════════════════════════════════════════════════════════════════════════
	redisClient, err := cache.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer redisClient.Close()

	orderCache := cache.NewOrderCache(redisClient, cfg.RedisOrderTTLSecs, cfg.RedisRGLimitTTLSecs, log)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 3. Kafka setup
	// ═══════════════════════════════════════════════════════════════════════════════
	publisher := events.NewPublisher(cfg.KafkaBrokers, log)
	defer publisher.Close()

	consumer := events.NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaTopicMarketVoided,
		cfg.KafkaConsumerGroupVoided,
		nil, // Will be set later after OrderService is created
		log,
	)
	defer consumer.Close()

	// ═══════════════════════════════════════════════════════════════════════════════
	// 4. gRPC clients to other services
	// ═══════════════════════════════════════════════════════════════════════════════
	grpcTimeout := time.Duration(cfg.GRPCTimeoutSecs) * time.Second

	walletConn, err := grpc.Dial(cfg.WalletServiceAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatal("failed to connect to wallet service", zap.Error(err))
	}
	defer walletConn.Close()

	marketConn, err := grpc.Dial(cfg.MarketServiceAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatal("failed to connect to market service", zap.Error(err))
	}
	defer marketConn.Close()

	walletClient := newWalletClient(walletConn, grpcTimeout)
	marketClient := newMarketClient(marketConn, grpcTimeout)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 5. Service layer setup
	// ═══════════════════════════════════════════════════════════════════════════════
	orderRepo := repository.NewOrderRepo(pool)
	rateLimit := service.NewRateLimiter(orderCache, cfg.RateLimitMaxOrdersPerMin, 60)
	rgSvc := service.NewRGService(orderRepo, cfg.RGDailyLimitCoins, cfg.RGWeeklyLimitCoins)

	orderSvc := service.NewOrderService(
		orderRepo,
		walletClient,
		marketClient,
		orderCache,
		rateLimit,
		rgSvc,
		&publisherAdapter{publisher: publisher, cfg: cfg},
		log,
	)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 6. gRPC server setup
	// ═══════════════════════════════════════════════════════════════════════════════
	grpcServer := grpc.NewServer()
	orderpb.RegisterOrderServiceServer(grpcServer, grpcapi.NewOrderGRPCServer(orderSvc, log))

	grpcLn, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		log.Fatal("failed to listen for gRPC", zap.Error(err))
	}

	go func() {
		log.Info("gRPC server listening", zap.String("addr", fmt.Sprintf(":%s", cfg.GRPCPort)))
		if err := grpcServer.Serve(grpcLn); err != nil {
			log.Error("gRPC server error", zap.Error(err))
		}
	}()

	// ═══════════════════════════════════════════════════════════════════════════════
	// 7. HTTP server setup
	// ═══════════════════════════════════════════════════════════════════════════════
	handler := httpapi.NewRouter(httpapi.NewOrderHandler(orderSvc, log), log)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("HTTP server listening", zap.String("addr", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", zap.Error(err))
		}
	}()

	// ═══════════════════════════════════════════════════════════════════════════════
	// 8. Kafka consumer loop (market.voided events)
	// ═══════════════════════════════════════════════════════════════════════════════
	go consumer.RunMarketVoidedConsumer(ctx)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 9. Graceful shutdown
	// ═══════════════════════════════════════════════════════════════════════════════
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	grpcServer.GracefulStop()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP shutdown error", zap.Error(err))
	}

	cancel()
	log.Info("order-service stopped")
}

func buildLogger(level string) *zap.Logger {
	config := zap.NewProductionConfig()
	config.Level = zapLevel(level)
	log, _ := config.Build()
	return log
}

func zapLevel(level string) zap.AtomicLevel {
	switch level {
	case "debug":
		return zap.NewAtomicLevelAt(zap.DebugLevel)
	case "warn":
		return zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		return zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	}
}

// ═════════════════════════════════════════════════════════════════════════════════
// Adapter types to bridge gRPC clients with service interfaces
// ═════════════════════════════════════════════════════════════════════════════════

type walletClientAdapter struct {
	client  interface{} // walletpb.WalletServiceClient
	timeout time.Duration
}

func newWalletClient(conn *grpc.ClientConn, timeout time.Duration) service.WalletServiceClient {
	// Implementation would use walletpb.WalletServiceClient
	// Stubbed for now — requires wallet service proto
	return &walletClientAdapter{client: nil, timeout: timeout}
}

func (a *walletClientAdapter) CheckBalance(ctx context.Context, userID string, currency string, amountMinor int64) (bool, int64, error) {
	// TODO: Implement actual gRPC call to wallet service
	// For now, return dummy implementation
	return true, 1000, nil
}

type marketClientAdapter struct {
	client  interface{} // marketpb.MarketServiceClient
	timeout time.Duration
}

func newMarketClient(conn *grpc.ClientConn, timeout time.Duration) service.MarketServiceClient {
	return &marketClientAdapter{client: nil, timeout: timeout}
}

func (a *marketClientAdapter) GetMarket(ctx context.Context, marketID string) (bool, int, error) {
	// TODO: Implement actual gRPC call to market service
	// For now, return dummy implementation
	return true, 2, nil
}

type publisherAdapter struct {
	publisher *events.Publisher
	cfg       *config.Config
}

func (p *publisherAdapter) PublishOrderPlaced(ctx context.Context, order *domain.Order) error {
	return p.publisher.PublishOrderPlaced(ctx, order, p.cfg.KafkaTopicOrdersPlaced)
}

func (p *publisherAdapter) PublishOrderCancelled(ctx context.Context, order *domain.Order) error {
	return p.publisher.PublishOrderCancelled(ctx, order, p.cfg.KafkaTopicOrdersCancelled)
}

func (p *publisherAdapter) Close() error {
	return p.publisher.Close()
}
