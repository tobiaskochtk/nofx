package trader

import (
	"sort"
	"strings"
	"time"

	"nofx/kernel"
)

const unsupportedCandidateSymbolTTL = 12 * time.Hour

func (at *AutoTrader) isUnsupportedCandidateSymbolCached(symbol string) bool {
	if at == nil {
		return false
	}
	symbol = normalizeTrackedSymbol(symbol)
	if symbol == "" {
		return false
	}

	at.unsupportedCandidateSymbolsMu.RLock()
	expiresAtMs, ok := at.unsupportedCandidateSymbols[symbol]
	at.unsupportedCandidateSymbolsMu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().UTC().UnixMilli() <= expiresAtMs {
		return true
	}

	at.unsupportedCandidateSymbolsMu.Lock()
	delete(at.unsupportedCandidateSymbols, symbol)
	at.unsupportedCandidateSymbolsMu.Unlock()
	return false
}

func (at *AutoTrader) markUnsupportedCandidateSymbol(symbol string) {
	if at == nil {
		return
	}
	symbol = normalizeTrackedSymbol(symbol)
	if symbol == "" {
		return
	}

	at.unsupportedCandidateSymbolsMu.Lock()
	if at.unsupportedCandidateSymbols == nil {
		at.unsupportedCandidateSymbols = make(map[string]int64)
	}
	at.unsupportedCandidateSymbols[symbol] = time.Now().UTC().Add(unsupportedCandidateSymbolTTL).UnixMilli()
	at.unsupportedCandidateSymbolsMu.Unlock()
}

func (at *AutoTrader) clearUnsupportedCandidateSymbol(symbol string) {
	if at == nil {
		return
	}
	symbol = normalizeTrackedSymbol(symbol)
	if symbol == "" {
		return
	}

	at.unsupportedCandidateSymbolsMu.Lock()
	delete(at.unsupportedCandidateSymbols, symbol)
	at.unsupportedCandidateSymbolsMu.Unlock()
}

func (at *AutoTrader) filterCachedUnsupportedCandidateCoins(candidateCoins []kernel.CandidateCoin) ([]kernel.CandidateCoin, []string) {
	if len(candidateCoins) == 0 {
		return nil, nil
	}

	filtered := make([]kernel.CandidateCoin, 0, len(candidateCoins))
	removedSet := make(map[string]struct{})
	for _, coin := range candidateCoins {
		symbol := normalizeTrackedSymbol(coin.Symbol)
		if symbol == "" {
			continue
		}
		if at.isUnsupportedCandidateSymbolCached(symbol) {
			removedSet[symbol] = struct{}{}
			continue
		}
		filtered = append(filtered, coin)
	}

	removed := sortedSymbolKeys(removedSet)
	return filtered, removed
}

func filterUnsupportedCandidateCoins(candidateCoins []kernel.CandidateCoin, unsupported map[string]struct{}) []kernel.CandidateCoin {
	if len(candidateCoins) == 0 || len(unsupported) == 0 {
		return candidateCoins
	}

	filtered := make([]kernel.CandidateCoin, 0, len(candidateCoins))
	for _, coin := range candidateCoins {
		symbol := normalizeTrackedSymbol(coin.Symbol)
		if symbol == "" {
			continue
		}
		if _, blocked := unsupported[symbol]; blocked {
			continue
		}
		filtered = append(filtered, coin)
	}
	return filtered
}

func normalizeTrackedSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func sortedSymbolKeys(items map[string]struct{}) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for symbol := range items {
		out = append(out, symbol)
	}
	sort.Strings(out)
	return out
}
