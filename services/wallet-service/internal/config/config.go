package config

import (
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
	RedisBalanceTTLSecs int    `mapstructure:"REDIS_BALANCE_TTL_SECONDS"`

	// Kafka
	KafkaBrokers               string `mapstructure:"KAFKA_BROKERS"`
	KafkaTopicPaymentsCompleted string `mapstructure:"KAFKA_TOPIC_PAYMENTS_COMPLETED"`

	// Fraud detection
	FraudMaxChangesPerMin      int   `mapstructure:"FRAUD_MAX_CHANGES_PER_MIN"`
	FraudLargeCreditThreshold  int64 `mapstructure:"FRAUD_LARGE_CREDIT_THRESHOLD"`
	FraudRapidDrainPct         int   `mapstructure:"FRAUD_RAPID_DRAIN_PCT"`

	// Gamification
	DailyRewardCoins int64 `mapstructure:"DAILY_REWARD_COINS"`
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

		// Defaults
		v.SetDefault("PORT", "8002")
		v.SetDefault("GRPC_PORT", "9002")
		v.SetDefault("LOG_LEVEL", "info")
		v.SetDefault("DATABASE_MAX_CONNS", 20)
		v.SetDefault("DATABASE_MIN_CONNS", 2)
		v.SetDefault("REDIS_BALANCE_TTL_SECONDS", 5)
		v.SetDefault("KAFKA_TOPIC_PAYMENTS_COMPLETED", "payments.completed")
		v.SetDefault("FRAUD_MAX_CHANGES_PER_MIN", 10)
		v.SetDefault("FRAUD_LARGE_CREDIT_THRESHOLD", 100000)
		v.SetDefault("FRAUD_RAPID_DRAIN_PCT", 80)
		v.SetDefault("DAILY_REWARD_COINS", 100)

		// Read from .env file if present (dev convenience)
		v.SetConfigFile(".env")
		v.SetConfigType("env")
		_ = v.ReadInConfig() // ignore error — env vars take precedence

		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		cfg := &Config{}
		if err := v.Unmarshal(cfg); err != nil {
			panic("config: failed to unmarshal: " + err.Error())
		}
		instance = cfg
	})
	return instance
}
