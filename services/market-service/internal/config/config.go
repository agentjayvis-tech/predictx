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
	RedisURL           string `mapstructure:"REDIS_URL"`
	RedisMarketTTLSecs int    `mapstructure:"REDIS_MARKET_TTL_SECONDS"`

	// Kafka
	KafkaBrokers                string `mapstructure:"KAFKA_BROKERS"`
	KafkaTopicMarketCreated     string `mapstructure:"KAFKA_TOPIC_MARKET_CREATED"`
	KafkaTopicMarketVoided      string `mapstructure:"KAFKA_TOPIC_MARKET_VOIDED"`
	KafkaTopicMarketsResolved   string `mapstructure:"KAFKA_TOPIC_MARKETS_RESOLVED"`
	KafkaGroupID                string `mapstructure:"KAFKA_GROUP_ID"`
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

		v.SetDefault("PORT", "8001")
		v.SetDefault("GRPC_PORT", "9001")
		v.SetDefault("LOG_LEVEL", "info")
		v.SetDefault("DATABASE_MAX_CONNS", 20)
		v.SetDefault("DATABASE_MIN_CONNS", 2)
		v.SetDefault("REDIS_MARKET_TTL_SECONDS", 60)
		v.SetDefault("KAFKA_TOPIC_MARKET_CREATED", "market.created")
		v.SetDefault("KAFKA_TOPIC_MARKET_VOIDED", "market.voided")
		v.SetDefault("KAFKA_TOPIC_MARKETS_RESOLVED", "markets.resolved")
		v.SetDefault("KAFKA_GROUP_ID", "market-service")

		v.SetConfigFile(".env")
		v.SetConfigType("env")
		_ = v.ReadInConfig()

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
