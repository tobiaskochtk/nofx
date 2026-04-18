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
  exchange_order_id?: string
  timestamp: string
  success: boolean
  error?: string
  reject_reasons?: string[]
  market_context?: DealReviewMarketContextSnapshot
  execution?: DecisionExecutionTelemetry
}

export interface DecisionExecutionTelemetry {
  handed_to_execution?: boolean
  order_submitted?: boolean
  exchange_order_id?: string
  terminal_status?: string
  failure_category?: string
  status_detail?: string
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
  reject_reasons?: string[]
  market_context?: DealReviewMarketContextSnapshot
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
  rejected_candidate_count?: number
  first_record_at?: string
  last_record_at?: string
  buckets: TraderAI500BucketSummary[]
  recent_cycles: TraderAI500BucketCycle[]
  reject_reasons?: TraderRejectReason[]
  execution_statuses?: TraderExecutionStatus[]
  recent_open_executions?: TraderOpenExecution[]
  regime_summaries?: TraderRegimeSummary[]
}

export interface TraderRejectReason {
  reason: string
  count: number
  share_pct?: number
}

export interface TraderExecutionStatus {
  status: string
  count: number
  share_pct?: number
}

export interface TraderOpenExecution {
  cycle_number: number
  timestamp: string
  symbol: string
  side?: string
  confidence?: number
  terminal_status: string
  failure_category?: string
  exchange_order_id?: string
  reasoning?: string
  trend_regime?: string
  volatility_regime?: string
  oi_regime?: string
  funding_regime?: string
  session_bucket?: string
  liquidity_tier?: string
  spread_bucket?: string
  slippage_bucket?: string
}

export interface TraderRegimeSummary {
  key: string
  trend_regime?: string
  volatility_regime?: string
  oi_regime?: string
  funding_regime?: string
  liquidity_tier?: string
  session_bucket?: string
  candidate_count: number
  open_decision_count: number
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
  open_trend_regime: string
  open_volatility_regime: string
  open_btc_strength_regime: string
  open_funding_regime: string
  open_oi_regime: string
  open_session_bucket: string
  open_weekday_bucket: string
  open_venue_tier: string
  open_liquidity_tier: string
  open_spread_bucket: string
  open_slippage_bucket: string
  close_trend_regime: string
  close_volatility_regime: string
  close_btc_strength_regime: string
  close_funding_regime: string
  close_oi_regime: string
  close_session_bucket: string
  close_weekday_bucket: string
  close_venue_tier: string
  close_liquidity_tier: string
  close_spread_bucket: string
  close_slippage_bucket: string
  analyst_note: string
  max_favorable_excursion: number
  max_favorable_excursion_pct: number
  max_adverse_excursion: number
  max_adverse_excursion_pct: number
  mfe_captured_pct: number
  profit_given_back: number
  profit_given_back_pct: number
  time_to_first_profit_ms: number
  time_to_max_drawdown_ms: number
  planned_risk_pct: number
  exit_efficiency_score: number
  entry_timing_score: number
  risk_sizing_score: number
  realized_pnl: number
  realized_pnl_pct: number
  fee: number
  hold_duration_ms: number
  close_reason: string
  exit_origin: string
  exit_reason_quality: string
  exit_evidence_summary: string
  exit_evidence?: DealReviewExitEvidence
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
  price_timeline_summary?: DealReviewPriceTimelineSummary
  classifier_assist?: DealReviewClassifierAssist
}

export interface DealReviewFilterPreset {
  id: string
  user_id: string
  trader_id: string
  name: string
  created_at: string
  updated_at: string
}

export interface DealReviewFilterPresetDetail {
  preset: DealReviewFilterPreset
  filters?: Record<string, unknown>
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
  avg_mfe_captured_pct: number
  avg_profit_given_back_pct: number
  avg_exit_efficiency_score: number
  avg_entry_timing_score: number
  avg_risk_sizing_score: number
  bad_entry_deals: number
  bad_exit_deals: number
  avoidable_loss_deals: number
  strong_entry_weak_exit_deals: number
  weak_entry_lucky_exit_deals: number
}

