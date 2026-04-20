package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"nofx/market"
	"nofx/pkg/types"
)

const (
	PayloadSchemaVersion     = "trade_snapshot.v4"
	defaultTopK              = 12
	defaultRecentTradesLimit = 5
	defaultMTFSummaryLimit   = 4
	featureCoverageThreshold = 0.95
	featureSLALimitSeconds   = 30
	requiredMaxStaleSeconds  = 300
	defaultContextTF         = "15m"
	defaultContextPriceType  = "mark"
	minScoreEpsilon          = 0.001
	breakoutVolumeRatioMin   = 1.15
	breakoutConfirmRatioMin  = 1.00
	breakoutChangePctMin     = 0.75
	symbolCooldownMin        = 45
	symbolLossCooldownMin    = 180
	symbolStreakCooldownMin  = 360
)

var requiredCandidateFields = []string{
	"sym",
	"px_mark",
	"side_bias",
	"score",
	"confidence",
	"ctx",
	"qos",
}

var requiredCandidateContextFields = []string{
	"tf",
	"px_type",
	"ema_fast",
	"macd",
	"rsi",
	"oi",
	"oi_d1h_pct",
	"fund_bps",
	"basis_pct",
}

var nullableOKFields = []string{
	"candidates[].spread_bps",
	"candidates[].liq_score",
	"candidates[].feat.orderflow",
	"candidates[].feat.risk",
	"candidates[].feat.levels",
	"candidates[].feat.volatility",
}

type livePayload struct {
	Schema                string                        `json:"schema"`
	TimestampUTC          string                        `json:"ts_utc"`
	Run                   runPayload                    `json:"run"`
	Account               accountPayload                `json:"account"`
	Benchmark             *benchmarkPayload             `json:"benchmark,omitempty"`
	MarketLeadership      *marketLeadershipPayload      `json:"market_leadership,omitempty"`
	RelativeValue         *relativeValuePayload         `json:"relative_value,omitempty"`
	Performance           *performancePayload           `json:"performance,omitempty"`
	RecentExecutionRegime *recentExecutionRegimePayload `json:"recent_execution_regime,omitempty"`
	RecentTrades          []recentTradePayload          `json:"recent_trades,omitempty"`
	Defs                  defsPayload                   `json:"defs"`
	Enums                 enumsPayload                  `json:"enums"`
	Req                   reqPayload                    `json:"req"`
	Policy                policyPayload                 `json:"pol"`
	Positions             []positionPayload             `json:"positions"`
	Candidates            []candidatePayload            `json:"candidates"`
	OutputContract        outputContractPayload         `json:"output_contract"`
}

type runPayload struct {
	Cycle    int `json:"cycle"`
	RuntimeS int `json:"runtime_s"`
}

type accountPayload struct {
	Equity     float64 `json:"equity"`
	Balance    float64 `json:"balance"`
	UPnL       float64 `json:"upnl"`
	MarginUsed float64 `json:"margin_used"`
}

type defsPayload struct {
	CtxTF      string `json:"ctx_tf"`
	PxType     string `json:"px_type"`
	StaleS     string `json:"stale_s"`
	FundBps    string `json:"fund_bps"`
	BasisPct   string `json:"basis_pct"`
	OID1hPct   string `json:"oi_d1h_pct"`
	Chg1h      string `json:"chg_1h"`
	Chg4h      string `json:"chg_4h"`
	TFChgPct   string `json:"tf_chg_pct"`
	EMAFast    string `json:"ema_fast"`
	EMASlow    string `json:"ema_slow,omitempty"`
	MACD       string `json:"macd"`
	RSIPeriod  string `json:"rsi_period,omitempty"`
	RSI        string `json:"rsi"`
	RSI7       string `json:"rsi7"`
	RSI14      string `json:"rsi14,omitempty"`
	Score      string `json:"score"`
	Confidence string `json:"confidence"`
	F13ResZ    string `json:"f13_residual_z,omitempty"`
	F14Hedge   string `json:"f14_hedge_ratio,omitempty"`
	F15Cost100 string `json:"f15_roundtrip_100usd_bps,omitempty"`
	F15Net     string `json:"f15_net_after_cost_bps,omitempty"`
	F16Carry   string `json:"f16_carry_bias,omitempty"`
}

type enumsPayload struct {
	OIDiv []string `json:"oi_div"`
}

type reqPayload struct {
	RequiredTF                 string   `json:"required_tf"`
	MaxStaleS                  int      `json:"max_stale_s"`
	RequiredCandidateFields    []string `json:"required_candidate_fields"`
	RequiredCandidateCtxFields []string `json:"required_candidate_ctx_fields"`
	EMAFastPeriod              int      `json:"ema_fast_period,omitempty"`
	EMASlowPeriod              int      `json:"ema_slow_period,omitempty"`
	RSIPeriod                  int      `json:"rsi_period,omitempty"`
	NullableOKAny              []string `json:"nullable_ok_any"`
}

type policyPayload struct {
	AllowedActions       []string         `json:"allowed_actions"`
	AllowedSides         []string         `json:"allowed_sides"`
	SizeCap              float64          `json:"size_cap"`
	SizeFloorUSDT        float64          `json:"size_floor_usdt,omitempty"`
	SizePctMin           float64          `json:"size_pct_min,omitempty"`
	MaxLeverage          int              `json:"max_leverage"`
	MaxNewPositions      int              `json:"max_new_positions"`
	ManagePositionsFirst bool             `json:"manage_positions_first"`
	ObeySideBias         bool             `json:"obey_side_bias"`
	ExitOnlyIfInPos      bool             `json:"exit_only_if_in_positions"`
	EntryRule            entryRulePayload `json:"entry_rule"`
	MinScore             float64          `json:"min_score"`
}

type entryRulePayload struct {
	MustEnterIfAnyEligible bool     `json:"must_enter_if_any_eligible"`
	SelectBestBy           []string `json:"select_best_by"`
}

type outputContractPayload struct {
	Format         string                   `json:"format"`
	DecisionMode   string                   `json:"decision_mode"`
	AllowedActions []string                 `json:"allowed_actions"`
	Fields         []string                 `json:"fields"`
	FieldSources   map[string]string        `json:"field_sources"`
	DecisionFields []string                 `json:"decision_fields"`
	Constraints    outputConstraintsPayload `json:"constraints"`
}

type outputConstraintsPayload struct {
	SizePctMax                 float64 `json:"size_pct_max"`
	SizeFloorUSDT              float64 `json:"size_floor_usdt,omitempty"`
	SizePctMin                 float64 `json:"size_pct_min,omitempty"`
	LeverageMax                int     `json:"leverage_max"`
	MaxNewPositions            int     `json:"max_new_positions"`
	SideMustMatchCandidateBias bool    `json:"side_must_match_candidate_bias"`
	ExitRequiresOpenPosition   bool    `json:"exit_requires_open_position"`
	DecisionsLenMax            int     `json:"decisions_len_max"`
	DecisionsLenMin            int     `json:"decisions_len_min"`
	DecisionsMustBeArray       bool    `json:"decisions_must_be_array"`
}

type benchmarkPayload struct {
	Symbol     string                 `json:"sym"`
	PxMark     float64                `json:"px_mark"`
	SideBias   string                 `json:"side_bias"`
	Context    contextBlock           `json:"ctx"`
	Timeframes []timeframeSummaryData `json:"mtf,omitempty"`
}

