package selfhostedai500

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

var supportedCoinIncludes = map[string]struct{}{
	"price":   {},
	"oi":      {},
	"netflow": {},
	"ai500":   {},
}

const (
	minSelfhostedCandidateCount     = 8
	targetSelfhostedCandidateCount  = 12
	maxSelfhostedCandidateCount     = 16
	maxSelfhostedExplorationCount   = 2
	minSelfhostedAdaptiveThreshold  = 64
	selfhostedAdaptiveThresholdStep = 2
)

type candidateSelectionResult struct {
	Selected            []*marketSnapshot
	Buckets             map[string]string
	AdaptiveThreshold   float64
	EligibleCount       int
	ThresholdMatchCount int
	ExplorationCount    int
}

type scoreContext struct {
	BTC1H        float64
	BTC4H        float64
	BTC24H       float64
	BasketMedian map[string]float64
}

type Service struct {
	cfg             Config
	store           *Store
	mu              sync.RWMutex
	snapshots       map[string]*marketSnapshot
	oiRankings      map[string]oiRankingCacheEntry
	priceRankings   map[string]priceRankingCacheEntry
	netflowRankings map[string]map[string]map[string]netflowRankingCacheEntry
	scoreState      map[string]scoreStateRecord
	bootstrapPrices map[string]bootstrapPriceRef
	lastRefresh     time.Time
	lastErr         string
	ready           bool
}

func NewService(cfg Config, store *Store) *Service {
	return &Service{
		cfg:             cfg,
		store:           store,
		snapshots:       make(map[string]*marketSnapshot),
		oiRankings:      make(map[string]oiRankingCacheEntry),
		priceRankings:   make(map[string]priceRankingCacheEntry),
		netflowRankings: make(map[string]map[string]map[string]netflowRankingCacheEntry),
		scoreState:      make(map[string]scoreStateRecord),
		bootstrapPrices: make(map[string]bootstrapPriceRef),
	}
}

func (s *Service) Start(ctx context.Context) error {
	state, err := s.store.LoadScoreState()
	if err != nil {
		return err
	}
	s.scoreState = state

	if err := s.Refresh(ctx); err != nil {
		return err
	}

	go s.loop(ctx)
	return nil
}

func (s *Service) loop(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			if err := s.Refresh(refreshCtx); err != nil {
				log.Printf("[selfhosted-ai500] refresh failed: %v", err)
			}
			cancel()
		}
	}
}

func (s *Service) Refresh(ctx context.Context) error {
	rawSnapshots, err := fetchUniverseSnapshots(ctx, s.cfg.Exchanges, s.cfg.UniverseLimit)
	if err != nil {
		s.setRefreshError(err)
		return err
	}

	now := time.Now().UTC()
	refsBySymbol := make(map[string]historyRefs, len(rawSnapshots))
	bootstrapNeeded := make([]rawUniverseSnapshot, 0, len(rawSnapshots))

	s.mu.RLock()
	cachedBootstrap := make(map[string]bootstrapPriceRef, len(s.bootstrapPrices))
	for symbol, item := range s.bootstrapPrices {
		cachedBootstrap[symbol] = item
	}
	s.mu.RUnlock()

	for _, item := range rawSnapshots {
		refs, err := s.loadHistoryRefs(item.Symbol, now)
		if err != nil {
			log.Printf("[selfhosted-ai500] history lookup failed for %s: %v", item.Symbol, err)
		}
		refsBySymbol[item.Symbol] = refs

		cached, ok := cachedBootstrap[item.Symbol]
		if ok && now.Sub(cached.FetchedAt) < 30*time.Minute {
			continue
		}
		if refs.OI1H == nil || refs.OI4H == nil {
			bootstrapNeeded = append(bootstrapNeeded, item)
		}
	}

	if len(bootstrapNeeded) > 0 {
		bootstrapped := fetchBootstrapPriceRefs(ctx, bootstrapNeeded, s.cfg.BootstrapConcurrency, s.cfg.Exchanges)
		s.mu.Lock()
		for symbol, item := range bootstrapped {
			s.bootstrapPrices[symbol] = item
		}
		s.mu.Unlock()
		for symbol, item := range bootstrapped {
			cachedBootstrap[symbol] = item
		}
	}

	snapshots := make(map[string]*marketSnapshot, len(rawSnapshots))
	volumeValues := make([]float64, 0, len(rawSnapshots))
	oiNotionalValues := make([]float64, 0, len(rawSnapshots))

	for _, item := range rawSnapshots {
		snapshot := &marketSnapshot{
			Symbol:        normalizePair(item.Symbol),
			Pair:          normalizePair(item.Symbol),
			Sources:       append([]string(nil), item.Sources...),
			Price:         item.Price,
			PrevDayPrice:  item.PrevDayPrice,
			OpenInterest:  item.OpenInterest,
			Volume24H:     item.Volume24H,
			VolumeBase24H: item.VolumeBase24H,
			Funding:       item.Funding,
			Premium:       item.Premium,
			SpreadBps:     item.SpreadBps,
			UpdatedAt:     now,
			PriceChange:   make(map[string]float64),
			OIDeltas:      make(map[string]oiDelta),
		}

		refs := refsBySymbol[item.Symbol]
		bootstrap := cachedBootstrap[item.Symbol]

		if snapshot.PrevDayPrice > 0 {
			snapshot.PriceChange["24h"] = pctChange(snapshot.Price, snapshot.PrevDayPrice)
		}

		if refs.OI1H != nil && withinAge(now, refs.OI1H.Timestamp, 2*time.Hour) {
			snapshot.PriceChange["1h"] = pctChange(snapshot.Price, refs.OI1H.Price)
			snapshot.OIDeltas["1h"] = buildOIDelta(snapshot.OpenInterest, refs.OI1H.OpenInterest, snapshot.Price)
		} else if item.RefPrice1H > 0 {
			snapshot.PriceChange["1h"] = pctChange(snapshot.Price, item.RefPrice1H)
			snapshot.OIDeltas["1h"] = oiDelta{}
		} else if bootstrap.Price1H > 0 {
			snapshot.PriceChange["1h"] = pctChange(snapshot.Price, bootstrap.Price1H)
			snapshot.OIDeltas["1h"] = oiDelta{}
		}

		if refs.OI4H != nil && withinAge(now, refs.OI4H.Timestamp, 6*time.Hour) {
			snapshot.PriceChange["4h"] = pctChange(snapshot.Price, refs.OI4H.Price)
			snapshot.OIDeltas["4h"] = buildOIDelta(snapshot.OpenInterest, refs.OI4H.OpenInterest, snapshot.Price)
		} else if item.RefPrice4H > 0 {
			snapshot.PriceChange["4h"] = pctChange(snapshot.Price, item.RefPrice4H)
			snapshot.OIDeltas["4h"] = oiDelta{}
		} else if bootstrap.Price4H > 0 {
			snapshot.PriceChange["4h"] = pctChange(snapshot.Price, bootstrap.Price4H)
			snapshot.OIDeltas["4h"] = oiDelta{}
		}

		if refs.OI24H != nil && withinAge(now, refs.OI24H.Timestamp, 30*time.Hour) {
			snapshot.OIDeltas["24h"] = buildOIDelta(snapshot.OpenInterest, refs.OI24H.OpenInterest, snapshot.Price)
		} else {
			snapshot.OIDeltas["24h"] = oiDelta{}
		}

		computeProxyFlows(snapshot)

		volumeValues = append(volumeValues, snapshot.Volume24H)
		oiNotionalValues = append(oiNotionalValues, snapshot.OpenInterest*snapshot.Price)
		snapshots[snapshot.Symbol] = snapshot
	}

	sort.Float64s(volumeValues)
	sort.Float64s(oiNotionalValues)
	scoreCtx := buildScoreContext(snapshots)

	for _, snapshot := range snapshots {
		snapshot.Score, snapshot.ReasonCodes, snapshot.ScoreComponents = scoreSnapshot(snapshot, scoreCtx, volumeValues, oiNotionalValues)
		snapshot.CandidateEligible, snapshot.CandidateFilterFailures = evaluateCandidateGate(snapshot)
		s.applyScoreState(snapshot)
	}

	ranked := make([]*marketSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		ranked = append(ranked, snapshot)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].Volume24H > ranked[j].Volume24H
		}
		return ranked[i].Score > ranked[j].Score
	})
	for idx, snapshot := range ranked {
		snapshot.Rank = idx + 1
	}

	for _, snapshot := range ranked {
		if err := s.store.InsertSnapshot(snapshot); err != nil {
			log.Printf("[selfhosted-ai500] failed storing snapshot for %s: %v", snapshot.Symbol, err)
		}
		if err := s.store.SaveScoreState(snapshot); err != nil {
			log.Printf("[selfhosted-ai500] failed storing score state for %s: %v", snapshot.Symbol, err)
		}
	}
	if err := s.store.PruneSnapshots(now.Add(-s.cfg.SnapshotRetention)); err != nil {
		log.Printf("[selfhosted-ai500] prune failed: %v", err)
	}

	oiRankings := buildOIRankingCaches(snapshots)
	priceRankings := buildPriceRankingCaches(snapshots)
	netflowRankings := buildNetflowRankingCaches(snapshots)

	s.mu.Lock()
	s.snapshots = snapshots
	s.oiRankings = oiRankings
	s.priceRankings = priceRankings
	s.netflowRankings = netflowRankings
	s.lastRefresh = now
	s.lastErr = ""
	s.ready = true
	s.mu.Unlock()

	selection := selectCandidateSnapshots(ranked, s.cfg.ScoreThreshold)

	log.Printf(
		"[selfhosted-ai500] refresh complete: %d symbols, selected=%d, eligible=%d, exploration=%d, adaptive_threshold=%.2f, top=%s score=%.2f",
		len(ranked),
		len(selection.Selected),
		selection.EligibleCount,
		selection.ExplorationCount,
		selection.AdaptiveThreshold,
		topSymbol(ranked),
		topScore(ranked),
	)
	log.Printf("[selfhosted-ai500] bucket_mix: %s", formatSelectionBucketMix(selection))
	return nil
}

