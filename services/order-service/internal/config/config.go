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
	RedisURL            string `mapstructure:"REDIS_URL"`
	RedisOrderTTLSecs   int    `mapstructure:"REDIS_ORDER_TTL_SECONDS"`
	RedisRGLimitTTLSecs int    `mapstructure:"REDIS_RG_LIMIT_TTL_SECONDS"`

	// Kafka
	KafkaBrokers              string `mapstructure:"KAFKA_BROKERS"`
	KafkaTopicOrdersPlaced    string `mapstructure:"KAFKA_TOPIC_ORDERS_PLACED"`
	KafkaTopicOrdersCancelled string `mapstructure:"KAFKA_TOPIC_ORDERS_CANCELLED"`
	KafkaTopicMarketVoided    string `mapstructure:"KAFKA_TOPIC_MARKET_VOIDED"`
	KafkaTopicOrdersMatched   string `mapstructure:"KAFKA_TOPIC_ORDERS_MATCHED"`
	KafkaConsumerGroupVoided  string `mapstructure:"KAFKA_CONSUMER_GROUP_MARKET_VOIDED"`
	KafkaConsumerGroupMatched string `mapstructure:"KAFKA_CONSUMER_GROUP_ORDERS_MATCHED"`

	// gRPC clients
	WalletServiceAddr  string `mapstructure:"WALLET_SERVICE_ADDR"`
	MarketServiceAddr  string `mapstructure:"MARKET_SERVICE_ADDR"`
	GRPCTimeoutSecs    int    `mapstructure:"GRPC_TIMEOUT_SECS"`

	// Rate limiting
	RateLimitMaxOrdersPerMin int `mapstructure:"RATE_LIMIT_MAX_ORDERS_PER_MIN"`

	// Responsible gambling
	RGDailyLimitCoins  int64 `mapstructure:"RG_DAILY_LIMIT_COINS"`
	RGWeeklyLimitCoins int64 `mapstructure:"RG_WEEKLY_LIMIT_COINS"`
}

// Validate checks that required fields are set and values are in acceptable ranges.
func (c *Config) Validate() error {
	// Required fields
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
	if c.MarketServiceAddr == "" {
		return fmt.Errorf("MARKET_SERVICE_ADDR is required")
	}

	// Range validation
	if c.DatabaseMaxConns <= 0 {
		return fmt.Errorf("DATABASE_MAX_CONNS must be positive, got %d", c.DatabaseMaxConns)
	}
	if c.DatabaseMinConns < 0 {
		return fmt.Errorf("DATABASE_MIN_CONNS must be non-negative, got %d", c.DatabaseMinConns)
	}
	if c.DatabaseMinConns > c.DatabaseMaxConns {
		return fmt.Errorf("DATABASE_MIN_CONNS (%d) must be <= DATABASE_MAX_CONNS (%d)",
			c.DatabaseMinConns, c.DatabaseMaxConns)
	}

	if c.RedisOrderTTLSecs <= 0 {
		return fmt.Errorf("REDIS_ORDER_TTL_SECONDS must be positive, got %d", c.RedisOrderTTLSecs)
	}
	if c.RedisRGLimitTTLSecs <= 0 {
		return fmt.Errorf("REDIS_RG_LIMIT_TTL_SECONDS must be positive, got %d", c.RedisRGLimitTTLSecs)
	}

	if c.RateLimitMaxOrdersPerMin <= 0 {
		return fmt.Errorf("RATE_LIMIT_MAX_ORDERS_PER_MIN must be positive, got %d", c.RateLimitMaxOrdersPerMin)
	}

	if c.RGDailyLimitCoins <= 0 {
		return fmt.Errorf("RG_DAILY_LIMIT_COINS must be positive, got %d", c.RGDailyLimitCoins)
	}
	if c.RGWeeklyLimitCoins <= 0 {
		return fmt.Errorf("RG_WEEKLY_LIMIT_COINS must be positive, got %d", c.RGWeeklyLimitCoins)
	}

	return nil
}

var (
	instance *Config
	once     sync.Once
)

// Load returns the singleton Config, reading from environment variables.
// Panics on error so misconfiguration is caught at startup.
func Load() *Config {
	once.Do(func() {
		v := viper.New()

		// Set defaults
		v.SetDefault("PORT", "8003")
		v.SetDefault("GRPC_PORT", "9003")
		v.SetDefault("LOG_LEVEL", "info")
		v.SetDefault("DATABASE_MAX_CONNS", 20)
		v.SetDefault("DATABASE_MIN_CONNS", 2)
		v.SetDefault("REDIS_ORDER_TTL_SECONDS", 60)
		v.SetDefault("REDIS_RG_LIMIT_TTL_SECONDS", 1800)
		v.SetDefault("KAFKA_TOPIC_ORDERS_PLACED", "orders.placed")
		v.SetDefault("KAFKA_TOPIC_ORDERS_CANCELLED", "orders.cancelled")
		v.SetDefault("KAFKA_TOPIC_MARKET_VOIDED", "markets.voided")
		v.SetDefault("KAFKA_TOPIC_ORDERS_MATCHED", "orders.matched")
		v.SetDefault("KAFKA_CONSUMER_GROUP_MARKET_VOIDED", "order-service-market-voided")
		v.SetDefault("KAFKA_CONSUMER_GROUP_ORDERS_MATCHED", "order-service-orders-matched")
		v.SetDefault("GRPC_TIMEOUT_SECS", 10)
		v.SetDefault("RATE_LIMIT_MAX_ORDERS_PER_MIN", 100)
		v.SetDefault("RG_DAILY_LIMIT_COINS", 10000)
		v.SetDefault("RG_WEEKLY_LIMIT_COINS", 50000)

		// Read from .env if present
		v.SetConfigFile(".env")
		v.SetConfigType("env")
		_ = v.ReadInConfig()

		// Auto-bind env vars
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
