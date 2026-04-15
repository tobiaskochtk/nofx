package selfhostedai500

import "time"

type historyPoint struct {
	Symbol       string
	Timestamp    time.Time
	Price        float64
	OpenInterest float64
}

type historyRefs struct {
	OI1H  *historyPoint
	OI4H  *historyPoint
	OI24H *historyPoint
}

type bootstrapPriceRef struct {
	Price1H   float64
	Price4H   float64
	FetchedAt time.Time
}

type scoreStateRecord struct {
	Symbol        string
	StartTime     int64
	StartPrice    float64
	LastScore     float64
	MaxScore      float64
	MaxPrice      float64
	LastUpdatedAt int64
}

type oiDelta struct {
	Delta        float64
	DeltaValue   float64
	DeltaPercent float64
}

type marketSnapshot struct {
	Symbol                  string
	Pair                    string
	Sources                 []string
	Price                   float64
	PrevDayPrice            float64
	OpenInterest            float64
	Volume24H               float64
	VolumeBase24H           float64
	Funding                 float64
	Premium                 float64
	SpreadBps               float64
	UpdatedAt               time.Time
	PriceChange             map[string]float64
	OIDeltas                map[string]oiDelta
	InstitutionFutureFlow   map[string]float64
	PersonalFutureFlow      map[string]float64
	InstitutionSpotFlow     map[string]float64
	PersonalSpotFlow        map[string]float64
	Score                   float64
	RelativeStrengthScore   float64
	RegimeQualityScore      float64
	RiskPenaltyScore        float64
	StartTime               int64
	StartPrice              float64
	LastScore               float64
	MaxScore                float64
	MaxPrice                float64
	IncreasePercent         float64
	Rank                    int
	ReasonCodes             []string
	ScoreComponents         scoreComponents
	StartRegimeActive       bool
	CandidateEligible       bool
	CandidateFilterFailures []string
	SelectionBucket         string
}

type scoreComponents struct {
	Liquidity        float64
	OI               float64
	Momentum         float64
	Flow             float64
	RelativeStrength float64
	RegimeQuality    float64
	RiskPenalty      float64
	Adjust           float64
	Total            float64
}

type ai500CoinResponse struct {
	Pair            string  `json:"pair"`
	Score           float64 `json:"score"`
	StartTime       int64   `json:"start_time"`
	StartPrice      float64 `json:"start_price"`
	LastScore       float64 `json:"last_score"`
	MaxScore        float64 `json:"max_score"`
	MaxPrice        float64 `json:"max_price"`
	IncreasePercent float64 `json:"increase_percent"`
	SelectionBucket string  `json:"selection_bucket,omitempty"`
}

type oiPositionResponse struct {
	Symbol            string  `json:"symbol"`
	Rank              int     `json:"rank"`
	Price             float64 `json:"price"`
	CurrentOI         float64 `json:"current_oi"`
	OIDelta           float64 `json:"oi_delta"`
	OIDeltaPercent    float64 `json:"oi_delta_percent"`
	OIDeltaValue      float64 `json:"oi_delta_value"`
	PriceDeltaPercent float64 `json:"price_delta_percent"`
	NetLong           float64 `json:"net_long"`
	NetShort          float64 `json:"net_short"`
}

type priceRankingItemResponse struct {
	Pair         string  `json:"pair"`
	Symbol       string  `json:"symbol"`
	PriceDelta   float64 `json:"price_delta"`
	Price        float64 `json:"price"`
	FutureFlow   float64 `json:"future_flow"`
	SpotFlow     float64 `json:"spot_flow"`
	OI           float64 `json:"oi"`
	OIDelta      float64 `json:"oi_delta"`
	OIDeltaValue float64 `json:"oi_delta_value"`
}

type netflowPositionResponse struct {
	Rank   int     `json:"rank"`
	Symbol string  `json:"symbol"`
	Amount float64 `json:"amount"`
	Price  float64 `json:"price"`
}