func (s *Service) loadHistoryRefs(symbol string, now time.Time) (historyRefs, error) {
	var refs historyRefs
	var err error

	refs.OI1H, err = s.store.LatestSnapshotBefore(symbol, now.Add(-1*time.Hour))
	if err != nil {
		return refs, err
	}
	refs.OI4H, err = s.store.LatestSnapshotBefore(symbol, now.Add(-4*time.Hour))
	if err != nil {
		return refs, err
	}
	refs.OI24H, err = s.store.LatestSnapshotBefore(symbol, now.Add(-24*time.Hour))
	if err != nil {
		return refs, err
	}
	return refs, nil
}

func (s *Service) applyScoreState(snapshot *marketSnapshot) {
	prev := s.scoreState[snapshot.Symbol]
	snapshot.LastScore = prev.LastScore
	if snapshot.LastScore == 0 {
		snapshot.LastScore = snapshot.Score
	}

	if snapshot.Score >= s.cfg.ScoreThreshold {
		snapshot.StartRegimeActive = true
		if prev.LastUpdatedAt == 0 || prev.LastScore < s.cfg.ScoreThreshold {
			snapshot.StartTime = snapshot.UpdatedAt.Unix()
			snapshot.StartPrice = snapshot.Price
			snapshot.MaxScore = snapshot.Score
			snapshot.MaxPrice = snapshot.Price
		} else {
			snapshot.StartTime = prev.StartTime
			snapshot.StartPrice = prev.StartPrice
			snapshot.MaxScore = math.Max(prev.MaxScore, snapshot.Score)
			snapshot.MaxPrice = math.Max(prev.MaxPrice, snapshot.Price)
		}
	} else {
		snapshot.StartRegimeActive = false
		snapshot.StartTime = snapshot.UpdatedAt.Unix()
		snapshot.StartPrice = snapshot.Price
		snapshot.MaxScore = snapshot.Score
		snapshot.MaxPrice = snapshot.Price
	}

	if snapshot.StartPrice > 0 {
		snapshot.IncreasePercent = ((snapshot.Price - snapshot.StartPrice) / snapshot.StartPrice) * 100
	}

	s.scoreState[snapshot.Symbol] = scoreStateRecord{
		Symbol:        snapshot.Symbol,
		StartTime:     snapshot.StartTime,
		StartPrice:    snapshot.StartPrice,
		LastScore:     snapshot.Score,
		MaxScore:      snapshot.MaxScore,
		MaxPrice:      snapshot.MaxPrice,
		LastUpdatedAt: snapshot.UpdatedAt.Unix(),
	}
}

func (s *Service) GetHealth() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"status":         ternaryString(s.ready, "ok", "degraded"),
		"ready":          s.ready,
		"snapshot_count": len(s.snapshots),
		"last_refresh":   s.lastRefresh,
		"error":          s.lastErr,
		"exchanges":      append([]string(nil), s.cfg.Exchanges...),
	}
}

func (s *Service) ExchangeLabel() string {
	if len(s.cfg.Exchanges) == 0 {
		return "hyperliquid"
	}
	return strings.Join(s.cfg.Exchanges, ",")
}

func (s *Service) GetAI500List() []ai500CoinResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ranked := s.sortedSnapshotsLocked()
	selection := selectCandidateSnapshots(ranked, s.cfg.ScoreThreshold)
	coins := make([]ai500CoinResponse, 0, len(selection.Selected))
	for _, snapshot := range selection.Selected {
		coins = append(coins, ai500CoinResponse{
			Pair:            snapshot.Pair,
			Score:           round(snapshot.Score, 2),
			StartTime:       snapshot.StartTime,
			StartPrice:      round(snapshot.StartPrice, 6),
			LastScore:       round(snapshot.LastScore, 2),
			MaxScore:        round(snapshot.MaxScore, 2),
			MaxPrice:        round(snapshot.MaxPrice, 6),
			IncreasePercent: round(snapshot.IncreasePercent, 4),
			SelectionBucket: selection.Buckets[snapshot.Symbol],
		})
	}
	return coins
}

func (s *Service) GetAI500Detail(symbol string) (*marketSnapshot, bool) {
	normalized := normalizePair(symbol)
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.snapshots[normalized]
	return snapshot, ok
}

