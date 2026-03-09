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

	// Responsible Gambling
	EnableRGFeatures                  bool   `mapstructure:"ENABLE_RG_FEATURES"`
	DefaultDailyDepositLimitMinor     int64  `mapstructure:"DEFAULT_DAILY_DEPOSIT_LIMIT_MINOR"`
	DefaultMonthlyDepositLimitMinor   int64  `mapstructure:"DEFAULT_MONTHLY_DEPOSIT_LIMIT_MINOR"`
	DefaultLossStreakNotificationThreshold int `mapstructure:"DEFAULT_LOSS_STREAK_NOTIFICATION_THRESHOLD"`
	RGPartnershipName                 string `mapstructure:"RG_PARTNERSHIP_NAME"`
	CoolOffCancellationPolicy         string `mapstructure:"COOL_OFF_CANCELLATION_POLICY"` // JSON: {"IN": true, "NG": false, ...}
}

// Validate checks that required fields are set and configuration values are in acceptable ranges.
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

	if c.RedisBalanceTTLSecs <= 0 {
		return fmt.Errorf("REDIS_BALANCE_TTL_SECONDS must be positive, got %d", c.RedisBalanceTTLSecs)
	}

	if c.FraudMaxChangesPerMin <= 0 {
		return fmt.Errorf("FRAUD_MAX_CHANGES_PER_MIN must be positive, got %d", c.FraudMaxChangesPerMin)
	}
	if c.FraudLargeCreditThreshold <= 0 {
		return fmt.Errorf("FRAUD_LARGE_CREDIT_THRESHOLD must be positive, got %d", c.FraudLargeCreditThreshold)
	}
	if c.FraudRapidDrainPct <= 0 || c.FraudRapidDrainPct > 100 {
		return fmt.Errorf("FRAUD_RAPID_DRAIN_PCT must be between 1 and 100, got %d", c.FraudRapidDrainPct)
	}

	if c.DailyRewardCoins <= 0 {
		return fmt.Errorf("DAILY_REWARD_COINS must be positive, got %d", c.DailyRewardCoins)
	}

	// Responsible Gambling validation
	if c.EnableRGFeatures {
		if c.DefaultDailyDepositLimitMinor <= 0 {
			return fmt.Errorf("DEFAULT_DAILY_DEPOSIT_LIMIT_MINOR must be positive when RG enabled, got %d", c.DefaultDailyDepositLimitMinor)
		}
		if c.DefaultMonthlyDepositLimitMinor > 0 && c.DefaultMonthlyDepositLimitMinor < c.DefaultDailyDepositLimitMinor {
			return fmt.Errorf("DEFAULT_MONTHLY_DEPOSIT_LIMIT_MINOR must be >= DEFAULT_DAILY_DEPOSIT_LIMIT_MINOR")
		}
		if c.DefaultLossStreakNotificationThreshold < 1 || c.DefaultLossStreakNotificationThreshold > 10 {
			return fmt.Errorf("DEFAULT_LOSS_STREAK_NOTIFICATION_THRESHOLD must be between 1 and 10, got %d", c.DefaultLossStreakNotificationThreshold)
		}
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

		// Responsible Gambling defaults
		v.SetDefault("ENABLE_RG_FEATURES", true)
		v.SetDefault("DEFAULT_DAILY_DEPOSIT_LIMIT_MINOR", 5000000)   // $50 USD equivalent
		v.SetDefault("DEFAULT_MONTHLY_DEPOSIT_LIMIT_MINOR", 150000000) // $1500 USD equivalent
		v.SetDefault("DEFAULT_LOSS_STREAK_NOTIFICATION_THRESHOLD", 3)
		v.SetDefault("RG_PARTNERSHIP_NAME", "Gambler's Anonymous")
		v.SetDefault("COOL_OFF_CANCELLATION_POLICY", `{"IN": true, "NG": false, "KE": false, "PH": false}`)

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
		if err := cfg.Validate(); err != nil {
			panic("config: validation failed: " + err.Error())
		}
		instance = cfg
	})
	return instance
}
