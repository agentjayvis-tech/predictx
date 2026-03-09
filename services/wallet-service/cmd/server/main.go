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

	"github.com/predictx/wallet-service/internal/cache"
	"github.com/predictx/wallet-service/internal/config"
	"github.com/predictx/wallet-service/internal/events"
	walletgrpc "github.com/predictx/wallet-service/internal/api/grpc"
	wallethttp "github.com/predictx/wallet-service/internal/api/http"
	"github.com/predictx/wallet-service/internal/repository"
	"github.com/predictx/wallet-service/internal/service"
	walletpb "github.com/predictx/wallet-service/internal/api/grpc/walletpb"
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

	balCache := cache.NewBalanceCache(redisClient, cfg.RedisBalanceTTLSecs)

	// ── Kafka ─────────────────────────────────────────────────────────────────
	publisher := events.NewPublisher(cfg.KafkaBrokers, cfg.KafkaTopicPaymentsCompleted, log)
	defer publisher.Close() //nolint:errcheck

	// ── Wire dependencies ─────────────────────────────────────────────────────
	walletRepo := repository.NewWalletRepo(pool)

	fraudSvc := service.NewFraudService(
		walletRepo, balCache, log,
		cfg.FraudMaxChangesPerMin,
		cfg.FraudLargeCreditThreshold,
		cfg.FraudRapidDrainPct,
	)

	walletSvc := service.NewWalletService(walletRepo, balCache, publisher, fraudSvc, log)

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcServer := grpc.NewServer()
	walletpb.RegisterWalletServiceServer(grpcServer, walletgrpc.NewWalletGRPCServer(walletSvc, log))

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
	handler := wallethttp.NewRouter(wallethttp.NewWalletHandler(walletSvc, log), log)
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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	grpcServer.GracefulStop()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP shutdown error", zap.Error(err))
	}

	log.Info("wallet service stopped")
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