func (s *Service) GetAI500DetailResponse(symbol string) (*ai500DetailResponse, bool) {
	normalized := normalizePair(symbol)

	s.mu.RLock()
	snapshot, ok := s.snapshots[normalized]
	if !ok {
		s.mu.RUnlock()
		return nil, false
	}
	ranked := s.sortedSnapshotsLocked()
	selection := selectCandidateSnapshots(ranked, s.cfg.ScoreThreshold)
	threshold := s.cfg.ScoreThreshold
	lastRefresh := s.lastRefresh
	s.mu.RUnlock()

	startAgeSeconds := int64(0)
	if snapshot.StartTime > 0 {
		startAgeSeconds = lastRefresh.Unix() - snapshot.StartTime
		if startAgeSeconds < 0 {
			startAgeSeconds = 0
		}
	}

	return &ai500DetailResponse{
		Pair:                    snapshot.Pair,
		Symbol:                  snapshot.Symbol,
		Sources:                 append([]string(nil), snapshot.Sources...),
		Rank:                    snapshot.Rank,
		Score:                   round(snapshot.Score, 2),
		CurrentPrice:            round(snapshot.Price, 6),
		StartTime:               snapshot.StartTime,
		StartPrice:              round(snapshot.StartPrice, 6),
		LastScore:               round(snapshot.LastScore, 2),
		MaxScore:                round(snapshot.MaxScore, 2),
		MaxPrice:                round(snapshot.MaxPrice, 6),
		IncreasePercent:         round(snapshot.IncreasePercent, 4),
		ReasonCodes:             append([]string(nil), snapshot.ReasonCodes...),
		CandidateEligible:       candidateEligible(snapshot),
		CandidateFilterFailures: append([]string(nil), snapshot.CandidateFilterFailures...),
		SelectionBucket:         selection.Buckets[snapshot.Symbol],
		AdaptiveThreshold:       round(selection.AdaptiveThreshold, 2),
		PriceChange: map[string]float64{
			"1h":  round(snapshot.PriceChange["1h"], 6),
			"4h":  round(snapshot.PriceChange["4h"], 6),
			"24h": round(snapshot.PriceChange["24h"], 6),
		},
		ScoreComponents: map[string]float64{
			"liquidity":            round(snapshot.ScoreComponents.Liquidity, 2),
			"oi":                   round(snapshot.ScoreComponents.OI, 2),
			"momentum":             round(snapshot.ScoreComponents.Momentum, 2),
			"flow":                 round(snapshot.ScoreComponents.Flow, 2),
			"s5_relative_strength": round(snapshot.ScoreComponents.RelativeStrength, 2),
			"s6_regime_quality":    round(snapshot.ScoreComponents.RegimeQuality, 2),
			"s7_risk_penalty":      round(snapshot.ScoreComponents.RiskPenalty, 2),
			"adjust":               round(snapshot.ScoreComponents.Adjust, 2),
			"total":                round(snapshot.ScoreComponents.Total, 2),
		},
		StartRegime: map[string]interface{}{
			"active":            snapshot.StartRegimeActive,
			"threshold":         round(threshold, 2),
			"age_seconds":       startAgeSeconds,
			"last_refreshed_at": lastRefresh,
		},
	}, true
}

func (s *Service) GetAI500Stats() map[string]interface{} {
	s.mu.RLock()
	ranked := s.sortedSnapshotsLocked()
	lastRefresh := s.lastRefresh
	threshold := s.cfg.ScoreThreshold
	selection := selectCandidateSnapshots(ranked, threshold)
	s.mu.RUnlock()

	if len(ranked) == 0 {
		return map[string]interface{}{
			"count":          0,
			"active_count":   0,
			"universe_count": 0,
		}
	}

	activeCount := 0
	thresholdMatchCount := 0
	scores := make([]float64, 0, len(ranked))
	topScore := 0.0
	distribution := map[string]int{
		"lt50":   0,
		"50_59":  0,
		"60_69":  0,
		"70_79":  0,
		"80_89":  0,
		"90_100": 0,
	}
	for _, snapshot := range ranked {
		score := round(snapshot.Score, 2)
		scores = append(scores, score)
		if score > topScore {
			topScore = score
		}
		if score >= threshold {
			thresholdMatchCount++
			if candidateEligible(snapshot) {
				activeCount++
			}
		}
		switch {
		case score < 50:
			distribution["lt50"]++
		case score < 60:
			distribution["50_59"]++
		case score < 70:
			distribution["60_69"]++
		case score < 80:
			distribution["70_79"]++
		case score < 90:
			distribution["80_89"]++
		default:
			distribution["90_100"]++
		}
	}
	sort.Float64s(scores)
	median := scores[len(scores)/2]

	return map[string]interface{}{
		"count":                 activeCount,
		"active_count":          activeCount,
		"selected_count":        len(selection.Selected),
		"eligible_count":        selection.EligibleCount,
		"exploration_count":     selection.ExplorationCount,
		"threshold_match_count": thresholdMatchCount,
		"universe_count":        len(ranked),
		"median_score":          round(median, 2),
		"top_score":             round(topScore, 2),
		"score_threshold":       round(threshold, 2),
		"adaptive_threshold":    round(selection.AdaptiveThreshold, 2),
		"score_distribution":    distribution,
		"refreshed_at":          lastRefresh,
	}
}

func (s *Service) GetScoreDebug(symbol string) (*scoreDebugResponse, bool) {
	normalized := normalizePair(symbol)

	s.mu.RLock()
	snapshot, ok := s.snapshots[normalized]
	if !ok {
		s.mu.RUnlock()
		return nil, false
	}
	ranked := s.sortedSnapshotsLocked()
	selection := selectCandidateSnapshots(ranked, s.cfg.ScoreThreshold)
	threshold := s.cfg.ScoreThreshold
	lastRefresh := s.lastRefresh
	s.mu.RUnlock()

	startAgeSeconds := int64(0)
	if snapshot.StartTime > 0 {
		startAgeSeconds = lastRefresh.Unix() - snapshot.StartTime
		if startAgeSeconds < 0 {
			startAgeSeconds = 0
		}
	}

	oiDeltas := make(map[string]*coinOIDeltaResponse, 3)
	for _, duration := range []string{"1h", "4h", "24h"} {
		delta := snapshot.OIDeltas[duration]
		oiDeltas[duration] = &coinOIDeltaResponse{
			OIDelta:        round(delta.Delta, 6),
			OIDeltaValue:   round(delta.DeltaValue, 2),
			OIDeltaPercent: round(delta.DeltaPercent, 4),
		}
	}

	return &scoreDebugResponse{
		Symbol:                  snapshot.Symbol,
		Pair:                    snapshot.Pair,
		Sources:                 append([]string(nil), snapshot.Sources...),
		Rank:                    snapshot.Rank,
		Score:                   round(snapshot.Score, 2),
		ScoreThreshold:          round(threshold, 2),
		AdaptiveThreshold:       round(selection.AdaptiveThreshold, 2),
		SelectionBucket:         selection.Buckets[snapshot.Symbol],
		CandidateEligible:       candidateEligible(snapshot),
		CandidateFilterFailures: append([]string(nil), snapshot.CandidateFilterFailures...),
		CurrentPrice:            round(snapshot.Price, 6),
		Volume24H:               round(snapshot.Volume24H, 2),
		OpenInterest:            round(snapshot.OpenInterest, 6),
		OINotional:              round(snapshot.OpenInterest*snapshot.Price, 2),
		Funding:                 round(snapshot.Funding, 8),
		Premium:                 round(snapshot.Premium, 8),
		SpreadBps:               round(snapshot.SpreadBps, 4),
		ReasonCodes:             append([]string(nil), snapshot.ReasonCodes...),
		ScoreComponents: map[string]float64{
			"liquidity":            round(snapshot.ScoreComponents.Liquidity, 2),
			"oi":                   round(snapshot.ScoreComponents.OI, 2),
			"momentum":             round(snapshot.ScoreComponents.Momentum, 2),
			"flow":                 round(snapshot.ScoreComponents.Flow, 2),
			"s5_relative_strength": round(snapshot.ScoreComponents.RelativeStrength, 2),
			"s6_regime_quality":    round(snapshot.ScoreComponents.RegimeQuality, 2),
			"s7_risk_penalty":      round(snapshot.ScoreComponents.RiskPenalty, 2),
			"adjust":               round(snapshot.ScoreComponents.Adjust, 2),
			"total":                round(snapshot.ScoreComponents.Total, 2),
		},
		PriceChange: map[string]float64{
			"1h":  round(snapshot.PriceChange["1h"], 6),
			"4h":  round(snapshot.PriceChange["4h"], 6),
			"24h": round(snapshot.PriceChange["24h"], 6),
		},
		OIDeltas: oiDeltas,
		NetflowProxy: map[string]*coinFlowTypeResponse{
			"institution": {
				Future: map[string]float64{
					"1h":  round(snapshot.InstitutionFutureFlow["1h"], 2),
					"4h":  round(snapshot.InstitutionFutureFlow["4h"], 2),
					"24h": round(snapshot.InstitutionFutureFlow["24h"], 2),
				},
				Spot: map[string]float64{
					"1h":  round(snapshot.InstitutionSpotFlow["1h"], 2),
					"4h":  round(snapshot.InstitutionSpotFlow["4h"], 2),
					"24h": round(snapshot.InstitutionSpotFlow["24h"], 2),
				},
			},
			"personal": {
				Future: map[string]float64{
					"1h":  round(snapshot.PersonalFutureFlow["1h"], 2),
					"4h":  round(snapshot.PersonalFutureFlow["4h"], 2),
					"24h": round(snapshot.PersonalFutureFlow["24h"], 2),
				},
				Spot: map[string]float64{
					"1h":  round(snapshot.PersonalSpotFlow["1h"], 2),
					"4h":  round(snapshot.PersonalSpotFlow["4h"], 2),
					"24h": round(snapshot.PersonalSpotFlow["24h"], 2),
				},
			},
		},
		StartRegime: map[string]interface{}{
			"active":            snapshot.StartRegimeActive,
			"threshold":         round(threshold, 2),
			"start_time":        snapshot.StartTime,
			"start_price":       round(snapshot.StartPrice, 6),
			"age_seconds":       startAgeSeconds,
			"last_refreshed_at": lastRefresh,
		},
		LastRefreshedAt: lastRefresh,
	}, true
}