export interface DealReviewListResponse {
  items: DealReviewCaseListItem[]
  summary: DealReviewDatasetSummary
  total: number
}

export interface DealReviewMarketContextSnapshot {
  source?: string
  symbol?: string
  timeframe?: string
  price_type?: string
  price?: number
  trend_regime?: string
  volatility_regime?: string
  btc_strength_regime?: string
  funding_regime?: string
  oi_regime?: string
  session_bucket?: string
  weekday_bucket?: string
  venue_tier?: string
  liquidity_tier?: string
  spread_bucket?: string
  slippage_bucket?: string
  price_change_1h?: number
  price_change_4h?: number
  ema_fast?: number
  macd?: number
  rsi?: number
  oi_delta_1h_pct?: number
  funding_bps?: number
  basis_pct?: number
  btc_relative_strength_1h?: number
  btc_relative_strength_4h?: number
  btc_relative_strength_ctx?: number
  spread_bps?: number
  slippage_est_25usd?: number
  slippage_est_100usd?: number
  liq_score?: number
  venue_supported?: boolean
  book_state?: string
  price_source?: string
  freshness_bucket?: string
}

export interface DealReviewEventSnapshot {
  account_state?: AccountSnapshot
  positions?: PositionSnapshot[]
  candidate_coins?: string[]
  candidate_details?: CandidateDetail[]
  market_context?: DealReviewMarketContextSnapshot
  execution_log?: string[]
  system_prompt?: string
  user_prompt?: string
  decision_json?: string
  raw_response?: string
  cot_trace?: string
  ai_request_duration_ms?: number
}

export interface DealReviewExitEvidence {
  matched_by?: string
  decision_cycle_number?: number
  decision_action?: string
  order_type?: string
  venue_order_type?: string
  trigger_subtype?: string
  trigger_source?: string
  order_action?: string
  order_status?: string
  client_order_id?: string
  exchange_order_id?: string
  trigger_price?: number
  order_price?: number
  fill_price?: number
  fill_quantity?: number
  reduce_only?: boolean
  close_position?: boolean
  target_stop_loss?: number
  target_take_profit?: number
  stop_loss_distance_bps?: number
  take_profit_distance_bps?: number
  trailing_update_id?: number
  previous_stop_price?: number
  new_stop_price?: number
  trailing_mode?: string
  trailing_tier_index?: number
  trailing_trigger_profit_pct?: number
  trailing_lock_profit_pct?: number
  trailing_offset_pct?: number
  trailing_updated_at_ms?: number
  intent_id?: string
  intent_type?: string
  intent_source_module?: string
  intent_summary?: string
  intent_reasoning?: string
  intent_confidence?: number
  intent_created_at_ms?: number
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
  exit_origin: string
  exit_reason_quality: string
  exit_evidence_summary: string
  exit_evidence?: DealReviewExitEvidence
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
  platform_samples: number
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
  classifier_assist?: DealReviewClassifierAssist
  ai_classifier_assist?: DealReviewClassifierAssist
}

export interface SemanticMemoryDocument {
  id: string
  user_id: string
  trader_id: string
  doc_type: string
  source_id: string
  source_updated_at: string
  title: string
  summary: string
  body: string
  content_hash: string
  token_estimate: number
  embedding_provider: string
  embedding_model: string
  embedding_dimensions: number
  embedding_status: string
  last_embedding_error?: string
  last_built_at?: string
  last_embedded_at?: string
  created_at: string
  updated_at: string
}

export interface SemanticMemorySearchHit {
  document: SemanticMemoryDocument
  similarity_score: number
  distance: number
  source_link?: string
}

export interface SemanticMemorySimilarityResponse {
  query_document?: SemanticMemoryDocument
  items: SemanticMemorySearchHit[]
  total: number
}