type oiRankingCacheEntry struct {
	Top []oiPositionResponse
	Low []oiPositionResponse
}

type priceRankingCacheEntry struct {
	Top []priceRankingItemResponse
	Low []priceRankingItemResponse
}

type netflowRankingCacheEntry struct {
	Top []netflowPositionResponse
	Low []netflowPositionResponse
}

type coinOIDeltaResponse struct {
	OIDelta        float64 `json:"oi_delta"`
	OIDeltaValue   float64 `json:"oi_delta_value"`
	OIDeltaPercent float64 `json:"oi_delta_percent"`
}

type coinOIExchangeResponse struct {
	CurrentOI float64                         `json:"current_oi"`
	NetLong   float64                         `json:"net_long"`
	NetShort  float64                         `json:"net_short"`
	Delta     map[string]*coinOIDeltaResponse `json:"delta,omitempty"`
}

type coinFlowTypeResponse struct {
	Future map[string]float64 `json:"future,omitempty"`
	Spot   map[string]float64 `json:"spot,omitempty"`
}

type coinNetflowResponse struct {
	Institution *coinFlowTypeResponse `json:"institution,omitempty"`
	Personal    *coinFlowTypeResponse `json:"personal,omitempty"`
}

type coinAI500Response struct {
	Pair            string   `json:"pair,omitempty"`
	Rank            int      `json:"rank,omitempty"`
	Score           float64  `json:"score,omitempty"`
	StartTime       int64    `json:"start_time,omitempty"`
	StartPrice      float64  `json:"start_price,omitempty"`
	LastScore       float64  `json:"last_score,omitempty"`
	MaxScore        float64  `json:"max_score,omitempty"`
	MaxPrice        float64  `json:"max_price,omitempty"`
	IncreasePercent float64  `json:"increase_percent,omitempty"`
	ReasonCodes     []string `json:"reason_codes,omitempty"`
}

type ai500DetailResponse struct {
	Pair                    string                 `json:"pair"`
	Symbol                  string                 `json:"symbol"`
	Sources                 []string               `json:"sources,omitempty"`
	Rank                    int                    `json:"rank"`
	Score                   float64                `json:"score"`
	CurrentPrice            float64                `json:"current_price"`
	StartTime               int64                  `json:"start_time"`
	StartPrice              float64                `json:"start_price"`
	LastScore               float64                `json:"last_score"`
	MaxScore                float64                `json:"max_score"`
	MaxPrice                float64                `json:"max_price"`
	IncreasePercent         float64                `json:"increase_percent"`
	ReasonCodes             []string               `json:"reason_codes,omitempty"`
	CandidateEligible       bool                   `json:"candidate_eligible"`
	CandidateFilterFailures []string               `json:"candidate_filter_failures,omitempty"`
	SelectionBucket         string                 `json:"selection_bucket,omitempty"`
	AdaptiveThreshold       float64                `json:"adaptive_threshold,omitempty"`
	PriceChange             map[string]float64     `json:"price_change,omitempty"`
	ScoreComponents         map[string]float64     `json:"score_components,omitempty"`
	StartRegime             map[string]interface{} `json:"start_regime,omitempty"`
}

