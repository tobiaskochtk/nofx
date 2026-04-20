package api

import (
	"nofx/store"
	"testing"
)

func TestValidateStrategyConfigRejectsUnknownSignalProvider(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.SignalProvider.Type = "mystery"

	if _, err := validateStrategyConfig(&cfg); err == nil {
		t.Fatal("expected invalid signal provider type to be rejected")
	}
}

func TestValidateStrategyConfigRejectsSelfhostedWithoutBaseURL(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.SignalProvider.Type = store.SignalProviderSelfhostedAI500
	cfg.SignalProvider.BaseURL = ""

	if _, err := validateStrategyConfig(&cfg); err == nil {
		t.Fatal("expected missing selfhosted base URL to be rejected")
	}
}

func TestValidateStrategyConfigAllowsCanonicalNofxOSProvider(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.SignalProvider.Type = store.SignalProviderNofxOS

	if _, err := validateStrategyConfig(&cfg); err != nil {
		t.Fatalf("expected canonical nofxos provider to be valid, got: %v", err)
	}
}

func TestValidateStrategyConfigRejectsInvalidTrailingStopMode(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.RiskControl.TrailingStop.Enabled = true
	cfg.RiskControl.TrailingStop.Tiers = []store.TrailingStopTier{
		{
			TriggerProfitPct: 1.0,
			Mode:             "mystery_mode",
			LockProfitPct:    0.5,
		},
	}

	if _, err := validateStrategyConfig(&cfg); err == nil {
		t.Fatal("expected invalid trailing-stop mode to be rejected")
	}
}

func TestValidateStrategyConfigRejectsTrailingStopLockAboveTrigger(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.RiskControl.TrailingStop.Enabled = true
	cfg.RiskControl.TrailingStop.Tiers = []store.TrailingStopTier{
		{
			TriggerProfitPct: 1.0,
			Mode:             store.TrailingStopModeLockProfit,
			LockProfitPct:    1.5,
		},
	}

	if _, err := validateStrategyConfig(&cfg); err == nil {
		t.Fatal("expected trailing-stop lock above trigger to be rejected")
	}
}
