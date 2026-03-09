package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// Config holds all service configuration loaded from environment variables.
type Config struct {
	// Server
	Port     string `mapstructure:"PORT"`
	GRPCPort string `mapstructure:"GRPC_PORT"`
	LogLevel string `mapstructure:"LOG_LEVEL"`

	// Database
	DatabaseURL      string `mapstructure:"DATABASE_URL"`
	DatabaseMaxConns int32  `mapstructure:"DATABASE_MAX_CONNS"`
	DatabaseMinConns int32  `mapstructure:"DATABASE_MIN_CONNS"`

	// Redis
	RedisURL              string `mapstructure:"REDIS_URL"`
	RedisPositionTTLSecs  int    `mapstructure:"REDIS_POSITION_TTL_SECONDS"`

	// Kafka
	KafkaBrokers              string `mapstructure:"KAFKA_BROKERS"`
	KafkaTopicOrderMatched    string `mapstructure:"KAFKA_TOPIC_ORDER_MATCHED"`
	KafkaTopicMarketsResolved string `mapstructure:"KAFKA_TOPIC_MARKETS_RESOLVED"`
	KafkaTopicMarketVoided    string `mapstructure:"KAFKA_TOPIC_MARKET_VOIDED"`
	KafkaGroupID              string `mapstructure:"KAFKA_GROUP_ID"`

	// Wallet Service (gRPC)
	WalletServiceAddr string `mapstructure:"WALLET_SERVICE_ADDR"`

	// Settlement config
	PlatformWalletID         string `mapstructure:"PLATFORM_WALLET_ID"`
	DefaultCurrency          string `mapstructure:"DEFAULT_CURRENCY"`
	PayoutWorkers            int    `mapstructure:"PAYOUT_WORKERS"`
	FraudBetWindowSecs       int    `mapstructure:"FRAUD_BET_WINDOW_SECS"`
	FraudConcentrationLimit  int64  `mapstructure:"FRAUD_CONCENTRATION_LIMIT"`
	FraudLargeStakeMinor     int64  `mapstructure:"FRAUD_LARGE_STAKE_MINOR"`
}

var (
	instance *Config
	once     sync.Once
)

// Load returns the singleton Config, reading from environment variables.
// Panics on misconfiguration so startup failures are caught early.
func Load() *Config {
	once.Do(func() {
		v := viper.New()

		v.SetDefault("PORT", "8003")
		v.SetDefault("GRPC_PORT", "9003")
		v.SetDefault("LOG_LEVEL", "info")
		v.SetDefault("DATABASE_MAX_CONNS", 20)
		v.SetDefault("DATABASE_MIN_CONNS", 2)
		v.SetDefault("REDIS_POSITION_TTL_SECONDS", 120)
		v.SetDefault("KAFKA_TOPIC_ORDER_MATCHED", "order.matched")
		v.SetDefault("KAFKA_TOPIC_MARKETS_RESOLVED", "markets.resolved")
		v.SetDefault("KAFKA_TOPIC_MARKET_VOIDED", "market.voided")
		v.SetDefault("KAFKA_GROUP_ID", "settlement-service")
		v.SetDefault("DEFAULT_CURRENCY", "COINS")
		v.SetDefault("PAYOUT_WORKERS", 10)
		v.SetDefault("FRAUD_BET_WINDOW_SECS", 60)
		v.SetDefault("FRAUD_CONCENTRATION_LIMIT", 50)
		v.SetDefault("FRAUD_LARGE_STAKE_MINOR", 1000000)

		v.SetConfigFile(".env")
		v.SetConfigType("env")
		_ = v.ReadInConfig()

		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		cfg := &Config{}
		if err := v.Unmarshal(cfg); err != nil {
			panic("config: failed to unmarshal: " + err.Error())
		}

		if err := cfg.Validate(); err != nil {
			panic("config: validation failed: " + err.Error())
		}

		instance = cfg
	})
	return instance
}

// Validate checks that all required configuration values are present.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if c.KafkaBrokers == "" {
		return fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.WalletServiceAddr == "" {
		return fmt.Errorf("WALLET_SERVICE_ADDR is required")
	}
	if c.PlatformWalletID == "" {
		return fmt.Errorf("PLATFORM_WALLET_ID is required")
	}
	if c.DatabaseMaxConns < 1 || c.DatabaseMaxConns > 500 {
		return fmt.Errorf("DATABASE_MAX_CONNS must be between 1 and 500, got %d", c.DatabaseMaxConns)
	}
	return nil
}
