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