func (s *Service) GetRankingDebug(kind, duration, durations string, limit int, flowType, trade string) (*rankingDebugResponse, error) {
	if limit <= 0 {
		limit = 10
	}

	normalizedKind := normalizeDebugRankingKind(kind)
	if normalizedKind == "" {
		return nil, fmt.Errorf("unsupported ranking kind %q", kind)
	}

	s.mu.RLock()
	lastRefresh := s.lastRefresh
	universeCount := len(s.snapshots)
	threshold := s.cfg.ScoreThreshold
	ranked := s.sortedSnapshotsLocked()
	selection := selectCandidateSnapshots(ranked, threshold)
	s.mu.RUnlock()

	response := &rankingDebugResponse{
		Kind:               normalizedKind,
		Limit:              limit,
		UniverseCount:      universeCount,
		ScoreThreshold:     round(threshold, 2),
		AdaptiveThreshold:  round(selection.AdaptiveThreshold, 2),
		SelectedCount:      len(selection.Selected),
		ExplorationCount:   selection.ExplorationCount,
		AvailableKinds:     []string{"ai500", "oi", "price", "netflow"},
		AvailableDurations: []string{"1h", "4h", "24h"},
		LastRefreshedAt:    lastRefresh,
	}

	switch normalizedKind {
	case "ai500":
		if limit > len(ranked) {
			limit = len(ranked)
		}
		activeCount := 0
		items := make([]ai500RankingDebugItem, 0, limit)
		for idx, snapshot := range ranked {
			if snapshot.Score >= threshold && candidateEligible(snapshot) {
				activeCount++
			}
			if idx >= limit {
				continue
			}
			items = append(items, ai500RankingDebugItem{
				Rank:              snapshot.Rank,
				Symbol:            snapshot.Symbol,
				Pair:              snapshot.Pair,
				Score:             round(snapshot.Score, 2),
				SelectionBucket:   selection.Buckets[snapshot.Symbol],
				CandidateEligible: candidateEligible(snapshot),
				Price:             round(snapshot.Price, 6),
				Volume24H:         round(snapshot.Volume24H, 2),
				OINotional:        round(snapshot.OpenInterest*snapshot.Price, 2),
				ReasonCodes:       append([]string(nil), snapshot.ReasonCodes...),
			})
		}
		response.ActiveCount = activeCount
		response.Limit = limit
		response.AI500 = items
	case "oi":
		duration = normalizeDuration(duration)
		response.Duration = duration
		response.Limit = limit
		response.OI = &oiRankingDebugPayload{
			Top: s.GetOIRanking(duration, limit, "top"),
			Low: s.GetOIRanking(duration, limit, "low"),
		}
	case "price":
		durationsList := splitDurations(durations)
		response.Durations = durationsList
		response.Price = s.GetPriceRanking(strings.Join(durationsList, ","), limit)
	case "netflow":
		duration = normalizeDuration(duration)
		flowType = normalizeFlowType(flowType)
		trade = normalizeTradeType(trade)
		response.Duration = duration
		response.FlowType = flowType
		response.Trade = trade
		response.AvailableTrades = []string{"future", "spot"}
		response.AvailableTypes = []string{"institution", "personal"}
		response.Netflow = &netflowRankingDebugPayload{
			Top: s.GetNetflowRanking(duration, limit, "top", flowType, trade),
			Low: s.GetNetflowRanking(duration, limit, "low", flowType, trade),
		}
	}

	return response, nil
}

func (s *Service) GetOIRanking(duration string, limit int, rankType string) []oiPositionResponse {
	if limit <= 0 {
		limit = 20
	}
	duration = normalizeDuration(duration)

	s.mu.RLock()
	entry, ok := s.oiRankings[duration]
	s.mu.RUnlock()

	if !ok {
		return []oiPositionResponse{}
	}

	positions := entry.Top
	if rankType == "low" {
		positions = entry.Low
	}
	return limitOIRankings(positions, limit)
}

func (s *Service) GetPriceRanking(durations string, limit int) map[string]map[string][]priceRankingItemResponse {
	if limit <= 0 {
		limit = 10
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string][]priceRankingItemResponse)
	for _, duration := range splitDurations(durations) {
		entry, ok := s.priceRankings[duration]
		if !ok {
			entry = priceRankingCacheEntry{}
		}
		top := limitPriceRankings(entry.Top, limit)
		low := limitPriceRankings(entry.Low, limit)
		result[duration] = map[string][]priceRankingItemResponse{
			"top": top,
			"low": low,
		}
	}
	return result
}

func (s *Service) GetNetflowRanking(duration string, limit int, rankType, flowType, trade string) []netflowPositionResponse {
	if limit <= 0 {
		limit = 10
	}
	duration = normalizeDuration(duration)
	trade = normalizeTradeType(trade)

	s.mu.RLock()
	tradeRankings, ok := s.netflowRankings[duration]
	if !ok {
		s.mu.RUnlock()
		return []netflowPositionResponse{}
	}
	flowRankings, ok := tradeRankings[trade]
	if !ok {
		s.mu.RUnlock()
		return []netflowPositionResponse{}
	}
	entry, ok := flowRankings[flowType]
	s.mu.RUnlock()
	if !ok {
		return []netflowPositionResponse{}
	}

	positions := entry.Top
	if rankType == "low" {
		positions = entry.Low
	}
	return limitNetflowRankings(positions, limit)
}

