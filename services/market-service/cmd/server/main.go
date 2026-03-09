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
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"

	"github.com/predictx/market-service/internal/cache"
	"github.com/predictx/market-service/internal/config"
	"github.com/predictx/market-service/internal/events"
	marketgrpc "github.com/predictx/market-service/internal/api/grpc"
	markethttp "github.com/predictx/market-service/internal/api/http"
	"github.com/predictx/market-service/internal/repository"
	"github.com/predictx/market-service/internal/service"
	marketpb "github.com/predictx/market-service/internal/api/grpc/marketpb"
)

func main() {
	cfg := config.Load()

	log := buildLogger(cfg.LogLevel)
	defer log.Sync() //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Database ──────────────────────────────────────────────────────────────
	pool, err := repository.NewPool(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns, cfg.DatabaseMinConns, log)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err := repository.RunMigrations(cfg.DatabaseURL, "/migrations", log); err != nil {
		log.Fatal("migrations failed", zap.Error(err))
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	redisClient, err := cache.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer redisClient.Close()

	marketCache := cache.NewMarketCache(redisClient, cfg.RedisMarketTTLSecs)

	// ── Kafka publisher ───────────────────────────────────────────────────────
	publisher := events.NewPublisher(cfg.KafkaBrokers, log)
	defer publisher.Close() //nolint:errcheck

	// ── Wire dependencies ─────────────────────────────────────────────────────
	marketRepo := repository.NewMarketRepo(pool)
	marketSvc := service.NewMarketService(
		marketRepo, marketCache, publisher,
		cfg.KafkaTopicMarketCreated, cfg.KafkaTopicMarketVoided,
		log,
	)

	// ── Kafka consumer ────────────────────────────────────────────────────────
	consumer := events.NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaTopicMarketsResolved,
		cfg.KafkaGroupID,
		marketSvc,
		log,
	)
	defer consumer.Close() //nolint:errcheck
	go consumer.Run(ctx)

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcServer := grpc.NewServer()
	marketpb.RegisterMarketServiceServer(grpcServer, marketgrpc.NewMarketGRPCServer(marketSvc, log))

	grpcAddr := fmt.Sprintf(":%s", cfg.GRPCPort)
	grpcLn, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal("failed to listen for gRPC", zap.Error(err))
	}

	go func() {
		log.Info("gRPC server listening", zap.String("addr", grpcAddr))
		if err := grpcServer.Serve(grpcLn); err != nil {
			log.Error("gRPC server error", zap.Error(err))
		}
	}()

	// ── HTTP server ───────────────────────────────────────────────────────────
	handler := markethttp.NewRouter(markethttp.NewMarketHandler(marketSvc, log), log)
	httpAddr := fmt.Sprintf(":%s", cfg.Port)
	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("HTTP server listening", zap.String("addr", httpAddr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutdown signal received")

	cancel() // stop Kafka consumer

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	grpcServer.GracefulStop()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP shutdown error", zap.Error(err))
	}

	log.Info("market service stopped")
}

func buildLogger(level string) *zap.Logger {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	log, err := cfg.Build()
	if err != nil {
		panic("failed to build logger: " + err.Error())
	}
	return log
}
