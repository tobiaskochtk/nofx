export interface SystemStatus {
  trader_id: string
  trader_name: string
  ai_model: string
  is_running: boolean
  start_time: string
  runtime_minutes: number
  call_count: number
  initial_balance: number
  scan_interval: string
  stop_until: string
  last_reset_time: string
  ai_provider: string
  strategy_type?: 'ai_trading' | 'grid_trading'
  grid_symbol?: string
}

export interface AccountInfo {
  total_equity: number
  wallet_balance: number
  unrealized_profit: number // 未实现盈亏（交易所API官方值）
  available_balance: number
  total_pnl: number
  total_pnl_pct: number
  initial_balance: number
  daily_pnl: number
  position_count: number
  margin_used: number
  margin_used_pct: number
}

export interface Position {
  symbol: string
  side: string
  entry_price: number
  mark_price: number
  quantity: number
  leverage: number
  unrealized_pnl: number
  unrealized_pnl_pct: number
  liquidation_price: number
  margin_used: number
}

export interface DecisionAction {
  action: string
  symbol: string
  quantity: number
  leverage: number
  price: number
  stop_loss?: number // Stop loss price
  take_profit?: number // Take profit price
  confidence?: number // AI confidence (0-100)
  reasoning?: string // Brief reasoning
  order_id: number
  timestamp: string
  success: boolean
  error?: string
}

export interface AccountSnapshot {
  total_balance: number
  available_balance: number
  total_unrealized_profit: number
  position_count: number
  margin_used_pct: number
  initial_balance?: number
}

export interface PositionSnapshot {
  symbol: string
  side: string
  position_amt: number
  entry_price: number
  mark_price: number
  unrealized_profit: number
  leverage: number
  liquidation_price: number
}

export interface CandidateDetail {
  symbol: string
  sources?: string[]
  selection_bucket?: string
}

export interface DecisionRecord {
  id?: number
  trader_id?: string
  timestamp: string
  cycle_number: number
  system_prompt: string
  input_prompt: string
  cot_trace: string
  decision_json: string
  raw_response?: string
  account_state: AccountSnapshot
  positions: PositionSnapshot[]
  candidate_coins: string[]
  candidate_details?: CandidateDetail[]
  candidate_metadata_version?: number
  decisions: DecisionAction[]
  execution_log: string[]
  success: boolean
  error_message?: string
  ai_request_duration_ms?: number
}

export interface Statistics {
  total_cycles: number
  successful_cycles: number
  failed_cycles: number
  total_open_positions: number
  total_close_positions: number
}

export interface TraderAI500BucketSummary {
  name: string
  candidate_count: number
  unique_symbols: number
  open_decision_count: number
  symbol_samples?: string[]
}

export interface TraderAI500BucketCycleBreak {
  name: string
  count: number
  symbols?: string[]
}

export interface TraderAI500BucketCycle {
  cycle_number: number
  timestamp: string
  candidate_count: number
  open_decision_symbols?: string[]
  buckets: TraderAI500BucketCycleBreak[]
}

export interface TraderAI500BucketReview {
  trader_id: string
  window_hours: number
  window_start: string
  window_end: string
  generated_at: string
  candidate_metadata_version: number
  has_full_window: boolean
  coverage_hours: number
  record_count: number
  legacy_record_count: number
  cycles_with_candidates: number
  cycles_without_candidates: number
  cycles_with_open_decisions: number
  total_candidates: number
  total_open_decisions: number
  first_record_at?: string
  last_record_at?: string
  buckets: TraderAI500BucketSummary[]
  recent_cycles: TraderAI500BucketCycle[]
}

export interface DealReviewCase {
  id: string
  user_id: string
  trader_id: string
  position_id: number
  exchange_id: string
  exchange_type: string
  ai_model_id: string
  strategy_id: string
  symbol: string
  side: string
  status: string
  outcome: string
  open_event_id: string
  close_event_id: string
  open_cycle_number: number
  close_cycle_number: number
  entry_order_id: string
  exit_order_id: string
  entry_time_ms: number
  exit_time_ms: number
  entry_price: number
  exit_price: number
  entry_quantity: number
  exit_quantity: number
  leverage: number
  open_stop_loss: number
  open_take_profit: number
  open_confidence: number
  close_confidence: number
  open_selection_bucket: string
  analyst_note: string
  realized_pnl: number
  realized_pnl_pct: number
  fee: number
  hold_duration_ms: number
  close_reason: string
  created_at: string
  updated_at: string
}