func (s *Service) GetCoin(symbol string, include string) (*coinQuantResponse, bool) {
	normalized := normalizePair(symbol)

	s.mu.RLock()
	snapshot, ok := s.snapshots[normalized]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}

	includeSet := make(map[string]bool)
	for key := range parseSupportedCoinIncludeSet(include) {
		includeSet[key] = true
	}
	if len(includeSet) == 0 {
		includeSet["price"] = true
		includeSet["oi"] = true
		includeSet["netflow"] = true
	}

	response := &coinQuantResponse{
		Symbol: snapshot.Symbol,
		Price:  round(snapshot.Price, 6),
	}

	if includeSet["price"] {
		response.PriceChange = map[string]float64{}
		for _, duration := range []string{"1h", "4h", "24h"} {
			response.PriceChange[duration] = round(snapshot.PriceChange[duration], 6)
		}
	}

	if includeSet["oi"] {
		deltas := make(map[string]*coinOIDeltaResponse)
		for _, duration := range []string{"1h", "4h", "24h"} {
			delta := snapshot.OIDeltas[duration]
			deltas[duration] = &coinOIDeltaResponse{
				OIDelta:        round(delta.Delta, 6),
				OIDeltaValue:   round(delta.DeltaValue, 2),
				OIDeltaPercent: round(delta.DeltaPercent, 4),
			}
		}
		response.OI = map[string]*coinOIExchangeResponse{
			"hyperliquid": {
				CurrentOI: round(snapshot.OpenInterest, 6),
				NetLong:   round(math.Max(snapshot.OIDeltas["1h"].DeltaValue, 0), 2),
				NetShort:  round(math.Abs(math.Min(snapshot.OIDeltas["1h"].DeltaValue, 0)), 2),
				Delta:     deltas,
			},
		}
	}

	if includeSet["netflow"] {
		instFuture := make(map[string]float64)
		personalFuture := make(map[string]float64)
		instSpot := make(map[string]float64)
		personalSpot := make(map[string]float64)
		for _, duration := range []string{"1h", "4h", "24h"} {
			instFuture[duration] = round(snapshot.InstitutionFutureFlow[duration], 2)
			personalFuture[duration] = round(snapshot.PersonalFutureFlow[duration], 2)
			instSpot[duration] = round(snapshot.InstitutionSpotFlow[duration], 2)
			personalSpot[duration] = round(snapshot.PersonalSpotFlow[duration], 2)
		}
		response.Netflow = &coinNetflowResponse{
			Institution: &coinFlowTypeResponse{Future: instFuture, Spot: instSpot},
			Personal:    &coinFlowTypeResponse{Future: personalFuture, Spot: personalSpot},
		}
	}

	if includeSet["ai500"] {
		response.AI500 = &coinAI500Response{
			Pair:            snapshot.Pair,
			Rank:            snapshot.Rank,
			Score:           round(snapshot.Score, 2),
			StartTime:       snapshot.StartTime,
			StartPrice:      round(snapshot.StartPrice, 6),
			LastScore:       round(snapshot.LastScore, 2),
			MaxScore:        round(snapshot.MaxScore, 2),
			MaxPrice:        round(snapshot.MaxPrice, 6),
			IncreasePercent: round(snapshot.IncreasePercent, 4),
			ReasonCodes:     append([]string(nil), snapshot.ReasonCodes...),
		}
	}

	return response, true
}

func (s *Service) sortedSnapshotsLocked() []*marketSnapshot {
	ranked := make([]*marketSnapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		ranked = append(ranked, snapshot)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].Volume24H > ranked[j].Volume24H
		}
		return ranked[i].Score > ranked[j].Score
	})
	return ranked
}

func buildScoreContext(snapshots map[string]*marketSnapshot) scoreContext {
	ctx := scoreContext{
		BasketMedian: map[string]float64{
			"1h":  0,
			"4h":  0,
			"24h": 0,
		},
	}

	if btc, ok := snapshots["BTCUSDT"]; ok {
		ctx.BTC1H = btc.PriceChange["1h"]
		ctx.BTC4H = btc.PriceChange["4h"]
		ctx.BTC24H = btc.PriceChange["24h"]
	}

	byVolume := make([]*marketSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		byVolume = append(byVolume, snapshot)
	}
	sort.Slice(byVolume, func(i, j int) bool {
		return byVolume[i].Volume24H > byVolume[j].Volume24H
	})
	if len(byVolume) > 20 {
		byVolume = byVolume[:20]
	}

	for _, duration := range []string{"1h", "4h", "24h"} {
		values := make([]float64, 0, len(byVolume))
		for _, snapshot := range byVolume {
			values = append(values, snapshot.PriceChange[duration])
		}
		ctx.BasketMedian[duration] = medianFloat64(values)
	}

	return ctx
}

func candidateEligible(snapshot *marketSnapshot) bool {
	if snapshot == nil {
		return false
	}
	if snapshot.CandidateEligible {
		return true
	}
	return len(snapshot.CandidateFilterFailures) == 0
}

func countEligibleCandidates(ranked []*marketSnapshot, threshold float64) int {
	count := 0
	for _, snapshot := range ranked {
		if snapshot.Score >= threshold && candidateEligible(snapshot) {
			count++
		}
	}
	return count
}

func selectCandidateSnapshots(ranked []*marketSnapshot, threshold float64) candidateSelectionResult {
	result := candidateSelectionResult{
		Selected:          make([]*marketSnapshot, 0, maxSelfhostedCandidateCount),
		Buckets:           make(map[string]string),
		AdaptiveThreshold: threshold,
	}
	if len(ranked) == 0 {
		return result
	}

	eligible := make([]*marketSnapshot, 0, len(ranked))
	for _, snapshot := range ranked {
		if snapshot.Score >= threshold {
			result.ThresholdMatchCount++
		}
		if candidateEligible(snapshot) {
			eligible = append(eligible, snapshot)
		}
	}
	result.EligibleCount = len(eligible)

	adaptiveThreshold := resolveAdaptiveThreshold(eligible, threshold)
	result.AdaptiveThreshold = adaptiveThreshold

	primary := make([]*marketSnapshot, 0, len(eligible))
	adaptive := make([]*marketSnapshot, 0, len(eligible))
	fallbackEligible := make([]*marketSnapshot, 0, len(eligible))
	exploration := make([]*marketSnapshot, 0, maxSelfhostedExplorationCount)
	for _, snapshot := range ranked {
		if candidateEligible(snapshot) {
			switch {
			case snapshot.Score >= threshold:
				primary = append(primary, snapshot)
			case snapshot.Score >= adaptiveThreshold:
				adaptive = append(adaptive, snapshot)
			default:
				fallbackEligible = append(fallbackEligible, snapshot)
			}
			continue
		}
		if qualifiesExplorationCandidate(snapshot, threshold, adaptiveThreshold) {
			exploration = append(exploration, snapshot)
		}
	}
	sort.SliceStable(exploration, func(i, j int) bool {
		leftFailures := len(exploration[i].CandidateFilterFailures)
		rightFailures := len(exploration[j].CandidateFilterFailures)
		if leftFailures != rightFailures {
			return leftFailures < rightFailures
		}
		if exploration[i].Score == exploration[j].Score {
			return exploration[i].Volume24H > exploration[j].Volume24H
		}
		return exploration[i].Score > exploration[j].Score
	})

	seen := make(map[string]bool)
	eligibleTarget := minInt(len(eligible), maxSelfhostedCandidateCount-maxSelfhostedExplorationCount)
	if eligibleTarget < minSelfhostedCandidateCount {
		eligibleTarget = minInt(len(eligible), minSelfhostedCandidateCount)
	}

	appendCandidateBucket(&result.Selected, primary, seen, maxSelfhostedCandidateCount, result.Buckets, "primary")
	if len(result.Selected) < eligibleTarget {
		appendCandidateBucket(&result.Selected, adaptive, seen, eligibleTarget, result.Buckets, "adaptive")
	}
	if len(result.Selected) < minSelfhostedCandidateCount {
		appendCandidateBucket(&result.Selected, fallbackEligible, seen, minSelfhostedCandidateCount, result.Buckets, "fallback_eligible")
	}
	if len(result.Selected) == 0 {
		fallbackCount := minInt(len(ranked), minSelfhostedCandidateCount)
		appendCandidateBucket(&result.Selected, ranked, seen, fallbackCount, result.Buckets, "fallback_ranked")
		return result
	}

	if len(result.Selected) < maxSelfhostedCandidateCount {
		target := minInt(maxSelfhostedCandidateCount, len(result.Selected)+maxSelfhostedExplorationCount)
		before := len(result.Selected)
		appendCandidateBucket(&result.Selected, exploration, seen, target, result.Buckets, "exploration")
		result.ExplorationCount = len(result.Selected) - before
	}

	return result
}

func appendCandidateBucket(dst *[]*marketSnapshot, src []*marketSnapshot, seen map[string]bool, target int, buckets map[string]string, bucket string) {
	for _, snapshot := range src {
		if len(*dst) >= target {
			return
		}
		if snapshot == nil || seen[snapshot.Symbol] {
			continue
		}
		seen[snapshot.Symbol] = true
		*dst = append(*dst, snapshot)
		if buckets != nil && bucket != "" {
			buckets[snapshot.Symbol] = bucket
		}
	}
}