export interface SemanticMemoryQueryResult {
  query: string
  embedding_provider: string
  embedding_model: string
  items: SemanticMemorySearchHit[]
}

export interface SemanticMemorySearchPreset {
  id: string
  user_id: string
  trader_id: string
  name: string
  created_at: string
  updated_at: string
}

export interface SemanticMemorySearchPresetDetail {
  preset: SemanticMemorySearchPreset
  config?: Record<string, unknown>
}

export interface SemanticMemoryCorpusDocTypeStatus {
  doc_type: string
  total_documents: number
  embedded_documents: number
  pending_documents: number
  failed_documents: number
  skipped_documents: number
  total_token_estimate: number
  embedded_token_estimate: number
  pending_token_estimate: number
  failed_token_estimate: number
  estimated_total_cost_usd: number
  estimated_pending_cost_usd: number
  estimated_embedded_cost_usd: number
  estimated_failed_reembed_cost_usd: number
  last_updated_at?: string
  last_embedded_at?: string
  dominant_model?: string
  dominant_provider?: string
}

export interface SemanticMemoryEmbeddingUsageSummary {
  recent_runs: number
  recent_embedded_documents: number
  recent_failed_documents: number
  recent_prompt_tokens: number
  recent_estimated_cost_usd: number
  last_completed_at?: string
  last_embedding_model?: string
  last_embedding_provider?: string
}

export interface SemanticMemoryFailureSummary {
  recent_failed_runs: number
  last_failed_at?: string
  last_failure_scope?: string
  last_failure_message?: string
}

export interface SemanticMemoryCostEstimate {
  embedding_model: string
  price_per_1m_tokens_usd: number
  total_token_estimate: number
  embedded_token_estimate: number
  pending_token_estimate: number
  failed_token_estimate: number
  estimated_total_cost_usd: number
  estimated_embedded_cost_usd: number
  estimated_pending_cost_usd: number
  estimated_failed_reembed_cost_usd: number
}

export interface SemanticMemoryBenchmarkConfig {
  doc_types?: string[]
  sample_per_doc_type: number
  top_k: number
}

export interface SemanticMemoryBenchmarkCorpusResult {
  doc_type: string
  document_count: number
  evaluated_queries: number
  skipped_queries: number
  zero_hit_queries: number
  avg_top1_similarity: number
  avg_top1_relevance: number
  avg_top_k_relevance: number
  hit_rate_at_k: number
  strong_top1_rate: number
  avg_returned_hits: number
}

export interface SemanticMemoryBenchmarkRun {
  id: string
  user_id: string
  trader_id: string
  scope: string
  status: string
  summary: string
  corpus_count: number
  query_count: number
  relevant_query_count: number
  avg_top1_similarity: number
  avg_top1_relevance: number
  avg_top_k_relevance: number
  hit_rate_at_k: number
  strong_top1_rate: number
  error_message?: string
  started_at: string
  completed_at?: string
  created_at: string
  updated_at: string
  config?: SemanticMemoryBenchmarkConfig
  corpus_results?: SemanticMemoryBenchmarkCorpusResult[]
}

export interface SemanticMemorySyncRun {
  id: string
  user_id: string
  trader_id: string
  scope: string
  status: string
  summary: string
  total_documents: number
  inserted_documents: number
  updated_documents: number
  unchanged_documents: number
  failed_documents: number
  error_message?: string
  embedding_provider?: string
  embedding_model?: string
  prompt_tokens?: number
  estimated_cost_usd?: number
  doc_types?: string[]
  started_at: string
  completed_at?: string
  created_at: string
  updated_at: string
}