export interface DealReviewCaseListItem {
  case: DealReviewCase
  trader_name: string
  strategy_name: string
  open_reasoning: string
  close_reasoning: string
  labels?: string[]
  open_candidate_sources?: string[]
}

export interface DealReviewDatasetSummary {
  total_deals: number
  open_deals: number
  closed_deals: number
  winning_deals: number
  losing_deals: number
  flat_deals: number
  win_rate: number
  net_pnl: number
  avg_pnl: number
  expectancy: number
  avg_pnl_pct: number
  avg_hold_ms: number
  profit_factor: number
  max_drawdown_pct: number
  long_deals: number
  short_deals: number
  long_net_pnl: number
  short_net_pnl: number
}

export interface DealReviewListResponse {
  items: DealReviewCaseListItem[]
  summary: DealReviewDatasetSummary
  total: number
}

export interface DealReviewEventSnapshot {
  account_state?: AccountSnapshot
  positions?: PositionSnapshot[]
  candidate_coins?: string[]
  candidate_details?: CandidateDetail[]
  execution_log?: string[]
  system_prompt?: string
  user_prompt?: string
  decision_json?: string
  raw_response?: string
  cot_trace?: string
  ai_request_duration_ms?: number
}

export interface DealReviewEvent {
  id: string
  user_id: string
  trader_id: string
  deal_id: string
  position_id: number
  stage: string
  source: string
  status: string
  decision_cycle_number: number
  decision_timestamp: string
  exchange_id: string
  exchange_order_id: string
  symbol: string
  side: string
  action: string
  quantity: number
  position_size_usd: number
  price: number
  leverage: number
  stop_loss: number
  take_profit: number
  confidence: number
  reasoning: string
  selection_bucket: string
  outcome_pnl: number
  outcome_pnl_pct: number
  close_reason: string
  created_at: string
  updated_at: string
}

export interface DealReviewEventDetail {
  event: DealReviewEvent
  candidate_sources?: string[]
  snapshot?: DealReviewEventSnapshot
  decision_record?: DecisionRecord
}

export interface DealReviewPriceTimelinePoint {
  source: string
  timestamp_ms: number
  decision_cycle_number: number
  mark_price: number
  entry_price: number
  quantity: number
  unrealized_pnl: number
  unrealized_pnl_pct: number
  in_profit: boolean
}

export interface DealReviewPriceTimelineSummary {
  cycle_samples: number
  point_count: number
  ever_in_profit: boolean
  max_unrealized_pnl: number
  max_unrealized_pnl_pct: number
  min_unrealized_pnl: number
  min_unrealized_pnl_pct: number
  highest_mark_price: number
  lowest_mark_price: number
}

export interface DealReviewPriceTimeline {
  points?: DealReviewPriceTimelinePoint[]
  summary: DealReviewPriceTimelineSummary
}

export interface DealReviewCaseDetail {
  case: DealReviewCase
  trader_name: string
  strategy_name: string
  labels?: string[]
  open_candidate_sources?: string[]
  open?: DealReviewEventDetail
  close?: DealReviewEventDetail
  price_timeline?: DealReviewPriceTimeline
}

export interface DealReviewActionItem {
  title: string
  rationale: string
  expected_impact?: string
  risk?: string
  config_patch?: Record<string, unknown>
}

export interface DealReviewAIScanResult {
  executive_summary: string
  strengths?: string[]
  weaknesses?: string[]
  patterns?: string[]
  immediate_actions?: DealReviewActionItem[]
  experiments?: DealReviewActionItem[]
  strategy_patch?: Record<string, unknown>
}

export interface DealReviewAIScan {
  id: string
  user_id: string
  trader_id: string
  strategy_id: string
  model_config_id: string
  provider: string
  model_name: string
  dataset_count: number
  status: string
  summary: string
  error_message: string
  validation_status: string
  validation_summary: string
  validated_at?: string
  applied_at?: string
  created_at: string
  updated_at: string
}