func resolveAdaptiveThreshold(eligible []*marketSnapshot, threshold float64) float64 {
	if len(eligible) == 0 {
		return threshold
	}

	threshold = clamp(threshold, 0, 100)
	target := minInt(len(eligible), maxSelfhostedCandidateCount-maxSelfhostedExplorationCount)
	if target < minSelfhostedCandidateCount {
		target = minInt(len(eligible), minSelfhostedCandidateCount)
	}
	if target <= 0 {
		return threshold
	}
	if countEligibleAtThreshold(eligible, threshold) >= target {
		return threshold
	}

	floor := threshold - 8
	if floor < minSelfhostedAdaptiveThreshold {
		floor = minSelfhostedAdaptiveThreshold
	}
	if floor > threshold {
		floor = threshold
	}

	adaptiveThreshold := threshold
	for next := threshold - selfhostedAdaptiveThresholdStep; next >= floor; next -= selfhostedAdaptiveThresholdStep {
		adaptiveThreshold = next
		if countEligibleAtThreshold(eligible, adaptiveThreshold) >= target {
			return adaptiveThreshold
		}
	}
	return adaptiveThreshold
}

func countEligibleAtThreshold(eligible []*marketSnapshot, threshold float64) int {
	count := 0
	for _, snapshot := range eligible {
		if snapshot.Score >= threshold {
			count++
		}
	}
	return count
}

func qualifiesExplorationCandidate(snapshot *marketSnapshot, threshold, adaptiveThreshold float64) bool {
	if snapshot == nil || candidateEligible(snapshot) {
		return false
	}
	failures := snapshot.CandidateFilterFailures
	if len(failures) == 0 || len(failures) > 2 {
		return false
	}

	scoreFloor := threshold - 6
	if adaptiveThreshold-4 > scoreFloor {
		scoreFloor = adaptiveThreshold - 4
	}
	if scoreFloor < minSelfhostedAdaptiveThreshold-2 {
		scoreFloor = minSelfhostedAdaptiveThreshold - 2
	}
	if snapshot.Score < scoreFloor {
		return false
	}

	for _, failure := range failures {
		switch failure {
		case "s5_relative_strength", "s6_regime_quality", "s6_no_short_term_confirmation":
		default:
			return false
		}
	}
	return true
}

func formatSelectionBucketMix(selection candidateSelectionResult) string {
	counts := map[string]int{
		"primary":           0,
		"adaptive":          0,
		"fallback_eligible": 0,
		"fallback_ranked":   0,
		"exploration":       0,
	}
	sample := make([]string, 0, minInt(len(selection.Selected), 6))
	for idx, snapshot := range selection.Selected {
		bucket := selection.Buckets[snapshot.Symbol]
		if bucket == "" {
			bucket = "unclassified"
		}
		if _, ok := counts[bucket]; ok {
			counts[bucket]++
		}
		if idx < 6 {
			sample = append(sample, fmt.Sprintf("%s[%s:%.2f]", snapshot.Symbol, bucket, round(snapshot.Score, 2)))
		}
	}
	return fmt.Sprintf(
		"primary=%d adaptive=%d fallback_eligible=%d fallback_ranked=%d exploration=%d sample=%s",
		counts["primary"],
		counts["adaptive"],
		counts["fallback_eligible"],
		counts["fallback_ranked"],
		counts["exploration"],
		strings.Join(sample, ", "),
	)
}

func scoreSnapshot(snapshot *marketSnapshot, ctx scoreContext, volumeValues, oiNotionalValues []float64) (float64, []string, scoreComponents) {
	reasons := make([]string, 0, 10)
	liquidityScore := percentile(snapshot.Volume24H, volumeValues) * 100
	oiScore := percentile(snapshot.OpenInterest*snapshot.Price, oiNotionalValues) * 100
	momentumScore := clamp(50+(snapshot.PriceChange["1h"]*800)+(snapshot.PriceChange["4h"]*320)+(snapshot.PriceChange["24h"]*80), 0, 100)
	flowScore := clamp(50+(snapshot.Premium*12000)+(snapshot.Funding*250000), 0, 100)
	relativeStrengthScore := scoreRelativeStrength(snapshot, ctx)
	regimeQualityScore := scoreRegimeQuality(snapshot)
	riskPenaltyScore := scoreRiskPenalty(snapshot)

	snapshot.RelativeStrengthScore = relativeStrengthScore
	snapshot.RegimeQualityScore = regimeQualityScore
	snapshot.RiskPenaltyScore = riskPenaltyScore

	score := (liquidityScore * 0.25) +
		(oiScore * 0.18) +
		(momentumScore * 0.22) +
		(flowScore * 0.10) +
		(relativeStrengthScore * 0.15) +
		(regimeQualityScore * 0.10)
	riskContribution := -(riskPenaltyScore * 0.18)
	adjustment := 0.0

	if snapshot.PriceChange["1h"] > 0 && snapshot.PriceChange["4h"] > 0 {
		adjustment += 4
		reasons = append(reasons, "trend_aligned")
	}
	if snapshot.PriceChange["24h"] > 0 {
		adjustment += 3
		reasons = append(reasons, "daily_strength")
	}
	if snapshot.Volume24H > 25_000_000 {
		reasons = append(reasons, "high_liquidity")
	}
	if snapshot.OpenInterest*snapshot.Price > 5_000_000 {
		reasons = append(reasons, "oi_supported")
	}
	if relativeStrengthScore >= 60 {
		adjustment += 3
		reasons = append(reasons, "relative_strength")
	}
	if regimeQualityScore >= 60 {
		adjustment += 2
		reasons = append(reasons, "regime_quality")
	}
	if snapshot.SpreadBps > 0 && snapshot.SpreadBps > 18 {
		adjustment -= 10
		reasons = append(reasons, "wide_spread")
	}
	if snapshot.Volume24H < 750_000 {
		adjustment -= 8
		reasons = append(reasons, "thin_volume")
	}
	if snapshot.PriceChange["1h"] < 0 && snapshot.PriceChange["4h"] < 0 {
		adjustment -= 12
		reasons = append(reasons, "negative_momentum")
	}
	if relativeStrengthScore < 40 {
		reasons = append(reasons, "relative_weakness")
	}
	if riskPenaltyScore > 25 {
		reasons = append(reasons, "elevated_risk")
	}

	clamped := clamp(score+riskContribution+adjustment, 0, 100)
	return clamped, reasons, scoreComponents{
		Liquidity:        liquidityScore * 0.25,
		OI:               oiScore * 0.18,
		Momentum:         momentumScore * 0.22,
		Flow:             flowScore * 0.10,
		RelativeStrength: relativeStrengthScore * 0.15,
		RegimeQuality:    regimeQualityScore * 0.10,
		RiskPenalty:      riskContribution,
		Adjust:           adjustment,
		Total:            clamped,
	}
}

func evaluateCandidateGate(snapshot *marketSnapshot) (bool, []string) {
	failures := make([]string, 0, 5)
	if snapshot == nil {
		return false, []string{"snapshot_missing"}
	}

	isMajor := snapshot.Symbol == "BTCUSDT" || snapshot.Symbol == "ETHUSDT"
	relativeFloor := 48.0
	regimeFloor := 46.0
	riskCeiling := 24.0
	if isMajor {
		relativeFloor = 40
		regimeFloor = 42
		riskCeiling = 30
	}

	if snapshot.RelativeStrengthScore < relativeFloor {
		failures = append(failures, "s5_relative_strength")
	}
	if snapshot.RegimeQualityScore < regimeFloor {
		failures = append(failures, "s6_regime_quality")
	}
	if snapshot.RiskPenaltyScore > riskCeiling {
		failures = append(failures, "s7_risk_penalty")
	}
	if snapshot.PriceChange["1h"] <= 0 && snapshot.PriceChange["4h"] <= 0 {
		failures = append(failures, "s6_no_short_term_confirmation")
	}
	if !isMajor && snapshot.Volume24H < 15_000_000 && snapshot.OpenInterest*snapshot.Price < 15_000_000 {
		failures = append(failures, "s7_low_depth")
	}

	return len(failures) == 0, failures
}

