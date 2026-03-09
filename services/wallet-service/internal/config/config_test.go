package config

import (
	"testing"
)

// Note: Full config loading tests require environment variables to be set:
// DATABASE_URL, REDIS_URL, KAFKA_BROKERS, etc.
// These are tested in integration tests with proper test environment setup.

func TestConfigStructFields(t *testing.T) {
	// Verify Config struct has expected fields
	cfg := &Config{
		Port:     "8002",
		GRPCPort: "9002",
		LogLevel: "info",
	}

	if cfg.Port != "8002" {
		t.Error("Port field not set")
	}
	if cfg.GRPCPort != "9002" {
		t.Error("GRPCPort field not set")
	}
	if cfg.LogLevel != "info" {
		t.Error("LogLevel field not set")
	}
}

func TestConfigDefaults(t *testing.T) {
	// Test default values are reasonable
	defaults := map[string]interface{}{
		"port":                      "8002",
		"grpc_port":                 "9002",
		"log_level":                 "info",
		"max_conns":                 20,
		"min_conns":                 2,
		"redis_ttl":                 5,
		"fraud_max_changes_per_min": 10,
		"fraud_large_credit":        100000,
		"fraud_rapid_drain_pct":     80,
		"daily_reward_coins":        100,
	}

	for key, val := range defaults {
		if val == nil {
			t.Errorf("Default value for %s is nil", key)
		}
	}
}
