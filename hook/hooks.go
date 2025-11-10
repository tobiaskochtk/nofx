package hook

import (
	"log"
	"sync"
)

type HookFunc func(args ...any) any

var (
	Hooks         map[string]HookFunc = map[string]HookFunc{}
	EnableHooks                       = true
	warnedHooks                       = make(map[string]bool)
	warnedHooksMu sync.Mutex
)

func HookExec[T any](key string, args ...any) *T {
	if !EnableHooks {
		// Nur beim ersten Mal loggen, wenn Hooks deaktiviert sind
		warnedHooksMu.Lock()
		if !warnedHooks["disabled"] {
			log.Printf("🔌 Hooks are disabled")
			warnedHooks["disabled"] = true
		}
		warnedHooksMu.Unlock()
		var zero *T
		return zero
	}
	if hook, exists := Hooks[key]; exists && hook != nil {
		log.Printf("🔌 Execute hook: %s", key)
		res := hook(args...)
		return res.(*T)
	} else {
		// Nur beim ersten Mal pro Hook warnen
		warnedHooksMu.Lock()
		if !warnedHooks[key] {
			log.Printf("🔌 Hook not registered: %s (will not warn again)", key)
			warnedHooks[key] = true
		}
		warnedHooksMu.Unlock()
	}
	var zero *T
	return zero
}

func RegisterHook(key string, hook HookFunc) {
	Hooks[key] = hook
}

// hook list
const (
	GETIP              = "GETIP"              // func (userID string) *IpResult
	NEW_BINANCE_TRADER = "NEW_BINANCE_TRADER" // func (userID string, client *futures.Client) *NewBinanceTraderResult
	NEW_ASTER_TRADER   = "NEW_ASTER_TRADER"   // func (userID string, client *http.Client) *NewAsterTraderResult
	SET_HTTP_CLIENT    = "SET_HTTP_CLIENT"    // func (client *http.Client) *SetHttpClientResult
)