func scoreRelativeStrength(snapshot *marketSnapshot, ctx scoreContext) float64 {
	score := 50.0
	basket1H := ctx.BasketMedian["1h"]
	basket4H := ctx.BasketMedian["4h"]
	basket24H := ctx.BasketMedian["24h"]

	if snapshot.Symbol == "BTCUSDT" {
		score += (snapshot.PriceChange["1h"] - basket1H) * 1000
		score += (snapshot.PriceChange["4h"] - basket4H) * 650
		score += (snapshot.PriceChange["24h"] - basket24H) * 180
	} else {
		score += (snapshot.PriceChange["1h"] - ctx.BTC1H) * 900
		score += (snapshot.PriceChange["4h"] - ctx.BTC4H) * 550
		score += (snapshot.PriceChange["1h"] - basket1H) * 700
		score += (snapshot.PriceChange["4h"] - basket4H) * 450
		score += (snapshot.PriceChange["24h"] - basket24H) * 140
	}

	if snapshot.PriceChange["1h"] > 0 && snapshot.PriceChange["4h"] > 0 {
		score += 6
	}
	if snapshot.PriceChange["1h"] < ctx.BTC1H && snapshot.PriceChange["4h"] < basket4H {
		score -= 10
	}

	return clamp(score, 0, 100)
}

func scoreRegimeQuality(snapshot *marketSnapshot) float64 {
	p1h := snapshot.PriceChange["1h"]
	p4h := snapshot.PriceChange["4h"]
	p24h := snapshot.PriceChange["24h"]

	score := 50.0
	score += clamp((math.Abs(p1h)*1200)+(math.Abs(p4h)*550)+(math.Abs(p24h)*160), 0, 18)

	if sameSignNonZero(p1h, p4h) {
		score += 12
	}
	if sameSignNonZero(p4h, p24h) {
		score += 8
	}
	if math.Abs(p1h) < 0.0008 && math.Abs(p4h) < 0.0025 {
		score -= 16
	}
	if math.Abs(p1h) > 0.06 || math.Abs(p4h) > 0.18 || math.Abs(p24h) > 0.40 {
		score -= 18
	}
	if snapshot.SpreadBps > 12 {
		score -= 8
	}
	if snapshot.Volume24H < 10_000_000 {
		score -= 10
	}

	return clamp(score, 0, 100)
}

func scoreRiskPenalty(snapshot *marketSnapshot) float64 {
	penalty := 0.0
	oiNotional := snapshot.OpenInterest * snapshot.Price

	if snapshot.SpreadBps > 12 {
		penalty += math.Min((snapshot.SpreadBps-12)*1.4, 18)
	}
	if snapshot.Volume24H < 25_000_000 {
		penalty += 8
	}
	if snapshot.Volume24H < 5_000_000 {
		penalty += 10
	}
	if len(snapshot.Sources) < 2 && snapshot.Volume24H < 250_000_000 {
		penalty += 12
	}
	if oiNotional < 15_000_000 {
		penalty += 8
	}
	if oiNotional < 5_000_000 {
		penalty += 10
	}
	if math.Abs(snapshot.Funding) > 0.0008 {
		penalty += 8
	}
	if math.Abs(snapshot.Funding) > 0.0015 {
		penalty += 8
	}
	if math.Abs(snapshot.Premium) > 0.003 {
		penalty += 8
	}
	if math.Abs(snapshot.PriceChange["1h"]) < 0.0005 && math.Abs(snapshot.PriceChange["4h"]) < 0.0015 {
		penalty += 6
	}

	return clamp(penalty, 0, 100)
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	mid := len(copyValues) / 2
	if len(copyValues)%2 == 1 {
		return copyValues[mid]
	}
	return (copyValues[mid-1] + copyValues[mid]) / 2
}