export interface DealReviewAIScanValidation {
  status: string
  validated_at?: string
  promotion_ready: boolean
  dataset_count: number
  closed_deal_count: number
  training_closed_deal_count: number
  holdout_closed_deal_count: number
  recent_closed_deal_count: number
  min_closed_deal_count: number
  min_holdout_closed_deal_count: number
  min_recent_closed_deal_count: number
  config_valid: boolean
  config_warnings?: string[]
  blocking_issues?: string[]
  checks?: DealReviewValidationCheck[]
  notes?: string[]
  recent_slice_label?: string
  training_summary?: DealReviewDatasetSummary
  holdout_summary?: DealReviewDatasetSummary
  recent_summary?: DealReviewDatasetSummary
  replay?: DealReviewAIScanReplay
}

export interface DealReviewValidationCheck {
  key: string
  label: string
  scope: string
  metric: string
  source?: string
  comparator: string
  threshold: number
  actual: number
  baseline?: number
  delta?: number
  passed: boolean
  blocking: boolean
  message: string
}

export interface DealReviewAIScanReplay {
  supported: boolean
  mode?: string
  supported_paths?: string[]
  unsupported_paths?: string[]
  training_replay_summary?: DealReviewDatasetSummary
  holdout_replay_summary?: DealReviewDatasetSummary
  recent_replay_summary?: DealReviewDatasetSummary
  training_net_pnl_delta: number
  holdout_net_pnl_delta: number
  recent_net_pnl_delta: number
  training_closed_deal_delta: number
  holdout_closed_deal_delta: number
  recent_closed_deal_delta: number
  blocking_issues?: string[]
  notes?: string[]
}

export interface DealReviewAIScanDetail {
  scan: DealReviewAIScan
  filters?: Record<string, unknown>
  result: DealReviewAIScanResult
  strategy_patch?: Record<string, unknown>
  validation?: DealReviewAIScanValidation
}

export interface DealReviewJSONDiffEntry {
  path: string
  left: string
  right: string
}

export interface DealReviewAIScanCompareResponse {
  left: DealReviewAIScanDetail
  right: DealReviewAIScanDetail
  same_filters: boolean
  filter_differences?: DealReviewJSONDiffEntry[]
  patch_differences?: DealReviewJSONDiffEntry[]
  strength_overlap?: string[]
  weakness_overlap?: string[]
  immediate_actions_a?: string[]
  immediate_actions_b?: string[]
}

export interface DealReviewStrategyVersion {
  id: string
  user_id: string
  trader_id: string
  strategy_id: string
  source_scan_id: string
  source_compare_id: string
  source_type: string
  summary: string
  created_at: string
  updated_at: string
}

export interface DealReviewStrategyVersionDetail {
  version: DealReviewStrategyVersion
  previous_config?: Record<string, unknown>
  next_config?: Record<string, unknown>
}

export interface DealReviewChallengerMetrics {
  evaluated_at?: string
  incumbent_pnl: number
  challenger_pnl: number
  incumbent_trade_count: number
  challenger_trade_count: number
  incumbent_fees: number
  challenger_fees: number
  incumbent_avg_hold_ms: number
  challenger_avg_hold_ms: number
}

export interface DealReviewChallengerProtocolEvent {
  timestamp: string
  type: string
  actor?: string
  message: string
  metrics?: DealReviewChallengerMetrics
  data?: Record<string, unknown>
}

export interface DealReviewChallengerCompare {
  id: string
  user_id: string
  trader_id: string
  incumbent_trader_id: string
  incumbent_strategy_id: string
  challenger_trader_id: string
  challenger_strategy_id: string
  challenger_exchange_id: string
  mode: string
  window_hours: number
  extension_count: number
  source_scan_id: string
  source_strategy_version_id: string
  status: string
  winner_trader_id: string
  loser_trader_id: string
  summary: string
  error_message: string
  started_at: string
  ends_at: string
  last_evaluated_at?: string
  resolved_at?: string
  created_at: string
  updated_at: string
}

export interface DealReviewChallengerCompareDetail {
  compare: DealReviewChallengerCompare
  metrics?: DealReviewChallengerMetrics
  protocol?: DealReviewChallengerProtocolEvent[]
  incumbent_trader_name: string
  challenger_trader_name: string
  incumbent_strategy_name: string
  challenger_strategy_name: string
  challenger_exchange_name: string
  source_scan_summary: string
}

