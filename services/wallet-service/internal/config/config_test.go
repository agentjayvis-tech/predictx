package config

import (
	"os"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	// Clear env vars to test defaults
	os.Setenv("PORT", "")
	os.Setenv("GRPC_PORT", "")

	// Reset singleton for test
	instance = nil
	cfg := Load()

	if cfg.Port != "8002" {
		t.Errorf("expected default PORT 8002, got %s", cfg.Port)
	}
	if cfg.GRPCPort != "9002" {
		t.Errorf("expected default GRPC_PORT 9002, got %s", cfg.GRPCPort)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default LOG_LEVEL info, got %s", cfg.LogLevel)
	}
	if cfg.FraudMaxChangesPerMin != 10 {
		t.Errorf("expected default FRAUD_MAX_CHANGES_PER_MIN 10, got %d", cfg.FraudMaxChangesPerMin)
	}
	if cfg.FraudLargeCreditThreshold != 100000 {
		t.Errorf("expected default FRAUD_LARGE_CREDIT_THRESHOLD 100000, got %d", cfg.FraudLargeCreditThreshold)
	}
	if cfg.DailyRewardCoins != 100 {
		t.Errorf("expected default DAILY_REWARD_COINS 100, got %d", cfg.DailyRewardCoins)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	os.Setenv("PORT", "9999")
	os.Setenv("GRPC_PORT", "7777")
	os.Setenv("LOG_LEVEL", "debug")

	instance = nil
	cfg := Load()

	if cfg.Port != "9999" {
		t.Errorf("expected PORT 9999, got %s", cfg.Port)
	}
	if cfg.GRPCPort != "7777" {
		t.Errorf("expected GRPC_PORT 7777, got %s", cfg.GRPCPort)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LOG_LEVEL debug, got %s", cfg.LogLevel)
	}
}

func TestLoadConfigSingleton(t *testing.T) {
	instance = nil
	cfg1 := Load()
	cfg2 := Load()

	if cfg1 != cfg2 {
		t.Errorf("expected Load() to return singleton")
	}
}