export interface SemanticMemoryCorpusStatus {
  user_id: string
  trader_id: string
  vector_available: boolean
  vector_extension_name?: string
  total_documents: number
  embedded_documents: number
  pending_documents: number
  failed_documents: number
  skipped_documents: number
  total_token_estimate: number
  embedded_token_estimate: number
  pending_token_estimate: number
  failed_token_estimate: number
  cost_estimate: SemanticMemoryCostEstimate
  embedding_usage: SemanticMemoryEmbeddingUsageSummary
  failure_summary: SemanticMemoryFailureSummary
  doc_types: SemanticMemoryCorpusDocTypeStatus[]
  recent_sync_runs?: SemanticMemorySyncRun[]
}

export interface DealReviewClassifierSuggestion {
  classifier_id: string
  suggestion_key: string
  label: string
  issue_type?: string
  score: number
  highlight_level?: string
  evidence_count: number
  accepted_count: number
  rejected_count: number
  feedback_verdict?: string
  rationale?: string
}

export interface DealReviewClassifierAssist {
  source: string
  summary: string
  highlight_score: number
  highlight_level?: string
  suggestions?: DealReviewClassifierSuggestion[]
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
  recommendation_overlap?: DealReviewAIScanCompareRecommendationBlock
  shared_evidence?: DealReviewAIScanCompareEvidenceBlock
  target_cohorts?: DealReviewAIScanCompareTargetCohortBlock
  conflict_score: number
  disagreement_level?: string
  disagreement_summary?: string
  flags?: DealReviewAIScanCompareFlag[]
  model_leaderboards?: DealReviewAIScanLeaderboardGroup[]
}

export interface DealReviewAIScanCompareRecommendationBlock {
  overlap_score: number
  shared?: string[]
  left_only?: string[]
  right_only?: string[]
}

export interface DealReviewAIScanCompareEvidenceBlock {
  shared_strengths?: string[]
  shared_weaknesses?: string[]
  shared_patterns?: string[]
  total_shared: number
}

export interface DealReviewAIScanCompareTargetCohortBlock {
  left_tags?: string[]
  right_tags?: string[]
  shared_tags?: string[]
  conflicting_dimensions?: string[]
}

export interface DealReviewAIScanCompareFlag {
  code: string
  tone: string
  title: string
  note: string
}

export interface DealReviewAIScanLeaderboardEntry {
  model_key: string
  model_label: string
  scan_count: number
  promotion_ready_count: number
  applied_count: number
  challenger_win_count: number
  challenger_loss_count: number
  net_pnl_delta: number
  avg_net_pnl_delta: number
  usefulness_score: number
}

export interface DealReviewAIScanLeaderboardGroup {
  cohort_key: string
  cohort_label: string
  relevant: boolean
  entries?: DealReviewAIScanLeaderboardEntry[]
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
  expected_effect: string
  applied_at?: string
  created_at: string
  updated_at: string
}

export interface DealReviewStrategyVersionAttribution {
  observation_ready: boolean
  before_start?: string
  before_end?: string
  after_start?: string
  after_end?: string
  full_before_summary?: DealReviewDatasetSummary
  full_after_summary?: DealReviewDatasetSummary
  target_before_summary?: DealReviewDatasetSummary
  target_after_summary?: DealReviewDatasetSummary
  non_target_before_summary?: DealReviewDatasetSummary
  non_target_after_summary?: DealReviewDatasetSummary
  warnings?: string[]
  rollback_suggested: boolean
  note?: string
}