export interface DealReviewAnomalySymbol {
  symbol: string
  deals: number
  net_pnl: number
  win_rate: number
  avg_hold_ms: number
  losing_deals: number
}

export interface DealReviewAnomalyBucket {
  bucket: string
  deals: number
  net_pnl: number
  win_rate: number
}

export interface DealReviewAnomalyCloseReason {
  reason: string
  deals: number
  net_pnl: number
  avg_pnl: number
}

export interface DealReviewAnomalySummary {
  closed_deals: number
  worst_symbols?: DealReviewAnomalySymbol[]
  overtraded_symbols?: DealReviewAnomalySymbol[]
  weak_buckets?: DealReviewAnomalyBucket[]
  weak_close_reasons?: DealReviewAnomalyCloseReason[]
  notes?: string[]
}

export interface RemoteModelInfo {
  id: string
  label: string
  provider: string
  available: boolean
}

// AI Trading相关类型
export interface TraderInfo {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange_id?: string
  is_running?: boolean
  startup_warning?: string
  show_in_competition?: boolean
  strategy_id?: string
  strategy_name?: string
  custom_prompt?: string
  use_ai500?: boolean
  use_oi_top?: boolean
  system_prompt_template?: string
}

// Competition related types
export interface CompetitionTraderData {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange: string
  total_equity: number
  total_pnl: number
  total_pnl_pct: number
  position_count: number
  margin_used_pct: number
  is_running: boolean
  realized_pnl?: number
  unrealized_pnl?: number
  total_fees?: number
  closed_trades?: number
}

export interface CompetitionData {
  traders: CompetitionTraderData[]
  count: number
}

// Trader Configuration Data for View Modal
export interface TraderConfigData {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  strategy_id?: string // 策略ID
  strategy_name?: string // 策略名称
  is_cross_margin: boolean
  show_in_competition: boolean // 是否在竞技场显示
  scan_interval_minutes: number
  initial_balance: number
  is_running: boolean
  // 以下为旧版字段（向后兼容）
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  system_prompt_template?: string
  use_ai500?: boolean
  use_oi_top?: boolean
}

// Position History Types
export interface HistoricalPosition {
  id: number
  trader_id: string
  exchange_id: string
  exchange_type: string
  symbol: string
  side: string
  quantity: number
  entry_quantity: number
  entry_price: number
  entry_order_id: string
  entry_time: string
  exit_price: number
  exit_order_id: string
  exit_time: string
  realized_pnl: number
  fee: number
  leverage: number
  status: string
  close_reason: string
  created_at: string
  updated_at: string
}

// Matches Go TraderStats struct exactly
export interface TraderStats {
  total_trades: number
  win_trades: number
  loss_trades: number
  win_rate: number
  profit_factor: number
  sharpe_ratio: number
  total_pnl: number
  total_fee: number
  avg_win: number
  avg_loss: number
  max_drawdown_pct: number
}

// Matches Go SymbolStats struct exactly
export interface SymbolStats {
  symbol: string
  total_trades: number
  win_trades: number
  win_rate: number
  total_pnl: number
  avg_pnl: number
  avg_hold_mins: number
}

// Matches Go DirectionStats struct exactly
export interface DirectionStats {
  side: string
  trade_count: number
  win_rate: number
  total_pnl: number
  avg_pnl: number
}

export interface PositionHistoryResponse {
  positions: HistoricalPosition[]
  stats: TraderStats | null
  symbol_stats: SymbolStats[]
  direction_stats: DirectionStats[]
}

// Grid Risk Information for frontend display
export interface GridRiskInfo {
  // Leverage info
  current_leverage: number
  effective_leverage: number
  recommended_leverage: number

  // Position info
  current_position: number
  max_position: number
  position_percent: number

  // Liquidation info
  liquidation_price: number
  liquidation_distance: number

  // Market state
  regime_level: string

  // Box state
  short_box_upper: number
  short_box_lower: number
  mid_box_upper: number
  mid_box_lower: number
  long_box_upper: number
  long_box_lower: number
  current_price: number

  // Breakout state
  breakout_level: string
  breakout_direction: string
}