type performancePayload struct {
	TotalTrades    int     `json:"total_trades"`
	WinRate        float64 `json:"win_rate"`
	ProfitFactor   float64 `json:"profit_factor"`
	SharpeRatio    float64 `json:"sharpe_ratio"`
	TotalPnL       float64 `json:"total_pnl"`
	AvgWin         float64 `json:"avg_win"`
	AvgLoss        float64 `json:"avg_loss"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
}

type recentExecutionRegimePayload struct {
	TradeCount         int     `json:"trade_count"`
	WinRatePct         float64 `json:"win_rate_pct"`
	AvgPnLPct          float64 `json:"avg_pnl_pct"`
	ConsecutiveLosses  int     `json:"consecutive_losses"`
	FollowThroughState string  `json:"follow_through_state"`
	ChurnRisk          string  `json:"churn_risk"`
}

type marketLeadershipPayload struct {
	OITop1h          []oiLeadershipPayload    `json:"oi_top_1h,omitempty"`
	OILow1h          []oiLeadershipPayload    `json:"oi_low_1h,omitempty"`
	InstInflowTop1h  []flowLeadershipPayload  `json:"inst_inflow_top_1h,omitempty"`
	InstOutflowTop1h []flowLeadershipPayload  `json:"inst_outflow_top_1h,omitempty"`
	PriceLeaders1h   []priceLeadershipPayload `json:"price_leaders_1h,omitempty"`
	PriceLeaders4h   []priceLeadershipPayload `json:"price_leaders_4h,omitempty"`
	PriceLosers1h    []priceLeadershipPayload `json:"price_losers_1h,omitempty"`
}

type recentTradePayload struct {
	Symbol       string  `json:"sym"`
	Side         string  `json:"side"`
	PnLPct       float64 `json:"pnl_pct"`
	HoldDuration string  `json:"hold_duration,omitempty"`
}

type oiLeadershipPayload struct {
	Symbol       string  `json:"sym"`
	OIDelta1hPct float64 `json:"oi_d1h_pct"`
	PriceChgPct  float64 `json:"chg_1h"`
}

type flowLeadershipPayload struct {
	Symbol string  `json:"sym"`
	Amount float64 `json:"amt"`
}

type priceLeadershipPayload struct {
	Symbol string  `json:"sym"`
	ChgPct float64 `json:"chg_pct"`
}

type timeframeSummaryData struct {
	TF            string   `json:"tf"`
	Close         float64  `json:"close"`
	EMAFastPeriod int      `json:"ema_fast_period,omitempty"`
	EMAFast       *float64 `json:"ema_fast,omitempty"`
	EMASlowPeriod int      `json:"ema_slow_period,omitempty"`
	EMASlow       *float64 `json:"ema_slow,omitempty"`
	EMA20         *float64 `json:"ema20,omitempty"`
	EMA50         *float64 `json:"ema50,omitempty"`
	MACD          *float64 `json:"macd,omitempty"`
	RSIPeriod     int      `json:"rsi_period,omitempty"`
	RSI           *float64 `json:"rsi,omitempty"`
	RSI7          *float64 `json:"rsi7,omitempty"`
	RSI14         *float64 `json:"rsi14,omitempty"`
	ATR14         *float64 `json:"atr14,omitempty"`
	BOLLUpper     *float64 `json:"boll_upper,omitempty"`
	BOLLMiddle    *float64 `json:"boll_middle,omitempty"`
	BOLLLower     *float64 `json:"boll_lower,omitempty"`
	ChgPct        *float64 `json:"chg_pct,omitempty"`
}

type positionPayload struct {
	Symbol      string                      `json:"sym"`
	Side        string                      `json:"side"`
	EntryPx     float64                     `json:"entry_px"`
	PxMark      float64                     `json:"px_mark"`
	PxLast      *float64                    `json:"px_last"`
	PxIndex     *float64                    `json:"px_index"`
	PnLPct      *float64                    `json:"pnl_pct"`
	PeakPnLPct  *float64                    `json:"peak_pnl_pct,omitempty"`
	Leverage    int                         `json:"leverage"`
	LiqPx       *float64                    `json:"liq_px"`
	AgeMin      *int                        `json:"age_min"`
	SpreadBps   *float64                    `json:"spread_bps"`
	LiqScore    *float64                    `json:"liq_score"`
	Context     contextBlock                `json:"ctx"`
	Timeframes  []timeframeSummaryData      `json:"mtf,omitempty"`
	Venue       *venueTradabilityPayload    `json:"venue_tradability,omitempty"`
	Execution   *executionQualityPayload    `json:"execution_quality,omitempty"`
	Volume      *volumeParticipationPayload `json:"volume_participation,omitempty"`
	SymbolMem   *symbolMemoryPayload        `json:"symbol_memory,omitempty"`
	QuantFlow   *quantFlowPayload           `json:"quant_flow,omitempty"`
	FeatureEnv  *featureAvailabilityPayload `json:"feature_availability,omitempty"`
	RelStrength *relativeStrengthPayload    `json:"relative_strength,omitempty"`
	Features    featurePayload              `json:"feat"`
	QoS         symbolQoS                   `json:"qos"`
}

type candidatePayload struct {
	Symbol          string                      `json:"sym"`
	PxMark          float64                     `json:"px_mark"`
	PxLast          *float64                    `json:"px_last"`
	PxIndex         *float64                    `json:"px_index"`
	SideBias        string                      `json:"side_bias"`
	Score           float64                     `json:"score"`
	Confidence      float64                     `json:"confidence"`
	SourceTags      []string                    `json:"src_tags,omitempty"`
	SelectionBucket string                      `json:"selection_bucket,omitempty"`
	SpreadBps       *float64                    `json:"spread_bps"`
	LiqScore        *float64                    `json:"liq_score"`
	Context         contextBlock                `json:"ctx"`
	Timeframes      []timeframeSummaryData      `json:"mtf,omitempty"`
	Venue           *venueTradabilityPayload    `json:"venue_tradability,omitempty"`
	Execution       *executionQualityPayload    `json:"execution_quality,omitempty"`
	Volume          *volumeParticipationPayload `json:"volume_participation,omitempty"`
	SymbolMem       *symbolMemoryPayload        `json:"symbol_memory,omitempty"`
	QuantFlow       *quantFlowPayload           `json:"quant_flow,omitempty"`
	FeatureEnv      *featureAvailabilityPayload `json:"feature_availability,omitempty"`
	RelStrength     *relativeStrengthPayload    `json:"relative_strength,omitempty"`
	Arbitrage       *candidateArbitragePayload  `json:"arb,omitempty"`
	Features        featurePayload              `json:"feat"`
	QoS             symbolQoS                   `json:"qos"`
}

type relativeValuePayload struct {
	TF       string                     `json:"tf"`
	Venue    string                     `json:"venue,omitempty"`
	TopPairs []relativeValuePairPayload `json:"top_pairs,omitempty"`
}

type relativeValuePairPayload struct {
	Symbol                    string   `json:"sym"`
	HedgeSymbol               string   `json:"hedge_sym"`
	TF                        string   `json:"tf"`
	F13ResidualBps            *float64 `json:"f13_residual_bps,omitempty"`
	F13ResidualZ              *float64 `json:"f13_residual_z,omitempty"`
	F13HalfLifeH              *float64 `json:"f13_half_life_h,omitempty"`
	F13SuggestedLeg           string   `json:"f13_suggested_leg,omitempty"`
	F14HedgeRatio             *float64 `json:"f14_hedge_ratio,omitempty"`
	F14ReturnsCorr24h         *float64 `json:"f14_returns_corr_24h,omitempty"`
	F14Confidence             *float64 `json:"f14_confidence,omitempty"`
	F15PairRoundtrip100USDBps *float64 `json:"f15_pair_roundtrip_100usd_bps,omitempty"`
	F15NetAfterCostBps        *float64 `json:"f15_net_after_cost_bps,omitempty"`
}

type candidateArbitragePayload struct {
	HedgeSymbol           string   `json:"hedge_sym,omitempty"`
	TF                    string   `json:"tf,omitempty"`
	F13ResidualBps        *float64 `json:"f13_residual_bps,omitempty"`
	F13ResidualZ          *float64 `json:"f13_residual_z,omitempty"`
	F13HalfLifeH          *float64 `json:"f13_half_life_h,omitempty"`
	F13SuggestedLeg       string   `json:"f13_suggested_leg,omitempty"`
	F14HedgeRatio         *float64 `json:"f14_hedge_ratio,omitempty"`
	F14ReturnsCorr24h     *float64 `json:"f14_returns_corr_24h,omitempty"`
	F14Confidence         *float64 `json:"f14_confidence,omitempty"`
	F15MakerFeeBps        *float64 `json:"f15_maker_fee_bps,omitempty"`
	F15TakerFeeBps        *float64 `json:"f15_taker_fee_bps,omitempty"`
	F15Slippage25USDBps   *float64 `json:"f15_slippage_25usd_bps,omitempty"`
	F15Slippage100USDBps  *float64 `json:"f15_slippage_100usd_bps,omitempty"`
	F15Entry25USDBps      *float64 `json:"f15_entry_25usd_bps,omitempty"`
	F15Entry100USDBps     *float64 `json:"f15_entry_100usd_bps,omitempty"`
	F15Roundtrip25USDBps  *float64 `json:"f15_roundtrip_25usd_bps,omitempty"`
	F15Roundtrip100USDBps *float64 `json:"f15_roundtrip_100usd_bps,omitempty"`
	F15Source             string   `json:"f15_source,omitempty"`
	F16BasisBps           *float64 `json:"f16_basis_bps,omitempty"`
	F16BasisZ             *float64 `json:"f16_basis_z,omitempty"`
	F16Funding8hBps       *float64 `json:"f16_funding_8h_bps,omitempty"`
	F16LongCarry8hBps     *float64 `json:"f16_long_carry_8h_bps,omitempty"`
	F16ShortCarry8hBps    *float64 `json:"f16_short_carry_8h_bps,omitempty"`
	F16CarryBias          string   `json:"f16_carry_bias,omitempty"`
	F16Confidence         *float64 `json:"f16_confidence,omitempty"`
}

type feeSchedulePayload struct {
	MakerBps float64
	TakerBps float64
	Source   string
}

type pairSignal struct {
	Symbol                    string
	HedgeSymbol               string
	TF                        string
	F13ResidualBps            *float64
	F13ResidualZ              *float64
	F13HalfLifeH              *float64
	F13SuggestedLeg           string
	F14HedgeRatio             *float64
	F14ReturnsCorr24h         *float64
	F14Confidence             *float64
	F15PairRoundtrip100USDBps *float64
	F15NetAfterCostBps        *float64
	Score                     float64
}

type quantFlowPayload struct {
	InstFuture15m     *float64 `json:"inst_future_15m,omitempty"`
	InstFuture1h      *float64 `json:"inst_future_1h,omitempty"`
	InstSpot1h        *float64 `json:"inst_spot_1h,omitempty"`
	RetailFuture15m   *float64 `json:"retail_future_15m,omitempty"`
	OIDelta15mPct     *float64 `json:"oi_delta_15m_pct,omitempty"`
	OIDelta1hPct      *float64 `json:"oi_delta_1h_pct,omitempty"`
	PriceChange15mPct *float64 `json:"price_change_15m_pct,omitempty"`
	PriceChange1hPct  *float64 `json:"price_change_1h_pct,omitempty"`
}

type executionQualityPayload struct {
	SpreadBps         *float64 `json:"spread_bps,omitempty"`
	LiqScore          *float64 `json:"liq_score,omitempty"`
	DepthBidUSD1Pct   *float64 `json:"depth_bid_usd_1pct,omitempty"`
	DepthAskUSD1Pct   *float64 `json:"depth_ask_usd_1pct,omitempty"`
	BookImbalance1Pct *float64 `json:"book_imbalance_1pct,omitempty"`
	SlippageEst25USD  *float64 `json:"slippage_est_25usd,omitempty"`
	SlippageEst100USD *float64 `json:"slippage_est_100usd,omitempty"`
}

type venueTradabilityPayload struct {
	Supported     bool    `json:"supported"`
	Book          string  `json:"book"`
	MinNotionalOK *bool   `json:"min_notional_ok,omitempty"`
	PriceSource   *string `json:"price_source,omitempty"`
}

type volumeParticipationPayload struct {
	TF                      string   `json:"tf"`
	ConfirmTF               *string  `json:"confirm_tf,omitempty"`
	VolRatio5               *float64 `json:"vol_ratio_5,omitempty"`
	VolRatio20              *float64 `json:"vol_ratio_20,omitempty"`
	ConfirmVolRatio20       *float64 `json:"confirm_vol_ratio_20,omitempty"`
	VolTrend                *float64 `json:"vol_trend,omitempty"`
	BreakoutVolumeConfirmed *bool    `json:"breakout_volume_confirmed,omitempty"`
}

type symbolMemoryPayload struct {
	LastSide             string   `json:"last_side"`
	LastPnLPct           *float64 `json:"last_pnl_pct,omitempty"`
	MinutesSinceClose    *int     `json:"minutes_since_close,omitempty"`
	SameSymbolLossStreak int      `json:"same_symbol_loss_streak"`
	CooldownHit          bool     `json:"cooldown_hit"`
}

type featureAvailabilityPayload struct {
	F4        string `json:"f4"`
	F5        string `json:"f5"`
	F6        string `json:"f6"`
	F7        string `json:"f7"`
	F10       string `json:"f10"`
	F11       string `json:"f11"`
	F12       string `json:"f12"`
	Freshness string `json:"freshness"`
}

type relativeStrengthPayload struct {
	TF       string   `json:"tf"`
	VsBTC1h  *float64 `json:"vs_btc_1h,omitempty"`
	VsBTC4h  *float64 `json:"vs_btc_4h,omitempty"`
	VsBTCCtx *float64 `json:"vs_btc_ctx,omitempty"`
	State    string   `json:"state,omitempty"`
}

type contextBlock struct {
	TF                   string         `json:"tf"`
	PxType               string         `json:"px_type"`
	EMAFastPeriod        int            `json:"ema_fast_period,omitempty"`
	EMAFast              *float64       `json:"ema_fast"`
	EMASlowPeriod        int            `json:"ema_slow_period,omitempty"`
	EMASlow              *float64       `json:"ema_slow,omitempty"`
	EMA20                *float64       `json:"ema20,omitempty"`
	MACD                 *float64       `json:"macd"`
	RSIPeriod            int            `json:"rsi_period,omitempty"`
	RSI                  *float64       `json:"rsi"`
	RSI7                 *float64       `json:"rsi7"`
	RSI14                *float64       `json:"rsi14,omitempty"`
	BOLLUpper            *float64       `json:"boll_upper,omitempty"`
	BOLLMiddle           *float64       `json:"boll_middle,omitempty"`
	BOLLLower            *float64       `json:"boll_lower,omitempty"`
	PriceChange1h        *float64       `json:"chg_1h,omitempty"`
	PriceChange4h        *float64       `json:"chg_4h,omitempty"`
	OI                   *float64       `json:"oi"`
	OIDelta1hPct         *float64       `json:"oi_d1h_pct"`
	OIPriceDiv           *string        `json:"oi_div"`
	OIPriceCorr24h       *float64       `json:"oi_corr"`
	OIZ7d                *float64       `json:"oi_z"`
	FundingBps           *float64       `json:"fund_bps"`
	FundingMedianZ7d     *float64       `json:"fund_z"`
	FundingDispersionBps *float64       `json:"fund_disp_bps"`
	BasisPct             *float64       `json:"basis_pct"`
	BasisZ14d            *float64       `json:"basis_z"`
	Source               contextSources `json:"src"`
}

type contextSources struct {
	OI    string `json:"oi"`
	Fund  string `json:"fund"`
	Basis string `json:"basis"`
}

type featurePayload struct {
	Orderflow  *orderflowPayload  `json:"orderflow"`
	Risk       *riskPayload       `json:"risk"`
	Levels     *levelsPayload     `json:"levels"`
	Volatility *volatilityPayload `json:"volatility"`
}

type orderflowPayload struct {
	CVDShort       *float64 `json:"cvd_short"`
	CVDLong        *float64 `json:"cvd_long"`
	ImbalanceShort *float64 `json:"imbalance_short"`
	ImbalanceLong  *float64 `json:"imbalance_long"`
	TakerBuyRatio  *float64 `json:"taker_buy_ratio"`
	SlopePx20      *float64 `json:"slope_px20"`
	SlopeCVD20     *float64 `json:"slope_cvd20"`
	Confidence     *float64 `json:"confidence"`
}

type riskPayload struct {
	DistUpATR       *float64 `json:"dist_up_atr"`
	DistDnATR       *float64 `json:"dist_dn_atr"`
	LiqRiskUp       *int     `json:"liq_risk_up"`
	LiqRiskDown     *int     `json:"liq_risk_down"`
	PreferDirection *string  `json:"prefer_direction"`
	Confidence      *float64 `json:"confidence"`
}

type levelsPayload struct {
	NearUp      *nearLevelPayload `json:"near_up"`
	NearDn      *nearLevelPayload `json:"near_dn"`
	Bias        *string           `json:"bias"`
	ReclaimUp   *bool             `json:"reclaim_up"`
	RejectionDn *bool             `json:"rejection_down"`
	Confidence  *float64          `json:"confidence"`
}

type nearLevelPayload struct {
	Name    *string  `json:"name"`
	DistATR *float64 `json:"dist_atr"`
}

type volatilityPayload struct {
	BBW            *float64 `json:"bbw"`
	Squeeze        *bool    `json:"squeeze"`
	SqueezeRelease *bool    `json:"squeeze_release"`
	RealizedVol    *float64 `json:"realized_vol"`
	Regime         *string  `json:"regime"`
	Confidence     *float64 `json:"confidence"`
}

type symbolQoS struct {
	StaleS   int      `json:"stale_s"`
	Missing  []string `json:"missing"`
	Coverage float64  `json:"coverage"`
}

type payloadDiagnostics struct {
	FeaturePass map[string][]string
	ByteSize    int
	RuneSize    int
}

// BuildUserPayload exports the compact payload builder for cross-package callers.
func BuildUserPayload(ctx *Context) (string, error) {
	return buildUserPayload(ctx)
}

func buildUserPayload(ctx *Context) (string, error) {
	payload, diag, err := assembleLivePayload(ctx)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode payload: %w", err)
	}
	diag.ByteSize = len(encoded)
	diag.RuneSize = utf8.RuneCount(encoded)
	logPayloadDiagnostics(payload.Schema, diag)
	return string(encoded), nil
}

func assembleLivePayload(ctx *Context) (*livePayload, *payloadDiagnostics, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("context is nil")
	}
	requiredTF := resolveRequiredTF(ctx)
	contextPriceType := resolveContextPriceType(ctx)

	version := strings.TrimSpace(ctx.PayloadVersion)
	if version == "" {
		version = PayloadSchemaVersion
	}
	if version != PayloadSchemaVersion {
		return nil, nil, fmt.Errorf("payload version %s not supported, upgrade to %s", version, PayloadSchemaVersion)
	}
	emaFastPeriod, emaSlowPeriod := resolveEMAPair(ctx)
	rsiPeriod := resolveRSIPeriod(ctx)
	sizeCap := normalizeSizeCap(ctx.AltcoinPosRatio)
	minPositionSize := normalizeMinPositionSize(ctx.MinPositionSize)
	sizePctMin := computeSizePctMin(ctx.Account.TotalEquity, minPositionSize)
	maxLeverage := normalizePayloadMaxLeverage(ctx.BTCETHLeverage, ctx.AltcoinLeverage)
	maxNewPositions := normalizeMaxNewPositions(ctx.MaxPositions, len(ctx.Positions))

	live := &livePayload{
		Schema:       version,
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
		Run: runPayload{
			Cycle:    ctx.CallCount,
			RuntimeS: maxInt(0, ctx.RuntimeMinutes*60),
		},
		Account: accountPayload{
			Equity:     roundTo(ctx.Account.TotalEquity, 3),
			Balance:    roundTo(ctx.Account.AvailableBalance, 3),
			UPnL:       roundTo(ctx.Account.UnrealizedPnL, 3),
			MarginUsed: roundTo(normalizeMarginUsedRatio(ctx.Account.MarginUsedPct), 3),
		},
		Defs: defsPayload{
			CtxTF:      "all_ctx_indicators_apply_to_this_tf",
			PxType:     "price_type_used_for_decisions (mark|last|index)",
			StaleS:     "seconds since the newest underlying market-data point used for this candidate",
			FundBps:    "funding_rate_bps_per_8h; positive=longs_pay_shorts",
			BasisPct:   "perp_minus_index_pct; negative=perp_below_index",
			OID1hPct:   "open_interest_change_pct_over_1h",
			Chg1h:      "price_change_pct_over_1h_from_current_market_snapshot",
			Chg4h:      "price_change_pct_over_4h_from_current_market_snapshot",
			TFChgPct:   "percent_change_over_the_loaded_series_window_for_this_timeframe_summary",
			EMAFast:    "strategy-configured_fast_ema_on_ctx.tf",
			EMASlow:    "strategy-configured_slow_ema_on_ctx.tf_when_defined",
			MACD:       "macd_line_minus_signal_line",
			RSIPeriod:  "strategy-configured_primary_rsi_period_on_ctx.tf",
			RSI:        "strategy-configured_primary_rsi_on_ctx.tf",
			RSI7:       "rsi_period_7_on_ctx.tf",
			RSI14:      "rsi_period_14_on_ctx.tf",
			Score:      "normalized_score_0_to_1; higher=better_for_side_bias",
			Confidence: "estimated_signal_reliability_0_to_1",
			F13ResZ:    "pair_spread_residual_zscore_on_relative_value.tf; positive=symbol_rich_vs_hedge",
			F14Hedge:   "ols_hedge_ratio_of_symbol_vs_hedge_on_log_prices",
			F15Cost100: "estimated_roundtrip_cost_for_100usd_notional_in_bps",
			F15Net:     "absolute_residual_edge_minus_estimated_roundtrip_cost_in_bps",
			F16Carry:   "basis_funding_implied_carry_bias_for_the_candidate",
		},
		Enums: enumsPayload{
			OIDiv: []string{"confirming", "divergent", "neutral"},
		},
		Req: reqPayload{
			RequiredTF:                 requiredTF,
			MaxStaleS:                  requiredMaxStaleSeconds,
			RequiredCandidateFields:    append([]string(nil), requiredCandidateFields...),
			RequiredCandidateCtxFields: append([]string(nil), requiredCandidateContextFields...),
			EMAFastPeriod:              emaFastPeriod,
			EMASlowPeriod:              emaSlowPeriod,
			RSIPeriod:                  rsiPeriod,
			NullableOKAny:              append([]string(nil), nullableOKFields...),
		},
		Policy: policyPayload{
			AllowedActions:       []string{"HOLD", "ENTER", "EXIT"},
			AllowedSides:         []string{"long", "short"},
			SizeCap:              sizeCap,
			SizeFloorUSDT:        minPositionSize,
			SizePctMin:           sizePctMin,
			MaxLeverage:          maxLeverage,
			MaxNewPositions:      maxNewPositions,
			ManagePositionsFirst: true,
			ObeySideBias:         true,
			ExitOnlyIfInPos:      true,
			EntryRule: entryRulePayload{
				MustEnterIfAnyEligible: true,
				SelectBestBy:           []string{"score", "confidence", "-qos.stale_s"},
			},
			MinScore: 0.70,
		},
		OutputContract: outputContractPayload{
			Format:         "json_only",
			DecisionMode:   "single_best",
			AllowedActions: []string{"HOLD", "ENTER", "EXIT"},
			Fields:         []string{"ts_utc", "cycle", "decisions"},
			FieldSources: map[string]string{
				"ts_utc": "ts_utc",
				"cycle":  "run.cycle",
			},
			DecisionFields: []string{"sym", "action", "side", "size_pct", "leverage", "confidence", "reason_codes", "stops_targets"},
			Constraints: outputConstraintsPayload{
				SizePctMax:                 sizeCap,
				SizeFloorUSDT:              minPositionSize,
				SizePctMin:                 sizePctMin,
				LeverageMax:                maxLeverage,
				MaxNewPositions:            maxNewPositions,
				SideMustMatchCandidateBias: true,
				ExitRequiresOpenPosition:   true,
				DecisionsLenMax:            1,
				DecisionsLenMin:            0,
				DecisionsMustBeArray:       true,
			},
		},
		Positions:  make([]positionPayload, 0, len(ctx.Positions)),
		Candidates: make([]candidatePayload, 0, len(ctx.CandidateCoins)),
	}

	diag := &payloadDiagnostics{FeaturePass: make(map[string][]string)}
	live.MarketLeadership = buildMarketLeadershipPayload(ctx.MarketLeadership)
	live.Performance = buildPerformancePayload(ctx.Performance)
	live.RecentExecutionRegime = buildRecentExecutionRegimePayload(ctx.RecentExecutionRegime)
	live.RecentTrades = buildRecentTradesPayload(ctx.RecentTrades)
	live.Benchmark = buildBenchmarkPayload(ctx, "BTCUSDT")

	positionSymbols := make(map[string]struct{}, len(ctx.Positions))
	for _, pos := range ctx.Positions {
		payloadPos := buildPositionPayload(ctx, &pos, diag, live.Req, contextPriceType)
		positionSymbols[payloadPos.Symbol] = struct{}{}
		live.Positions = append(live.Positions, payloadPos)
	}

	candidates := make([]candidatePayload, 0, len(ctx.CandidateCoins))
	for _, coin := range ctx.CandidateCoins {
		symbol := strings.ToUpper(coin.Symbol)
		if _, held := positionSymbols[symbol]; held {
			continue
		}
		payloadCand, ok := buildCandidatePayload(ctx, coin, diag, live.Req, contextPriceType)
		if ok {
			candidates = append(candidates, payloadCand)
		}
	}
	if len(candidates) > 0 {
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})
		limit := defaultTopK
		if len(candidates) < limit {
			limit = len(candidates)
		}
		live.Candidates = candidates[:limit]
	}

	pairAnchors, topPairs := buildPairSignals(ctx, live.Candidates)
	if rv := buildRelativeValuePayload(ctx, topPairs); rv != nil {
		live.RelativeValue = rv
	}
	if len(live.Candidates) > 0 {
		for i := range live.Candidates {
			live.Candidates[i].Arbitrage = buildCandidateArbitragePayload(ctx, live.Candidates[i], pairAnchors[live.Candidates[i].Symbol])
		}
	}

	if err := validateLivePayload(live); err != nil {
		return nil, nil, err
	}
	return live, diag, nil
}

func resolveRequiredTF(ctx *Context) string {
	if ctx == nil {
		return defaultContextTF
	}
	tf := strings.TrimSpace(strings.ToLower(ctx.ContextTF))
	if tf != "" {
		return tf
	}
	return defaultContextTF
}

func resolveContextPriceType(ctx *Context) string {
	if ctx == nil {
		return defaultContextPriceType
	}
	pxType := strings.TrimSpace(strings.ToLower(ctx.PriceType))
	switch pxType {
	case "mark", "last", "index":
		return pxType
	default:
		return defaultContextPriceType
	}
}

func buildPositionPayload(ctx *Context, pos *PositionInfo, diag *payloadDiagnostics, req reqPayload, contextPriceType string) positionPayload {
	symbol := strings.ToUpper(pos.Symbol)
	data := ctx.MarketDataMap[symbol]
	context := buildContextBlock(ctx, data)
	timeframes := buildTimeframeSummaries(ctx, data)
	venue := buildVenueTradabilityPayload(ctx.VenueTradability[symbol])
	execution := buildExecutionQualityPayload(ctx.ExecutionQuality[symbol])
	volume := buildVolumeParticipationPayload(ctx, data, req.RequiredTF)
	symbolMemory := buildSymbolMemoryPayload(ctx.RecentTrades, symbol)
	features := buildFeatureBlock(ctx, data, diag, symbol)
	missing := collectMissingFields(&context)
	qos := buildSymbolQoS(data, pos.UpdateTime, missing)
	featureEnv := buildFeatureAvailabilityPayload(ctx, data, ctx.VenueTradability[symbol], execution, volume, symbolMemory, qos.StaleS, req.RequiredTF)
	relStrength := buildRelativeStrengthPayload(ctx, symbol, data, req.RequiredTF)

	pxMark := roundTo(pos.MarkPrice, 6)
	if pxMark <= 0 && data != nil {
		pxMark = roundTo(data.CurrentPrice, 6)
	}
	pxLast := optionalRounded(dataCurrentPrice(data), 6)
	pxIndex := deriveIndexPrice(pxMark, context.BasisPct)

	payload := positionPayload{
		Symbol:      symbol,
		Side:        normalizeSide(pos.Side),
		EntryPx:     roundTo(pos.EntryPrice, 6),
		PxMark:      pxMark,
		PxLast:      pxLast,
		PxIndex:     pxIndex,
		Leverage:    pos.Leverage,
		SpreadBps:   executionSpreadBps(execution),
		LiqScore:    executionLiqScore(execution),
		Context:     context,
		Timeframes:  timeframes,
		Venue:       venue,
		Execution:   execution,
		Volume:      volume,
		SymbolMem:   symbolMemory,
		QuantFlow:   buildQuantFlowPayload(ctx.QuantFlowMap[symbol]),
		FeatureEnv:  featureEnv,
		RelStrength: relStrength,
		Features:    features,
		QoS:         qos,
	}
	if pos.EntryPrice > 0 {
		pnlPct := roundTo(pos.UnrealizedPnLPct, 2)
		payload.PnLPct = &pnlPct
	}
	if pos.PeakPnLPct != 0 {
		peak := roundTo(pos.PeakPnLPct, 2)
		payload.PeakPnLPct = &peak
	}
	if pos.LiquidationPrice > 0 {
		val := roundTo(pos.LiquidationPrice, 6)
		payload.LiqPx = &val
	}
	if pos.UpdateTime > 0 {
		age := int(maxFloat(0, float64(time.Now().UnixMilli()-pos.UpdateTime)/60000))
		payload.AgeMin = &age
	}

	if req.RequiredTF != "" {
		payload.Context.TF = req.RequiredTF
	}
	if contextPriceType != "" {
		payload.Context.PxType = contextPriceType
	}
	return payload
}

func buildCandidatePayload(ctx *Context, coin CandidateCoin, diag *payloadDiagnostics, req reqPayload, contextPriceType string) (candidatePayload, bool) {
	symbol := strings.ToUpper(coin.Symbol)
	data := ctx.MarketDataMap[symbol]
	if data == nil {
		return candidatePayload{}, false
	}

	context := buildContextBlock(ctx, data)
	timeframes := buildTimeframeSummaries(ctx, data)
	venue := buildVenueTradabilityPayload(ctx.VenueTradability[symbol])
	execution := buildExecutionQualityPayload(ctx.ExecutionQuality[symbol])
	volume := buildVolumeParticipationPayload(ctx, data, req.RequiredTF)
	symbolMemory := buildSymbolMemoryPayload(ctx.RecentTrades, symbol)
	features := buildFeatureBlock(ctx, data, diag, symbol)
	missing := collectMissingFields(&context)
	qos := buildSymbolQoS(data, 0, missing)
	featureEnv := buildFeatureAvailabilityPayload(ctx, data, ctx.VenueTradability[symbol], execution, volume, symbolMemory, qos.StaleS, req.RequiredTF)
	relStrength := buildRelativeStrengthPayload(ctx, symbol, data, req.RequiredTF)

	pxMark := roundTo(data.CurrentPrice, 6)
	pxLast := optionalRounded(data.CurrentPrice, 6)
	pxIndex := deriveIndexPrice(pxMark, context.BasisPct)

	score := roundTo(computeScore(data), 3)
	if score < minScoreEpsilon {
		score = minScoreEpsilon
	}
	confidence := roundTo(derivsConfidenceScore(microDerivs(data)), 3)

	cand := candidatePayload{
		Symbol:          symbol,
		PxMark:          pxMark,
		PxLast:          pxLast,
		PxIndex:         pxIndex,
		SideBias:        deriveSideBias(data),
		Score:           score,
		Confidence:      confidence,
		SourceTags:      normalizeSourceTags(coin.Sources),
		SelectionBucket: normalizeSelectionBucket(coin.SelectionBucket),
		SpreadBps:       executionSpreadBps(execution),
		LiqScore:        executionLiqScore(execution),
		Context:         context,
		Timeframes:      timeframes,
		Venue:           venue,
		Execution:       execution,
		Volume:          volume,
		SymbolMem:       symbolMemory,
		QuantFlow:       buildQuantFlowPayload(ctx.QuantFlowMap[symbol]),
		FeatureEnv:      featureEnv,
		RelStrength:     relStrength,
		Features:        features,
		QoS:             qos,
	}
	if req.RequiredTF != "" {
		cand.Context.TF = req.RequiredTF
	}
	if contextPriceType != "" {
		cand.Context.PxType = contextPriceType
	}
	return cand, true
}

func buildBenchmarkPayload(ctx *Context, symbol string) *benchmarkPayload {
	if ctx == nil {
		return nil
	}
	data := ctx.MarketDataMap[strings.ToUpper(symbol)]
	if data == nil {
		return nil
	}
	context := buildContextBlock(ctx, data)
	if reqTF := resolveRequiredTF(ctx); reqTF != "" {
		context.TF = reqTF
	}
	if pxType := resolveContextPriceType(ctx); pxType != "" {
		context.PxType = pxType
	}
	return &benchmarkPayload{
		Symbol:     strings.ToUpper(symbol),
		PxMark:     roundTo(data.CurrentPrice, 6),
		SideBias:   deriveSideBias(data),
		Context:    context,
		Timeframes: buildTimeframeSummaries(ctx, data),
	}
}

func buildPerformancePayload(perf *PerformanceSummary) *performancePayload {
	if perf == nil || perf.TotalTrades <= 0 {
		return nil
	}
	return &performancePayload{
		TotalTrades:    perf.TotalTrades,
		WinRate:        roundTo(perf.WinRate, 2),
		ProfitFactor:   roundTo(perf.ProfitFactor, 2),
		SharpeRatio:    roundTo(perf.SharpeRatio, 2),
		TotalPnL:       roundTo(perf.TotalPnL, 2),
		AvgWin:         roundTo(perf.AvgWin, 2),
		AvgLoss:        roundTo(perf.AvgLoss, 2),
		MaxDrawdownPct: roundTo(perf.MaxDrawdownPct, 2),
	}
}

func buildMarketLeadershipPayload(summary *MarketLeadershipSummary) *marketLeadershipPayload {
	if summary == nil {
		return nil
	}
	out := &marketLeadershipPayload{
		OITop1h:          buildOILeadershipPayload(summary.OITop1h),
		OILow1h:          buildOILeadershipPayload(summary.OILow1h),
		InstInflowTop1h:  buildFlowLeadershipPayload(summary.InstInflowTop1h),
		InstOutflowTop1h: buildFlowLeadershipPayload(summary.InstOutflowTop1h),
		PriceLeaders1h:   buildPriceLeadershipPayload(summary.PriceLeaders1h),
		PriceLeaders4h:   buildPriceLeadershipPayload(summary.PriceLeaders4h),
		PriceLosers1h:    buildPriceLeadershipPayload(summary.PriceLosers1h),
	}
	if len(out.OITop1h) == 0 &&
		len(out.OILow1h) == 0 &&
		len(out.InstInflowTop1h) == 0 &&
		len(out.InstOutflowTop1h) == 0 &&
		len(out.PriceLeaders1h) == 0 &&
		len(out.PriceLeaders4h) == 0 &&
		len(out.PriceLosers1h) == 0 {
		return nil
	}
	return out
}

func buildRelativeValuePayload(ctx *Context, topPairs []pairSignal) *relativeValuePayload {
	if len(topPairs) == 0 {
		return nil
	}
	tf := topPairs[0].TF
	for _, pair := range topPairs[1:] {
		if pair.TF != tf {
			tf = "mixed"
			break
		}
	}
	out := &relativeValuePayload{
		TF:       tf,
		Venue:    normalizeExchange(ctx),
		TopPairs: make([]relativeValuePairPayload, 0, len(topPairs)),
	}
	for _, pair := range topPairs {
		out.TopPairs = append(out.TopPairs, relativeValuePairPayload{
			Symbol:                    pair.Symbol,
			HedgeSymbol:               pair.HedgeSymbol,
			TF:                        pair.TF,
			F13ResidualBps:            roundedCopy(pair.F13ResidualBps, 3),
			F13ResidualZ:              roundedCopy(pair.F13ResidualZ, 3),
			F13HalfLifeH:              roundedCopy(pair.F13HalfLifeH, 2),
			F13SuggestedLeg:           pair.F13SuggestedLeg,
			F14HedgeRatio:             roundedCopy(pair.F14HedgeRatio, 4),
			F14ReturnsCorr24h:         roundedCopy(pair.F14ReturnsCorr24h, 3),
			F14Confidence:             roundedCopy(pair.F14Confidence, 3),
			F15PairRoundtrip100USDBps: roundedCopy(pair.F15PairRoundtrip100USDBps, 3),
			F15NetAfterCostBps:        roundedCopy(pair.F15NetAfterCostBps, 3),
		})
	}
	return out
}

func buildCandidateArbitragePayload(ctx *Context, cand candidatePayload, pair *pairSignal) *candidateArbitragePayload {
	out := &candidateArbitragePayload{}
	if pair != nil {
		out.HedgeSymbol = pair.HedgeSymbol
		out.TF = pair.TF
		out.F13ResidualBps = roundedCopy(pair.F13ResidualBps, 3)
		out.F13ResidualZ = roundedCopy(pair.F13ResidualZ, 3)
		out.F13HalfLifeH = roundedCopy(pair.F13HalfLifeH, 2)
		out.F13SuggestedLeg = pair.F13SuggestedLeg
		out.F14HedgeRatio = roundedCopy(pair.F14HedgeRatio, 4)
		out.F14ReturnsCorr24h = roundedCopy(pair.F14ReturnsCorr24h, 3)
		out.F14Confidence = roundedCopy(pair.F14Confidence, 3)
	}
	if fee := buildFeeEdgeSignal(ctx, cand.Execution); fee != nil {
		out.F15MakerFeeBps = roundedCopy(fee.MakerFeeBps, 3)
		out.F15TakerFeeBps = roundedCopy(fee.TakerFeeBps, 3)
		out.F15Slippage25USDBps = roundedCopy(fee.Slippage25USDBps, 3)
		out.F15Slippage100USDBps = roundedCopy(fee.Slippage100USDBps, 3)
		out.F15Entry25USDBps = roundedCopy(fee.Entry25USDBps, 3)
		out.F15Entry100USDBps = roundedCopy(fee.Entry100USDBps, 3)
		out.F15Roundtrip25USDBps = roundedCopy(fee.Roundtrip25USDBps, 3)
		out.F15Roundtrip100USDBps = roundedCopy(fee.Roundtrip100USDBps, 3)
		out.F15Source = fee.Source
	}
	if carry := buildBasisCarrySignal(cand.Context); carry != nil {
		out.F16BasisBps = roundedCopy(carry.BasisBps, 3)
		out.F16BasisZ = roundedCopy(carry.BasisZ, 3)
		out.F16Funding8hBps = roundedCopy(carry.Funding8hBps, 3)
		out.F16LongCarry8hBps = roundedCopy(carry.LongCarry8hBps, 3)
		out.F16ShortCarry8hBps = roundedCopy(carry.ShortCarry8hBps, 3)
		out.F16CarryBias = carry.CarryBias
		out.F16Confidence = roundedCopy(carry.Confidence, 3)
	}
	if out.HedgeSymbol == "" &&
		out.F13ResidualBps == nil &&
		out.F13ResidualZ == nil &&
		out.F13HalfLifeH == nil &&
		out.F14HedgeRatio == nil &&
		out.F14ReturnsCorr24h == nil &&
		out.F14Confidence == nil &&
		out.F15MakerFeeBps == nil &&
		out.F15TakerFeeBps == nil &&
		out.F15Slippage25USDBps == nil &&
		out.F15Slippage100USDBps == nil &&
		out.F15Entry25USDBps == nil &&
		out.F15Entry100USDBps == nil &&
		out.F15Roundtrip25USDBps == nil &&
		out.F15Roundtrip100USDBps == nil &&
		out.F15Source == "" &&
		out.F16BasisBps == nil &&
		out.F16BasisZ == nil &&
		out.F16Funding8hBps == nil &&
		out.F16LongCarry8hBps == nil &&
		out.F16ShortCarry8hBps == nil &&
		out.F16CarryBias == "" &&
		out.F16Confidence == nil {
		return nil
	}
	return out
}

type feeEdgeSignal struct {
	MakerFeeBps        *float64
	TakerFeeBps        *float64
	Slippage25USDBps   *float64
	Slippage100USDBps  *float64
	Entry25USDBps      *float64
	Entry100USDBps     *float64
	Roundtrip25USDBps  *float64
	Roundtrip100USDBps *float64
	Source             string
}

func buildFeeEdgeSignal(ctx *Context, execution *executionQualityPayload) *feeEdgeSignal {
	schedule := feeScheduleForExchange(normalizeExchange(ctx))
	if schedule == nil || execution == nil {
		return nil
	}
	spreadHalf := 0.0
	spreadFull := 0.0
	if execution.SpreadBps != nil {
		spreadHalf = *execution.SpreadBps / 2
		spreadFull = *execution.SpreadBps
	}
	entry25 := feeEdgeSignal{
		MakerFeeBps:       roundedFinitePtr(schedule.MakerBps, 3),
		TakerFeeBps:       roundedFinitePtr(schedule.TakerBps, 3),
		Slippage25USDBps:  roundedCopy(execution.SlippageEst25USD, 3),
		Slippage100USDBps: roundedCopy(execution.SlippageEst100USD, 3),
		Source:            schedule.Source,
	}
	if execution.SlippageEst25USD != nil {
		v := spreadHalf + schedule.TakerBps + *execution.SlippageEst25USD
		entry25.Entry25USDBps = roundedFinitePtr(v, 3)
		v = spreadFull + 2*schedule.TakerBps + 2*(*execution.SlippageEst25USD)
		entry25.Roundtrip25USDBps = roundedFinitePtr(v, 3)
	}
	if execution.SlippageEst100USD != nil {
		v := spreadHalf + schedule.TakerBps + *execution.SlippageEst100USD
		entry25.Entry100USDBps = roundedFinitePtr(v, 3)
		v = spreadFull + 2*schedule.TakerBps + 2*(*execution.SlippageEst100USD)
		entry25.Roundtrip100USDBps = roundedFinitePtr(v, 3)
	}
	return &entry25
}

type basisCarrySignal struct {
	BasisBps        *float64
	BasisZ          *float64
	Funding8hBps    *float64
	LongCarry8hBps  *float64
	ShortCarry8hBps *float64
	CarryBias       string
	Confidence      *float64
}

func buildBasisCarrySignal(ctx contextBlock) *basisCarrySignal {
	if ctx.BasisPct == nil && ctx.BasisZ14d == nil && ctx.FundingBps == nil {
		return nil
	}
	out := &basisCarrySignal{
		BasisZ:       roundedCopy(ctx.BasisZ14d, 3),
		Funding8hBps: roundedCopy(ctx.FundingBps, 3),
	}
	if ctx.BasisPct != nil {
		v := *ctx.BasisPct * 100
		out.BasisBps = roundedFinitePtr(v, 3)
	}
	if ctx.FundingBps != nil {
		longCarry := -*ctx.FundingBps
		shortCarry := *ctx.FundingBps
		out.LongCarry8hBps = roundedFinitePtr(longCarry, 3)
		out.ShortCarry8hBps = roundedFinitePtr(shortCarry, 3)
	}
	switch {
	case ctx.BasisPct != nil && ctx.FundingBps != nil && *ctx.BasisPct > 0 && *ctx.FundingBps > 0:
		out.CarryBias = "short_perp_favored"
	case ctx.BasisPct != nil && ctx.FundingBps != nil && *ctx.BasisPct < 0 && *ctx.FundingBps < 0:
		out.CarryBias = "long_perp_favored"
	case ctx.BasisPct != nil && *ctx.BasisPct > 0:
		out.CarryBias = "basis_short_perp_reversion"
	case ctx.BasisPct != nil && *ctx.BasisPct < 0:
		out.CarryBias = "basis_long_perp_reversion"
	case ctx.FundingBps != nil && *ctx.FundingBps > 0:
		out.CarryBias = "funding_short_perp_carry"
	case ctx.FundingBps != nil && *ctx.FundingBps < 0:
		out.CarryBias = "funding_long_perp_carry"
	default:
		out.CarryBias = "neutral"
	}
	confidence := 0.0
	if out.BasisZ != nil {
		confidence += 0.55 * clampFloat(math.Abs(*out.BasisZ)/3, 0, 1)
	}
	if out.Funding8hBps != nil {
		confidence += 0.45 * clampFloat(math.Abs(*out.Funding8hBps)/8, 0, 1)
	}
	out.Confidence = roundedFinitePtr(confidence, 3)
	return out
}

func buildPairSignals(ctx *Context, candidates []candidatePayload) (map[string]*pairSignal, []pairSignal) {
	anchors := make(map[string]*pairSignal)
	if ctx == nil || len(candidates) < 2 {
		return anchors, nil
	}
	all := make([]pairSignal, 0, len(candidates)*2)
	seenPairs := make(map[string]pairSignal)
	for i := 0; i < len(candidates); i++ {
		for j := 0; j < len(candidates); j++ {
			if i == j {
				continue
			}
			pair, ok := buildPairSignal(ctx, candidates[i], candidates[j])
			if !ok {
				continue
			}
			all = append(all, pair)
			if current, exists := anchors[pair.Symbol]; !exists || pair.Score > current.Score {
				cp := pair
				anchors[pair.Symbol] = &cp
			}
			key := canonicalPairKey(pair.Symbol, pair.HedgeSymbol)
			if existing, exists := seenPairs[key]; !exists || pair.Score > existing.Score {
				seenPairs[key] = pair
			}
		}
	}
	if len(seenPairs) == 0 {
		return anchors, nil
	}
	topPairs := make([]pairSignal, 0, len(seenPairs))
	for _, pair := range seenPairs {
		topPairs = append(topPairs, pair)
	}
	sort.SliceStable(topPairs, func(i, j int) bool {
		if topPairs[i].Score == topPairs[j].Score {
			return topPairs[i].Symbol < topPairs[j].Symbol
		}
		return topPairs[i].Score > topPairs[j].Score
	})
	if len(topPairs) > 3 {
		topPairs = topPairs[:3]
	}
	return anchors, topPairs
}

func buildPairSignal(ctx *Context, base, hedge candidatePayload) (pairSignal, bool) {
	baseData := ctx.MarketDataMap[base.Symbol]
	hedgeData := ctx.MarketDataMap[hedge.Symbol]
	tf, baseSeries, hedgeSeries, barHours, ok := selectPairSeries(baseData, hedgeData)
	if !ok {
		return pairSignal{}, false
	}
	baseLogs, ok := toLogSeries(baseSeries)
	if !ok {
		return pairSignal{}, false
	}
	hedgeLogs, ok := toLogSeries(hedgeSeries)
	if !ok {
		return pairSignal{}, false
	}
	hedgeRatio, intercept, ok := olsSlopeIntercept(hedgeLogs, baseLogs)
	if !ok {
		return pairSignal{}, false
	}
	residuals := make([]float64, 0, len(baseLogs))
	for i := range baseLogs {
		residuals = append(residuals, baseLogs[i]-(intercept+hedgeRatio*hedgeLogs[i]))
	}
	residualBps, residualZ, ok := residualMetrics(residuals)
	if !ok {
		return pairSignal{}, false
	}
	corr := returnsCorrelation(baseLogs, hedgeLogs, 24)
	halfLife := halfLifeHours(residuals, barHours)
	confidence := pairConfidence(residualZ, corr, len(residuals), halfLife)
	costBps := pairRoundtripCostBps(ctx, base.Symbol, hedge.Symbol)
	netAfterCost := residualNetAfterCost(residualBps, costBps)
	score := pairSignalScore(residualBps, netAfterCost, confidence)
	return pairSignal{
		Symbol:                    base.Symbol,
		HedgeSymbol:               hedge.Symbol,
		TF:                        tf,
		F13ResidualBps:            residualBps,
		F13ResidualZ:              residualZ,
		F13HalfLifeH:              halfLife,
		F13SuggestedLeg:           suggestedPairLeg(residualZ),
		F14HedgeRatio:             roundedFinitePtr(hedgeRatio, 4),
		F14ReturnsCorr24h:         corr,
		F14Confidence:             confidence,
		F15PairRoundtrip100USDBps: costBps,
		F15NetAfterCostBps:        netAfterCost,
		Score:                     score,
	}, true
}

func selectPairSeries(baseData, hedgeData *market.Data) (string, []float64, []float64, float64, bool) {
	if baseData == nil || hedgeData == nil {
		return "", nil, nil, 0, false
	}
	preferred := []string{"1h", "4h", "15m", "5m"}
	for _, tf := range preferred {
		baseSeries := timeframeCloseSeries(baseData, tf)
		hedgeSeries := timeframeCloseSeries(hedgeData, tf)
		n := minInt(len(baseSeries), len(hedgeSeries))
		if n < 24 {
			continue
		}
		if n > 72 {
			n = 72
		}
		baseSeries = append([]float64(nil), baseSeries[len(baseSeries)-n:]...)
		hedgeSeries = append([]float64(nil), hedgeSeries[len(hedgeSeries)-n:]...)
		return tf, baseSeries, hedgeSeries, timeframeHours(tf), true
	}
	return "", nil, nil, 0, false
}

func timeframeCloseSeries(data *market.Data, tf string) []float64 {
	if data == nil || data.TimeframeData == nil {
		return nil
	}
	tfData := data.TimeframeData[tf]
	if tfData == nil {
		return nil
	}
	if len(tfData.Klines) > 0 {
		out := make([]float64, 0, len(tfData.Klines))
		for _, bar := range tfData.Klines {
			if bar.Close <= 0 || math.IsNaN(bar.Close) || math.IsInf(bar.Close, 0) {
				continue
			}
			out = append(out, bar.Close)
		}
		return out
	}
	out := make([]float64, 0, len(tfData.MidPrices))
	for _, price := range tfData.MidPrices {
		if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			continue
		}
		out = append(out, price)
	}
	return out
}

func timeframeHours(tf string) float64 {
	switch strings.ToLower(strings.TrimSpace(tf)) {
	case "1m":
		return 1.0 / 60.0
	case "3m":
		return 3.0 / 60.0
	case "5m":
		return 5.0 / 60.0
	case "15m":
		return 15.0 / 60.0
	case "1h":
		return 1
	case "2h":
		return 2
	case "4h":
		return 4
	case "1d":
		return 24
	default:
		return 1
	}
}

func toLogSeries(series []float64) ([]float64, bool) {
	if len(series) < 2 {
		return nil, false
	}
	out := make([]float64, 0, len(series))
	for _, value := range series {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}
		out = append(out, math.Log(value))
	}
	return out, true
}

func olsSlopeIntercept(x, y []float64) (float64, float64, bool) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0, false
	}
	meanX := mean(x)
	meanY := mean(y)
	varXX := 0.0
	covXY := 0.0
	for i := range x {
		dx := x[i] - meanX
		dy := y[i] - meanY
		varXX += dx * dx
		covXY += dx * dy
	}
	if varXX == 0 {
		return 0, 0, false
	}
	beta := covXY / varXX
	alpha := meanY - beta*meanX
	if math.IsNaN(beta) || math.IsInf(beta, 0) || math.IsNaN(alpha) || math.IsInf(alpha, 0) {
		return 0, 0, false
	}
	return beta, alpha, true
}

func residualMetrics(residuals []float64) (*float64, *float64, bool) {
	if len(residuals) < 24 {
		return nil, nil, false
	}
	meanResidual := mean(residuals)
	stdResidual := stddev(residuals, meanResidual)
	if stdResidual <= 0 || math.IsNaN(stdResidual) || math.IsInf(stdResidual, 0) {
		return nil, nil, false
	}
	last := residuals[len(residuals)-1]
	bps := (last - meanResidual) * 10_000
	z := (last - meanResidual) / stdResidual
	return roundedFinitePtr(bps, 3), roundedFinitePtr(z, 3), true
}

func returnsCorrelation(a, b []float64, maxLookback int) *float64 {
	if len(a) != len(b) || len(a) < 3 {
		return nil
	}
	returnsA := make([]float64, 0, len(a)-1)
	returnsB := make([]float64, 0, len(b)-1)
	for i := 1; i < len(a); i++ {
		returnsA = append(returnsA, a[i]-a[i-1])
		returnsB = append(returnsB, b[i]-b[i-1])
	}
	if maxLookback > 0 && len(returnsA) > maxLookback {
		returnsA = returnsA[len(returnsA)-maxLookback:]
		returnsB = returnsB[len(returnsB)-maxLookback:]
	}
	corr := correlation(returnsA, returnsB)
	if corr == nil {
		return nil
	}
	return roundedFinitePtr(*corr, 3)
}

func halfLifeHours(residuals []float64, barHours float64) *float64 {
	if len(residuals) < 10 || barHours <= 0 {
		return nil
	}
	lagged := make([]float64, 0, len(residuals)-1)
	delta := make([]float64, 0, len(residuals)-1)
	for i := 1; i < len(residuals); i++ {
		lagged = append(lagged, residuals[i-1])
		delta = append(delta, residuals[i]-residuals[i-1])
	}
	beta, _, ok := olsSlopeIntercept(lagged, delta)
	if !ok || beta >= 0 {
		return nil
	}
	halfLife := -math.Log(2) / beta * barHours
	if halfLife <= 0 || math.IsNaN(halfLife) || math.IsInf(halfLife, 0) || halfLife > 24*14 {
		return nil
	}
	return roundedFinitePtr(halfLife, 2)
}

func pairConfidence(residualZ, corr *float64, observations int, halfLife *float64) *float64 {
	confidence := 0.0
	if residualZ != nil {
		confidence += 0.35 * clampFloat(math.Abs(*residualZ)/3, 0, 1)
	}
	if corr != nil {
		confidence += 0.40 * clampFloat(math.Abs(*corr), 0, 1)
	}
	confidence += 0.25 * clampFloat(float64(observations)/72, 0, 1)
	if halfLife != nil {
		switch {
		case *halfLife <= 12:
			confidence = clampFloat(confidence+0.08, 0, 1)
		case *halfLife > 48:
			confidence = clampFloat(confidence-0.10, 0, 1)
		}
	}
	return roundedFinitePtr(confidence, 3)
}

func pairRoundtripCostBps(ctx *Context, symbol, hedgeSymbol string) *float64 {
	baseCost := roundtrip100CostBps(ctx, symbol)
	hedgeCost := roundtrip100CostBps(ctx, hedgeSymbol)
	if baseCost == nil && hedgeCost == nil {
		return nil
	}
	total := 0.0
	if baseCost != nil {
		total += *baseCost
	}
	if hedgeCost != nil {
		total += *hedgeCost
	}
	return roundedFinitePtr(total, 3)
}

func roundtrip100CostBps(ctx *Context, symbol string) *float64 {
	if ctx == nil || ctx.ExecutionQuality == nil {
		return nil
	}
	exec := buildExecutionQualityPayload(ctx.ExecutionQuality[strings.ToUpper(symbol)])
	if exec == nil {
		return nil
	}
	fee := buildFeeEdgeSignal(ctx, exec)
	if fee == nil {
		return nil
	}
	return fee.Roundtrip100USDBps
}

func residualNetAfterCost(residualBps, costBps *float64) *float64 {
	if residualBps == nil {
		return nil
	}
	gross := math.Abs(*residualBps)
	if costBps == nil {
		return roundedFinitePtr(gross, 3)
	}
	return roundedFinitePtr(gross-*costBps, 3)
}

func pairSignalScore(residualBps, netAfterCost, confidence *float64) float64 {
	base := 0.0
	if netAfterCost != nil {
		base = math.Abs(*netAfterCost)
	} else if residualBps != nil {
		base = math.Abs(*residualBps)
	}
	conf := 0.5
	if confidence != nil {
		conf = clampFloat(*confidence, 0, 1)
	}
	return roundTo(base*conf, 3)
}

func suggestedPairLeg(residualZ *float64) string {
	if residualZ == nil {
		return ""
	}
	if *residualZ > 0 {
		return "short_sym_long_hedge"
	}
	if *residualZ < 0 {
		return "long_sym_short_hedge"
	}
	return "neutral"
}

func feeScheduleForExchange(exchange string) *feeSchedulePayload {
	switch strings.ToLower(strings.TrimSpace(exchange)) {
	case "bybit":
		return &feeSchedulePayload{
			MakerBps: 2.0,
			TakerBps: 5.5,
			Source:   "bybit_derivatives_vip0_2026-04-11",
		}
	default:
		return nil
	}
}

func normalizeExchange(ctx *Context) string {
	if ctx == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(ctx.Exchange))
}

func canonicalPairKey(a, b string) string {
	a = strings.ToUpper(strings.TrimSpace(a))
	b = strings.ToUpper(strings.TrimSpace(b))
	if a > b {
		a, b = b, a
	}
	return a + ":" + b
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func stddev(values []float64, meanValue float64) float64 {
	if len(values) < 2 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		diff := value - meanValue
		total += diff * diff
	}
	return math.Sqrt(total / float64(len(values)-1))
}

func correlation(a, b []float64) *float64 {
	if len(a) != len(b) || len(a) < 2 {
		return nil
	}
	meanA := mean(a)
	meanB := mean(b)
	varAB := 0.0
	varA := 0.0
	varB := 0.0
	for i := range a {
		da := a[i] - meanA
		db := b[i] - meanB
		varAB += da * db
		varA += da * da
		varB += db * db
	}
	if varA == 0 || varB == 0 {
		return nil
	}
	corr := varAB / math.Sqrt(varA*varB)
	if math.IsNaN(corr) || math.IsInf(corr, 0) {
		return nil
	}
	return &corr
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func buildRecentTradesPayload(trades []RecentTrade) []recentTradePayload {
	if len(trades) == 0 {
		return nil
	}
	limit := len(trades)
	if limit > defaultRecentTradesLimit {
		limit = defaultRecentTradesLimit
	}
	out := make([]recentTradePayload, 0, limit)
	for _, trade := range trades[:limit] {
		out = append(out, recentTradePayload{
			Symbol:       strings.ToUpper(strings.TrimSpace(trade.Symbol)),
			Side:         normalizeSide(trade.Side),
			PnLPct:       roundTo(trade.PnLPct, 2),
			HoldDuration: strings.TrimSpace(trade.HoldDuration),
		})
	}
	return out
}

func buildRecentExecutionRegimePayload(regime *RecentExecutionRegime) *recentExecutionRegimePayload {
	if regime == nil || regime.TradeCount <= 0 {
		return nil
	}
	return &recentExecutionRegimePayload{
		TradeCount:         regime.TradeCount,
		WinRatePct:         roundTo(regime.WinRatePct, 2),
		AvgPnLPct:          roundTo(regime.AvgPnLPct, 2),
		ConsecutiveLosses:  regime.ConsecutiveLosses,
		FollowThroughState: strings.TrimSpace(strings.ToLower(regime.FollowThroughState)),
		ChurnRisk:          strings.TrimSpace(strings.ToLower(regime.ChurnRisk)),
	}
}

func buildExecutionQualityPayload(summary *ExecutionQualitySummary) *executionQualityPayload {
	if summary == nil {
		return nil
	}
	out := &executionQualityPayload{
		SpreadBps:         roundedCopy(summary.SpreadBps, 3),
		LiqScore:          roundedCopy(summary.LiqScore, 3),
		DepthBidUSD1Pct:   roundedCopy(summary.DepthBidUSD1Pct, 3),
		DepthAskUSD1Pct:   roundedCopy(summary.DepthAskUSD1Pct, 3),
		BookImbalance1Pct: roundedCopy(summary.BookImbalance1Pct, 3),
		SlippageEst25USD:  roundedCopy(summary.SlippageEst25USD, 3),
		SlippageEst100USD: roundedCopy(summary.SlippageEst100USD, 3),
	}
	if out.SpreadBps == nil &&
		out.LiqScore == nil &&
		out.DepthBidUSD1Pct == nil &&
		out.DepthAskUSD1Pct == nil &&
		out.BookImbalance1Pct == nil &&
		out.SlippageEst25USD == nil &&
		out.SlippageEst100USD == nil {
		return nil
	}
	return out
}

func buildVenueTradabilityPayload(summary *VenueTradabilitySummary) *venueTradabilityPayload {
	if summary == nil {
		return nil
	}
	book := strings.TrimSpace(strings.ToLower(summary.OrderBookState))
	if book == "" {
		book = "unknown"
	}
	out := &venueTradabilityPayload{
		Supported:     summary.VenueSupported,
		Book:          book,
		MinNotionalOK: summary.MinNotionalOK,
	}
	if src := strings.TrimSpace(strings.ToLower(summary.PriceSource)); src != "" {
		out.PriceSource = &src
	}
	return out
}

func buildVolumeParticipationPayload(ctx *Context, data *market.Data, requiredTF string) *volumeParticipationPayload {
	primaryTFData, primaryTF := resolvePrimaryTimeframeData(ctx, data, requiredTF)
	if primaryTFData == nil || primaryTF == "" {
		return nil
	}

	volumes := timeframeVolumes(primaryTFData)
	volRatio5 := rollingVolumeRatio(volumes, 5)
	volRatio20 := rollingVolumeRatio(volumes, 20)
	volTrend := rollingVolumeTrend(volumes)

	confirmTFData, confirmTF := resolveConfirmTimeframeData(ctx, data, primaryTF)
	var confirmVolRatio20 *float64
	if confirmTFData != nil {
		confirmVolRatio20 = rollingVolumeRatio(timeframeVolumes(confirmTFData), 20)
	}

	if volRatio5 == nil && volRatio20 == nil && volTrend == nil && confirmVolRatio20 == nil {
		return nil
	}

	out := &volumeParticipationPayload{
		TF:                primaryTF,
		VolRatio5:         volRatio5,
		VolRatio20:        volRatio20,
		ConfirmVolRatio20: confirmVolRatio20,
		VolTrend:          volTrend,
	}
	if confirmTF != "" {
		out.ConfirmTF = &confirmTF
	}

	if confirmed := breakoutVolumeConfirmed(primaryTFData, confirmTFData, volRatio20, confirmVolRatio20); confirmed != nil {
		out.BreakoutVolumeConfirmed = confirmed
	}

	return out
}

func buildFeatureAvailabilityPayload(
	ctx *Context,
	data *market.Data,
	venue *VenueTradabilitySummary,
	execution *executionQualityPayload,
	volume *volumeParticipationPayload,
	symbolMemory *symbolMemoryPayload,
	staleS int,
	requiredTF string,
) *featureAvailabilityPayload {
	return &featureAvailabilityPayload{
		F4:        featureGateState(data, market.FeatureKeyF4, featureEnabled(ctx, market.FeatureKeyF4)),
		F5:        featureGateState(data, market.FeatureKeyF5, featureEnabled(ctx, market.FeatureKeyF5)),
		F6:        featureGateState(data, market.FeatureKeyF6, featureEnabled(ctx, market.FeatureKeyF6)),
		F7:        featureGateState(data, market.FeatureKeyF7, featureEnabled(ctx, market.FeatureKeyF7)),
		F10:       executionAvailabilityState(venue, execution),
		F11:       volumeAvailabilityState(data, volume, requiredTF),
		F12:       symbolMemoryAvailabilityState(symbolMemory),
		Freshness: freshnessBucket(staleS),
	}
}

func buildSymbolMemoryPayload(trades []RecentTrade, symbol string) *symbolMemoryPayload {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" || len(trades) == 0 {
		return nil
	}

	sameSymbolTrades := make([]RecentTrade, 0, len(trades))
	for _, trade := range trades {
		if strings.ToUpper(strings.TrimSpace(trade.Symbol)) != symbol {
			continue
		}
		sameSymbolTrades = append(sameSymbolTrades, trade)
	}
	if len(sameSymbolTrades) == 0 {
		return nil
	}

	latest := sameSymbolTrades[0]
	out := &symbolMemoryPayload{
		LastSide:             normalizeSide(latest.Side),
		SameSymbolLossStreak: sameSymbolLossStreak(sameSymbolTrades),
	}
	if latest.PnLPct != 0 {
		v := roundTo(latest.PnLPct, 2)
		out.LastPnLPct = &v
	}
	if latest.ExitTimestamp > 0 {
		minutes := int(maxFloat(0, float64(time.Now().Unix()-latest.ExitTimestamp)/60))
		out.MinutesSinceClose = &minutes
		out.CooldownHit = shouldHitSymbolCooldown(minutes, latest.PnLPct, out.SameSymbolLossStreak)
	}
	return out
}

func buildRelativeStrengthPayload(ctx *Context, symbol string, data *market.Data, requiredTF string) *relativeStrengthPayload {
	if ctx == nil || data == nil {
		return nil
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" || symbol == "BTCUSDT" {
		return nil
	}
	btcData := ctx.MarketDataMap["BTCUSDT"]
	if btcData == nil {
		return nil
	}

	out := &relativeStrengthPayload{
		TF: strings.TrimSpace(strings.ToLower(requiredTF)),
	}
	if out.TF == "" {
		out.TF = resolveRequiredTF(ctx)
	}
	out.VsBTC1h = roundedFinitePtr(data.PriceChange1h-btcData.PriceChange1h, 3)
	out.VsBTC4h = roundedFinitePtr(data.PriceChange4h-btcData.PriceChange4h, 3)
	out.VsBTCCtx = timeframeRelativeStrength(data, btcData, out.TF)
	out.State = relativeStrengthState(out.VsBTC1h, out.VsBTC4h, out.VsBTCCtx)
	if out.VsBTC1h == nil && out.VsBTC4h == nil && out.VsBTCCtx == nil && out.State == "" {
		return nil
	}
	return out
}

func buildQuantFlowPayload(summary *QuantFlowSummary) *quantFlowPayload {
	if summary == nil {
		return nil
	}
	out := &quantFlowPayload{
		InstFuture15m:     roundedCopy(summary.InstFuture15m, 3),
		InstFuture1h:      roundedCopy(summary.InstFuture1h, 3),
		InstSpot1h:        roundedCopy(summary.InstSpot1h, 3),
		RetailFuture15m:   roundedCopy(summary.RetailFuture15m, 3),
		OIDelta15mPct:     roundedCopy(summary.OIDelta15mPct, 3),
		OIDelta1hPct:      roundedCopy(summary.OIDelta1hPct, 3),
		PriceChange15mPct: roundedCopy(summary.PriceChange15mPct, 3),
		PriceChange1hPct:  roundedCopy(summary.PriceChange1hPct, 3),
	}
	if out.InstFuture15m == nil &&
		out.InstFuture1h == nil &&
		out.InstSpot1h == nil &&
		out.RetailFuture15m == nil &&
		out.OIDelta15mPct == nil &&
		out.OIDelta1hPct == nil &&
		out.PriceChange15mPct == nil &&
		out.PriceChange1hPct == nil {
		return nil
	}
	return out
}

func executionSpreadBps(summary *executionQualityPayload) *float64 {
	if summary == nil {
		return nil
	}
	return summary.SpreadBps
}

func executionLiqScore(summary *executionQualityPayload) *float64 {
	if summary == nil {
		return nil
	}
	return summary.LiqScore
}

func buildOILeadershipPayload(items []OILeadershipItem) []oiLeadershipPayload {
	if len(items) == 0 {
		return nil
	}
	out := make([]oiLeadershipPayload, 0, len(items))
	for _, item := range items {
		out = append(out, oiLeadershipPayload{
			Symbol:       strings.ToUpper(strings.TrimSpace(item.Symbol)),
			OIDelta1hPct: roundTo(item.OIDelta1hPct, 3),
			PriceChgPct:  roundTo(item.PriceChgPct, 3),
		})
	}
	return out
}

func buildFlowLeadershipPayload(items []FlowLeadershipItem) []flowLeadershipPayload {
	if len(items) == 0 {
		return nil
	}
	out := make([]flowLeadershipPayload, 0, len(items))
	for _, item := range items {
		out = append(out, flowLeadershipPayload{
			Symbol: strings.ToUpper(strings.TrimSpace(item.Symbol)),
			Amount: roundTo(item.Amount, 3),
		})
	}
	return out
}

func buildPriceLeadershipPayload(items []PriceLeadershipItem) []priceLeadershipPayload {
	if len(items) == 0 {
		return nil
	}
	out := make([]priceLeadershipPayload, 0, len(items))
	for _, item := range items {
		out = append(out, priceLeadershipPayload{
			Symbol: strings.ToUpper(strings.TrimSpace(item.Symbol)),
			ChgPct: roundTo(item.ChgPct, 3),
		})
	}
	return out
}

func buildTimeframeSummaries(ctx *Context, data *market.Data) []timeframeSummaryData {
	if data == nil || len(data.TimeframeData) == 0 {
		return nil
	}
	emaFastPeriod, emaSlowPeriod := resolveEMAPair(ctx)
	rsiPeriod := resolveRSIPeriod(ctx)
	keys := selectTimeframeKeys(ctx, data)
	if len(keys) == 0 {
		return nil
	}
	out := make([]timeframeSummaryData, 0, len(keys))
	for _, tf := range keys {
		tfData := data.TimeframeData[tf]
		if tfData == nil {
			continue
		}
		closePx, ok := timeframeClose(tfData)
		if !ok {
			continue
		}
		summary := timeframeSummaryData{
			TF:            tf,
			Close:         roundTo(closePx, 6),
			EMAFastPeriod: emaFastPeriod,
			EMAFast:       timeframeEMAValue(tfData, emaFastPeriod),
			EMASlowPeriod: emaSlowPeriod,
			EMASlow:       timeframeEMAValue(tfData, emaSlowPeriod),
			EMA20:         timeframeEMAValue(tfData, 20),
			EMA50:         timeframeEMAValue(tfData, 50),
			MACD:          roundedSeriesLastPtr(tfData.MACDValues, 3),
			RSIPeriod:     rsiPeriod,
			RSI:           timeframeRSIValue(tfData, rsiPeriod),
			RSI7:          roundedSeriesLastPtr(tfData.RSI7Values, 3),
			RSI14:         roundedSeriesLastPtr(tfData.RSI14Values, 3),
			ATR14:         roundedFinitePtr(tfData.ATR14, 3),
			BOLLUpper:     timeframeBOLLValue(tfData.BOLLUpper),
			BOLLMiddle:    timeframeBOLLValue(tfData.BOLLMiddle),
			BOLLLower:     timeframeBOLLValue(tfData.BOLLLower),
		}
		if chg := timeframeChangePct(tfData); chg != nil {
			summary.ChgPct = chg
		}
		out = append(out, summary)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildContextBlock(decisionCtx *Context, data *market.Data) contextBlock {
	emaFastPeriod, emaSlowPeriod := resolveEMAPair(decisionCtx)
	rsiPeriod := resolveRSIPeriod(decisionCtx)
	block := contextBlock{
		TF:            defaultContextTF,
		PxType:        defaultContextPriceType,
		EMAFastPeriod: emaFastPeriod,
		EMASlowPeriod: emaSlowPeriod,
		RSIPeriod:     rsiPeriod,
		Source: contextSources{
			OI:    "unknown",
			Fund:  "unknown",
			Basis: "unknown",
		},
	}
	if data == nil {
		return block
	}

	if tf := strings.TrimSpace(strings.ToLower(resolveRequiredTF(decisionCtx))); tf != "" {
		block.TF = tf
	}
	if pxType := resolveContextPriceType(decisionCtx); pxType != "" {
		block.PxType = pxType
	}

	if tfData, _ := resolvePrimaryTimeframeData(decisionCtx, data, block.TF); tfData != nil {
		block.EMAFast = timeframeEMAValue(tfData, emaFastPeriod)
		block.EMASlow = timeframeEMAValue(tfData, emaSlowPeriod)
		block.EMA20 = timeframeEMAValue(tfData, 20)
		block.RSI = timeframeRSIValue(tfData, rsiPeriod)
		block.RSI7 = timeframeRSIValue(tfData, 7)
		block.RSI14 = timeframeRSIValue(tfData, 14)
		block.BOLLUpper = timeframeBOLLValue(tfData.BOLLUpper)
		block.BOLLMiddle = timeframeBOLLValue(tfData.BOLLMiddle)
		block.BOLLLower = timeframeBOLLValue(tfData.BOLLLower)
	}
	if block.EMAFast == nil && emaFastPeriod == 20 && data.CurrentEMA20 != 0 {
		ema := roundTo(data.CurrentEMA20, 3)
		block.EMAFast = &ema
	}
	if block.EMA20 == nil && data.CurrentEMA20 != 0 {
		ema := roundTo(data.CurrentEMA20, 3)
		block.EMA20 = &ema
	}

	macd := roundTo(data.CurrentMACD, 3)
	block.MACD = &macd
	if block.RSI7 == nil && data.CurrentRSI7 != 0 {
		rsi7 := roundTo(clampFloat(data.CurrentRSI7, 0, 100), 3)
		block.RSI7 = &rsi7
	}
	if block.RSI == nil && rsiPeriod == 7 && block.RSI7 != nil {
		block.RSI = roundedCopy(block.RSI7, 3)
	}
	block.PriceChange1h = roundedFinitePtr(data.PriceChange1h, 3)
	block.PriceChange4h = roundedFinitePtr(data.PriceChange4h, 3)

	if data.OpenInterest != nil && data.OpenInterest.Latest > 0 {
		oi := roundTo(data.OpenInterest.Latest, 3)
		block.OI = &oi
	}

	derivs := microDerivs(data)
	if derivs != nil {
		if derivs.OIDelta1hPct != nil {
			v := roundTo(*derivs.OIDelta1hPct*100, 3)
			block.OIDelta1hPct = &v
		}
		if derivs.OIPriceDiv != nil {
			s := safeString(derivs.OIPriceDiv)
			if s != "" {
				block.OIPriceDiv = &s
			}
		}
		if derivs.OIPriceCorr24h != nil {
			v := roundTo(*derivs.OIPriceCorr24h, 3)
			block.OIPriceCorr24h = &v
		}
		if derivs.OIZ7d != nil {
			v := roundTo(*derivs.OIZ7d, 3)
			block.OIZ7d = &v
		}
		if derivs.FundingLatestBps != nil {
			v := roundTo(*derivs.FundingLatestBps, 3)
			block.FundingBps = &v
		} else if data.FundingRate != 0 {
			v := roundTo(data.FundingRate*10_000, 3)
			block.FundingBps = &v
		}
		if derivs.FundingMedianZ7d != nil {
			v := roundTo(*derivs.FundingMedianZ7d, 3)
			block.FundingMedianZ7d = &v
		}
		if derivs.FundingDispersionBps != nil {
			v := roundTo(*derivs.FundingDispersionBps, 3)
			block.FundingDispersionBps = &v
		}
		if derivs.BasisPct != nil {
			v := roundTo(*derivs.BasisPct*100, 3)
			block.BasisPct = &v
		}
		if derivs.BasisZ14d != nil {
			v := roundTo(*derivs.BasisZ14d, 3)
			block.BasisZ14d = &v
		}
		if status := strings.TrimSpace(derivs.OISourceStatus); status != "" {
			block.Source.OI = status
		}
		if status := strings.TrimSpace(derivs.FundingSourceStatus); status != "" {
			block.Source.Fund = status
		}
		if status := strings.TrimSpace(derivs.BasisSourceStatus); status != "" {
			block.Source.Basis = status
		}
	}
	return block
}

func normalizeSelectionBucket(bucket string) string {
	switch strings.ToLower(strings.TrimSpace(bucket)) {
	case "primary", "adaptive", "fallback_eligible", "fallback_ranked", "exploration":
		return strings.ToLower(strings.TrimSpace(bucket))
	default:
		return ""
	}
}

func resolveEMAPair(ctx *Context) (int, int) {
	periods := normalizeEMAPeriods(nil)
	if ctx != nil {
		periods = normalizeEMAPeriods(ctx.EMAPeriods)
	}
	fast := periods[0]
	slow := 0
	if len(periods) > 1 {
		slow = periods[1]
	}
	return fast, slow
}

func resolveRSIPeriod(ctx *Context) int {
	periods := normalizeRSIPeriods(nil)
	if ctx != nil {
		periods = normalizeRSIPeriods(ctx.RSIPeriods)
	}
	return periods[0]
}

func normalizeEMAPeriods(periods []int) []int {
	if len(periods) == 0 {
		return []int{20, 50}
	}

	seen := make(map[int]struct{}, len(periods))
	out := make([]int, 0, 2)
	for _, period := range periods {
		if period <= 0 {
			continue
		}
		if _, exists := seen[period]; exists {
			continue
		}
		seen[period] = struct{}{}
		out = append(out, period)
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 0 {
		return []int{20, 50}
	}
	return out
}

func normalizeRSIPeriods(periods []int) []int {
	if len(periods) == 0 {
		return []int{7, 14}
	}
	out := make([]int, 0, len(periods))
	seen := make(map[int]struct{}, len(periods))
	for _, period := range periods {
		if period <= 0 {
			continue
		}
		if _, exists := seen[period]; exists {
			continue
		}
		seen[period] = struct{}{}
		out = append(out, period)
	}
	if len(out) == 0 {
		return []int{7, 14}
	}
	return out
}

func timeframeEMAValue(data *market.TimeframeSeriesData, period int) *float64 {
	if data == nil || period <= 0 {
		return nil
	}

	switch period {
	case 20:
		if v := roundedSeriesLastPtr(data.EMA20Values, 3); v != nil {
			return v
		}
	case 50:
		if v := roundedSeriesLastPtr(data.EMA50Values, 3); v != nil {
			return v
		}
	}

	if len(data.Klines) >= period {
		klines := make([]market.Kline, 0, len(data.Klines))
		for _, bar := range data.Klines {
			klines = append(klines, market.Kline{
				OpenTime:  bar.Time,
				Open:      bar.Open,
				High:      bar.High,
				Low:       bar.Low,
				Close:     bar.Close,
				Volume:    bar.Volume,
				CloseTime: bar.Time,
			})
		}
		ema := market.ExportCalculateEMA(klines, period)
		if ema > 0 && !math.IsNaN(ema) && !math.IsInf(ema, 0) {
			return roundedFinitePtr(ema, 3)
		}
	}

	if ema, ok := calculateEMAFromPrices(data.MidPrices, period); ok {
		return roundedFinitePtr(ema, 3)
	}

	return nil
}

func timeframeRSIValue(data *market.TimeframeSeriesData, period int) *float64 {
	if data == nil || period <= 0 {
		return nil
	}
	switch period {
	case 7:
		if v := roundedSeriesLastPtr(data.RSI7Values, 3); v != nil {
			return v
		}
	case 14:
		if v := roundedSeriesLastPtr(data.RSI14Values, 3); v != nil {
			return v
		}
	}

	if len(data.Klines) >= period+1 {
		klines := make([]market.Kline, 0, len(data.Klines))
		for _, bar := range data.Klines {
			klines = append(klines, market.Kline{
				OpenTime:  bar.Time,
				Open:      bar.Open,
				High:      bar.High,
				Low:       bar.Low,
				Close:     bar.Close,
				Volume:    bar.Volume,
				CloseTime: bar.Time,
			})
		}
		rsi := market.ExportCalculateRSI(klines, period)
		if rsi > 0 {
			return roundedFinitePtr(rsi, 3)
		}
	}

	return nil
}

func timeframeBOLLValue(values []float64) *float64 {
	return roundedSeriesLastPtr(values, 3)
}

func calculateEMAFromPrices(prices []float64, period int) (float64, bool) {
	if len(prices) < period || period <= 0 {
		return 0, false
	}

	sum := 0.0
	for i := 0; i < period; i++ {
		price := prices[i]
		if math.IsNaN(price) || math.IsInf(price, 0) {
			return 0, false
		}
		sum += price
	}
	ema := sum / float64(period)
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(prices); i++ {
		price := prices[i]
		if math.IsNaN(price) || math.IsInf(price, 0) {
			return 0, false
		}
		ema = (price-ema)*multiplier + ema
	}
	return ema, true
}

func buildFeatureBlock(ctx *Context, data *market.Data, diag *payloadDiagnostics, symbol string) featurePayload {
	derivs := microDerivs(data)
	if derivs == nil {
		return featurePayload{}
	}
	features := featurePayload{}
	if f4 := buildOrderflowFeature(ctx, data, derivs, diag, symbol); f4 != nil {
		features.Orderflow = f4
	}
	if f5 := buildRiskFeature(ctx, data, derivs, diag, symbol); f5 != nil {
		features.Risk = f5
	}
	if f6 := buildLevelsFeature(ctx, data, derivs, diag, symbol); f6 != nil {
		features.Levels = f6
	}
	if f7 := buildVolatilityFeature(ctx, data, derivs, diag, symbol); f7 != nil {
		features.Volatility = f7
	}
	return features
}

func buildOrderflowFeature(ctx *Context, data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *orderflowPayload {
	if !featureEnabled(ctx, market.FeatureKeyF4) {
		return nil
	}
	_, pass := buildQoS(data, market.FeatureKeyF4)
	if !pass {
		return nil
	}
	if diag != nil {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF4)
	}
	return &orderflowPayload{
		CVDShort:       clampPtr(derivs.CVDNotionalZ3mShort),
		CVDLong:        clampPtr(derivs.CVDNotionalZ3mLong),
		ImbalanceShort: clampPtr(derivs.ImbNotionalZ3mShort),
		ImbalanceLong:  clampPtr(derivs.ImbNotionalZ3mLong),
		TakerBuyRatio:  clampPtr(derivs.TBRNotional3m),
		SlopePx20:      clampPtr(derivs.SlopePrice3mShort),
		SlopeCVD20:     clampPtr(derivs.SlopeCVDZ3mShort),
		Confidence:     normalizedConfidence(derivs.ConfidenceCVD3m),
	}
}

func buildRiskFeature(ctx *Context, data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *riskPayload {
	if !featureEnabled(ctx, market.FeatureKeyF5) {
		return nil
	}
	_, pass := buildQoS(data, market.FeatureKeyF5)
	if !pass {
		return nil
	}
	if diag != nil {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF5)
	}
	var prefer *string
	if derivs.PreferDirection3m != nil {
		s := strings.ToLower(strings.TrimSpace(*derivs.PreferDirection3m))
		prefer = &s
	}
	return &riskPayload{
		DistUpATR:       positivePtr(derivs.DistUpAtr3m),
		DistDnATR:       positivePtr(derivs.DistDnAtr3m),
		LiqRiskUp:       derivs.LiqRiskUp3m,
		LiqRiskDown:     derivs.LiqRiskDown3m,
		PreferDirection: prefer,
		Confidence:      normalizedConfidence(derivs.ConfidenceLiq3m),
	}
}

func buildLevelsFeature(ctx *Context, data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *levelsPayload {
	if !featureEnabled(ctx, market.FeatureKeyF6) {
		return nil
	}
	_, pass := buildQoS(data, market.FeatureKeyF6)
	if !pass {
		return nil
	}
	if diag != nil {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF6)
	}
	out := &levelsPayload{Confidence: normalizedConfidence(derivs.ConfidenceAVWAP3m)}
	if derivs.AVWAPUpName3m != nil || derivs.AVWAPUpDistAtr3m != nil {
		out.NearUp = &nearLevelPayload{
			Name:    normalizeStringPtr(derivs.AVWAPUpName3m),
			DistATR: positivePtr(derivs.AVWAPUpDistAtr3m),
		}
	}
	if derivs.AVWAPDnName3m != nil || derivs.AVWAPDnDistAtr3m != nil {
		out.NearDn = &nearLevelPayload{
			Name:    normalizeStringPtr(derivs.AVWAPDnName3m),
			DistATR: positivePtr(derivs.AVWAPDnDistAtr3m),
		}
	}
	if derivs.AVWAPBias3m != nil {
		bias := strings.ToLower(strings.TrimSpace(*derivs.AVWAPBias3m))
		out.Bias = &bias
	}
	out.ReclaimUp = intFlagToBool(derivs.AVWAPReclaimUp3m)
	out.RejectionDn = intFlagToBool(derivs.AVWAPRejectionDn3m)
	return out
}

func buildVolatilityFeature(ctx *Context, data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *volatilityPayload {
	if !featureEnabled(ctx, market.FeatureKeyF7) {
		return nil
	}
	_, pass := buildQoS(data, market.FeatureKeyF7)
	if !pass {
		return nil
	}
	if diag != nil {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF7)
	}
	out := &volatilityPayload{
		BBW:            positivePtr(derivs.BBW3m),
		Squeeze:        intFlagToBool(derivs.SqueezeOn3m),
		SqueezeRelease: intFlagToBool(derivs.SqueezeRelease3m),
		RealizedVol:    positivePtr(derivs.RvRatio3m),
		Confidence:     normalizedConfidence(derivs.ConfidenceVol3m),
	}
	if derivs.VolRegime3m != nil {
		regime := strings.ToLower(strings.TrimSpace(*derivs.VolRegime3m))
		out.Regime = &regime
	}
	return out
}

func collectMissingFields(ctx *contextBlock) []string {
	if ctx == nil {
		return append([]string(nil), requiredCandidateContextFields...)
	}
	missing := make([]string, 0)
	if ctx.EMAFast == nil {
		missing = append(missing, "ema_fast")
	}
	if ctx.MACD == nil {
		missing = append(missing, "macd")
	}
	if ctx.RSI == nil {
		missing = append(missing, "rsi")
	}
	if ctx.OI == nil {
		missing = append(missing, "oi")
	}
	if ctx.OIDelta1hPct == nil {
		missing = append(missing, "oi_d1h_pct")
	}
	if ctx.FundingBps == nil {
		missing = append(missing, "fund_bps")
	}
	if ctx.BasisPct == nil {
		missing = append(missing, "basis_pct")
	}
	return missing
}

func buildSymbolQoS(data *market.Data, updateMs int64, missing []string) symbolQoS {
	staleS := deriveStaleSeconds(data, updateMs)
	if missing == nil {
		missing = []string{}
	}
	requiredCount := float64(len(requiredCandidateContextFields))
	coverage := 1.0
	if requiredCount > 0 {
		coverage = clampFloat((requiredCount-float64(len(missing)))/requiredCount, 0, 1)
	}
	return symbolQoS{
		StaleS:   staleS,
		Missing:  missing,
		Coverage: roundTo(coverage, 3),
	}
}

func featureEnabled(ctx *Context, key string) bool {
	if ctx == nil || !ctx.FeatureFlagsSet {
		return true
	}
	switch key {
	case market.FeatureKeyF4:
		return ctx.EnableF4
	case market.FeatureKeyF5:
		return ctx.EnableF5
	case market.FeatureKeyF6:
		return ctx.EnableF6
	case market.FeatureKeyF7:
		return ctx.EnableF7
	default:
		return true
	}
}

func featureGateState(data *market.Data, key string, enabled bool) string {
	if !enabled {
		return "disabled"
	}
	if data == nil || microDerivs(data) == nil {
		return "missing"
	}
	stat, ok := data.FeatureQuality(key)
	if !ok {
		return "missing"
	}
	if stat.Coverage < featureCoverageThreshold {
		return "low_coverage"
	}
	age := featureSLALimitSeconds * 2
	if !stat.UpdatedAt.IsZero() {
		age = int(maxFloat(0, time.Since(stat.UpdatedAt).Seconds()))
	}
	if age > featureSLALimitSeconds {
		return "stale"
	}
	return "ok"
}

func executionAvailabilityState(venue *VenueTradabilitySummary, execution *executionQualityPayload) string {
	if execution != nil {
		return "ok"
	}
	if venue == nil {
		return "not_checked"
	}
	if !venue.VenueSupported {
		return "venue_unsupported"
	}
	switch strings.TrimSpace(strings.ToLower(venue.OrderBookState)) {
	case "ok":
		return "missing"
	case "not_exposed":
		return "orderbook_not_exposed"
	case "empty":
		return "orderbook_empty"
	case "venue_unsupported":
		return "venue_unsupported"
	case "unknown", "":
		return "orderbook_unavailable"
	default:
		return "orderbook_" + strings.TrimSpace(strings.ToLower(venue.OrderBookState))
	}
}

func volumeAvailabilityState(data *market.Data, volume *volumeParticipationPayload, requiredTF string) string {
	if volume != nil {
		return "ok"
	}
	if data == nil || len(data.TimeframeData) == 0 {
		return "missing"
	}
	tfData := data.TimeframeData[strings.TrimSpace(strings.ToLower(requiredTF))]
	if tfData == nil {
		for _, candidate := range data.TimeframeData {
			tfData = candidate
			break
		}
	}
	if tfData == nil {
		return "missing"
	}
	volumes := timeframeVolumes(tfData)
	switch {
	case len(volumes) == 0:
		return "no_volume_data"
	case len(volumes) < 6:
		return "insufficient_history"
	default:
		return "derived_unavailable"
	}
}

func symbolMemoryAvailabilityState(symbolMemory *symbolMemoryPayload) string {
	if symbolMemory != nil {
		return "ok"
	}
	return "no_recent_symbol_trades"
}

func freshnessBucket(staleS int) string {
	switch {
	case staleS <= featureSLALimitSeconds:
		return "fresh"
	case staleS <= requiredMaxStaleSeconds:
		return "warm"
	default:
		return "stale"
	}
}

func deriveStaleSeconds(data *market.Data, updateMs int64) int {
	stale := requiredMaxStaleSeconds * 2
	if data != nil && !data.CollectedAt.IsZero() {
		stale = int(maxFloat(0, time.Since(data.CollectedAt).Seconds()))
	}
	if updateMs > 0 {
		updateStale := int(maxFloat(0, float64(time.Now().UnixMilli()-updateMs)/1000))
		if stale == 0 || updateStale < stale {
			stale = updateStale
		}
	}
	return stale
}

func deriveIndexPrice(pxMark float64, basisPct *float64) *float64 {
	if pxMark <= 0 || basisPct == nil {
		return nil
	}
	denom := 1 + (*basisPct / 100)
	if math.Abs(denom) < 1e-9 {
		return nil
	}
	indexPx := roundTo(pxMark/denom, 6)
	return &indexPx
}

func deriveSideBias(data *market.Data) string {
	if data == nil {
		return "neutral"
	}
	trendDirRaw := data.PriceChange1h + data.PriceChange4h*0.5
	direction := signFloat(trendDirRaw)
	if direction > 0 {
		return "long"
	}
	if direction < 0 {
		return "short"
	}
	return "neutral"
}

func optionalRounded(value float64, decimals int) *float64 {
	if value == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	v := roundTo(value, decimals)
	return &v
}

func roundedFinitePtr(value float64, decimals int) *float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	v := roundTo(value, decimals)
	return &v
}

func roundedCopy(value *float64, decimals int) *float64 {
	if value == nil {
		return nil
	}
	return roundedFinitePtr(*value, decimals)
}

func roundedSeriesLastPtr(values []float64, decimals int) *float64 {
	if len(values) == 0 {
		return nil
	}
	return roundedFinitePtr(values[len(values)-1], decimals)
}

func dataCurrentPrice(data *market.Data) float64 {
	if data == nil {
		return 0
	}
	return data.CurrentPrice
}

func timeframeClose(data *market.TimeframeSeriesData) (float64, bool) {
	if data == nil {
		return 0, false
	}
	if n := len(data.Klines); n > 0 {
		closePx := data.Klines[n-1].Close
		if closePx > 0 && !math.IsNaN(closePx) && !math.IsInf(closePx, 0) {
			return closePx, true
		}
	}
	if n := len(data.MidPrices); n > 0 {
		closePx := data.MidPrices[n-1]
		if closePx > 0 && !math.IsNaN(closePx) && !math.IsInf(closePx, 0) {
			return closePx, true
		}
	}
	return 0, false
}

func timeframeChangePct(data *market.TimeframeSeriesData) *float64 {
	if data == nil {
		return nil
	}
	if n := len(data.Klines); n >= 2 {
		first := data.Klines[0].Close
		last := data.Klines[n-1].Close
		if first > 0 {
			change := ((last - first) / first) * 100
			return roundedFinitePtr(change, 3)
		}
	}
	if n := len(data.MidPrices); n >= 2 {
		first := data.MidPrices[0]
		last := data.MidPrices[n-1]
		if first > 0 {
			change := ((last - first) / first) * 100
			return roundedFinitePtr(change, 3)
		}
	}
	return nil
}

func timeframeRelativeStrength(data *market.Data, btcData *market.Data, tf string) *float64 {
	if data == nil || btcData == nil {
		return nil
	}
	tf = strings.TrimSpace(strings.ToLower(tf))
	if tf == "" {
		return nil
	}
	symbolTF := data.TimeframeData[tf]
	btcTF := btcData.TimeframeData[tf]
	if symbolTF == nil || btcTF == nil {
		return nil
	}
	symbolChange := timeframeChangePct(symbolTF)
	btcChange := timeframeChangePct(btcTF)
	if symbolChange == nil || btcChange == nil {
		return nil
	}
	return roundedFinitePtr(*symbolChange-*btcChange, 3)
}

func relativeStrengthState(vsBTC1h, vsBTC4h, vsBTCCtx *float64) string {
	sum := 0.0
	count := 0.0
	for _, value := range []*float64{vsBTC1h, vsBTC4h, vsBTCCtx} {
		if value == nil {
			continue
		}
		sum += *value
		count++
	}
	if count == 0 {
		return ""
	}
	avg := sum / count
	switch {
	case avg >= 1.0:
		return "outperform"
	case avg <= -1.0:
		return "lagging"
	default:
		return "neutral"
	}
}

func resolvePrimaryTimeframeData(ctx *Context, data *market.Data, requiredTF string) (*market.TimeframeSeriesData, string) {
	if data == nil || len(data.TimeframeData) == 0 {
		return nil, ""
	}
	primaryTF := strings.TrimSpace(strings.ToLower(requiredTF))
	if primaryTF == "" {
		primaryTF = strings.TrimSpace(strings.ToLower(resolveRequiredTF(ctx)))
	}
	if primaryTF != "" {
		if tfData, ok := data.TimeframeData[primaryTF]; ok && tfData != nil {
			return tfData, primaryTF
		}
	}
	keys := selectTimeframeKeys(ctx, data)
	for _, tf := range keys {
		if tfData := data.TimeframeData[tf]; tfData != nil {
			return tfData, tf
		}
	}
	return nil, ""
}

func resolveConfirmTimeframeData(ctx *Context, data *market.Data, primaryTF string) (*market.TimeframeSeriesData, string) {
	if data == nil || len(data.TimeframeData) == 0 {
		return nil, ""
	}
	keys := selectTimeframeKeys(ctx, data)
	primaryMinutes := timeframeMinutes(primaryTF)
	for _, tf := range keys {
		if tf == primaryTF {
			continue
		}
		if primaryMinutes > 0 && timeframeMinutes(tf) < primaryMinutes {
			continue
		}
		if tfData := data.TimeframeData[tf]; tfData != nil {
			return tfData, tf
		}
	}
	for _, tf := range keys {
		if tf == primaryTF {
			continue
		}
		if tfData := data.TimeframeData[tf]; tfData != nil {
			return tfData, tf
		}
	}
	return nil, ""
}

func timeframeVolumes(data *market.TimeframeSeriesData) []float64 {
	if data == nil {
		return nil
	}
	if len(data.Klines) > 0 {
		out := make([]float64, 0, len(data.Klines))
		for _, k := range data.Klines {
			if k.Volume > 0 {
				out = append(out, k.Volume)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if len(data.Volume) > 0 {
		out := make([]float64, 0, len(data.Volume))
		for _, volume := range data.Volume {
			if volume > 0 {
				out = append(out, volume)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func rollingVolumeRatio(volumes []float64, lookback int) *float64 {
	if len(volumes) < 2 || lookback <= 0 {
		return nil
	}
	if lookback > len(volumes)-1 {
		lookback = len(volumes) - 1
	}
	if lookback <= 0 {
		return nil
	}
	avg := averagePositiveVolumes(volumes[len(volumes)-1-lookback : len(volumes)-1])
	last := volumes[len(volumes)-1]
	if avg <= 0 || last <= 0 {
		return nil
	}
	return roundedFinitePtr(last/avg, 3)
}

func rollingVolumeTrend(volumes []float64) *float64 {
	if len(volumes) < 6 {
		return nil
	}
	recent := averagePositiveVolumes(volumes[len(volumes)-3:])
	prior := averagePositiveVolumes(volumes[len(volumes)-6 : len(volumes)-3])
	if recent <= 0 || prior <= 0 {
		return nil
	}
	return roundedFinitePtr(recent/prior, 3)
}

func averagePositiveVolumes(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	count := 0.0
	for _, value := range values {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		sum += value
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / count
}

func breakoutVolumeConfirmed(
	primaryTFData *market.TimeframeSeriesData,
	confirmTFData *market.TimeframeSeriesData,
	volRatio20 *float64,
	confirmVolRatio20 *float64,
) *bool {
	if primaryTFData == nil || volRatio20 == nil {
		return nil
	}
	primaryChange := timeframeChangePct(primaryTFData)
	if primaryChange == nil {
		return nil
	}
	result := false
	if math.Abs(*primaryChange) >= breakoutChangePctMin && *volRatio20 >= breakoutVolumeRatioMin {
		result = true
		if confirmTFData != nil {
			confirmChange := timeframeChangePct(confirmTFData)
			if confirmChange != nil {
				if signFloat(*primaryChange) != 0 && signFloat(*primaryChange) != signFloat(*confirmChange) {
					result = false
				}
			}
			if confirmVolRatio20 != nil && *confirmVolRatio20 < breakoutConfirmRatioMin {
				result = false
			}
		}
	}
	return &result
}

func sameSymbolLossStreak(trades []RecentTrade) int {
	streak := 0
	for _, trade := range trades {
		if trade.PnLPct < 0 {
			streak++
			continue
		}
		break
	}
	return streak
}

func shouldHitSymbolCooldown(minutesSinceClose int, lastPnLPct float64, lossStreak int) bool {
	if minutesSinceClose <= 0 {
		return false
	}
	if minutesSinceClose < symbolCooldownMin {
		return true
	}
	if lastPnLPct < 0 && minutesSinceClose < symbolLossCooldownMin {
		return true
	}
	if lossStreak >= 2 && minutesSinceClose < symbolStreakCooldownMin {
		return true
	}
	return false
}

func selectTimeframeKeys(ctx *Context, data *market.Data) []string {
	if data == nil || len(data.TimeframeData) == 0 {
		return nil
	}
	primary := strings.TrimSpace(strings.ToLower(resolveRequiredTF(ctx)))
	seen := make(map[string]struct{}, len(data.TimeframeData))
	keys := make([]string, 0, len(data.TimeframeData))

	appendIfAvailable := func(tf string) {
		tf = strings.TrimSpace(strings.ToLower(tf))
		if tf == "" {
			return
		}
		if _, ok := data.TimeframeData[tf]; !ok {
			return
		}
		if _, exists := seen[tf]; exists {
			return
		}
		keys = append(keys, tf)
		seen[tf] = struct{}{}
	}

	if primary != "" {
		appendIfAvailable(primary)
	}

	longer := make([]string, 0, len(data.TimeframeData))
	shorter := make([]string, 0, len(data.TimeframeData))
	for tf := range data.TimeframeData {
		tf = strings.TrimSpace(strings.ToLower(tf))
		if tf == "" || tf == primary {
			continue
		}
		switch {
		case timeframeMinutes(tf) >= timeframeMinutes(primary):
			longer = append(longer, tf)
		default:
			shorter = append(shorter, tf)
		}
	}

	sort.SliceStable(longer, func(i, j int) bool {
		return timeframeMinutes(longer[i]) < timeframeMinutes(longer[j])
	})
	sort.SliceStable(shorter, func(i, j int) bool {
		return timeframeMinutes(shorter[i]) > timeframeMinutes(shorter[j])
	})

	for _, tf := range longer {
		appendIfAvailable(tf)
		if len(keys) >= defaultMTFSummaryLimit {
			return keys
		}
	}
	for _, tf := range shorter {
		appendIfAvailable(tf)
		if len(keys) >= defaultMTFSummaryLimit {
			return keys
		}
	}
	return keys
}

func timeframeMinutes(tf string) int {
	switch strings.TrimSpace(strings.ToLower(tf)) {
	case "1m":
		return 1
	case "3m":
		return 3
	case "5m":
		return 5
	case "15m":
		return 15
	case "30m":
		return 30
	case "1h":
		return 60
	case "2h":
		return 120
	case "4h":
		return 240
	case "6h":
		return 360
	case "8h":
		return 480
	case "12h":
		return 720
	case "1d":
		return 1440
	case "3d":
		return 4320
	case "1w":
		return 10080
	default:
		return 0
	}
}

func normalizeSourceTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		out = append(out, tag)
		seen[tag] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	s := strings.ToLower(strings.TrimSpace(*value))
	if s == "" {
		return nil
	}
	return &s
}

func intFlagToBool(value *int) *bool {
	if value == nil {
		return nil
	}
	b := *value != 0
	return &b
}

func normalizeSide(side string) string {
	s := strings.ToLower(strings.TrimSpace(side))
	s = strings.TrimPrefix(s, "open_")
	s = strings.TrimPrefix(s, "close_")
	if s == "long" || s == "short" {
		return s
	}
	return "long"
}

func normalizeMarginUsedRatio(marginUsedPct float64) float64 {
	if marginUsedPct <= 0 {
		return 0
	}
	if marginUsedPct > 1 {
		return marginUsedPct / 100
	}
	return marginUsedPct
}

func normalizeSizeCap(positionRatio float64) float64 {
	switch {
	case math.IsNaN(positionRatio), math.IsInf(positionRatio, 0), positionRatio <= 0:
		return 0.25
	case positionRatio > 1:
		return 1
	default:
		return roundTo(positionRatio, 3)
	}
}

func normalizeMinPositionSize(minPositionSize float64) float64 {
	switch {
	case math.IsNaN(minPositionSize), math.IsInf(minPositionSize, 0), minPositionSize <= 0:
		return 12
	default:
		return roundTo(minPositionSize, 3)
	}
}

func computeSizePctMin(equity float64, minPositionSize float64) float64 {
	if equity <= 0 || minPositionSize <= 0 || math.IsNaN(equity) || math.IsInf(equity, 0) {
		return 0
	}
	return roundTo(clampFloat(minPositionSize/equity, 0, 1), 3)
}

func normalizePayloadMaxLeverage(btcEthLeverage, altcoinLeverage int) int {
	maxLeverage := maxInt(btcEthLeverage, altcoinLeverage)
	if maxLeverage <= 0 {
		return 5
	}
	return maxLeverage
}

func normalizeMaxNewPositions(maxPositions, currentPositions int) int {
	remaining := maxPositions - currentPositions
	if remaining <= 0 {
		return 0
	}
	if remaining > 1 {
		return 1
	}
	return remaining
}

func microDerivs(data *market.Data) *types.DerivsFeatures {
	if data == nil || data.Snapshot == nil || data.Snapshot.Features.Derivs == nil {
		return nil
	}
	return data.Snapshot.Features.Derivs
}

func computeScore(data *market.Data) float64 {
	if data == nil {
		return 0
	}
	trendDirRaw := data.PriceChange1h + data.PriceChange4h*0.5
	direction := signFloat(trendDirRaw)

	trendStrength := clampFloat((math.Abs(data.PriceChange1h)+math.Abs(data.PriceChange4h)*0.6)/8.0, 0, 1)

	dirCoherence := 0.5
	if direction != 0 {
		macdDir := signFloat(data.CurrentMACD)
		if macdDir == direction {
			dirCoherence += 0.25
		} else if macdDir != 0 {
			dirCoherence -= 0.20
		}
		rsi := clampFloat(data.CurrentRSI7, 0, 100)
		if direction > 0 {
			if rsi >= 45 && rsi <= 72 {
				dirCoherence += 0.20
			}
			if rsi > 80 {
				dirCoherence -= 0.20
			}
		} else {
			if rsi >= 28 && rsi <= 55 {
				dirCoherence += 0.20
			}
			if rsi < 20 {
				dirCoherence -= 0.20
			}
		}
	}
	dirCoherence = clampFloat(dirCoherence, 0, 1)

	d := microDerivs(data)
	confScore := derivsConfidenceScore(d)
	flowAlignment := directionalFlowAlignmentScore(direction, d)
	regimeScore := volatilityRegimeScore(d)

	score := 0.22*trendStrength + 0.26*dirCoherence + 0.22*flowAlignment + 0.14*regimeScore + 0.16*confScore
	return clampFloat(score, 0, 1)
}

// computeRank is kept for compatibility with existing tests and callers.
func computeRank(data *market.Data) float64 {
	return computeScore(data)
}

func derivsConfidenceScore(d *types.DerivsFeatures) float64 {
	if d == nil {
		return 0.5
	}
	confComponents := []*float64{d.ConfidenceCVD3m, d.ConfidenceLiq3m, d.ConfidenceAVWAP3m, d.ConfidenceVol3m}
	var sum float64
	var count float64
	for _, c := range confComponents {
		if c == nil {
			continue
		}
		sum += clampFloat(*c, 0, 1)
		count++
	}
	if count == 0 {
		return 0.5
	}
	return clampFloat(sum/count, 0, 1)
}

func directionalFlowAlignmentScore(direction float64, d *types.DerivsFeatures) float64 {
	if d == nil {
		return 0.5
	}
	score := 0.5
	if d.OIPriceDiv != nil {
		switch safeString(d.OIPriceDiv) {
		case "confirming":
			score += 0.10
		case "divergent":
			score -= 0.15
		}
	}
	if direction != 0 {
		if d.OIDelta1hPct != nil {
			oiDir := signFloat(*d.OIDelta1hPct)
			if oiDir == direction {
				score += 0.20
			} else if oiDir != 0 {
				score -= 0.15
			}
		}
		if d.PreferDirection3m != nil {
			pref := safeString(d.PreferDirection3m)
			if direction > 0 {
				if strings.Contains(pref, "long") {
					score += 0.20
				} else if strings.Contains(pref, "short") {
					score -= 0.20
				}
			} else {
				if strings.Contains(pref, "short") {
					score += 0.20
				} else if strings.Contains(pref, "long") {
					score -= 0.20
				}
			}
		}
		if d.AVWAPBias3m != nil {
			bias := safeString(d.AVWAPBias3m)
			if direction > 0 {
				if strings.Contains(bias, "long") || strings.Contains(bias, "bull") {
					score += 0.15
				} else if strings.Contains(bias, "short") || strings.Contains(bias, "bear") {
					score -= 0.15
				}
			} else {
				if strings.Contains(bias, "short") || strings.Contains(bias, "bear") {
					score += 0.15
				} else if strings.Contains(bias, "long") || strings.Contains(bias, "bull") {
					score -= 0.15
				}
			}
		}
	}
	return clampFloat(score, 0, 1)
}

func volatilityRegimeScore(d *types.DerivsFeatures) float64 {
	if d == nil {
		return 0.55
	}
	score := 0.55
	if d.VolRegime3m != nil {
		regime := safeString(d.VolRegime3m)
		switch {
		case strings.Contains(regime, "expansion"), strings.Contains(regime, "trend"):
			score = 0.90
		case strings.Contains(regime, "volatile"):
			score = 0.75
		case strings.Contains(regime, "compression"), strings.Contains(regime, "squeeze"):
			score = 0.35
		case strings.Contains(regime, "normal"), strings.Contains(regime, "neutral"), strings.Contains(regime, "balanced"):
			score = 0.60
		}
	}
	if d.SqueezeOn3m != nil && *d.SqueezeOn3m == 1 {
		score -= 0.08
	}
	if d.SqueezeRelease3m != nil && *d.SqueezeRelease3m == 1 {
		score += 0.08
	}
	if d.RvRatio3m != nil {
		if *d.RvRatio3m > 1.3 {
			score += 0.05
		}
		if *d.RvRatio3m < 0.8 {
			score -= 0.05
		}
	}
	return clampFloat(score, 0, 1)
}

func buildQoS(data *market.Data, key string) (symbolQoS, bool) {
	if data == nil {
		return symbolQoS{StaleS: featureSLALimitSeconds * 2, Missing: []string{}, Coverage: 0}, false
	}
	stat, ok := data.FeatureQuality(key)
	coverage := 0.0
	age := featureSLALimitSeconds * 2
	if ok {
		coverage = clampFloat(stat.Coverage, 0, 1)
		if !stat.UpdatedAt.IsZero() {
			age = int(maxFloat(0, time.Since(stat.UpdatedAt).Seconds()))
		}
	} else if !data.CollectedAt.IsZero() {
		age = int(maxFloat(0, time.Since(data.CollectedAt).Seconds()))
	}
	qos := symbolQoS{
		StaleS:   age,
		Missing:  []string{},
		Coverage: roundTo(coverage, 3),
	}
	pass := coverage >= featureCoverageThreshold && age <= featureSLALimitSeconds
	return qos, pass
}

func validateLivePayload(payload *livePayload) error {
	if payload.Schema != PayloadSchemaVersion {
		return fmt.Errorf("invalid payload schema %s", payload.Schema)
	}
	if payload.TimestampUTC == "" {
		return fmt.Errorf("ts_utc missing")
	}
	if payload.Req.RequiredTF == "" {
		return fmt.Errorf("req.required_tf missing")
	}
	if payload.Req.MaxStaleS <= 0 {
		return fmt.Errorf("req.max_stale_s invalid")
	}
	if len(payload.Req.RequiredCandidateFields) == 0 {
		return fmt.Errorf("req.required_candidate_fields missing")
	}
	if len(payload.Req.RequiredCandidateCtxFields) == 0 {
		return fmt.Errorf("req.required_candidate_ctx_fields missing")
	}
	if math.IsNaN(payload.Account.Equity) || payload.Account.Equity < 0 {
		return fmt.Errorf("account equity invalid")
	}
	if payload.Policy.SizeCap <= 0 || payload.Policy.SizeCap > 1 {
		return fmt.Errorf("policy size_cap invalid")
	}
	if payload.Policy.SizeFloorUSDT < 0 {
		return fmt.Errorf("policy size_floor_usdt invalid")
	}
	if payload.Policy.SizePctMin < 0 || payload.Policy.SizePctMin > payload.Policy.SizeCap {
		return fmt.Errorf("policy size_pct_min invalid")
	}
	if payload.Policy.MinScore < 0 || payload.Policy.MinScore > 1 {
		return fmt.Errorf("policy min_score invalid")
	}
	if payload.OutputContract.Constraints.SizeFloorUSDT < 0 {
		return fmt.Errorf("output_contract.constraints.size_floor_usdt invalid")
	}
	if payload.OutputContract.Constraints.SizePctMin < 0 || payload.OutputContract.Constraints.SizePctMin > payload.OutputContract.Constraints.SizePctMax {
		return fmt.Errorf("output_contract.constraints.size_pct_min invalid")
	}
	if payload.OutputContract.Constraints.DecisionsLenMin < 0 {
		return fmt.Errorf("output_contract.constraints.decisions_len_min invalid")
	}
	if payload.OutputContract.Constraints.DecisionsLenMax < payload.OutputContract.Constraints.DecisionsLenMin {
		return fmt.Errorf("output_contract.constraints.decisions_len_max invalid")
	}
	if err := validateAssets(payload.Positions, payload.Candidates, payload.Req); err != nil {
		return err
	}
	return nil
}

func validateAssets(positions []positionPayload, candidates []candidatePayload, req reqPayload) error {
	for _, pos := range positions {
		if pos.Symbol == "" {
			return fmt.Errorf("position symbol missing")
		}
		if pos.Side != "long" && pos.Side != "short" {
			return fmt.Errorf("position %s side invalid", pos.Symbol)
		}
		if err := validateContext(pos.Context, req.RequiredTF, pos.Symbol, "position"); err != nil {
			return err
		}
		if err := validateSymbolQoS(pos.QoS, pos.Symbol); err != nil {
			return err
		}
	}
	for _, cand := range candidates {
		if cand.Symbol == "" {
			return fmt.Errorf("candidate symbol missing")
		}
		if cand.Score < 0 || cand.Score > 1 {
			return fmt.Errorf("candidate score out of range for %s", cand.Symbol)
		}
		if cand.Confidence < 0 || cand.Confidence > 1 {
			return fmt.Errorf("candidate confidence out of range for %s", cand.Symbol)
		}
		if cand.SideBias != "long" && cand.SideBias != "short" && cand.SideBias != "neutral" {
			return fmt.Errorf("candidate side_bias invalid for %s", cand.Symbol)
		}
		if err := validateContext(cand.Context, req.RequiredTF, cand.Symbol, "candidate"); err != nil {
			return err
		}
		if err := validateSymbolQoS(cand.QoS, cand.Symbol); err != nil {
			return err
		}
	}
	return nil
}

func validateContext(ctx contextBlock, requiredTF, symbol, assetType string) error {
	if requiredTF != "" && ctx.TF != requiredTF {
		return fmt.Errorf("%s %s tf mismatch: got %s want %s", assetType, symbol, ctx.TF, requiredTF)
	}
	if ctx.RSI != nil {
		if *ctx.RSI < 0 || *ctx.RSI > 100 {
			return fmt.Errorf("%s %s rsi out of range", assetType, symbol)
		}
	}
	return nil
}

func validateSymbolQoS(q symbolQoS, symbol string) error {
	if q.Coverage < 0 || q.Coverage > 1 {
		return fmt.Errorf("%s qos coverage out of range", symbol)
	}
	if q.StaleS < 0 {
		return fmt.Errorf("%s qos stale_s negative", symbol)
	}
	return nil
}

func logPayloadDiagnostics(version string, diag *payloadDiagnostics) {
	if diag == nil {
		return
	}
	log.Printf("📦 payload ver=%s bytes=%d runes=%d", version, diag.ByteSize, diag.RuneSize)
	if len(diag.FeaturePass) == 0 {
		log.Printf("📉 QoS: no features passed gates this cycle")
		return
	}
	var entries []string
	for sym, feats := range diag.FeaturePass {
		sort.Strings(feats)
		entries = append(entries, fmt.Sprintf("%s[%s]", sym, strings.Join(feats, ",")))
	}
	sort.Strings(entries)
	log.Printf("✅ QoS pass: %s", strings.Join(entries, " | "))
}

func roundTo(val float64, decimals int) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	factor := math.Pow(10, float64(decimals))
	return math.Round(val*factor) / factor
}

func floatPtr(value float64) *float64 {
	return &value
}

func clampPtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := roundTo(*value, 4)
	return &v
}

func positivePtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := math.Abs(*value)
	return floatPtr(roundTo(v, 3))
}

func normalizedConfidence(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := clampFloat(*value, 0, 1)
	return floatPtr(roundTo(v, 3))
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if math.IsNaN(value) {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(*value))
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func signFloat(value float64) float64 {
	if value > 1e-9 {
		return 1
	}
	if value < -1e-9 {
		return -1
	}
	return 0
}
