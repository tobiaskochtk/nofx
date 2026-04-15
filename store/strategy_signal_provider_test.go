package store

import (
	"nofx/provider/nofxos"
	"os"
	"testing"
)

func TestResolveSignalProvider_DefaultConfig(t *testing.T) {
	config := GetDefaultStrategyConfig("en")

	provider := config.ResolveSignalProvider()
	if provider.Type != SignalProviderNofxOS {
		t.Fatalf("provider type = %q, want %q", provider.Type, SignalProviderNofxOS)
	}
	if provider.BaseURL != nofxos.DefaultBaseURL {
		t.Fatalf("provider base URL = %q, want %q", provider.BaseURL, nofxos.DefaultBaseURL)
	}
	if provider.APIKey != nofxos.DefaultAuthKey {
		t.Fatalf("provider API key = %q, want %q", provider.APIKey, nofxos.DefaultAuthKey)
	}
}

func TestResolveSignalProvider_LegacyAPIKeyFallback(t *testing.T) {
	config := StrategyConfig{
		Indicators: IndicatorConfig{
			NofxOSAPIKey: "legacy-key",
		},
	}

	provider := config.ResolveSignalProvider()
	if provider.Type != SignalProviderNofxOS {
		t.Fatalf("provider type = %q, want %q", provider.Type, SignalProviderNofxOS)
	}
	if provider.APIKey != "legacy-key" {
		t.Fatalf("provider API key = %q, want legacy fallback", provider.APIKey)
	}
}

func TestResolveSignalProvider_LegacyOfficialAlias(t *testing.T) {
	config := StrategyConfig{
		SignalProvider: SignalProviderConfig{
			Type: SignalProviderOfficialNofxOS,
		},
	}

	provider := config.ResolveSignalProvider()
	if provider.Type != SignalProviderNofxOS {
		t.Fatalf("provider type = %q, want %q", provider.Type, SignalProviderNofxOS)
	}
}

func TestResolveSignalProvider_SelfhostedConfig(t *testing.T) {
	config := StrategyConfig{
		SignalProvider: SignalProviderConfig{
			Type:    SignalProviderSelfhostedAI500,
			BaseURL: "http://selfhosted-ai500:8080",
			APIKey:  "selfhosted-token",
		},
	}

	provider := config.ResolveSignalProvider()
	if provider.Type != SignalProviderSelfhostedAI500 {
		t.Fatalf("provider type = %q, want %q", provider.Type, SignalProviderSelfhostedAI500)
	}
	if provider.BaseURL != "http://selfhosted-ai500:8080" {
		t.Fatalf("provider base URL = %q, want selfhosted URL", provider.BaseURL)
	}
	if provider.APIKey != "selfhosted-token" {
		t.Fatalf("provider API key = %q, want selfhosted token", provider.APIKey)
	}
}

func TestResolveSignalProvider_SelfhostedDefaultBaseURL(t *testing.T) {
	t.Setenv("SELFHOSTED_AI500_INTERNAL_URL", "http://selfhosted-ai500:9090")
	t.Setenv("SELFHOSTED_AI500_BASE_URL", "")

	config := StrategyConfig{
		SignalProvider: SignalProviderConfig{
			Type:   SignalProviderSelfhostedAI500,
			APIKey: "selfhosted-token",
		},
	}

	provider := config.ResolveSignalProvider()
	if provider.BaseURL != "http://selfhosted-ai500:9090" {
		t.Fatalf("provider base URL = %q, want env default", provider.BaseURL)
	}
}

func TestResolveSignalProvider_SelfhostedUsesEnvTokenForLegacyDefaultKey(t *testing.T) {
	t.Setenv("SELFHOSTED_AI500_AUTH_TOKEN", "local-selfhosted-token")

	config := StrategyConfig{
		SignalProvider: SignalProviderConfig{
			Type:   SignalProviderSelfhostedAI500,
			APIKey: nofxos.DefaultAuthKey,
		},
	}

	provider := config.ResolveSignalProvider()
	if provider.APIKey != "local-selfhosted-token" {
		t.Fatalf("provider API key = %q, want env token", provider.APIKey)
	}
}

func TestResolveSignalProvider_SelfhostedKeepsExplicitCustomToken(t *testing.T) {
	t.Setenv("SELFHOSTED_AI500_AUTH_TOKEN", "local-selfhosted-token")

	config := StrategyConfig{
		SignalProvider: SignalProviderConfig{
			Type:   SignalProviderSelfhostedAI500,
			APIKey: "custom-selfhosted-token",
		},
	}

	provider := config.ResolveSignalProvider()
	if provider.APIKey != "custom-selfhosted-token" {
		t.Fatalf("provider API key = %q, want explicit custom token", provider.APIKey)
	}
}

func TestResolveSelfhostedAI500BaseURL_Fallback(t *testing.T) {
	prevInternal, hadInternal := os.LookupEnv("SELFHOSTED_AI500_INTERNAL_URL")
	prevBase, hadBase := os.LookupEnv("SELFHOSTED_AI500_BASE_URL")
	t.Cleanup(func() {
		if hadInternal {
			_ = os.Setenv("SELFHOSTED_AI500_INTERNAL_URL", prevInternal)
		} else {
			_ = os.Unsetenv("SELFHOSTED_AI500_INTERNAL_URL")
		}
		if hadBase {
			_ = os.Setenv("SELFHOSTED_AI500_BASE_URL", prevBase)
		} else {
			_ = os.Unsetenv("SELFHOSTED_AI500_BASE_URL")
		}
	})
	_ = os.Unsetenv("SELFHOSTED_AI500_INTERNAL_URL")
	_ = os.Unsetenv("SELFHOSTED_AI500_BASE_URL")

	if baseURL := ResolveSelfhostedAI500BaseURL(); baseURL != DefaultSelfhostedAI500BaseURL {
		t.Fatalf("default selfhosted base URL = %q, want %q", baseURL, DefaultSelfhostedAI500BaseURL)
	}
}

func TestRequiresSignalProvider(t *testing.T) {
	config := GetDefaultStrategyConfig("en")
	if !config.RequiresSignalProvider() {
		t.Fatal("default config should require signal provider")
	}

	config.CoinSource.SourceType = "static"
	config.CoinSource.UseAI500 = false
	config.CoinSource.UseOITop = false
	config.CoinSource.UseOILow = false
	config.Indicators.EnableQuantData = false
	config.Indicators.EnableOIRanking = false
	config.Indicators.EnableNetFlowRanking = false
	config.Indicators.EnablePriceRanking = false

	if config.RequiresSignalProvider() {
		t.Fatal("static config without provider-backed features should not require signal provider")
	}
}