func sameSignNonZero(left, right float64) bool {
	switch {
	case left > 0 && right > 0:
		return true
	case left < 0 && right < 0:
		return true
	default:
		return false
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func computeProxyFlows(snapshot *marketSnapshot) {
	snapshot.InstitutionFutureFlow = make(map[string]float64)
	snapshot.PersonalFutureFlow = make(map[string]float64)
	snapshot.InstitutionSpotFlow = make(map[string]float64)
	snapshot.PersonalSpotFlow = make(map[string]float64)

	for _, duration := range []string{"1h", "4h", "24h"} {
		priceChange := snapshot.PriceChange[duration]
		delta := snapshot.OIDeltas[duration]
		durationFactor := durationWeight(duration)
		baseFlow := snapshot.Volume24H * durationFactor * priceChange
		basisComponent := snapshot.Premium * snapshot.Volume24H * 8
		fundingComponent := snapshot.Funding * snapshot.OpenInterest * snapshot.Price * durationFactor * 32

		snapshot.InstitutionFutureFlow[duration] = baseFlow*0.55 + delta.DeltaValue*0.75 + basisComponent + fundingComponent
		snapshot.PersonalFutureFlow[duration] = baseFlow*0.30 + delta.DeltaValue*0.25 - basisComponent*0.35 - fundingComponent*0.15
		snapshot.InstitutionSpotFlow[duration] = baseFlow*0.48 + delta.DeltaValue*0.12 + basisComponent*0.18 - fundingComponent*0.05
		snapshot.PersonalSpotFlow[duration] = baseFlow*0.34 - delta.DeltaValue*0.08 - basisComponent*0.06 + fundingComponent*0.02
	}
}

func buildOIRankingCaches(snapshots map[string]*marketSnapshot) map[string]oiRankingCacheEntry {
	result := make(map[string]oiRankingCacheEntry)
	for _, duration := range []string{"1h", "4h", "24h"} {
		items := make([]oiPositionResponse, 0, len(snapshots))
		for _, snapshot := range snapshots {
			delta := snapshot.OIDeltas[duration]
			items = append(items, oiPositionResponse{
				Symbol:            snapshot.Symbol,
				Price:             round(snapshot.Price, 6),
				CurrentOI:         round(snapshot.OpenInterest, 6),
				OIDelta:           round(delta.Delta, 6),
				OIDeltaPercent:    round(delta.DeltaPercent, 4),
				OIDeltaValue:      round(delta.DeltaValue, 2),
				PriceDeltaPercent: round(snapshot.PriceChange[duration]*100, 4),
				NetLong:           round(math.Max(delta.DeltaValue, 0), 2),
				NetShort:          round(math.Abs(math.Min(delta.DeltaValue, 0)), 2),
			})
		}
		top := append([]oiPositionResponse(nil), items...)
		low := append([]oiPositionResponse(nil), items...)
		sort.Slice(top, func(i, j int) bool {
			if top[i].OIDeltaPercent == top[j].OIDeltaPercent {
				return top[i].CurrentOI > top[j].CurrentOI
			}
			return top[i].OIDeltaPercent > top[j].OIDeltaPercent
		})
		sort.Slice(low, func(i, j int) bool {
			if low[i].OIDeltaPercent == low[j].OIDeltaPercent {
				return low[i].CurrentOI > low[j].CurrentOI
			}
			return low[i].OIDeltaPercent < low[j].OIDeltaPercent
		})
		assignOIRanks(top)
		assignOIRanks(low)
		result[duration] = oiRankingCacheEntry{Top: top, Low: low}
	}
	return result
}

func buildPriceRankingCaches(snapshots map[string]*marketSnapshot) map[string]priceRankingCacheEntry {
	result := make(map[string]priceRankingCacheEntry)
	for _, duration := range []string{"1h", "4h", "24h"} {
		items := make([]priceRankingItemResponse, 0, len(snapshots))
		for _, snapshot := range snapshots {
			delta := snapshot.OIDeltas[duration]
			items = append(items, priceRankingItemResponse{
				Pair:         snapshot.Pair,
				Symbol:       snapshot.Symbol,
				PriceDelta:   round(snapshot.PriceChange[duration], 6),
				Price:        round(snapshot.Price, 6),
				FutureFlow:   round(snapshot.InstitutionFutureFlow[duration], 2),
				SpotFlow:     round(snapshot.InstitutionSpotFlow[duration], 2),
				OI:           round(snapshot.OpenInterest, 6),
				OIDelta:      round(delta.Delta, 6),
				OIDeltaValue: round(delta.DeltaValue, 2),
			})
		}
		top := append([]priceRankingItemResponse(nil), items...)
		low := append([]priceRankingItemResponse(nil), items...)
		sort.Slice(top, func(i, j int) bool {
			if top[i].PriceDelta == top[j].PriceDelta {
				return (top[i].FutureFlow + top[i].SpotFlow) > (top[j].FutureFlow + top[j].SpotFlow)
			}
			return top[i].PriceDelta > top[j].PriceDelta
		})
		sort.Slice(low, func(i, j int) bool {
			if low[i].PriceDelta == low[j].PriceDelta {
				return (low[i].FutureFlow + low[i].SpotFlow) < (low[j].FutureFlow + low[j].SpotFlow)
			}
			return low[i].PriceDelta < low[j].PriceDelta
		})
		result[duration] = priceRankingCacheEntry{Top: top, Low: low}
	}
	return result
}

func buildNetflowRankingCaches(snapshots map[string]*marketSnapshot) map[string]map[string]map[string]netflowRankingCacheEntry {
	result := make(map[string]map[string]map[string]netflowRankingCacheEntry)
	for _, duration := range []string{"1h", "4h", "24h"} {
		if _, ok := result[duration]; !ok {
			result[duration] = make(map[string]map[string]netflowRankingCacheEntry)
		}
		for _, trade := range []string{"future", "spot"} {
			if _, ok := result[duration][trade]; !ok {
				result[duration][trade] = make(map[string]netflowRankingCacheEntry)
			}
			for _, flowType := range []string{"institution", "personal"} {
				items := make([]netflowPositionResponse, 0, len(snapshots))
				for _, snapshot := range snapshots {
					amount := netflowAmountFor(snapshot, duration, flowType, trade)
					items = append(items, netflowPositionResponse{
						Symbol: snapshot.Symbol,
						Amount: round(amount, 2),
						Price:  round(snapshot.Price, 6),
					})
				}
				top := append([]netflowPositionResponse(nil), items...)
				low := append([]netflowPositionResponse(nil), items...)
				sort.Slice(top, func(i, j int) bool { return top[i].Amount > top[j].Amount })
				sort.Slice(low, func(i, j int) bool { return low[i].Amount < low[j].Amount })
				assignNetflowRanks(top)
				assignNetflowRanks(low)
				result[duration][trade][flowType] = netflowRankingCacheEntry{Top: top, Low: low}
			}
		}
	}
	return result
}

func netflowAmountFor(snapshot *marketSnapshot, duration, flowType, trade string) float64 {
	if snapshot == nil {
		return 0
	}
	switch trade {
	case "spot":
		if flowType == "personal" {
			return snapshot.PersonalSpotFlow[duration]
		}
		return snapshot.InstitutionSpotFlow[duration]
	default:
		if flowType == "personal" {
			return snapshot.PersonalFutureFlow[duration]
		}
		return snapshot.InstitutionFutureFlow[duration]
	}
}

func limitOIRankings(items []oiPositionResponse, limit int) []oiPositionResponse {
	if limit > len(items) {
		limit = len(items)
	}
	if limit < 0 {
		limit = 0
	}
	out := make([]oiPositionResponse, limit)
	copy(out, items[:limit])
	return out
}

func limitPriceRankings(items []priceRankingItemResponse, limit int) []priceRankingItemResponse {
	if limit > len(items) {
		limit = len(items)
	}
	if limit < 0 {
		limit = 0
	}
	out := make([]priceRankingItemResponse, limit)
	copy(out, items[:limit])
	return out
}

func limitNetflowRankings(items []netflowPositionResponse, limit int) []netflowPositionResponse {
	if limit > len(items) {
		limit = len(items)
	}
	if limit < 0 {
		limit = 0
	}
	out := make([]netflowPositionResponse, limit)
	copy(out, items[:limit])
	return out
}

func assignOIRanks(items []oiPositionResponse) {
	for idx := range items {
		items[idx].Rank = idx + 1
	}
}

func assignNetflowRanks(items []netflowPositionResponse) {
	for idx := range items {
		items[idx].Rank = idx + 1
	}
}

func parseSupportedCoinIncludeSet(raw string) map[string]bool {
	result := make(map[string]bool)
	for _, item := range strings.Split(raw, ",") {
		key := strings.TrimSpace(strings.ToLower(item))
		if key == "" {
			continue
		}
		if _, ok := supportedCoinIncludes[key]; ok {
			result[key] = true
		}
	}
	return result
}

func parseSupportedCoinIncludeNames(raw string) []string {
	result := make([]string, 0, len(supportedCoinIncludes))
	for key := range parseSupportedCoinIncludeSet(raw) {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func supportedCoinIncludeNames() []string {
	result := make([]string, 0, len(supportedCoinIncludes))
	for key := range supportedCoinIncludes {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func buildOIDelta(current, previous, price float64) oiDelta {
	if current <= 0 || previous <= 0 || price <= 0 {
		return oiDelta{}
	}
	delta := current - previous
	return oiDelta{
		Delta:        delta,
		DeltaValue:   delta * price,
		DeltaPercent: (delta / previous) * 100,
	}
}

func normalizeDuration(duration string) string {
	switch strings.TrimSpace(strings.ToLower(duration)) {
	case "4h":
		return "4h"
	case "24h":
		return "24h"
	default:
		return "1h"
	}
}

func splitDurations(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{"1h"}
	}
	seen := make(map[string]bool)
	result := make([]string, 0, 3)
	for _, part := range strings.Split(raw, ",") {
		duration := normalizeDuration(part)
		if !seen[duration] {
			seen[duration] = true
			result = append(result, duration)
		}
	}
	if len(result) == 0 {
		return []string{"1h"}
	}
	return result
}

func normalizePair(symbol string) string {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	upper = strings.TrimSuffix(upper, "USDT")
	return upper + "USDT"
}

func durationWeight(duration string) float64 {
	switch duration {
	case "4h":
		return 0.35
	case "24h":
		return 1.0
	default:
		return 0.15
	}
}

func pctChange(current, previous float64) float64 {
	if current <= 0 || previous <= 0 {
		return 0
	}
	return (current - previous) / previous
}

func percentile(value float64, sortedValues []float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	idx := sort.SearchFloat64s(sortedValues, value)
	if idx >= len(sortedValues) {
		return 1
	}
	if len(sortedValues) == 1 {
		return 1
	}
	return float64(idx) / float64(len(sortedValues)-1)
}

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func round(value float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(value*pow) / pow
}

func withinAge(now, point time.Time, maxAge time.Duration) bool {
	if point.IsZero() {
		return false
	}
	return now.Sub(point) <= maxAge
}

func (s *Service) setRefreshError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastErr = err.Error()
	if s.lastRefresh.IsZero() {
		s.ready = false
	}
}

func ternaryString(condition bool, left, right string) string {
	if condition {
		return left
	}
	return right
}

func topSymbol(items []*marketSnapshot) string {
	if len(items) == 0 {
		return ""
	}
	return items[0].Symbol
}

func topScore(items []*marketSnapshot) float64 {
	if len(items) == 0 {
		return 0
	}
	return items[0].Score
}

func (s *Service) DebugSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fmt.Sprintf("ready=%v snapshots=%d last_refresh=%s", s.ready, len(s.snapshots), s.lastRefresh.Format(time.RFC3339))
}