type scoreDebugResponse struct {
	Symbol                  string                           `json:"symbol"`
	Pair                    string                           `json:"pair"`
	Sources                 []string                         `json:"sources,omitempty"`
	Rank                    int                              `json:"rank"`
	Score                   float64                          `json:"score"`
	ScoreThreshold          float64                          `json:"score_threshold"`
	AdaptiveThreshold       float64                          `json:"adaptive_threshold,omitempty"`
	SelectionBucket         string                           `json:"selection_bucket,omitempty"`
	CandidateEligible       bool                             `json:"candidate_eligible"`
	CandidateFilterFailures []string                         `json:"candidate_filter_failures,omitempty"`
	CurrentPrice            float64                          `json:"current_price"`
	Volume24H               float64                          `json:"volume_24h"`
	OpenInterest            float64                          `json:"open_interest"`
	OINotional              float64                          `json:"oi_notional"`
	Funding                 float64                          `json:"funding"`
	Premium                 float64                          `json:"premium"`
	SpreadBps               float64                          `json:"spread_bps"`
	ReasonCodes             []string                         `json:"reason_codes,omitempty"`
	ScoreComponents         map[string]float64               `json:"score_components,omitempty"`
	PriceChange             map[string]float64               `json:"price_change,omitempty"`
	OIDeltas                map[string]*coinOIDeltaResponse  `json:"oi_deltas,omitempty"`
	NetflowProxy            map[string]*coinFlowTypeResponse `json:"netflow_proxy,omitempty"`
	StartRegime             map[string]interface{}           `json:"start_regime,omitempty"`
	LastRefreshedAt         time.Time                        `json:"last_refreshed_at"`
}

type ai500RankingDebugItem struct {
	Rank              int      `json:"rank"`
	Symbol            string   `json:"symbol"`
	Pair              string   `json:"pair"`
	Score             float64  `json:"score"`
	SelectionBucket   string   `json:"selection_bucket,omitempty"`
	CandidateEligible bool     `json:"candidate_eligible"`
	Price             float64  `json:"price"`
	Volume24H         float64  `json:"volume_24h"`
	OINotional        float64  `json:"oi_notional"`
	ReasonCodes       []string `json:"reason_codes,omitempty"`
}

type oiRankingDebugPayload struct {
	Top []oiPositionResponse `json:"top"`
	Low []oiPositionResponse `json:"low"`
}

type netflowRankingDebugPayload struct {
	Top []netflowPositionResponse `json:"top"`
	Low []netflowPositionResponse `json:"low"`
}

type rankingDebugResponse struct {
	Kind               string                                           `json:"kind"`
	Limit              int                                              `json:"limit"`
	Duration           string                                           `json:"duration,omitempty"`
	Durations          []string                                         `json:"durations,omitempty"`
	FlowType           string                                           `json:"flow_type,omitempty"`
	Trade              string                                           `json:"trade,omitempty"`
	UniverseCount      int                                              `json:"universe_count"`
	ActiveCount        int                                              `json:"active_count,omitempty"`
	SelectedCount      int                                              `json:"selected_count,omitempty"`
	ExplorationCount   int                                              `json:"exploration_count,omitempty"`
	ScoreThreshold     float64                                          `json:"score_threshold,omitempty"`
	AdaptiveThreshold  float64                                          `json:"adaptive_threshold,omitempty"`
	AvailableDurations []string                                         `json:"available_durations,omitempty"`
	AvailableKinds     []string                                         `json:"available_kinds,omitempty"`
	AvailableTrades    []string                                         `json:"available_trades,omitempty"`
	AvailableTypes     []string                                         `json:"available_types,omitempty"`
	LastRefreshedAt    time.Time                                        `json:"last_refreshed_at"`
	AI500              []ai500RankingDebugItem                          `json:"ai500,omitempty"`
	OI                 *oiRankingDebugPayload                           `json:"oi,omitempty"`
	Price              map[string]map[string][]priceRankingItemResponse `json:"price,omitempty"`
	Netflow            *netflowRankingDebugPayload                      `json:"netflow,omitempty"`
}

type coinQuantResponse struct {
	Symbol      string                             `json:"symbol"`
	Price       float64                            `json:"price"`
	Netflow     *coinNetflowResponse               `json:"netflow,omitempty"`
	OI          map[string]*coinOIExchangeResponse `json:"oi,omitempty"`
	AI500       *coinAI500Response                 `json:"ai500,omitempty"`
	PriceChange map[string]float64                 `json:"price_change,omitempty"`
}