export interface DealReviewStrategyVersionDetail {
  version: DealReviewStrategyVersion
  previous_config?: Record<string, unknown>
  next_config?: Record<string, unknown>
  target_cohort?: Record<string, unknown>
  source_scan_summary?: string
  compare_summary?: string
  attribution?: DealReviewStrategyVersionAttribution
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

export interface DealReviewAnomalyGiveBack {
  symbol: string
  deals: number
  net_pnl: number
  avg_give_back_pct: number
  avg_mfe_captured_pct: number
}

export interface DealReviewAnomalyStopOut {
  symbol: string
  deals: number
  net_pnl: number
  avg_hold_ms: number
  avg_mae_pct: number
}

export interface DealReviewAnomalySizing {
  symbol: string
  deals: number
  net_pnl: number
  avg_risk_sizing_score: number
  avg_planned_risk_pct: number
}

export interface DealReviewAnomalyCloseReasonQuality {
  reason: string
  deals: number
  net_pnl: number
  avg_exit_efficiency_score: number
  avg_mfe_captured_pct: number
  avg_give_back_pct: number
}

export interface DealReviewAnomalyExitUncertainty {
  label: string
  close_reason: string
  exit_reason_quality: string
  exit_origin: string
  deals: number
  net_pnl: number
  avg_pnl: number
  share_pct: number
}

export interface DealReviewAnomalySummary {
  closed_deals: number
  worst_symbols?: DealReviewAnomalySymbol[]
  overtraded_symbols?: DealReviewAnomalySymbol[]
  weak_buckets?: DealReviewAnomalyBucket[]
  weak_close_reasons?: DealReviewAnomalyCloseReason[]
  profit_give_back_hotspots?: DealReviewAnomalyGiveBack[]
  early_stop_out_hotspots?: DealReviewAnomalyStopOut[]
  oversized_loss_hotspots?: DealReviewAnomalySizing[]
  close_reason_quality?: DealReviewAnomalyCloseReasonQuality[]
  exit_uncertainty?: DealReviewAnomalyExitUncertainty[]
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

export interface AutonomousOptimizerConfig {
  id: string
  user_id: string
  trader_id: string
  enabled: boolean
  status: string
  review_interval_hours: number
  auto_apply_cooldown_hours: number
  max_consecutive_auto_applies: number
  auto_apply_config_patch: boolean
  auto_apply_prompt_patch: boolean
  auto_rollback_enabled: boolean
  self_pause_enabled: boolean
  proposal_prompt_instructions: string
  critic_prompt_instructions: string
  primary_model_config_id: string
  primary_model_name: string
  critic_model_config_id: string
  critic_model_name: string
  seed_source_trader_id: string
  seed_source_strategy_id: string
  baseline_strategy_id: string
  current_seed_strategy_id: string
  last_run_id: string
  last_run_at: string
  next_run_at: string
  seeded_at: string
  created_at: string
  updated_at: string
}

export interface AutonomousOptimizerRun {
  id: string
  user_id: string
  trader_id: string
  config_id: string
  trigger: string
  status: string
  summary: string
  primary_model_config_id: string
  primary_model_name: string
  critic_model_config_id: string
  critic_model_name: string
  source_trader_id: string
  source_strategy_id: string
  applied_strategy_version_id: string
  started_at: string
  completed_at: string
  created_at: string
  updated_at: string
}

export interface AutonomousOptimizerModelOutcome {
  model_key: string
  model_label: string
  primary_model_config_id: string
  primary_model_name: string
  primary_model_label: string
  critic_model_config_id: string
  critic_model_name: string
  critic_model_label: string
  total_runs: number
  apply_count: number
  apply_rate: number
  monitoring_count: number
  kept_count: number
  kept_win_count: number
  kept_win_rate: number
  rollback_count: number
  rollback_rate: number
  failed_count: number
  stale_recovery_count: number
  insufficient_evidence_count: number
  failure_overlap_insufficient_evidence_count: number
  operational_health_score: number
  backlog_item_count: number
  useful_backlog_count: number
  done_backlog_count: number
  rejected_backlog_count: number
  backlog_usefulness: number
  outcome_score: number
  last_used_at: string
  status_counts?: Record<string, number>
}

export interface AutonomousOptimizerTrailingStopTelemetry {
  trailing_exit_count: number
  trailing_profit_exit_count: number
  trailing_loss_exit_count: number
  initial_stop_loss_count: number
  trailing_exit_avg_pnl_pct: number
  initial_stop_loss_avg_pnl_pct: number
  avg_minutes_to_first_update: number
  avg_minutes_from_first_update_to_exit: number
  early_tightening_count: number
  early_tightening_loss_count: number
  first_update_audit_count: number
  breakeven_protected_count: number
  sample_updates?: AutonomousOptimizerTrailingStopUpdateAuditItem[]
}

export interface AutonomousOptimizerTrailingStopUpdateAuditItem {
  position_id?: number
  symbol?: string
  side?: string
  close_reason?: string
  trailing_mode?: string
  update_time_ms?: number
  minutes_to_first_update?: number
  minutes_from_update_to_exit?: number
  pre_update_unrealized_pnl?: number
  pre_update_unrealized_pnl_pct?: number
  stop_profit_pct?: number
  protects_breakeven?: boolean
  realized_pnl_pct?: number
  previous_stop_price?: number
  new_stop_price?: number
  tier_trigger_profit_pct?: number
}

export interface AutonomousOptimizerAdaptiveCooldownConfig {
  enabled: boolean
  require_weak_execution_regime: boolean
  recent_trade_window: number
  min_recent_trades: number
  same_symbol_loss_cooldown_minutes: number
  pair_loss_lookback_hours: number
}

export interface AutonomousOptimizerCooldownOutcomeSummary {
  trade_count: number
  win_count: number
  loss_count: number
  net_pnl_pct: number
  avg_pnl_pct: number
}

export interface AutonomousOptimizerAdaptiveCooldownSymbol {
  symbol: string
  reentry_count: number
  repeat_after_loss_count: number
  avg_pnl_pct: number
  last_gap_minutes?: number
  blocked_outcomes?: AutonomousOptimizerCooldownOutcomeSummary
  post_cooldown_outcomes?: AutonomousOptimizerCooldownOutcomeSummary
}

export interface AutonomousOptimizerAdaptiveCooldownRegime {
  trend_regime?: string
  volatility_regime?: string
  oi_regime?: string
  reentry_count: number
  repeat_after_loss_count: number
  avg_pnl_pct: number
  blocked_outcomes?: AutonomousOptimizerCooldownOutcomeSummary
  post_cooldown_outcomes?: AutonomousOptimizerCooldownOutcomeSummary
}

export interface AutonomousOptimizerAdaptiveCooldownTelemetry {
  config: AutonomousOptimizerAdaptiveCooldownConfig
  cooldown_candidate_count: number
  same_session_reentry_count: number
  repeat_after_loss_count: number
  regime_repeat_loss_count: number
  blocked_symbol_reentry_outcomes?: AutonomousOptimizerCooldownOutcomeSummary
  post_symbol_cooldown_outcomes?: AutonomousOptimizerCooldownOutcomeSummary
  blocked_regime_reentry_outcomes?: AutonomousOptimizerCooldownOutcomeSummary
  post_regime_cooldown_outcomes?: AutonomousOptimizerCooldownOutcomeSummary
  top_symbols?: AutonomousOptimizerAdaptiveCooldownSymbol[]
  top_regimes?: AutonomousOptimizerAdaptiveCooldownRegime[]
}

export interface AutonomousOptimizerRunDetail {
  run: AutonomousOptimizerRun
  config_patch?: Record<string, unknown>
  prompt_patch?: Record<string, unknown>
  validation?: Record<string, unknown>
  metadata?: Record<string, unknown>
  gate_reasons?: string[]
  strategy_differences?: DealReviewJSONDiffEntry[]
  trader_differences?: DealReviewJSONDiffEntry[]
  optimizer_differences?: DealReviewJSONDiffEntry[]
  review_window_start_ms?: number
  review_window_end_ms?: number
  linked_strategy_version?: DealReviewStrategyVersionDetail
}

export interface AutonomousOptimizerBacklogItem {
  id: string
  user_id: string
  trader_id: string
  run_id: string
  title: string
  category: string
  description: string
  expected_impact: string
  confidence: number
  implementation_cost: number
  urgency: number
  recurrence_count: number
  composite_score: number
  status: string
  ai_generated: boolean
  user_edited: boolean
  merged_finding_count: number
  created_at: string
  updated_at: string
}
