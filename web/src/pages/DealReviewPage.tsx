import { useEffect, useRef, useState } from 'react'
import { mutate } from 'swr'
import { api } from '../lib/api'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { NofxSelect } from '../components/ui/select'
import { DealReviewTimelineChart } from '../components/trader/DealReviewTimelineChart'
import { confirmToast, notify } from '../lib/notify'
import type {
  AIModel,
  DealReviewAIScanCompareResponse,
  DealReviewAIScanDetail,
  DealReviewClassifierAssist,
  DealReviewClassifierSuggestion,
  DealReviewChallengerCompareDetail,
  DealReviewChallengerProtocolEvent,
  DealReviewAnomalySummary,
  DealReviewCase,
  DealReviewCaseDetail,
  DealReviewEventDetail,
  DealReviewEventSnapshot,
  DealReviewFilterPresetDetail,
  DealReviewCaseListItem,
  DealReviewDatasetSummary,
  DealReviewStrategyVersionDetail,
  DealReviewValidationCheck,
  Exchange,
  RemoteModelInfo,
  TraderInfo,
} from '../types'

interface DealReviewPageProps {
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
}

function formatMoney(value: number): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}`
}

function formatPct(value: number): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
}

function formatValidationScope(value: string): string {
  if (!value) return '-'
  if (value === 'recent') return 'Recent live-like'
  return value.charAt(0).toUpperCase() + value.slice(1)
}

function formatValidationMetricValue(
  metric: string,
  value: number | undefined
): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  switch (metric) {
    case 'closed_deal_count':
      return `${Math.round(value)}`
    case 'profit_factor':
      return value.toFixed(2)
    case 'max_drawdown_pct':
    case 'win_rate_delta':
    case 'avg_pnl_pct_delta':
      return formatPct(value)
    case 'net_pnl_delta':
      return formatMoney(value)
    default:
      return value.toFixed(2)
  }
}

function formatValidationCheckSummary(check: DealReviewValidationCheck): string {
  if (check.comparator === 'delta_gte') {
    return `delta ${formatValidationMetricValue(check.metric, check.delta)} | floor ${formatValidationMetricValue(check.metric, check.threshold)}`
  }
  if (check.comparator === 'lte') {
    return `actual ${formatValidationMetricValue(check.metric, check.actual)} | ceiling ${formatValidationMetricValue(check.metric, check.threshold)}`
  }
  return `actual ${formatValidationMetricValue(check.metric, check.actual)} | floor ${formatValidationMetricValue(check.metric, check.threshold)}`
}

function formatVersionCohort(targetCohort?: Record<string, unknown>): string {
  if (!targetCohort || Object.keys(targetCohort).length === 0) {
    return 'No explicit target cohort stored'
  }
  return Object.entries(targetCohort)
    .map(([key, value]) => `${key}: ${String(value)}`)
    .join(' | ')
}

function formatDatasetMini(summary?: DealReviewDatasetSummary): string {
  if (!summary) return 'No closed deals'
  return `${summary.closed_deals} deals | ${formatMoney(summary.net_pnl)} | ${summary.win_rate.toFixed(1)}% win | PF ${summary.profit_factor.toFixed(2)}`
}

function formatClassifierIssueType(issueType?: string): string {
  switch (issueType) {
    case 'likely_bad_trade':
      return 'Likely bad trade'
    case 'likely_bad_exit':
      return 'Likely bad exit'
    case 'likely_avoidable_loss':
      return 'Likely avoidable loss'
    case 'likely_regime_mismatch':
      return 'Likely regime mismatch'
    case 'other_review_signal':
      return 'Other review signal'
    default:
      return 'Review signal'
  }
}

function classifierToneClasses(level?: string): string {
  switch (level) {
    case 'high':
      return 'bg-rose-500/15 text-rose-300 border-rose-400/20'
    case 'medium':
      return 'bg-amber-500/15 text-amber-200 border-amber-400/20'
    default:
      return 'bg-sky-500/15 text-sky-200 border-sky-400/20'
  }
}

function formatDate(ms: number): string {
  if (!ms) return '-'
  return new Date(ms).toLocaleString()
}

function formatHold(ms: number): string {
  if (!ms) return '-'
  const mins = Math.round(ms / 60000)
  if (mins < 60) return `${mins}m`
  const hours = Math.floor(mins / 60)
  const rem = mins % 60
  if (hours < 24) return rem === 0 ? `${hours}h` : `${hours}h ${rem}m`
  const days = Math.floor(hours / 24)
  const remHours = hours % 24
  return remHours === 0 ? `${days}d` : `${days}d ${remHours}h`
}

function formatScore(value: number): string {
  return `${Math.round(value || 0)}/100`
}

function qualityBadgeClasses(tone: 'good' | 'warn' | 'bad'): string {
  switch (tone) {
    case 'good':
      return 'bg-emerald-500/15 text-emerald-300 border-emerald-400/20'
    case 'warn':
      return 'bg-amber-500/15 text-amber-200 border-amber-400/20'
    default:
      return 'bg-rose-500/15 text-rose-300 border-rose-400/20'
  }
}

function getDealGiveBackPct(caseRec: DealReviewCase): number {
  return Math.max(caseRec.profit_given_back_pct || 0, 0)
}

function getDealGiveBackMoney(caseRec: DealReviewCase): number {
  return Math.max(caseRec.profit_given_back || 0, 0)
}

function buildQualityNarrative(caseRec: DealReviewCase): string {
  if (
    caseRec.entry_timing_score >= 70 &&
    caseRec.max_favorable_excursion > 0.05 &&
    caseRec.exit_efficiency_score < 40
  ) {
    return `Entry was valid, but exit captured only ${Math.round(caseRec.mfe_captured_pct || 0)}% of available MFE.`
  }
  if (caseRec.realized_pnl < 0 && caseRec.profit_given_back > 0.05) {
    return `This loss was avoidable: the trade gave back ${formatMoney(caseRec.profit_given_back)} after being in profit.`
  }
  if (
    caseRec.realized_pnl > 0 &&
    caseRec.entry_timing_score < 35
  ) {
    return 'Weak entry recovered into profit. Treat this as a lucky exit, not a clean edge.'
  }
  if (caseRec.risk_sizing_score > 0 && caseRec.risk_sizing_score < 35) {
    return `Risk sizing was aggressive for this path: planned risk was ${formatPct(caseRec.planned_risk_pct || 0)}.`
  }
  return ''
}

function buildQualityBadges(caseRec: DealReviewCase): Array<{
  label: string
  tone: 'good' | 'warn' | 'bad'
}> {
  const badges: Array<{ label: string; tone: 'good' | 'warn' | 'bad' }> = []

  if (
    caseRec.entry_timing_score >= 70 &&
    caseRec.max_favorable_excursion > 0.05 &&
    caseRec.exit_efficiency_score < 40
  ) {
    badges.push({ label: 'Strong entry / weak exit', tone: 'warn' })
  } else if (caseRec.entry_timing_score >= 70) {
    badges.push({ label: 'Strong entry', tone: 'good' })
  } else if (caseRec.entry_timing_score < 35) {
    badges.push({ label: 'Weak entry', tone: 'bad' })
  }

  if (caseRec.max_favorable_excursion > 0.05) {
    if (caseRec.exit_efficiency_score >= 75) {
      badges.push({ label: 'Strong exit', tone: 'good' })
    } else if (caseRec.exit_efficiency_score < 35) {
      badges.push({ label: 'Weak exit', tone: 'bad' })
    }
  }

  if (caseRec.realized_pnl < 0 && caseRec.profit_given_back > 0.05) {
    badges.push({ label: 'Avoidable loss', tone: 'warn' })
  }

  if (
    caseRec.realized_pnl > 0 &&
    caseRec.entry_timing_score < 35
  ) {
    badges.push({ label: 'Weak entry / lucky exit', tone: 'warn' })
  }

  if (caseRec.risk_sizing_score > 0 && caseRec.risk_sizing_score < 35) {
    badges.push({ label: 'Oversized risk', tone: 'bad' })
  }

  return badges.slice(0, 4)
}

function formatCompareMode(value: string): string {
  switch (value) {
    case 'shared_live':
      return 'Shared live wallet'
    case 'isolated_live':
      return 'Isolated live wallet'
    case 'paper':
      return 'Paper / simulation'
    default:
      return value || '-'
  }
}

function formatCompareStatus(value: string): string {
  if (!value) return '-'
  return value.replace(/_/g, ' ')
}

function compareFlagClasses(tone?: string): string {
  switch (tone) {
    case 'good':
      return 'bg-emerald-500/15 text-emerald-300 border-emerald-400/20'
    case 'warn':
      return 'bg-amber-500/15 text-amber-200 border-amber-400/20'
    default:
      return 'bg-white/5 text-nofx-text-muted border-white/10'
  }
}

function compareLevelClasses(level?: string): string {
  switch (level) {
    case 'high':
      return 'text-rose-300'
    case 'medium':
      return 'text-amber-200'
    default:
      return 'text-emerald-300'
  }
}

function formatCompareFlagTitle(code?: string, fallback?: string): string {
  switch (code) {
    case 'strong_consensus':
      return 'Strong consensus'
    case 'mixed_recommendation':
      return 'Mixed recommendation'
    case 'low_confidence_disagreement':
      return 'Low-confidence disagreement'
    default:
      return fallback || 'Compare signal'
  }
}

function isActiveCompareStatus(value: string): boolean {
  return value === 'starting' || value === 'running'
}

function formatCompareProtocolType(value: string): string {
  if (!value) return 'Event'
  return value.replace(/_/g, ' ')
}

function formatStrategyVersionSourceType(value: string): string {
  switch (value) {
    case 'ai_apply':
      return 'AI apply'
    case 'ai_challenger_candidate':
      return 'Challenger candidate'
    case 'rollback':
      return 'Rollback'
    default:
      return value.replace(/_/g, ' ') || 'Strategy change'
  }
}

function buildCompareProtocolMetricSummary(
  event?: DealReviewChallengerProtocolEvent
): string {
  const metrics = event?.metrics
  if (!metrics) return ''
  return `Incumbent ${formatMoney(metrics.incumbent_pnl)} (${metrics.incumbent_trade_count} trades) | Challenger ${formatMoney(metrics.challenger_pnl)} (${metrics.challenger_trade_count} trades)`
}

function isKeyboardTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName.toLowerCase()
  return (
    target.isContentEditable ||
    tag === 'input' ||
    tag === 'textarea' ||
    tag === 'select' ||
    Boolean(target.closest('[contenteditable="true"]'))
  )
}

function getVersionSummaryDelta(
  before?: DealReviewDatasetSummary,
  after?: DealReviewDatasetSummary
): number | null {
  if (!before || !after) return null
  return after.net_pnl - before.net_pnl
}

function getPatchOutcomeTone(delta: number | null, rollbackSuggested?: boolean): string {
  if (rollbackSuggested) return 'text-rose-300'
  if (typeof delta === 'number' && delta < -0.01) return 'text-rose-300'
  if (typeof delta === 'number' && delta > 0.01) return 'text-emerald-300'
  return 'text-amber-200'
}

function getPatchOutcomeLabel(delta: number | null, rollbackSuggested?: boolean): string {
  if (rollbackSuggested) return 'Regression risk'
  if (typeof delta === 'number' && delta < -0.01) return 'Weaker'
  if (typeof delta === 'number' && delta > 0.01) return 'Improved'
  return 'Mixed / flat'
}

function normalizePresetText(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function buildQueueLabel(queueMode: string): string {
  switch (queueMode) {
    case 'unlabeled_losses':
      return 'Unlabeled losses'
    case 'biggest_giveback':
      return 'Biggest give-back exits'
    case 'regime_mismatch':
      return 'Regime mismatch candidates'
    default:
      return 'All filtered deals'
  }
}

function buildReviewQueueItems(
  items: DealReviewCaseListItem[],
  queueMode: string
): DealReviewCaseListItem[] {
  if (!queueMode) return items

  const labelsContain = (labels: string[] | undefined, needle: string) =>
    (labels || []).some((item) =>
      item.toLowerCase().includes(needle.toLowerCase())
    )
  const hasRegimeSignal = (item: DealReviewCaseListItem) =>
    labelsContain(item.labels, 'regime mismatch') ||
    (item.classifier_assist?.suggestions || []).some(
      (entry) => entry.issue_type === 'likely_regime_mismatch'
    )

  if (queueMode === 'unlabeled_losses') {
    return items.filter(
      (item) =>
        item.case.status === 'CLOSED' &&
        item.case.outcome === 'loss' &&
        (!item.labels || item.labels.length === 0)
    )
  }

  if (queueMode === 'biggest_giveback') {
    return [...items]
      .filter((item) => {
        return (
          item.case.status === 'CLOSED' &&
          (getDealGiveBackPct(item.case) > 25 ||
            getDealGiveBackMoney(item.case) > 0.05)
        )
      })
      .sort((left, right) => {
        const leftGiveBack = getDealGiveBackPct(left.case)
        const rightGiveBack = getDealGiveBackPct(right.case)
        return rightGiveBack - leftGiveBack
      })
  }

  if (queueMode === 'regime_mismatch') {
    return items.filter((item) => hasRegimeSignal(item))
  }

  return items
}

function formatDealReasonPreview(value?: string): string {
  const text = (value || '').trim()
  if (!text) return 'No linked rationale snapshot.'
  return text.length > 140 ? `${text.slice(0, 137)}...` : text
}

function escapeCSVValue(value: unknown): string {
  const text =
    value === null || value === undefined
      ? ''
      : typeof value === 'string'
        ? value
        : String(value)
  const escaped = text.replace(/"/g, '""')
  return /[",\n]/.test(escaped) ? `"${escaped}"` : escaped
}

function downloadTextFile(filename: string, content: string, mimeType: string): void {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

function buildDealReviewCasesCSV(items: DealReviewCaseListItem[]): string {
  const headers = [
    'case_id',
    'symbol',
    'side',
    'status',
    'outcome',
    'entry_time_ms',
    'exit_time_ms',
    'entry_price',
    'exit_price',
    'entry_quantity',
    'exit_quantity',
    'realized_pnl',
    'realized_pnl_pct',
    'hold_duration_ms',
    'open_selection_bucket',
    'open_trend_regime',
    'open_volatility_regime',
    'open_btc_strength_regime',
    'open_funding_regime',
    'open_oi_regime',
    'open_session_bucket',
    'open_weekday_bucket',
    'open_venue_tier',
    'open_liquidity_tier',
    'open_spread_bucket',
    'open_slippage_bucket',
    'close_reason',
    'max_favorable_excursion',
    'max_favorable_excursion_pct',
    'max_adverse_excursion',
    'max_adverse_excursion_pct',
    'mfe_captured_pct',
    'profit_given_back',
    'profit_given_back_pct',
    'time_to_first_profit_ms',
    'time_to_max_drawdown_ms',
    'planned_risk_pct',
    'exit_efficiency_score',
    'entry_timing_score',
    'risk_sizing_score',
    'labels',
    'open_reasoning',
    'close_reasoning',
    'cycle_samples',
    'platform_samples',
    'ever_in_profit',
    'max_unrealized_pnl',
    'max_unrealized_pnl_pct',
    'min_unrealized_pnl',
    'min_unrealized_pnl_pct',
  ]
  const rows = items.map((item) => [
    item.case.id,
    item.case.symbol,
    item.case.side,
    item.case.status,
    item.case.outcome,
    item.case.entry_time_ms,
    item.case.exit_time_ms,
    item.case.entry_price,
    item.case.exit_price,
    item.case.entry_quantity,
    item.case.exit_quantity,
    item.case.realized_pnl,
    item.case.realized_pnl_pct,
    item.case.hold_duration_ms,
    item.case.open_selection_bucket,
    item.case.open_trend_regime,
    item.case.open_volatility_regime,
    item.case.open_btc_strength_regime,
    item.case.open_funding_regime,
    item.case.open_oi_regime,
    item.case.open_session_bucket,
    item.case.open_weekday_bucket,
    item.case.open_venue_tier,
    item.case.open_liquidity_tier,
    item.case.open_spread_bucket,
    item.case.open_slippage_bucket,
    item.case.close_reason,
    item.case.max_favorable_excursion,
    item.case.max_favorable_excursion_pct,
    item.case.max_adverse_excursion,
    item.case.max_adverse_excursion_pct,
    item.case.mfe_captured_pct,
    item.case.profit_given_back,
    item.case.profit_given_back_pct,
    item.case.time_to_first_profit_ms,
    item.case.time_to_max_drawdown_ms,
    item.case.planned_risk_pct,
    item.case.exit_efficiency_score,
    item.case.entry_timing_score,
    item.case.risk_sizing_score,
    (item.labels || []).join(' | '),
    item.open_reasoning,
    item.close_reasoning,
    item.price_timeline_summary?.cycle_samples || 0,
    item.price_timeline_summary?.platform_samples || 0,
    item.price_timeline_summary?.ever_in_profit ? 'true' : 'false',
    item.price_timeline_summary?.max_unrealized_pnl || 0,
    item.price_timeline_summary?.max_unrealized_pnl_pct || 0,
    item.price_timeline_summary?.min_unrealized_pnl || 0,
    item.price_timeline_summary?.min_unrealized_pnl_pct || 0,
  ])

  return [headers, ...rows]
    .map((row) => row.map((value) => escapeCSVValue(value)).join(','))
    .join('\n')
}

function getLatestCompareForScan(
  scanId: string,
  compares: DealReviewChallengerCompareDetail[]
): DealReviewChallengerCompareDetail | undefined {
  return compares.find((item) => item.compare.source_scan_id === scanId)
}

function getScanPromotionState(
  scan: DealReviewAIScanDetail,
  relatedCompare?: DealReviewChallengerCompareDetail
): { label: string; tone: string; detail: string } {
  const validationStatus =
    scan.validation?.status || scan.scan.validation_status || 'pending'
  if (validationStatus !== 'passed') {
    return {
      label:
        validationStatus === 'failed'
          ? 'Blocked by validation'
          : 'Validation pending',
      tone: validationStatus === 'failed' ? 'rose' : 'amber',
      detail:
        validationStatus === 'failed'
          ? 'This scan cannot be promoted until the blocking validation checks pass.'
          : 'Validation has not completed yet.',
    }
  }

  if (!relatedCompare) {
    return {
      label: 'Validated, challenger not started',
      tone: 'amber',
      detail:
        'Validation passed. The patch is ready for direct apply or for a challenger launch.',
    }
  }

  switch (relatedCompare.compare.status) {
    case 'starting':
    case 'running':
      return {
        label: 'Challenger running',
        tone: 'sky',
        detail:
          relatedCompare.compare.summary ||
          'Incumbent and challenger are currently in the comparison window.',
      }
    case 'completed':
      if (
        relatedCompare.compare.winner_trader_id &&
        relatedCompare.compare.winner_trader_id ===
          relatedCompare.compare.challenger_trader_id
      ) {
        return {
          label: 'Winner promoted',
          tone: 'emerald',
          detail:
            relatedCompare.compare.summary ||
            'The challenger won on realized PnL and the incumbent was deactivated automatically.',
        }
      }
      if (
        relatedCompare.compare.winner_trader_id &&
        relatedCompare.compare.winner_trader_id ===
          relatedCompare.compare.incumbent_trader_id
      ) {
        return {
          label: 'Challenger rejected',
          tone: 'rose',
          detail:
            relatedCompare.compare.summary ||
            'The incumbent stayed ahead on realized PnL and the challenger was deactivated automatically.',
        }
      }
      return {
        label: 'Challenger finished',
        tone: 'emerald',
        detail:
          relatedCompare.compare.summary ||
          'The challenger comparison finished.',
      }
    case 'stopped':
      return {
        label: 'Challenger finished',
        tone: 'amber',
        detail:
          relatedCompare.compare.summary ||
          'The challenger comparison was stopped manually before auto resolution.',
      }
    case 'failed':
      return {
        label: 'Challenger finished',
        tone: 'rose',
        detail:
          relatedCompare.compare.summary ||
          relatedCompare.compare.error_message ||
          'The challenger workflow failed before completion.',
      }
    default:
      return {
        label: 'Validated, challenger not started',
        tone: 'amber',
        detail:
          'Validation passed. The patch is ready for direct apply or for a challenger launch.',
      }
  }
}

function formatRequestDuration(ms?: number): string {
  if (!ms) return '-'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

function firstNonEmpty(...values: Array<string | undefined>): string {
  for (const value of values) {
    if (value && value.trim()) return value
  }
  return ''
}

function formatJsonBlock(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value ?? '')
  }
}

function formatMarketContextToken(value?: string): string {
  const token = (value || '').trim()
  if (!token) return '-'
  return token
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase())
}

function formatMarketContextValue(
  value: number | undefined,
  mode: 'plain' | 'pct' | 'bps' = 'plain'
): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  if (mode === 'pct') return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
  if (mode === 'bps') return `${value >= 0 ? '+' : ''}${value.toFixed(2)} bps`
  return value.toFixed(3)
}

function buildOpenRegimeBadges(detail: DealReviewCaseDetail | DealReviewCaseListItem | null) {
  if (!detail) return []
  const dealCase = detail.case
  return [
    dealCase.open_trend_regime,
    dealCase.open_volatility_regime,
    dealCase.open_btc_strength_regime,
    dealCase.open_funding_regime,
    dealCase.open_session_bucket,
  ].filter(Boolean)
}

function getEventSystemPrompt(detail?: DealReviewEventDetail | null): string {
  return firstNonEmpty(
    detail?.snapshot?.system_prompt,
    detail?.decision_record?.system_prompt
  )
}

function getEventInputPrompt(detail?: DealReviewEventDetail | null): string {
  return firstNonEmpty(
    detail?.snapshot?.user_prompt,
    detail?.decision_record?.input_prompt
  )
}

function getEventDecisionJson(detail?: DealReviewEventDetail | null): string {
  return firstNonEmpty(
    detail?.snapshot?.decision_json,
    detail?.decision_record?.decision_json
  )
}

function getEventRawResponse(detail?: DealReviewEventDetail | null): string {
  return firstNonEmpty(
    detail?.snapshot?.raw_response,
    detail?.decision_record?.raw_response
  )
}

function getEventRequestMs(detail?: DealReviewEventDetail | null): number {
  return (
    detail?.snapshot?.ai_request_duration_ms ||
    detail?.decision_record?.ai_request_duration_ms ||
    0
  )
}

function hasStructuredSnapshot(snapshot?: DealReviewEventSnapshot): boolean {
  if (!snapshot) return false
  return Boolean(
    snapshot.account_state ||
      (snapshot.positions && snapshot.positions.length > 0) ||
      (snapshot.candidate_coins && snapshot.candidate_coins.length > 0) ||
      (snapshot.candidate_details && snapshot.candidate_details.length > 0) ||
      snapshot.market_context
  )
}

const sideOptions = [
  { value: '', label: 'All Sides' },
  { value: 'LONG', label: 'LONG' },
  { value: 'SHORT', label: 'SHORT' },
]

const statusOptions = [
  { value: '', label: 'All Statuses' },
  { value: 'OPEN', label: 'OPEN' },
  { value: 'CLOSED', label: 'CLOSED' },
]

const outcomeOptions = [
  { value: '', label: 'All Outcomes' },
  { value: 'profit', label: 'Profit' },
  { value: 'loss', label: 'Loss' },
  { value: 'flat', label: 'Flat' },
  { value: 'open', label: 'Open' },
]

const rangeOptions = [
  { value: '1h', label: 'Last 1h' },
  { value: '4h', label: 'Last 4h' },
  { value: '12h', label: 'Last 12h' },
  { value: '24h', label: 'Last 24h' },
  { value: '7d', label: 'Last 7d' },
  { value: '30d', label: 'Last 30d' },
  { value: '90d', label: 'Last 90d' },
  { value: 'all', label: 'All time' },
]

const trendRegimeOptions = [
  { value: '', label: 'All trend regimes' },
  { value: 'uptrend', label: 'Uptrend' },
  { value: 'downtrend', label: 'Downtrend' },
  { value: 'chop', label: 'Chop' },
  { value: 'mixed', label: 'Mixed' },
]

const volatilityRegimeOptions = [
  { value: '', label: 'All vol regimes' },
  { value: 'high_vol', label: 'High vol' },
  { value: 'low_vol', label: 'Low vol' },
  { value: 'normal', label: 'Normal' },
]

const btcStrengthOptions = [
  { value: '', label: 'All BTC-relative states' },
  { value: 'outperform', label: 'Outperforming BTC' },
  { value: 'lagging', label: 'Lagging BTC' },
  { value: 'neutral', label: 'Neutral vs BTC' },
]

const fundingRegimeOptions = [
  { value: '', label: 'All funding states' },
  { value: 'extreme_longs', label: 'Extreme longs' },
  { value: 'longs_pay', label: 'Longs pay' },
  { value: 'neutral', label: 'Neutral funding' },
  { value: 'shorts_pay', label: 'Shorts pay' },
  { value: 'extreme_shorts', label: 'Extreme shorts' },
]

const oiRegimeOptions = [
  { value: '', label: 'All OI states' },
  { value: 'oi_surge', label: 'OI surge' },
  { value: 'oi_rising', label: 'OI rising' },
  { value: 'oi_flat', label: 'OI flat' },
  { value: 'oi_falling', label: 'OI falling' },
  { value: 'oi_flush', label: 'OI flush' },
]

const sessionOptions = [
  { value: '', label: 'All sessions' },
  { value: 'asia', label: 'Asia' },
  { value: 'eu', label: 'EU' },
  { value: 'us', label: 'US' },
  { value: 'off_hours', label: 'Off hours' },
]

const weekdayOptions = [
  { value: '', label: 'All weekdays' },
  { value: 'monday', label: 'Monday' },
  { value: 'tuesday', label: 'Tuesday' },
  { value: 'wednesday', label: 'Wednesday' },
  { value: 'thursday', label: 'Thursday' },
  { value: 'friday', label: 'Friday' },
  { value: 'saturday', label: 'Saturday' },
  { value: 'sunday', label: 'Sunday' },
]

const venueTierOptions = [
  { value: '', label: 'All venue states' },
  { value: 'tradable', label: 'Tradable' },
  { value: 'thin_book', label: 'Thin book' },
  { value: 'restricted', label: 'Restricted' },
  { value: 'unsupported', label: 'Unsupported' },
  { value: 'unknown', label: 'Unknown' },
]

const liquidityTierOptions = [
  { value: '', label: 'All liquidity tiers' },
  { value: 'high', label: 'High liquidity' },
  { value: 'medium', label: 'Medium liquidity' },
  { value: 'low', label: 'Low liquidity' },
]

const executionBucketOptions = [
  { value: '', label: 'All execution buckets' },
  { value: 'tight', label: 'Tight' },
  { value: 'normal', label: 'Normal' },
  { value: 'wide', label: 'Wide' },
  { value: 'extreme', label: 'Extreme' },
]

const reviewQueueOptions = [
  { value: '', label: 'All filtered deals' },
  { value: 'unlabeled_losses', label: 'Review queue: unlabeled losses' },
  { value: 'biggest_giveback', label: 'Review queue: biggest give-back exits' },
  {
    value: 'regime_mismatch',
    label: 'Review queue: regime mismatch candidates',
  },
]

const challengerModeOptions = [
  { value: 'shared_live', label: 'Shared live wallet' },
  { value: 'isolated_live', label: 'Isolated live wallet' },
  { value: 'paper', label: 'Paper / testnet' },
]

const challengerWindowOptions = [
  { value: '12', label: '12h' },
  { value: '24', label: '24h' },
  { value: '36', label: '36h' },
  { value: '48', label: '48h' },
  { value: '96', label: '96h' },
]

function getDateRangeStart(range: string): number | undefined {
  const normalized = range.trim().toLowerCase()
  if (!normalized || normalized === 'all') return undefined

  const match = normalized.match(/^(\d+)([hd])$/)
  if (!match) return undefined

  const amount = Number(match[1])
  if (!Number.isFinite(amount) || amount <= 0) return undefined

  const unit = match[2]
  const ms =
    unit === 'h' ? amount * 60 * 60 * 1000 : amount * 24 * 60 * 60 * 1000
  return Date.now() - ms
}

export function DealReviewPage({
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
}: DealReviewPageProps) {
  const [symbol, setSymbol] = useState('')
  const [side, setSide] = useState('')
  const [status, setStatus] = useState('CLOSED')
  const [outcome, setOutcome] = useState('')
  const [openSelectionBucket, setOpenSelectionBucket] = useState('')
  const [closeReason, setCloseReason] = useState('')
  const [openTrendRegime, setOpenTrendRegime] = useState('')
  const [openVolatilityRegime, setOpenVolatilityRegime] = useState('')
  const [openBTCStrengthRegime, setOpenBTCStrengthRegime] = useState('')
  const [openFundingRegime, setOpenFundingRegime] = useState('')
  const [openOIRegime, setOpenOIRegime] = useState('')
  const [openSessionBucket, setOpenSessionBucket] = useState('')
  const [openWeekdayBucket, setOpenWeekdayBucket] = useState('')
  const [openVenueTier, setOpenVenueTier] = useState('')
  const [openLiquidityTier, setOpenLiquidityTier] = useState('')
  const [openSpreadBucket, setOpenSpreadBucket] = useState('')
  const [openSlippageBucket, setOpenSlippageBucket] = useState('')
  const [dateRange, setDateRange] = useState('30d')
  const [minPnl, setMinPnl] = useState('')
  const [maxPnl, setMaxPnl] = useState('')
  const [reviewQueueMode, setReviewQueueMode] = useState('')
  const [filterPresets, setFilterPresets] = useState<DealReviewFilterPresetDetail[]>([])
  const [selectedPresetId, setSelectedPresetId] = useState('')
  const [presetName, setPresetName] = useState('')
  const [savingPreset, setSavingPreset] = useState(false)
  const [deletingPresetId, setDeletingPresetId] = useState<string | null>(null)

  const [items, setItems] = useState<DealReviewCaseListItem[]>([])
  const [summary, setSummary] = useState<DealReviewDatasetSummary | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [selectedCaseId, setSelectedCaseId] = useState<string | null>(null)
  const [detail, setDetail] = useState<DealReviewCaseDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [comparePeerCaseId, setComparePeerCaseId] = useState('')
  const [comparePeerDetail, setComparePeerDetail] =
    useState<DealReviewCaseDetail | null>(null)
  const [comparePeerLoading, setComparePeerLoading] = useState(false)
  const [aiClassifierAssist, setAIClassifierAssist] =
    useState<DealReviewClassifierAssist | null>(null)
  const [runningCaseAIAssist, setRunningCaseAIAssist] = useState(false)
  const [classifierActionKey, setClassifierActionKey] = useState<string | null>(
    null
  )

  const [models, setModels] = useState<AIModel[]>([])
  const [scans, setScans] = useState<DealReviewAIScanDetail[]>([])
  const [selectedModelId, setSelectedModelId] = useState('')
  const [availableRemoteModels, setAvailableRemoteModels] = useState<
    RemoteModelInfo[]
  >([])
  const [selectedRemoteModel, setSelectedRemoteModel] = useState('')
  const [runningScan, setRunningScan] = useState(false)
  const [applyingScanId, setApplyingScanId] = useState<string | null>(null)
  const [reviewLabelsText, setReviewLabelsText] = useState('')
  const [reviewNote, setReviewNote] = useState('')
  const [savingReview, setSavingReview] = useState(false)
  const [anomalies, setAnomalies] = useState<DealReviewAnomalySummary | null>(
    null
  )
  const [versions, setVersions] = useState<DealReviewStrategyVersionDetail[]>(
    []
  )
  const [selectedVersionId, setSelectedVersionId] = useState<string | null>(null)
  const [exchanges, setExchanges] = useState<Exchange[]>([])
  const [challengerCompares, setChallengerCompares] = useState<
    DealReviewChallengerCompareDetail[]
  >([])
  const [selectedCompareId, setSelectedCompareId] = useState<string | null>(
    null
  )
  const [selectedCompareDetail, setSelectedCompareDetail] =
    useState<DealReviewChallengerCompareDetail | null>(null)
  const [compareDetailLoading, setCompareDetailLoading] = useState(false)
  const [compareActionKey, setCompareActionKey] = useState<string | null>(null)
  const [openingCompareId, setOpeningCompareId] = useState<string | null>(null)
  const [rollingBackVersionId, setRollingBackVersionId] = useState<
    string | null
  >(null)
  const [validatingScanId, setValidatingScanId] = useState<string | null>(null)
  const [launchingScanId, setLaunchingScanId] = useState<string | null>(null)
  const [launchMode, setLaunchMode] = useState('shared_live')
  const [launchExchangeId, setLaunchExchangeId] = useState('')
  const [launchWindowHours, setLaunchWindowHours] = useState('24')
  const [compareLeftScanId, setCompareLeftScanId] = useState('')
  const [compareRightScanId, setCompareRightScanId] = useState('')
  const [compareResult, setCompareResult] =
    useState<DealReviewAIScanCompareResponse | null>(null)
  const [compareLoading, setCompareLoading] = useState(false)
  const compareDetailRef = useRef<HTMLDivElement | null>(null)
  const versionDetailRef = useRef<HTMLDivElement | null>(null)

  const selectedTrader = traders?.find(
    (item) => item.trader_id === selectedTraderId
  )
  const selectedPreset =
    filterPresets.find((item) => item.preset.id === selectedPresetId) || null
  const selectedVersion =
    versions.find((item) => item.version.id === selectedVersionId) || null
  const latestVersion = versions[0] || null
  const latestFullDelta = getVersionSummaryDelta(
    latestVersion?.attribution?.full_before_summary,
    latestVersion?.attribution?.full_after_summary
  )
  const latestTargetDelta = getVersionSummaryDelta(
    latestVersion?.attribution?.target_before_summary,
    latestVersion?.attribution?.target_after_summary
  )
  const displayedItems = buildReviewQueueItems(items, reviewQueueMode)
  const selectedCaseIndex = displayedItems.findIndex(
    (item) => item.case.id === selectedCaseId
  )

  const buildFilterPayload = (
    overrides?: Record<string, string | number | undefined>
  ) => {
    const filter: Record<string, string | number | undefined> = {
      symbol: symbol.trim().toUpperCase() || undefined,
      side: side || undefined,
      status: status || undefined,
      outcome: outcome || undefined,
      open_selection_bucket: openSelectionBucket.trim() || undefined,
      close_reason: closeReason.trim() || undefined,
      open_trend_regime: openTrendRegime || undefined,
      open_volatility_regime: openVolatilityRegime || undefined,
      open_btc_strength_regime: openBTCStrengthRegime || undefined,
      open_funding_regime: openFundingRegime || undefined,
      open_oi_regime: openOIRegime || undefined,
      open_session_bucket: openSessionBucket || undefined,
      open_weekday_bucket: openWeekdayBucket || undefined,
      open_venue_tier: openVenueTier || undefined,
      open_liquidity_tier: openLiquidityTier || undefined,
      open_spread_bucket: openSpreadBucket || undefined,
      open_slippage_bucket: openSlippageBucket || undefined,
      limit: 150,
    }
    const rangeStart = getDateRangeStart(dateRange)
    if (rangeStart !== undefined) {
      filter.from_time = rangeStart
    }
    if (minPnl.trim()) {
      const parsed = Number(minPnl.trim())
      filter.min_pnl = Number.isFinite(parsed) ? parsed : undefined
    }
    if (maxPnl.trim()) {
      const parsed = Number(maxPnl.trim())
      filter.max_pnl = Number.isFinite(parsed) ? parsed : undefined
    }
    if (overrides) {
      Object.entries(overrides).forEach(([key, value]) => {
        filter[key] = value
      })
    }
    return filter
  }

  const buildPresetFilters = () => ({
    symbol: symbol.trim().toUpperCase(),
    side,
    status,
    outcome,
    open_selection_bucket: openSelectionBucket.trim(),
    close_reason: closeReason.trim(),
    open_trend_regime: openTrendRegime,
    open_volatility_regime: openVolatilityRegime,
    open_btc_strength_regime: openBTCStrengthRegime,
    open_funding_regime: openFundingRegime,
    open_oi_regime: openOIRegime,
    open_session_bucket: openSessionBucket,
    open_weekday_bucket: openWeekdayBucket,
    open_venue_tier: openVenueTier,
    open_liquidity_tier: openLiquidityTier,
    open_spread_bucket: openSpreadBucket,
    open_slippage_bucket: openSlippageBucket,
    date_range: dateRange,
    min_pnl_text: minPnl.trim(),
    max_pnl_text: maxPnl.trim(),
    review_queue: reviewQueueMode,
  })

  const applyPresetFilters = (filters?: Record<string, unknown>) => {
    const hasStatus = Boolean(
      filters && Object.prototype.hasOwnProperty.call(filters, 'status')
    )
    setSymbol(normalizePresetText(filters?.symbol).toUpperCase())
    setSide(normalizePresetText(filters?.side))
    setStatus(hasStatus ? normalizePresetText(filters?.status) : 'CLOSED')
    setOutcome(normalizePresetText(filters?.outcome))
    setOpenSelectionBucket(normalizePresetText(filters?.open_selection_bucket))
    setCloseReason(normalizePresetText(filters?.close_reason))
    setOpenTrendRegime(normalizePresetText(filters?.open_trend_regime))
    setOpenVolatilityRegime(normalizePresetText(filters?.open_volatility_regime))
    setOpenBTCStrengthRegime(normalizePresetText(filters?.open_btc_strength_regime))
    setOpenFundingRegime(normalizePresetText(filters?.open_funding_regime))
    setOpenOIRegime(normalizePresetText(filters?.open_oi_regime))
    setOpenSessionBucket(normalizePresetText(filters?.open_session_bucket))
    setOpenWeekdayBucket(normalizePresetText(filters?.open_weekday_bucket))
    setOpenVenueTier(normalizePresetText(filters?.open_venue_tier))
    setOpenLiquidityTier(normalizePresetText(filters?.open_liquidity_tier))
    setOpenSpreadBucket(normalizePresetText(filters?.open_spread_bucket))
    setOpenSlippageBucket(normalizePresetText(filters?.open_slippage_bucket))
    setDateRange(normalizePresetText(filters?.date_range) || '30d')
    setMinPnl(normalizePresetText(filters?.min_pnl_text))
    setMaxPnl(normalizePresetText(filters?.max_pnl_text))
    setReviewQueueMode(normalizePresetText(filters?.review_queue))
  }

  const applyDrilldownFilters = (
    patch: Record<string, string | number | undefined>,
    note?: string
  ) => {
    setSelectedPresetId('')
    if (typeof patch.symbol === 'string') {
      setSymbol(patch.symbol)
    }
    if (typeof patch.side === 'string') {
      setSide(patch.side)
    }
    if (typeof patch.status === 'string') {
      setStatus(patch.status)
    }
    if (typeof patch.outcome === 'string') {
      setOutcome(patch.outcome)
    }
    if (typeof patch.open_selection_bucket === 'string') {
      setOpenSelectionBucket(patch.open_selection_bucket)
    }
    if (typeof patch.close_reason === 'string') {
      setCloseReason(patch.close_reason)
    }
    if (typeof patch.open_trend_regime === 'string') {
      setOpenTrendRegime(patch.open_trend_regime)
    }
    if (typeof patch.open_volatility_regime === 'string') {
      setOpenVolatilityRegime(patch.open_volatility_regime)
    }
    if (typeof patch.open_btc_strength_regime === 'string') {
      setOpenBTCStrengthRegime(patch.open_btc_strength_regime)
    }
    if (typeof patch.open_funding_regime === 'string') {
      setOpenFundingRegime(patch.open_funding_regime)
    }
    if (typeof patch.open_oi_regime === 'string') {
      setOpenOIRegime(patch.open_oi_regime)
    }
    if (typeof patch.open_session_bucket === 'string') {
      setOpenSessionBucket(patch.open_session_bucket)
    }
    if (typeof patch.open_weekday_bucket === 'string') {
      setOpenWeekdayBucket(patch.open_weekday_bucket)
    }
    if (typeof patch.open_venue_tier === 'string') {
      setOpenVenueTier(patch.open_venue_tier)
    }
    if (typeof patch.open_liquidity_tier === 'string') {
      setOpenLiquidityTier(patch.open_liquidity_tier)
    }
    if (typeof patch.open_spread_bucket === 'string') {
      setOpenSpreadBucket(patch.open_spread_bucket)
    }
    if (typeof patch.open_slippage_bucket === 'string') {
      setOpenSlippageBucket(patch.open_slippage_bucket)
    }
    setReviewQueueMode('')
    if (note) {
      notify.success(note)
    }
  }

  const loadCases = async () => {
    if (!selectedTraderId) return
    setLoading(true)
    setError(null)
    try {
      const result = await api.getDealReviewCases(
        selectedTraderId,
        buildFilterPayload()
      )
      setItems(result.items)
      setSummary(result.summary)
      if (result.items.length > 0) {
        const nextCaseId =
          selectedCaseId &&
          result.items.some((item) => item.case.id === selectedCaseId)
            ? selectedCaseId
            : result.items[0].case.id
        setSelectedCaseId(nextCaseId)
      } else {
        setSelectedCaseId(null)
        setDetail(null)
      }
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to fetch deal review cases'
      )
    } finally {
      setLoading(false)
    }
  }

  const loadDetail = async () => {
    if (!selectedTraderId || !selectedCaseId) {
      setDetail(null)
      return
    }
    setDetailLoading(true)
    try {
      const result = await api.getDealReviewCaseDetail(
        selectedTraderId,
        selectedCaseId
      )
      setDetail(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch deal detail'
      )
    } finally {
      setDetailLoading(false)
    }
  }

  const exportFilteredDealsJSON = () => {
    if (!selectedTraderId || displayedItems.length === 0) return
    downloadTextFile(
      `deal-review-${selectedTraderId}-${Date.now()}.json`,
      JSON.stringify(
        {
          trader_id: selectedTraderId,
          trader_name: selectedTrader?.trader_name || '',
          filters: buildFilterPayload(),
          review_queue: reviewQueueMode || 'all_filtered_deals',
          summary,
          items: displayedItems,
        },
        null,
        2
      ),
      'application/json;charset=utf-8'
    )
    notify.success('Filtered deal dataset exported as JSON')
  }

  const exportFilteredDealsCSV = () => {
    if (!selectedTraderId || displayedItems.length === 0) return
    downloadTextFile(
      `deal-review-${selectedTraderId}-${Date.now()}.csv`,
      buildDealReviewCasesCSV(displayedItems),
      'text/csv;charset=utf-8'
    )
    notify.success('Filtered deal dataset exported as CSV')
  }

  const exportScansJSON = () => {
    if (!selectedTraderId || scans.length === 0) return
    downloadTextFile(
      `deal-review-scans-${selectedTraderId}-${Date.now()}.json`,
      JSON.stringify(
        {
          trader_id: selectedTraderId,
          trader_name: selectedTrader?.trader_name || '',
          filters: buildFilterPayload(),
          scans,
        },
        null,
        2
      ),
      'application/json;charset=utf-8'
    )
    notify.success('Saved scan outputs exported as JSON')
  }

  const exportScanCompareJSON = () => {
    if (!selectedTraderId || !compareResult) return
    downloadTextFile(
      `deal-review-scan-compare-${selectedTraderId}-${Date.now()}.json`,
      JSON.stringify(
        {
          trader_id: selectedTraderId,
          left_scan_id: compareLeftScanId,
          right_scan_id: compareRightScanId,
          compare: compareResult,
        },
        null,
        2
      ),
      'application/json;charset=utf-8'
    )
    notify.success('Scan compare exported as JSON')
  }

  const loadScanHistory = async () => {
    if (!selectedTraderId) return
    try {
      const result = await api.getDealReviewAIScans(selectedTraderId, 8)
      setScans(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch AI scans'
      )
    }
  }

  const loadAnomalies = async () => {
    if (!selectedTraderId) return
    try {
      const result = await api.getDealReviewAnomalies(
        selectedTraderId,
        buildFilterPayload()
      )
      setAnomalies(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch anomalies'
      )
    }
  }

  const loadFilterPresets = async () => {
    if (!selectedTraderId) return
    try {
      const result = await api.getDealReviewFilterPresets(selectedTraderId)
      setFilterPresets(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch review presets'
      )
    }
  }

  const loadVersions = async () => {
    if (!selectedTraderId) return
    try {
      const result = await api.getDealReviewStrategyVersions(
        selectedTraderId,
        8
      )
      setVersions(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch strategy history'
      )
    }
  }

  const loadExchanges = async () => {
    try {
      const result = await api.getExchangeConfigs()
      setExchanges(result.filter((item) => item.enabled))
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch exchange wallets'
      )
    }
  }

  const loadChallengerCompares = async () => {
    if (!selectedTraderId) return
    try {
      const result = await api.getDealReviewChallengerCompares(
        selectedTraderId,
        8
      )
      setChallengerCompares(result)
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to fetch challenger history'
      )
    }
  }

  const loadChallengerCompareDetail = async () => {
    if (!selectedTraderId || !selectedCompareId) {
      setSelectedCompareDetail(null)
      return
    }
    setCompareDetailLoading(true)
    try {
      const result = await api.getDealReviewChallengerCompare(
        selectedTraderId,
        selectedCompareId
      )
      setSelectedCompareDetail(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch challenger detail'
      )
    } finally {
      setCompareDetailLoading(false)
    }
  }

  const scrollToCompareDetail = () => {
    window.setTimeout(() => {
      compareDetailRef.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
      })
    }, 80)
  }

  useEffect(() => {
    api
      .getModelConfigs()
      .then((result) => {
        setModels(result)
        const preferred =
          result.find((item) => item.enabled && item.provider === 'openai') ||
          result.find((item) => item.enabled)
        if (preferred) setSelectedModelId(preferred.id)
      })
      .catch(() => undefined)
    void loadExchanges()
  }, [])

  useEffect(() => {
    void loadCases()
  }, [
    selectedTraderId,
    symbol,
    side,
    status,
    outcome,
    openSelectionBucket,
    closeReason,
    dateRange,
    minPnl,
    maxPnl,
  ])

  useEffect(() => {
    void loadDetail()
  }, [selectedTraderId, selectedCaseId])

  useEffect(() => {
    if (!selectedCaseId || !comparePeerCaseId) return
    if (selectedCaseId === comparePeerCaseId) {
      setComparePeerCaseId('')
      setComparePeerDetail(null)
    }
  }, [selectedCaseId, comparePeerCaseId])

  useEffect(() => {
    if (!selectedTraderId || !comparePeerCaseId) {
      setComparePeerDetail(null)
      setComparePeerLoading(false)
      return
    }
    if (comparePeerCaseId === selectedCaseId) {
      setComparePeerDetail(null)
      setComparePeerLoading(false)
      return
    }

    let active = true
    setComparePeerLoading(true)
    void (async () => {
      try {
        if (detail?.case.id === comparePeerCaseId) {
          if (active) setComparePeerDetail(detail)
          return
        }
        const result = await api.getDealReviewCaseDetail(
          selectedTraderId,
          comparePeerCaseId
        )
        if (active) setComparePeerDetail(result)
      } catch (err) {
        if (active) {
          setComparePeerDetail(null)
          notify.error(
            err instanceof Error
              ? err.message
              : 'Failed to load compare deal detail'
          )
        }
      } finally {
        if (active) setComparePeerLoading(false)
      }
    })()

    return () => {
      active = false
    }
  }, [selectedTraderId, selectedCaseId, comparePeerCaseId, detail])

  useEffect(() => {
    void loadScanHistory()
  }, [selectedTraderId])

  useEffect(() => {
    void loadAnomalies()
  }, [
    selectedTraderId,
    symbol,
    side,
    status,
    outcome,
    openSelectionBucket,
    closeReason,
    dateRange,
    minPnl,
    maxPnl,
  ])

  useEffect(() => {
    if (!selectedTraderId) {
      setFilterPresets([])
      setSelectedPresetId('')
      setPresetName('')
      return
    }
    void loadFilterPresets()
  }, [selectedTraderId])

  useEffect(() => {
    void loadVersions()
  }, [selectedTraderId])

  useEffect(() => {
    void loadChallengerCompares()
  }, [selectedTraderId])

  useEffect(() => {
    if (challengerCompares.length === 0) {
      setSelectedCompareId(null)
      setSelectedCompareDetail(null)
      return
    }
    if (
      !selectedCompareId ||
      !challengerCompares.some((item) => item.compare.id === selectedCompareId)
    ) {
      setSelectedCompareId(challengerCompares[0].compare.id)
    }
  }, [challengerCompares, selectedCompareId])

  useEffect(() => {
    void loadChallengerCompareDetail()
  }, [selectedTraderId, selectedCompareId])

  useEffect(() => {
    if (versions.length === 0) {
      setSelectedVersionId(null)
      return
    }
    if (
      !selectedVersionId ||
      !versions.some((item) => item.version.id === selectedVersionId)
    ) {
      setSelectedVersionId(versions[0].version.id)
    }
  }, [versions, selectedVersionId])

  useEffect(() => {
    if (!selectedPreset) {
      return
    }
    setPresetName(selectedPreset.preset.name)
  }, [selectedPreset])

  useEffect(() => {
    const queueItems = buildReviewQueueItems(items, reviewQueueMode)
    if (queueItems.length === 0) {
      if (reviewQueueMode) {
        setSelectedCaseId(null)
        setDetail(null)
      }
      return
    }
    if (!selectedCaseId || !queueItems.some((item) => item.case.id === selectedCaseId)) {
      setSelectedCaseId(queueItems[0].case.id)
    }
  }, [items, reviewQueueMode, selectedCaseId])

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented) return
      if (event.metaKey || event.ctrlKey || event.altKey) return
      if (isKeyboardTypingTarget(event.target)) return
      if (displayedItems.length === 0) return

      if (event.key === 'ArrowDown' || event.key === 'j' || event.key === 'J') {
        event.preventDefault()
        goToReviewCase(1)
        return
      }
      if (event.key === 'ArrowUp' || event.key === 'k' || event.key === 'K') {
        event.preventDefault()
        goToReviewCase(-1)
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [displayedItems, selectedCaseId, selectedCaseIndex])

  useEffect(() => {
    if (selectedTrader?.exchange_id) {
      setLaunchExchangeId(selectedTrader.exchange_id)
      return
    }
    if (!launchExchangeId && exchanges[0]?.id) {
      setLaunchExchangeId(exchanges[0].id)
    }
  }, [selectedTrader?.exchange_id, exchanges, launchExchangeId])

  useEffect(() => {
    setReviewLabelsText(detail?.labels?.join(', ') || '')
    setReviewNote(detail?.case.analyst_note || '')
  }, [detail])

  useEffect(() => {
    setAIClassifierAssist(null)
  }, [selectedCaseId])

  useEffect(() => {
    if (!selectedModelId) {
      setAvailableRemoteModels([])
      setSelectedRemoteModel('')
      return
    }
    api
      .getAvailableRemoteModels(selectedModelId)
      .then((result) => {
        setAvailableRemoteModels(result)
        setSelectedRemoteModel(result[0]?.id || '')
      })
      .catch(() => {
        setAvailableRemoteModels([])
        setSelectedRemoteModel('')
      })
  }, [selectedModelId])

  useEffect(() => {
    if (scans.length === 0) {
      setCompareLeftScanId('')
      setCompareRightScanId('')
      setCompareResult(null)
      return
    }
    if (!compareLeftScanId) {
      setCompareLeftScanId(scans[0].scan.id)
    }
    if (!compareRightScanId && scans[1]) {
      setCompareRightScanId(scans[1].scan.id)
    }
  }, [scans, compareLeftScanId, compareRightScanId])

  useEffect(() => {
    if (
      !selectedTraderId ||
      !compareLeftScanId ||
      !compareRightScanId ||
      compareLeftScanId === compareRightScanId
    ) {
      setCompareResult(null)
      return
    }

    setCompareLoading(true)
    api
      .compareDealReviewAIScans(
        selectedTraderId,
        compareLeftScanId,
        compareRightScanId
      )
      .then(setCompareResult)
      .catch((err) => {
        notify.error(
          err instanceof Error ? err.message : 'Failed to compare AI scans'
        )
        setCompareResult(null)
      })
      .finally(() => setCompareLoading(false))
  }, [selectedTraderId, compareLeftScanId, compareRightScanId])

  const runAIScan = async (
    overrides?: Record<string, string | number | undefined>,
    successMessage: string = 'AI scan completed'
  ) => {
    if (!selectedTraderId) return
    setRunningScan(true)
    try {
      const result = await api.runDealReviewAIScan(selectedTraderId, {
        model_id: selectedModelId || undefined,
        override_model_name: selectedRemoteModel || undefined,
        ...buildFilterPayload(overrides),
      })
      setScans((current) =>
        [
          result,
          ...current.filter((item) => item.scan.id !== result.scan.id),
        ].slice(0, 8)
      )
      notify.success(successMessage)
      await mutate(`status-${selectedTraderId}`)
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'AI scan failed')
    } finally {
      setRunningScan(false)
    }
  }

  const saveFilterPreset = async () => {
    if (!selectedTraderId) return
    const trimmedName = presetName.trim()
    if (!trimmedName) {
      notify.error('Preset name is required')
      return
    }
    setSavingPreset(true)
    try {
      const nextItems = await api.saveDealReviewFilterPreset(selectedTraderId, {
        id:
          selectedPreset &&
          selectedPreset.preset.name.trim().toLowerCase() ===
            trimmedName.toLowerCase()
            ? selectedPreset.preset.id
            : undefined,
        name: trimmedName,
        filters: buildPresetFilters(),
      })
      setFilterPresets(nextItems)
      const savedPreset =
        nextItems.find(
          (item) => item.preset.name.trim().toLowerCase() === trimmedName.toLowerCase()
        ) || null
      setSelectedPresetId(savedPreset?.preset.id || '')
      notify.success(
        savedPreset && selectedPreset?.preset.id === savedPreset.preset.id
          ? 'Review preset updated'
          : 'Review preset saved'
      )
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to save review preset'
      )
    } finally {
      setSavingPreset(false)
    }
  }

  const deleteFilterPreset = async () => {
    if (!selectedTraderId || !selectedPreset) return
    const confirmed = await confirmToast(
      `Delete the preset "${selectedPreset.preset.name}"?`,
      {
        title: 'Delete Preset',
        okText: 'Delete',
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    setDeletingPresetId(selectedPreset.preset.id)
    try {
      await api.deleteDealReviewFilterPreset(selectedTraderId, selectedPreset.preset.id)
      setFilterPresets((current) =>
        current.filter((item) => item.preset.id !== selectedPreset.preset.id)
      )
      setSelectedPresetId('')
      setPresetName('')
      notify.success('Review preset deleted')
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to delete review preset'
      )
    } finally {
      setDeletingPresetId(null)
    }
  }

  const validateScan = async (scan: DealReviewAIScanDetail) => {
    if (!selectedTraderId) return
    setValidatingScanId(scan.scan.id)
    try {
      const result = await api.validateDealReviewAIScan(
        selectedTraderId,
        scan.scan.id
      )
      setScans((current) =>
        current.map((item) =>
          item.scan.id === result.scan.id ? result : item
        )
      )
      notify.success(
        result.validation?.status === 'passed'
          ? 'AI scan validation passed'
          : 'AI scan validation updated'
      )
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Validation failed')
    } finally {
      setValidatingScanId(null)
    }
  }

  const applyScan = async (scan: DealReviewAIScanDetail) => {
    if (!selectedTraderId) return
    const confirmed = await confirmToast(
      'Apply the strategy patch from this AI scan and reload the trader configuration?',
      {
        title: 'Apply AI Patch',
        okText: 'Apply',
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    setApplyingScanId(scan.scan.id)
    try {
      await api.applyDealReviewAIScan(selectedTraderId, scan.scan.id)
      notify.success('Strategy patch applied')
      await loadScanHistory()
      await loadVersions()
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to apply AI patch'
      )
    } finally {
      setApplyingScanId(null)
    }
  }

  const launchChallenger = async (scan: DealReviewAIScanDetail) => {
    if (!selectedTraderId || !launchExchangeId) return
    const confirmed = await confirmToast(
      `Launch a challenger trader in ${launchMode} mode for ${launchWindowHours}h using the selected wallet?`,
      {
        title: 'Launch Challenger',
        okText: 'Launch',
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    setLaunchingScanId(scan.scan.id)
    try {
      const detail = await api.launchDealReviewChallenger(
        selectedTraderId,
        scan.scan.id,
        {
          mode: launchMode,
          exchange_id: launchExchangeId,
          window_hours: Number(launchWindowHours),
        }
      )
      setSelectedCompareId(detail.compare.id)
      setSelectedCompareDetail(detail)
      notify.success('Challenger compare started')
      await loadScanHistory()
      await loadVersions()
      await loadChallengerCompares()
      await mutate(`status-${selectedTraderId}`)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to launch challenger'
      )
    } finally {
      setLaunchingScanId(null)
    }
  }

  const stopCompare = async (detail: DealReviewChallengerCompareDetail) => {
    if (!selectedTraderId) return
    const confirmed = await confirmToast(
      'Stop this running comparison and keep the incumbent trader active?',
      {
        title: 'Stop Compare',
        okText: 'Stop compare',
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    const actionKey = `stop:${detail.compare.id}`
    setCompareActionKey(actionKey)
    try {
      const result = await api.stopDealReviewChallengerCompare(
        selectedTraderId,
        detail.compare.id
      )
      setSelectedCompareDetail(result)
      await loadChallengerCompares()
      notify.success('Challenger compare stopped')
      await mutate(`status-${selectedTraderId}`)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to stop challenger compare'
      )
    } finally {
      setCompareActionKey(null)
    }
  }

  const openVersionCompare = async (version: DealReviewStrategyVersionDetail) => {
    if (!selectedTraderId) return
    const compareId = version.version.source_compare_id?.trim()
    if (!compareId) return

    const cached =
      challengerCompares.find((item) => item.compare.id === compareId) ||
      (selectedCompareDetail?.compare.id === compareId
        ? selectedCompareDetail
        : null)

    if (cached) {
      setSelectedCompareId(compareId)
      setSelectedCompareDetail(cached)
      scrollToCompareDetail()
      return
    }

    setOpeningCompareId(compareId)
    try {
      const detail = await api.getDealReviewChallengerCompare(
        selectedTraderId,
        compareId
      )
      setSelectedCompareId(compareId)
      setSelectedCompareDetail(detail)
      setChallengerCompares((current) => {
        const withoutCurrent = current.filter(
          (item) => item.compare.id !== detail.compare.id
        )
        return [detail, ...withoutCurrent]
      })
      scrollToCompareDetail()
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to open linked compare'
      )
    } finally {
      setOpeningCompareId(null)
    }
  }

  const resolveCompare = async (
    detail: DealReviewChallengerCompareDetail,
    winnerTraderId: string,
    winnerLabel: string
  ) => {
    if (!selectedTraderId) return
    const confirmed = await confirmToast(
      `Resolve this comparison manually in favor of the ${winnerLabel}? The other trader will be disabled automatically.`,
      {
        title: 'Resolve Compare',
        okText: `Keep ${winnerLabel}`,
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    const actionKey = `resolve:${detail.compare.id}:${winnerTraderId}`
    setCompareActionKey(actionKey)
    try {
      const result = await api.resolveDealReviewChallengerCompare(
        selectedTraderId,
        detail.compare.id,
        winnerTraderId
      )
      setSelectedCompareDetail(result)
      await loadChallengerCompares()
      notify.success('Challenger compare resolved')
      await mutate(`status-${selectedTraderId}`)
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to resolve challenger compare'
      )
    } finally {
      setCompareActionKey(null)
    }
  }

  const saveCaseReview = async () => {
    if (!selectedTraderId || !selectedCaseId) return
    setSavingReview(true)
    try {
      const labels = reviewLabelsText
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean)
      const result = await api.updateDealReviewCaseReview(
        selectedTraderId,
        selectedCaseId,
        {
          labels,
          analyst_note: reviewNote,
        }
      )
      setDetail(result)
      setItems((current) =>
        current.map((item) =>
          item.case.id === selectedCaseId ? { ...item, labels } : item
        )
      )
      await loadCases()
      notify.success('Deal review notes saved')
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to save deal review notes'
      )
    } finally {
      setSavingReview(false)
    }
  }

  const runCaseAIAssist = async () => {
    if (!selectedTraderId || !selectedCaseId) return
    setRunningCaseAIAssist(true)
    try {
      const result = await api.runDealReviewCaseAIAssist(
        selectedTraderId,
        selectedCaseId,
        {
          model_id: selectedModelId || undefined,
          override_model_name: selectedRemoteModel || undefined,
        }
      )
      setAIClassifierAssist(result)
      notify.success('AI review assist updated')
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to run AI review assist'
      )
    } finally {
      setRunningCaseAIAssist(false)
    }
  }

  const applyClassifierFeedback = async (
    suggestion: DealReviewClassifierSuggestion,
    verdict: 'accepted' | 'rejected',
    source: 'heuristic' | 'ai'
  ) => {
    if (!selectedTraderId || !selectedCaseId) return
    const actionKey = `${source}:${verdict}:${suggestion.suggestion_key}`
    setClassifierActionKey(actionKey)
    try {
      const result = await api.applyDealReviewClassifierFeedback(
        selectedTraderId,
        selectedCaseId,
        {
          classifier_id: suggestion.classifier_id,
          suggestion_key: suggestion.suggestion_key,
          label: suggestion.label,
          issue_type: suggestion.issue_type,
          verdict,
          rationale: suggestion.rationale,
          apply_label: verdict === 'accepted',
        }
      )
      setDetail(result)
      await loadCases()
      if (source === 'ai') {
        setAIClassifierAssist((current) => {
          if (!current?.suggestions) return current
          const nextSuggestions = current.suggestions.filter(
            (item) => item.suggestion_key !== suggestion.suggestion_key
          )
          return {
            ...current,
            suggestions: nextSuggestions,
            summary:
              nextSuggestions.length > 0
                ? current.summary
                : verdict === 'accepted'
                  ? 'AI review assist suggestion accepted and applied to the deal labels.'
                  : 'AI review assist suggestion rejected for this deal.',
          }
        })
      }
      notify.success(
        verdict === 'accepted'
          ? `Accepted "${suggestion.label}"`
          : `Rejected "${suggestion.label}"`
      )
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to apply classifier feedback'
      )
    } finally {
      setClassifierActionKey(null)
    }
  }

  const rollbackVersion = async (version: DealReviewStrategyVersionDetail) => {
    if (!selectedTraderId) return
    const confirmed = await confirmToast(
      'Rollback the active strategy to the previous config captured in this version?',
      {
        title: 'Rollback Strategy',
        okText: 'Rollback',
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    setRollingBackVersionId(version.version.id)
    try {
      await api.rollbackDealReviewStrategyVersion(
        selectedTraderId,
        version.version.id
      )
      notify.success('Strategy rollback completed')
      await loadVersions()
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to rollback strategy'
      )
    } finally {
      setRollingBackVersionId(null)
    }
  }

  const openVersionDetail = (versionId: string) => {
    setSelectedVersionId(versionId)
    window.setTimeout(() => {
      versionDetailRef.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
      })
    }, 50)
  }

  const goToReviewCase = (direction: -1 | 1) => {
    if (displayedItems.length === 0) return
    const currentIndex =
      selectedCaseIndex >= 0 ? selectedCaseIndex : direction > 0 ? -1 : 1
    const nextIndex = Math.min(
      displayedItems.length - 1,
      Math.max(0, currentIndex + direction)
    )
    const nextCase = displayedItems[nextIndex]
    if (nextCase && nextCase.case.id !== selectedCaseId) {
      setSelectedCaseId(nextCase.case.id)
    }
  }

  if (tradersError) {
    return (
      <div className="p-8 text-red-400">
        Failed to load traders: {tradersError.message}
      </div>
    )
  }

  return (
    <DeepVoidBackground className="min-h-screen pb-12" disableAnimation>
      <div className="w-full px-4 md:px-8 relative z-10 pt-6 space-y-6">
        <div className="nofx-glass rounded-xl p-6">
          <div className="flex flex-col lg:flex-row lg:items-end gap-4">
            <div className="flex-1">
              <div className="text-xs uppercase tracking-[0.24em] text-nofx-gold/80 mb-2">
                Deal Review
              </div>
              <h1 className="text-3xl font-semibold text-nofx-text-main">
                Open/Close rationale per deal
              </h1>
              <p className="text-sm text-nofx-text-muted mt-2">
                Review one compact open entry and one close entry per deal, then
                run AI scans against the filtered dataset.
              </p>
            </div>
            <div className="w-full lg:w-80">
              <label className="text-xs text-nofx-text-muted block mb-2">
                Trader
              </label>
              <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center nofx-glass">
                <NofxSelect
                  value={selectedTraderId || ''}
                  onChange={onTraderSelect}
                  options={(traders || []).map((item) => ({
                    value: item.trader_id,
                    label: item.trader_name,
                  }))}
                />
              </div>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-1 xl:grid-cols-[1.2fr_0.8fr] gap-6">
          <div className="space-y-6">
            <div className="nofx-glass rounded-xl p-5">
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3">
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={selectedPresetId}
                    onChange={(value) => {
                      setSelectedPresetId(value)
                      if (!value) {
                        setPresetName('')
                        return
                      }
                      const preset =
                        filterPresets.find((item) => item.preset.id === value) ||
                        null
                      if (preset) {
                        applyPresetFilters(preset.filters)
                        setPresetName(preset.preset.name)
                      }
                    }}
                    options={[
                      { value: '', label: 'Load saved cohort preset' },
                      ...filterPresets.map((item) => ({
                        value: item.preset.id,
                        label: item.preset.name,
                      })),
                    ]}
                  />
                </div>
                <input
                  value={presetName}
                  onChange={(event) => setPresetName(event.target.value)}
                  placeholder="Preset name"
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <button
                  onClick={saveFilterPreset}
                  disabled={!selectedTraderId || savingPreset}
                  className="h-11 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 text-nofx-gold font-semibold disabled:opacity-50"
                >
                  {savingPreset
                    ? 'Saving…'
                    : selectedPreset &&
                        selectedPreset.preset.name.trim().toLowerCase() ===
                          presetName.trim().toLowerCase()
                      ? 'Update preset'
                      : 'Save preset'}
                </button>
                <button
                  onClick={deleteFilterPreset}
                  disabled={!selectedPreset || deletingPresetId === selectedPreset?.preset.id}
                  className="h-11 rounded-lg border border-white/10 bg-black/20 font-semibold disabled:opacity-50"
                >
                  {deletingPresetId === selectedPreset?.preset.id
                    ? 'Deleting…'
                    : 'Delete preset'}
                </button>
              </div>

              <div className="flex flex-wrap gap-2 mt-3">
                {[
                  {
                    label: 'Trend + high vol',
                    patch: {
                      open_trend_regime: 'uptrend',
                      open_volatility_regime: 'high_vol',
                    },
                  },
                  {
                    label: 'Low-vol chop',
                    patch: {
                      open_trend_regime: 'chop',
                      open_volatility_regime: 'low_vol',
                    },
                  },
                  {
                    label: 'BTC-leading alt weakness',
                    patch: { open_btc_strength_regime: 'lagging' },
                  },
                  {
                    label: 'Funding extreme longs',
                    patch: { open_funding_regime: 'extreme_longs' },
                  },
                  { label: 'Asia session', patch: { open_session_bucket: 'asia' } },
                  { label: 'EU session', patch: { open_session_bucket: 'eu' } },
                  { label: 'US session', patch: { open_session_bucket: 'us' } },
                ].map((chip) => (
                  <button
                    key={chip.label}
                    onClick={() => applyDrilldownFilters(chip.patch)}
                    className="h-8 px-3 rounded-full border border-white/10 bg-black/20 text-xs"
                  >
                    {chip.label}
                  </button>
                ))}
                <button
                  onClick={() => {
                    setOpenTrendRegime('')
                    setOpenVolatilityRegime('')
                    setOpenBTCStrengthRegime('')
                    setOpenFundingRegime('')
                    setOpenOIRegime('')
                    setOpenSessionBucket('')
                    setOpenWeekdayBucket('')
                    setOpenVenueTier('')
                    setOpenLiquidityTier('')
                    setOpenSpreadBucket('')
                    setOpenSlippageBucket('')
                  }}
                  className="h-8 px-3 rounded-full border border-white/10 bg-black/20 text-xs text-nofx-text-muted"
                >
                  Reset regime filters
                </button>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 xl:grid-cols-8 gap-3 mt-3">
                <input
                  value={symbol}
                  onChange={(event) => setSymbol(event.target.value)}
                  placeholder="Symbol"
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={side}
                    onChange={setSide}
                    options={sideOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={status}
                    onChange={setStatus}
                    options={statusOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={outcome}
                    onChange={setOutcome}
                    options={outcomeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={dateRange}
                    onChange={setDateRange}
                    options={rangeOptions}
                  />
                </div>
                <input
                  value={openSelectionBucket}
                  onChange={(event) =>
                    setOpenSelectionBucket(event.target.value)
                  }
                  placeholder="Selection bucket"
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <input
                  value={closeReason}
                  onChange={(event) => setCloseReason(event.target.value)}
                  placeholder="Close reason"
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <button
                  onClick={() => {
                    void loadCases()
                    void loadAnomalies()
                  }}
                  className="h-11 rounded-lg bg-nofx-gold text-black font-semibold hover:opacity-90 transition"
                >
                  Refresh
                </button>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-6 gap-3 mt-3">
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openTrendRegime}
                    onChange={setOpenTrendRegime}
                    options={trendRegimeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openVolatilityRegime}
                    onChange={setOpenVolatilityRegime}
                    options={volatilityRegimeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openBTCStrengthRegime}
                    onChange={setOpenBTCStrengthRegime}
                    options={btcStrengthOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openFundingRegime}
                    onChange={setOpenFundingRegime}
                    options={fundingRegimeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openOIRegime}
                    onChange={setOpenOIRegime}
                    options={oiRegimeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openSessionBucket}
                    onChange={setOpenSessionBucket}
                    options={sessionOptions}
                  />
                </div>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3 mt-3">
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openWeekdayBucket}
                    onChange={setOpenWeekdayBucket}
                    options={weekdayOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openVenueTier}
                    onChange={setOpenVenueTier}
                    options={venueTierOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openLiquidityTier}
                    onChange={setOpenLiquidityTier}
                    options={liquidityTierOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openSpreadBucket}
                    onChange={setOpenSpreadBucket}
                    options={executionBucketOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openSlippageBucket}
                    onChange={setOpenSlippageBucket}
                    options={executionBucketOptions}
                  />
                </div>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mt-3">
                <input
                  value={minPnl}
                  onChange={(event) => setMinPnl(event.target.value)}
                  placeholder="Min PnL"
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <input
                  value={maxPnl}
                  onChange={(event) => setMaxPnl(event.target.value)}
                  placeholder="Max PnL"
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={reviewQueueMode}
                    onChange={setReviewQueueMode}
                    options={reviewQueueOptions}
                  />
                </div>
              </div>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Deals</div>
                <div className="text-2xl font-semibold">
                  {summary?.total_deals || 0}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Win Rate</div>
                <div className="text-2xl font-semibold">
                  {summary ? `${summary.win_rate.toFixed(1)}%` : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Net PnL</div>
                <div
                  className={`text-2xl font-semibold ${summary && summary.net_pnl >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}
                >
                  {summary ? formatMoney(summary.net_pnl) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Avg Hold</div>
                <div className="text-2xl font-semibold">
                  {summary ? formatHold(summary.avg_hold_ms) : '-'}
                </div>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3">
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Bad entries</div>
                <div className="text-2xl font-semibold mt-1">
                  {summary?.bad_entry_deals || 0}
                </div>
                <div className="text-xs text-nofx-text-muted mt-2">
                  Avg entry score {summary ? formatScore(summary.avg_entry_timing_score) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Bad exits</div>
                <div className="text-2xl font-semibold mt-1">
                  {summary?.bad_exit_deals || 0}
                </div>
                <div className="text-xs text-nofx-text-muted mt-2">
                  Avg exit score {summary ? formatScore(summary.avg_exit_efficiency_score) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Avoidable losses</div>
                <div className="text-2xl font-semibold mt-1">
                  {summary?.avoidable_loss_deals || 0}
                </div>
                <div className="text-xs text-nofx-text-muted mt-2">
                  Avg give-back {summary ? formatPct(summary.avg_profit_given_back_pct) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Strong entry / weak exit</div>
                <div className="text-2xl font-semibold mt-1">
                  {summary?.strong_entry_weak_exit_deals || 0}
                </div>
                <div className="text-xs text-nofx-text-muted mt-2">
                  Avg MFE capture {summary ? formatPct(summary.avg_mfe_captured_pct) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Weak entry / lucky exit</div>
                <div className="text-2xl font-semibold mt-1">
                  {summary?.weak_entry_lucky_exit_deals || 0}
                </div>
                <div className="text-xs text-nofx-text-muted mt-2">
                  Avg sizing score {summary ? formatScore(summary.avg_risk_sizing_score) : '-'}
                </div>
              </div>
            </div>

            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h2 className="font-semibold text-lg">
                    What changed after last patch?
                  </h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    Fast readout from the most recent strategy version and its
                    observed attribution window.
                  </p>
                </div>
                {latestVersion && (
                  <button
                    onClick={() => openVersionDetail(latestVersion.version.id)}
                    className="h-10 px-4 rounded-lg border border-nofx-gold/30 text-nofx-gold font-semibold"
                  >
                    Open patch detail
                  </button>
                )}
              </div>

              {!latestVersion ? (
                <div className="text-sm text-nofx-text-muted mt-4">
                  No strategy patch has been applied yet for this trader.
                </div>
              ) : (
                <div className="space-y-4 mt-4">
                  <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-gold">
                      {formatStrategyVersionSourceType(
                        latestVersion.version.source_type
                      )}
                    </div>
                    <div className="font-semibold mt-2">
                      {latestVersion.version.summary || 'Strategy change'}
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-2">
                      Applied{' '}
                      {new Date(
                        latestVersion.version.applied_at ||
                          latestVersion.version.created_at
                      ).toLocaleString()}
                    </div>
                    {latestVersion.version.expected_effect && (
                      <div className="text-sm text-nofx-text-muted mt-3">
                        Intended effect: {latestVersion.version.expected_effect}
                      </div>
                    )}
                    {latestVersion.attribution?.note && (
                      <div className="text-sm text-nofx-text-muted mt-3">
                        {latestVersion.attribution.note}
                      </div>
                    )}
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3">
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">Status</div>
                      <div
                        className={`text-lg font-semibold mt-1 ${getPatchOutcomeTone(
                          latestFullDelta,
                          latestVersion.attribution?.rollback_suggested
                        )}`}
                      >
                        {getPatchOutcomeLabel(
                          latestFullDelta,
                          latestVersion.attribution?.rollback_suggested
                        )}
                      </div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        Full-strategy delta
                      </div>
                      <div
                        className={`text-lg font-semibold mt-1 ${getPatchOutcomeTone(
                          latestFullDelta
                        )}`}
                      >
                        {latestFullDelta === null
                          ? 'No observation yet'
                          : formatMoney(latestFullDelta)}
                      </div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        Target-cohort delta
                      </div>
                      <div
                        className={`text-lg font-semibold mt-1 ${getPatchOutcomeTone(
                          latestTargetDelta
                        )}`}
                      >
                        {latestTargetDelta === null
                          ? 'Not cohort-scoped'
                          : formatMoney(latestTargetDelta)}
                      </div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        Rollback suggestion
                      </div>
                      <div
                        className={`text-lg font-semibold mt-1 ${
                          latestVersion.attribution?.rollback_suggested
                            ? 'text-rose-300'
                            : 'text-emerald-300'
                        }`}
                      >
                        {latestVersion.attribution?.rollback_suggested
                          ? 'Suggested'
                          : 'Not suggested'}
                      </div>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        Full before
                      </div>
                      <div className="text-sm text-nofx-text-muted mt-1">
                        {formatDatasetMini(
                          latestVersion.attribution?.full_before_summary
                        )}
                      </div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        Full after
                      </div>
                      <div className="text-sm text-nofx-text-muted mt-1">
                        {formatDatasetMini(
                          latestVersion.attribution?.full_after_summary
                        )}
                      </div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        Target cohort
                      </div>
                      <div className="text-sm text-nofx-text-muted mt-1">
                        {formatVersionCohort(latestVersion.target_cohort)}
                      </div>
                    </div>
                  </div>

                  {latestVersion.attribution?.warnings &&
                    latestVersion.attribution.warnings.length > 0 && (
                      <div className="rounded-xl border border-rose-400/20 bg-rose-500/10 p-4 text-sm text-rose-200">
                        {latestVersion.attribution.warnings[0]}
                      </div>
                    )}
                </div>
              )}
            </div>

            <div className="nofx-glass rounded-xl overflow-hidden">
              <div className="px-5 py-4 border-b border-white/10 flex items-center justify-between">
                <div>
                  <h2 className="font-semibold text-lg">Filtered deals</h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    {selectedTrader
                      ? `Trader: ${selectedTrader.trader_name}`
                      : 'Select a trader'}
                  </p>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    Queue: {buildQueueLabel(reviewQueueMode)} · {displayedItems.length}{' '}
                    visible / {items.length} loaded
                  </p>
                </div>
                <div className="text-right">
                  <div className="flex items-center justify-end gap-2 mb-2">
                    <button
                      onClick={exportFilteredDealsJSON}
                      disabled={displayedItems.length === 0}
                      className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs disabled:opacity-40"
                    >
                      Export JSON
                    </button>
                    <button
                      onClick={exportFilteredDealsCSV}
                      disabled={displayedItems.length === 0}
                      className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs disabled:opacity-40"
                    >
                      Export CSV
                    </button>
                  </div>
                  <div className="text-xs text-nofx-text-muted">
                    Hotkeys: `J` / `K` or `↑` / `↓`
                  </div>
                  {loading && (
                    <div className="text-xs text-nofx-text-muted mt-1">Loading…</div>
                  )}
                </div>
              </div>
              {error ? (
                <div className="p-5 text-rose-400">{error}</div>
              ) : displayedItems.length === 0 ? (
                <div className="p-5 text-nofx-text-muted">
                  {reviewQueueMode
                    ? 'No deals matched the current review queue.'
                    : 'No deals matched the current filters.'}
                </div>
              ) : (
                <div className="divide-y divide-white/5">
                  {displayedItems.map((item) => {
                    const topSuggestion = item.classifier_assist?.suggestions?.[0]
                    const assistClasses = classifierToneClasses(
                      item.classifier_assist?.highlight_level
                    )
                    const giveBackPct = getDealGiveBackPct(item.case)
                    const qualityBadges = buildQualityBadges(item.case)
                    return (
                      <button
                        key={item.case.id}
                        onClick={() => setSelectedCaseId(item.case.id)}
                        className={`w-full text-left px-5 py-4 transition hover:bg-white/5 ${
                          selectedCaseId === item.case.id ? 'bg-white/5' : ''
                        } ${
                          item.classifier_assist?.suggestions?.length
                            ? 'border-l-2 border-l-amber-400/40'
                            : ''
                        }`}
                      >
                        <div className="flex items-start justify-between gap-4">
                          <div>
                            <div className="flex items-center gap-2">
                              <span className="font-semibold">
                                {item.case.symbol}
                              </span>
                              <span
                                className={`text-xs px-2 py-0.5 rounded-full ${item.case.side === 'LONG' ? 'bg-emerald-500/15 text-emerald-300' : 'bg-rose-500/15 text-rose-300'}`}
                              >
                                {item.case.side}
                              </span>
                              <span className="text-xs text-nofx-text-muted">
                                {item.case.outcome}
                              </span>
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-2">
                              Open: {formatDate(item.case.entry_time_ms)} | Close:{' '}
                              {formatDate(item.case.exit_time_ms)}
                            </div>
                            <div className="text-sm text-nofx-text-muted mt-2 line-clamp-2">
                              {item.open_reasoning ||
                                'No open reasoning snapshot linked yet.'}
                            </div>
                            {buildOpenRegimeBadges(item).length > 0 && (
                              <div className="flex flex-wrap gap-2 mt-3">
                                {buildOpenRegimeBadges(item).slice(0, 5).map((badge) => (
                                  <span
                                    key={`${item.case.id}-regime-${badge}`}
                                    className="px-2 py-1 rounded-full text-[11px] bg-sky-500/10 border border-sky-400/20 text-sky-200"
                                  >
                                    {formatMarketContextToken(badge)}
                                  </span>
                                ))}
                              </div>
                            )}
                            {topSuggestion && (
                              <div className="mt-3 flex flex-wrap items-center gap-2">
                                <span
                                  className={`px-2 py-1 rounded-full text-[11px] border ${assistClasses}`}
                                >
                                  Review assist
                                </span>
                                <span className="text-xs text-amber-200">
                                  {topSuggestion.label}
                                </span>
                                <span className="text-xs text-nofx-text-muted">
                                  {formatClassifierIssueType(
                                    topSuggestion.issue_type
                                  )}
                                </span>
                              </div>
                            )}
                            {qualityBadges.length > 0 && (
                              <div className="flex flex-wrap gap-2 mt-3">
                                {qualityBadges.map((badge) => (
                                  <span
                                    key={`${item.case.id}-quality-${badge.label}`}
                                    className={`px-2 py-1 rounded-full text-[11px] border ${qualityBadgeClasses(
                                      badge.tone
                                    )}`}
                                  >
                                    {badge.label}
                                  </span>
                                ))}
                              </div>
                            )}
                            {item.labels && item.labels.length > 0 && (
                              <div className="flex flex-wrap gap-2 mt-3">
                                {item.labels.slice(0, 3).map((label) => (
                                  <span
                                    key={`${item.case.id}-${label}`}
                                    className="px-2 py-1 rounded-full text-[11px] bg-nofx-gold/10 border border-nofx-gold/20 text-nofx-gold"
                                  >
                                    {label}
                                  </span>
                                ))}
                              </div>
                            )}
                            {reviewQueueMode === 'biggest_giveback' &&
                              giveBackPct > 0 && (
                                <div className="mt-2 text-xs text-amber-300">
                                  Give-back {formatPct(giveBackPct)}
                                  {' · '}Peak{' '}
                                  {formatPct(
                                    item.case.max_favorable_excursion_pct || 0
                                  )}
                                </div>
                              )}
                          </div>
                          <div className="text-right">
                            <div
                              className={
                                item.case.realized_pnl >= 0
                                  ? 'text-emerald-400 font-semibold'
                                  : 'text-rose-400 font-semibold'
                              }
                            >
                              {formatMoney(item.case.realized_pnl)}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {formatPct(item.case.realized_pnl_pct)}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-2">
                              {formatHold(item.case.hold_duration_ms)}
                            </div>
                          </div>
                        </div>
                      </button>
                    )
                  })}
                </div>
              )}
            </div>
          </div>

          <div className="space-y-6">
            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4 mb-4">
                <div>
                  <h2 className="font-semibold text-lg">Deal detail</h2>
                  {detail?.case && (
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {detail.trader_name}{' '}
                      {detail.strategy_name ? `| ${detail.strategy_name}` : ''}
                    </div>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => goToReviewCase(-1)}
                    disabled={selectedCaseIndex <= 0}
                    className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-sm disabled:opacity-40"
                  >
                    Previous
                  </button>
                  <button
                    onClick={() => goToReviewCase(1)}
                    disabled={
                      selectedCaseIndex < 0 ||
                      selectedCaseIndex >= displayedItems.length - 1
                    }
                    className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-sm disabled:opacity-40"
                  >
                    Next
                  </button>
                </div>
              </div>
              {detailLoading ? (
                <div className="text-nofx-text-muted">Loading detail…</div>
              ) : !detail ? (
                <div className="text-nofx-text-muted">
                  Select a deal to inspect its open and close rationale.
                </div>
              ) : (
                <div className="space-y-5">
                  <div className="grid grid-cols-2 gap-3 text-sm">
                    <div>
                      <span className="text-nofx-text-muted">Symbol</span>
                      <div className="font-semibold mt-1">
                        {detail.case.symbol}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">Side</span>
                      <div className="font-semibold mt-1">
                        {detail.case.side}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">Outcome</span>
                      <div className="font-semibold mt-1">
                        {detail.case.outcome}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">Hold</span>
                      <div className="font-semibold mt-1">
                        {formatHold(detail.case.hold_duration_ms)}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">Entry</span>
                      <div className="font-semibold mt-1">
                        {formatMoney(detail.case.entry_price)} @{' '}
                        {formatDate(detail.case.entry_time_ms)}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">Exit</span>
                      <div className="font-semibold mt-1">
                        {detail.case.exit_time_ms
                          ? `${formatMoney(detail.case.exit_price)} @ ${formatDate(detail.case.exit_time_ms)}`
                          : '-'}
                      </div>
                    </div>
                    <div className="col-span-2">
                      <span className="text-nofx-text-muted">PnL</span>
                      <div
                        className={
                          detail.case.realized_pnl >= 0
                            ? 'text-emerald-400 font-semibold mt-1'
                            : 'text-rose-400 font-semibold mt-1'
                        }
                      >
                        {formatMoney(detail.case.realized_pnl)} /{' '}
                        {formatPct(detail.case.realized_pnl_pct)}
                      </div>
                    </div>
                  </div>

                  {buildOpenRegimeBadges(detail).length > 0 && (
                    <div className="flex flex-wrap gap-2">
                      {buildOpenRegimeBadges(detail).map((badge) => (
                        <span
                          key={`${detail.case.id}-detail-regime-${badge}`}
                          className="px-2 py-1 rounded-full text-xs bg-sky-500/10 border border-sky-400/20 text-sky-200"
                        >
                          {formatMarketContextToken(badge)}
                        </span>
                      ))}
                    </div>
                  )}

                  {buildQualityBadges(detail.case).length > 0 && (
                    <div className="flex flex-wrap gap-2">
                      {buildQualityBadges(detail.case).map((badge) => (
                        <span
                          key={`${detail.case.id}-detail-quality-${badge.label}`}
                          className={`px-2 py-1 rounded-full text-xs border ${qualityBadgeClasses(
                            badge.tone
                          )}`}
                        >
                          {badge.label}
                        </span>
                      ))}
                    </div>
                  )}

                  <DealReviewTimelineChart
                    timeline={detail.price_timeline}
                    entryPrice={detail.case.entry_price}
                    side={detail.case.side}
                    stopLoss={detail.case.open_stop_loss}
                    takeProfit={detail.case.open_take_profit}
                  />

                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <div className="font-semibold">Quality readout</div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          Entry, exit, and sizing quality derived from the full
                          deal path.
                        </div>
                      </div>
                      <div className="text-right text-xs text-nofx-text-muted">
                        MFE capture {formatPct(detail.case.mfe_captured_pct || 0)}
                      </div>
                    </div>

                    {buildQualityNarrative(detail.case) && (
                      <div className="rounded-lg border border-amber-400/20 bg-amber-500/10 px-3 py-2 text-sm text-amber-100">
                        {buildQualityNarrative(detail.case)}
                      </div>
                    )}

                    <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">Entry timing score</div>
                        <div className="text-lg font-semibold mt-1">
                          {formatScore(detail.case.entry_timing_score)}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-2">
                          First profit {formatHold(detail.case.time_to_first_profit_ms)}
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">Exit efficiency score</div>
                        <div className="text-lg font-semibold mt-1">
                          {formatScore(detail.case.exit_efficiency_score)}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-2">
                          Give-back {formatMoney(detail.case.profit_given_back)} /{' '}
                          {formatPct(detail.case.profit_given_back_pct)}
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">Risk sizing score</div>
                        <div className="text-lg font-semibold mt-1">
                          {formatScore(detail.case.risk_sizing_score)}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-2">
                          Planned risk {formatPct(detail.case.planned_risk_pct)}
                        </div>
                      </div>
                    </div>

                    <div className="grid grid-cols-2 xl:grid-cols-4 gap-3 text-sm">
                      <div>
                        <div className="text-xs text-nofx-text-muted">Max favorable excursion</div>
                        <div className="font-semibold mt-1">
                          {formatMoney(detail.case.max_favorable_excursion)} /{' '}
                          {formatPct(detail.case.max_favorable_excursion_pct)}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">Max adverse excursion</div>
                        <div className="font-semibold mt-1 text-rose-300">
                          {formatMoney(detail.case.max_adverse_excursion)} /{' '}
                          {formatPct(detail.case.max_adverse_excursion_pct)}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">Time to first profit</div>
                        <div className="font-semibold mt-1">
                          {detail.case.time_to_first_profit_ms
                            ? formatHold(detail.case.time_to_first_profit_ms)
                            : 'Never'}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">Time to max drawdown</div>
                        <div className="font-semibold mt-1">
                          {detail.case.time_to_max_drawdown_ms
                            ? formatHold(detail.case.time_to_max_drawdown_ms)
                            : '-'}
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <div className="font-semibold">Deal compare</div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          Compare the current deal against another loaded case
                          side by side.
                        </div>
                      </div>
                      <div className="w-full max-w-xs h-10 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                        <NofxSelect
                          value={comparePeerCaseId}
                          onChange={setComparePeerCaseId}
                          options={[
                            { value: '', label: 'Select compare deal' },
                            ...displayedItems
                              .filter((item) => item.case.id !== detail.case.id)
                              .map((item) => ({
                                value: item.case.id,
                                label: `${item.case.symbol} · ${item.case.side} · ${formatMoney(item.case.realized_pnl)}`,
                              })),
                          ]}
                        />
                      </div>
                    </div>

                    {!comparePeerCaseId ? (
                      <div className="text-sm text-nofx-text-muted">
                        Choose another visible deal to compare it against the
                        currently selected one.
                      </div>
                    ) : comparePeerLoading ? (
                      <div className="text-sm text-nofx-text-muted">
                        Loading compare deal…
                      </div>
                    ) : !comparePeerDetail ? (
                      <div className="text-sm text-nofx-text-muted">
                        No compare deal loaded yet.
                      </div>
                    ) : (
                      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                        {[detail, comparePeerDetail].map((entry, index) => {
                          const maxGiveBackPct = getDealGiveBackPct(entry.case)
                          const tone =
                            entry.case.realized_pnl >= 0
                              ? 'text-emerald-400'
                              : 'text-rose-400'
                          return (
                            <div
                              key={`${entry.case.id}-${index}`}
                              className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3"
                            >
                              <div className="flex items-start justify-between gap-3">
                                <div>
                                  <div className="text-xs text-nofx-text-muted">
                                    {index === 0 ? 'Current deal' : 'Compare deal'}
                                  </div>
                                  <div className="font-semibold mt-1">
                                    {entry.case.symbol} · {entry.case.side}
                                  </div>
                                  <div className="text-xs text-nofx-text-muted mt-1">
                                    {entry.trader_name}
                                  </div>
                                </div>
                                {index === 1 && (
                                  <button
                                    onClick={() => setSelectedCaseId(entry.case.id)}
                                    className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs"
                                  >
                                    Focus deal
                                  </button>
                                )}
                              </div>

                              <div className="grid grid-cols-2 gap-3 text-xs">
                                <div>
                                  Outcome:{' '}
                                  <span className="text-white">
                                    {entry.case.outcome}
                                  </span>
                                </div>
                                <div>
                                  Hold:{' '}
                                  <span className="text-white">
                                    {formatHold(entry.case.hold_duration_ms)}
                                  </span>
                                </div>
                                <div>
                                  PnL:{' '}
                                  <span className={tone}>
                                    {formatMoney(entry.case.realized_pnl)} /{' '}
                                    {formatPct(entry.case.realized_pnl_pct)}
                                  </span>
                                </div>
                                <div>
                                  Close reason:{' '}
                                  <span className="text-white">
                                    {entry.case.close_reason || '-'}
                                  </span>
                                </div>
                                <div>
                                  Entry:{' '}
                                  <span className="text-white">
                                    {formatMoney(entry.case.entry_price)}
                                  </span>
                                </div>
                                <div>
                                  Exit:{' '}
                                  <span className="text-white">
                                    {entry.case.exit_price
                                      ? formatMoney(entry.case.exit_price)
                                      : '-'}
                                  </span>
                                </div>
                                <div>
                                  Peak MFE:{' '}
                                  <span className="text-emerald-300">
                                    {formatPct(entry.case.max_favorable_excursion_pct || 0)}
                                  </span>
                                </div>
                                <div>
                                  Max adverse:{' '}
                                  <span className="text-rose-300">
                                    {formatPct(entry.case.max_adverse_excursion_pct || 0)}
                                  </span>
                                </div>
                                <div>
                                  Give-back:{' '}
                                  <span className="text-amber-300">
                                    {formatPct(maxGiveBackPct)}
                                  </span>
                                </div>
                                <div>
                                  Exit efficiency:{' '}
                                  <span className="text-white">
                                    {formatScore(entry.case.exit_efficiency_score)}
                                  </span>
                                </div>
                                <div>
                                  Entry timing:{' '}
                                  <span className="text-white">
                                    {formatScore(entry.case.entry_timing_score)}
                                  </span>
                                </div>
                                <div>
                                  Risk sizing:{' '}
                                  <span className="text-white">
                                    {formatScore(entry.case.risk_sizing_score)}
                                  </span>
                                </div>
                                <div>
                                  Path points:{' '}
                                  <span className="text-white">
                                    {entry.price_timeline?.summary.point_count || 0}
                                  </span>
                                </div>
                                <div>
                                  Cycle / platform:{' '}
                                  <span className="text-white">
                                    {entry.price_timeline?.summary.cycle_samples || 0} /{' '}
                                    {entry.price_timeline?.summary.platform_samples || 0}
                                  </span>
                                </div>
                                <div>
                                  Labels:{' '}
                                  <span className="text-white">
                                    {(entry.labels || []).length
                                      ? entry.labels?.join(', ')
                                      : '-'}
                                  </span>
                                </div>
                              </div>

                              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                  Open rationale
                                </div>
                                <div className="text-sm text-nofx-text-muted mt-2">
                                  {formatDealReasonPreview(entry.open?.event.reasoning)}
                                </div>
                              </div>

                              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                  Close rationale
                                </div>
                                <div className="text-sm text-nofx-text-muted mt-2">
                                  {formatDealReasonPreview(entry.close?.event.reasoning)}
                                </div>
                              </div>
                            </div>
                          )
                        })}
                      </div>
                    )}
                  </div>

                  {detail.open_candidate_sources &&
                    detail.open_candidate_sources.length > 0 && (
                      <div>
                        <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                          Open sources
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {detail.open_candidate_sources.map((source) => (
                            <span
                              key={source}
                              className="px-2 py-1 rounded-full text-xs bg-white/5 border border-white/10"
                            >
                              {source}
                            </span>
                          ))}
                        </div>
                      </div>
                    )}

                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <div className="font-semibold">Learned review assist</div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          Label-memory heuristic trained from previously reviewed
                          deals for this trader.
                        </div>
                      </div>
                      {detail.classifier_assist?.highlight_level && (
                        <span
                          className={`px-2 py-1 rounded-full text-[11px] border ${classifierToneClasses(
                            detail.classifier_assist.highlight_level
                          )}`}
                        >
                          {detail.classifier_assist.highlight_level} signal
                        </span>
                      )}
                    </div>

                    <div className="text-sm text-nofx-text-muted">
                      {detail.classifier_assist?.summary ||
                        'No learned review signal yet. Add more labels to give the heuristic model better examples.'}
                    </div>

                    {detail.classifier_assist?.suggestions &&
                      detail.classifier_assist.suggestions.length > 0 && (
                        <div className="space-y-3">
                          {detail.classifier_assist.suggestions.map(
                            (suggestion) => {
                              const actionKeyAccept = `heuristic:accepted:${suggestion.suggestion_key}`
                              const actionKeyReject = `heuristic:rejected:${suggestion.suggestion_key}`
                              return (
                                <div
                                  key={suggestion.suggestion_key}
                                  className="rounded-lg border border-white/10 bg-black/20 p-3"
                                >
                                  <div className="flex flex-wrap items-center gap-2">
                                    <span className="px-2 py-1 rounded-full text-xs bg-white/5 border border-white/10 text-white">
                                      {suggestion.label}
                                    </span>
                                    <span className="text-xs text-nofx-text-muted">
                                      {formatClassifierIssueType(
                                        suggestion.issue_type
                                      )}
                                    </span>
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${classifierToneClasses(
                                        suggestion.highlight_level
                                      )}`}
                                    >
                                      {suggestion.highlight_level || 'low'}
                                    </span>
                                    <span className="text-xs text-nofx-text-muted">
                                      {suggestion.evidence_count} matches
                                    </span>
                                  </div>
                                  {suggestion.rationale && (
                                    <div className="text-sm text-nofx-text-muted mt-2">
                                      {suggestion.rationale}
                                    </div>
                                  )}
                                  <div className="flex flex-wrap items-center gap-2 mt-3">
                                    <button
                                      onClick={() =>
                                        void applyClassifierFeedback(
                                          suggestion,
                                          'accepted',
                                          'heuristic'
                                        )
                                      }
                                      disabled={
                                        classifierActionKey === actionKeyAccept
                                      }
                                      className="h-8 px-3 rounded-lg border border-emerald-400/25 text-emerald-300 disabled:opacity-50"
                                    >
                                      {classifierActionKey === actionKeyAccept
                                        ? 'Applying…'
                                        : 'Accept + label'}
                                    </button>
                                    <button
                                      onClick={() =>
                                        void applyClassifierFeedback(
                                          suggestion,
                                          'rejected',
                                          'heuristic'
                                        )
                                      }
                                      disabled={
                                        classifierActionKey === actionKeyReject
                                      }
                                      className="h-8 px-3 rounded-lg border border-white/15 text-white disabled:opacity-50"
                                    >
                                      {classifierActionKey === actionKeyReject
                                        ? 'Saving…'
                                        : 'Reject'}
                                    </button>
                                    {(suggestion.accepted_count > 0 ||
                                      suggestion.rejected_count > 0) && (
                                      <span className="text-xs text-nofx-text-muted">
                                        Accepted {suggestion.accepted_count} ·
                                        Rejected {suggestion.rejected_count}
                                      </span>
                                    )}
                                  </div>
                                </div>
                              )
                            }
                          )}
                        </div>
                      )}
                  </div>

                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <div className="font-semibold">AI review assist</div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          Optional single-deal review using the current AI model
                          selection from the scan controls.
                        </div>
                      </div>
                      <button
                        onClick={runCaseAIAssist}
                        disabled={runningCaseAIAssist}
                        className="h-9 px-3 rounded-lg border border-sky-400/30 text-sky-300 disabled:opacity-50"
                      >
                        {runningCaseAIAssist ? 'Running…' : 'Run AI assist'}
                      </button>
                    </div>

                    <div className="text-sm text-nofx-text-muted">
                      {aiClassifierAssist?.summary ||
                        'Run AI assist to get case-level review suggestions such as bad trade, bad exit, avoidable loss, or regime mismatch.'}
                    </div>

                    {aiClassifierAssist?.suggestions &&
                      aiClassifierAssist.suggestions.length > 0 && (
                        <div className="space-y-3">
                          {aiClassifierAssist.suggestions.map((suggestion) => {
                            const actionKeyAccept = `ai:accepted:${suggestion.suggestion_key}`
                            const actionKeyReject = `ai:rejected:${suggestion.suggestion_key}`
                            return (
                              <div
                                key={suggestion.suggestion_key}
                                className="rounded-lg border border-white/10 bg-black/20 p-3"
                              >
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="px-2 py-1 rounded-full text-xs bg-white/5 border border-white/10 text-white">
                                    {suggestion.label}
                                  </span>
                                  <span className="text-xs text-nofx-text-muted">
                                    {formatClassifierIssueType(
                                      suggestion.issue_type
                                    )}
                                  </span>
                                  <span
                                    className={`px-2 py-1 rounded-full text-[11px] border ${classifierToneClasses(
                                      suggestion.highlight_level
                                    )}`}
                                  >
                                    {suggestion.highlight_level || 'low'}
                                  </span>
                                </div>
                                {suggestion.rationale && (
                                  <div className="text-sm text-nofx-text-muted mt-2">
                                    {suggestion.rationale}
                                  </div>
                                )}
                                <div className="flex flex-wrap items-center gap-2 mt-3">
                                  <button
                                    onClick={() =>
                                      void applyClassifierFeedback(
                                        suggestion,
                                        'accepted',
                                        'ai'
                                      )
                                    }
                                    disabled={
                                      classifierActionKey === actionKeyAccept
                                    }
                                    className="h-8 px-3 rounded-lg border border-emerald-400/25 text-emerald-300 disabled:opacity-50"
                                  >
                                    {classifierActionKey === actionKeyAccept
                                      ? 'Applying…'
                                      : 'Accept + label'}
                                  </button>
                                  <button
                                    onClick={() =>
                                      void applyClassifierFeedback(
                                        suggestion,
                                        'rejected',
                                        'ai'
                                      )
                                    }
                                    disabled={
                                      classifierActionKey === actionKeyReject
                                    }
                                    className="h-8 px-3 rounded-lg border border-white/15 text-white disabled:opacity-50"
                                  >
                                    {classifierActionKey === actionKeyReject
                                      ? 'Saving…'
                                      : 'Reject'}
                                  </button>
                                </div>
                              </div>
                            )
                          })}
                        </div>
                      )}
                  </div>

                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3">
                    <div className="flex items-center justify-between gap-4">
                      <div>
                        <div className="font-semibold">Analyst review</div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          Add reusable labels and a manual note for this deal.
                        </div>
                      </div>
                      <button
                        onClick={saveCaseReview}
                        disabled={savingReview}
                        className="h-9 px-3 rounded-lg border border-nofx-gold/30 text-nofx-gold disabled:opacity-50"
                      >
                        {savingReview ? 'Saving…' : 'Save review'}
                      </button>
                    </div>
                    {detail.labels && detail.labels.length > 0 && (
                      <div className="flex flex-wrap gap-2">
                        {detail.labels.map((label) => (
                          <span
                            key={label}
                            className="px-2 py-1 rounded-full text-xs bg-nofx-gold/10 border border-nofx-gold/20 text-nofx-gold"
                          >
                            {label}
                          </span>
                        ))}
                      </div>
                    )}
                    <input
                      value={reviewLabelsText}
                      onChange={(event) =>
                        setReviewLabelsText(event.target.value)
                      }
                      placeholder="Comma-separated labels, e.g. good entry, avoidable loss"
                      className="w-full h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                    />
                    <textarea
                      value={reviewNote}
                      onChange={(event) => setReviewNote(event.target.value)}
                      placeholder="Manual note for this deal"
                      className="w-full min-h-[96px] rounded-lg border border-white/10 bg-black/20 px-3 py-3 text-sm resize-y"
                    />
                  </div>

                  {[
                    {
                      label: 'Open',
                      data: detail.open,
                      border: 'border-emerald-400/20',
                    },
                    {
                      label: 'Close',
                      data: detail.close,
                      border: 'border-rose-400/20',
                    },
                  ].map((section) => {
                    const snapshot = section.data?.snapshot
                    const systemPrompt = getEventSystemPrompt(section.data)
                    const inputPrompt = getEventInputPrompt(section.data)
                    const decisionJson = getEventDecisionJson(section.data)
                    const rawResponse = getEventRawResponse(section.data)
                    const aiRequestMs = getEventRequestMs(section.data)
                    const showSnapshot = hasStructuredSnapshot(snapshot)
                    const showPromptBundle = Boolean(
                      systemPrompt || inputPrompt || decisionJson || rawResponse
                    )

                    return (
                      <div
                        key={section.label}
                        className={`rounded-xl border ${section.border} bg-black/20 p-4 space-y-4`}
                      >
                      <div className="flex items-center justify-between">
                        <h3 className="font-semibold">{section.label} entry</h3>
                        <div className="text-xs text-nofx-text-muted">
                          Cycle{' '}
                          {section.data?.event?.decision_cycle_number || '-'}
                        </div>
                      </div>

                      <div className="text-sm text-nofx-text-muted whitespace-pre-wrap">
                        {section.data?.event?.reasoning ||
                          'No linked rationale snapshot.'}
                      </div>

                      <div className="grid grid-cols-2 gap-3 text-xs">
                        <div>
                          Price:{' '}
                          <span className="text-white">
                            {section.data?.event?.price || '-'}
                          </span>
                        </div>
                        <div>
                          Qty:{' '}
                          <span className="text-white">
                            {section.data?.event?.quantity || '-'}
                          </span>
                        </div>
                        <div>
                          Confidence:{' '}
                          <span className="text-white">
                            {section.data?.event?.confidence || '-'}
                          </span>
                        </div>
                        <div>
                          Action:{' '}
                          <span className="text-white">
                            {section.data?.event?.action || '-'}
                          </span>
                        </div>
                        <div>
                          SL:{' '}
                          <span className="text-white">
                            {section.data?.event?.stop_loss || '-'}
                          </span>
                        </div>
                        <div>
                          TP:{' '}
                          <span className="text-white">
                            {section.data?.event?.take_profit || '-'}
                          </span>
                        </div>
                        {section.label === 'Close' && (
                          <>
                            <div>
                              Outcome:{' '}
                              <span
                                className={
                                  detail.case.realized_pnl >= 0
                                    ? 'text-emerald-400'
                                    : 'text-rose-400'
                                }
                              >
                                {formatMoney(
                                  section.data?.event?.outcome_pnl ||
                                    detail.case.realized_pnl
                                )}
                              </span>
                            </div>
                            <div>
                              Close reason:{' '}
                              <span className="text-white">
                                {section.data?.event?.close_reason ||
                                  detail.case.close_reason ||
                                  '-'}
                              </span>
                            </div>
                          </>
                        )}
                      </div>

                      {section.data?.candidate_sources &&
                        section.data.candidate_sources.length > 0 && (
                          <div>
                            <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                              Candidate sources
                            </div>
                            <div className="flex flex-wrap gap-2">
                              {section.data.candidate_sources.map((source) => (
                                <span
                                  key={source}
                                  className="px-2 py-1 rounded-full text-xs bg-white/5 border border-white/10"
                                >
                                  {source}
                                </span>
                              ))}
                            </div>
                          </div>
                        )}

                      {section.data?.snapshot?.market_context && (
                        <div className="rounded-lg border border-white/10 bg-black/30 p-3 space-y-3">
                          <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                            Market context
                          </div>
                          <div className="flex flex-wrap gap-2">
                            {[
                              section.data.snapshot.market_context.trend_regime,
                              section.data.snapshot.market_context.volatility_regime,
                              section.data.snapshot.market_context.btc_strength_regime,
                              section.data.snapshot.market_context.funding_regime,
                              section.data.snapshot.market_context.oi_regime,
                              section.data.snapshot.market_context.session_bucket,
                              section.data.snapshot.market_context.weekday_bucket,
                              section.data.snapshot.market_context.venue_tier,
                              section.data.snapshot.market_context.liquidity_tier,
                              section.data.snapshot.market_context.spread_bucket,
                              section.data.snapshot.market_context.slippage_bucket,
                            ]
                              .filter(Boolean)
                              .map((badge) => (
                                <span
                                  key={`${section.label}-ctx-${badge}`}
                                  className="px-2 py-1 rounded-full text-[11px] bg-sky-500/10 border border-sky-400/20 text-sky-200"
                                >
                                  {formatMarketContextToken(badge)}
                                </span>
                              ))}
                          </div>
                          <div className="grid grid-cols-2 gap-3 text-xs">
                            <div>
                              Timeframe:{' '}
                              <span className="text-white">
                                {section.data.snapshot.market_context.timeframe || '-'}
                              </span>
                            </div>
                            <div>
                              Price type:{' '}
                              <span className="text-white">
                                {section.data.snapshot.market_context.price_type || '-'}
                              </span>
                            </div>
                            <div>
                              1h change:{' '}
                              <span className="text-white">
                                {formatMarketContextValue(
                                  section.data.snapshot.market_context.price_change_1h,
                                  'pct'
                                )}
                              </span>
                            </div>
                            <div>
                              4h change:{' '}
                              <span className="text-white">
                                {formatMarketContextValue(
                                  section.data.snapshot.market_context.price_change_4h,
                                  'pct'
                                )}
                              </span>
                            </div>
                            <div>
                              Funding:{' '}
                              <span className="text-white">
                                {formatMarketContextValue(
                                  section.data.snapshot.market_context.funding_bps,
                                  'bps'
                                )}
                              </span>
                            </div>
                            <div>
                              OI delta 1h:{' '}
                              <span className="text-white">
                                {formatMarketContextValue(
                                  section.data.snapshot.market_context.oi_delta_1h_pct,
                                  'pct'
                                )}
                              </span>
                            </div>
                            <div>
                              Spread:{' '}
                              <span className="text-white">
                                {formatMarketContextValue(
                                  section.data.snapshot.market_context.spread_bps,
                                  'bps'
                                )}
                              </span>
                            </div>
                            <div>
                              Slippage 100 USD:{' '}
                              <span className="text-white">
                                {formatMarketContextValue(
                                  section.data.snapshot.market_context.slippage_est_100usd,
                                  'bps'
                                )}
                              </span>
                            </div>
                          </div>
                        </div>
                      )}

                      {section.data?.snapshot?.execution_log &&
                        section.data.snapshot.execution_log.length > 0 && (
                          <div>
                            <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                              Execution log
                            </div>
                            <div className="space-y-2 text-xs text-nofx-text-muted">
                              {section.data.snapshot.execution_log
                                .slice(0, 6)
                                .map((line, index) => (
                                  <div
                                    key={`${section.label}-${index}`}
                                    className="rounded-lg border border-white/10 bg-black/30 px-3 py-2"
                                  >
                                    {line}
                                  </div>
                                ))}
                            </div>
                          </div>
                        )}

                      {showSnapshot && (
                        <details className="rounded-xl border border-white/10 bg-black/30 p-4">
                          <summary className="cursor-pointer text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                            Structured snapshot
                          </summary>
                          <div className="mt-4 space-y-4">
                            {snapshot?.account_state && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  Account snapshot
                                </div>
                                <div className="grid grid-cols-2 gap-3 text-xs">
                                  <div>
                                    Equity:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.total_balance || '-'}
                                    </span>
                                  </div>
                                  <div>
                                    Available:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.available_balance || '-'}
                                    </span>
                                  </div>
                                  <div>
                                    Unrealized:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.total_unrealized_profit || '-'}
                                    </span>
                                  </div>
                                  <div>
                                    Margin used:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.margin_used_pct || '-'}%
                                    </span>
                                  </div>
                                  <div>
                                    Positions:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.position_count || 0}
                                    </span>
                                  </div>
                                  <div>
                                    Initial balance:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.initial_balance || '-'}
                                    </span>
                                  </div>
                                </div>
                              </div>
                            )}

                            {snapshot?.positions && snapshot.positions.length > 0 && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  Position snapshot
                                </div>
                                <div className="space-y-2">
                                  {snapshot.positions.map((position) => (
                                    <div
                                      key={`${section.label}-${position.symbol}-${position.side}`}
                                      className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-xs"
                                    >
                                      <div className="font-medium text-white">
                                        {position.symbol} {position.side}
                                      </div>
                                      <div className="text-nofx-text-muted mt-1">
                                        Qty {position.position_amt} | Entry {position.entry_price} | Mark {position.mark_price} | PnL {position.unrealized_profit}
                                      </div>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            )}

                            {snapshot?.candidate_coins &&
                              snapshot.candidate_coins.length > 0 && (
                                <div>
                                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                    Candidate coins
                                  </div>
                                  <div className="flex flex-wrap gap-2">
                                    {snapshot.candidate_coins.map((coin) => (
                                      <span
                                        key={`${section.label}-${coin}`}
                                        className="px-2 py-1 rounded-full text-xs bg-white/5 border border-white/10"
                                      >
                                        {coin}
                                      </span>
                                    ))}
                                  </div>
                                </div>
                              )}

                            {snapshot?.candidate_details &&
                              snapshot.candidate_details.length > 0 && (
                                <div>
                                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                    Candidate details
                                  </div>
                                  <pre className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-nofx-text-muted overflow-x-auto whitespace-pre-wrap">
                                    {formatJsonBlock(snapshot.candidate_details)}
                                  </pre>
                                </div>
                              )}
                          </div>
                        </details>
                      )}

                      {showPromptBundle && (
                        <details className="rounded-xl border border-white/10 bg-black/30 p-4">
                          <summary className="cursor-pointer text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                            AI prompt bundle
                          </summary>
                          <div className="mt-4 space-y-4">
                            <div className="text-xs text-nofx-text-muted">
                              AI request duration:{' '}
                              <span className="text-white">
                                {formatRequestDuration(aiRequestMs)}
                              </span>
                            </div>

                            {inputPrompt && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  Input / market context prompt
                                </div>
                                <pre className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-nofx-text-muted overflow-x-auto whitespace-pre-wrap max-h-[26rem]">
                                  {inputPrompt}
                                </pre>
                              </div>
                            )}

                            {systemPrompt && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  System prompt
                                </div>
                                <pre className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-nofx-text-muted overflow-x-auto whitespace-pre-wrap max-h-[20rem]">
                                  {systemPrompt}
                                </pre>
                              </div>
                            )}

                            {decisionJson && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  Decision JSON
                                </div>
                                <pre className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-nofx-text-muted overflow-x-auto whitespace-pre-wrap max-h-[18rem]">
                                  {decisionJson}
                                </pre>
                              </div>
                            )}

                            {rawResponse && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  Raw AI response
                                </div>
                                <pre className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-nofx-text-muted overflow-x-auto whitespace-pre-wrap max-h-[20rem]">
                                  {rawResponse}
                                </pre>
                              </div>
                            )}
                          </div>
                        </details>
                      )}
                    </div>
                    )
                  })}
                </div>
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h2 className="font-semibold text-lg">Anomaly scan</h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    Heuristic hotspots from the currently filtered closed deals.
                  </p>
                </div>
                <div className="text-xs text-nofx-text-muted">
                  {anomalies?.closed_deals || 0} closed deals
                </div>
              </div>

              {!anomalies ? (
                <div className="text-sm text-nofx-text-muted mt-4">
                  No anomaly data available.
                </div>
              ) : (
                <div className="space-y-4 mt-4">
                  {anomalies.notes && anomalies.notes.length > 0 && (
                    <div className="space-y-2">
                      {anomalies.notes.map((note) => (
                        <div
                          key={note}
                          className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm text-nofx-text-muted"
                        >
                          {note}
                        </div>
                      ))}
                    </div>
                  )}

                  {[
                    {
                      title: 'Worst symbols',
                      items: anomalies.worst_symbols?.map((item) => ({
                        key: item.symbol,
                        primary: item.symbol,
                        secondary: `${item.deals} deals | ${item.win_rate.toFixed(1)}% win rate`,
                        value: formatMoney(item.net_pnl),
                        filterPatch: {
                          symbol: item.symbol,
                          status: 'CLOSED',
                        },
                        filterNote: `Filtered deals to symbol ${item.symbol}.`,
                        scanNote: `AI scan started for symbol cohort ${item.symbol}.`,
                      })),
                    },
                    {
                      title: 'Overtraded symbols',
                      items: anomalies.overtraded_symbols?.map((item) => ({
                        key: item.symbol,
                        primary: item.symbol,
                        secondary: `${item.deals} deals | avg hold ${formatHold(item.avg_hold_ms)}`,
                        value: formatMoney(item.net_pnl),
                        filterPatch: {
                          symbol: item.symbol,
                          status: 'CLOSED',
                        },
                        filterNote: `Filtered deals to overtraded symbol ${item.symbol}.`,
                        scanNote: `AI scan started for overtraded symbol ${item.symbol}.`,
                      })),
                    },
                    {
                      title: 'Weak buckets',
                      items: anomalies.weak_buckets?.map((item) => ({
                        key: item.bucket,
                        primary: item.bucket,
                        secondary: `${item.deals} deals | ${item.win_rate.toFixed(1)}% win rate`,
                        value: formatMoney(item.net_pnl),
                        filterPatch: {
                          open_selection_bucket: item.bucket,
                          status: 'CLOSED',
                        },
                        filterNote: `Filtered deals to bucket ${item.bucket}.`,
                        scanNote: `AI scan started for bucket ${item.bucket}.`,
                      })),
                    },
                    {
                      title: 'Weak close reasons',
                      items: anomalies.weak_close_reasons?.map((item) => ({
                        key: item.reason,
                        primary: item.reason,
                        secondary: `${item.deals} closes | avg ${formatMoney(item.avg_pnl)}`,
                        value: formatMoney(item.net_pnl),
                        filterPatch: {
                          close_reason: item.reason,
                          status: 'CLOSED',
                        },
                        filterNote: `Filtered deals to close reason ${item.reason}.`,
                        scanNote: `AI scan started for close reason ${item.reason}.`,
                      })),
                    },
                    {
                      title: 'Frequent profit give-back',
                      items: anomalies.profit_give_back_hotspots?.map((item) => ({
                        key: item.symbol,
                        primary: item.symbol,
                        secondary: `${item.deals} deals | avg give-back ${formatPct(item.avg_give_back_pct)}`,
                        value: `${formatPct(item.avg_mfe_captured_pct)} kept`,
                        filterPatch: {
                          symbol: item.symbol,
                          status: 'CLOSED',
                        },
                        filterNote: `Filtered deals to profit give-back hotspot ${item.symbol}.`,
                        scanNote: `AI scan started for give-back hotspot ${item.symbol}.`,
                      })),
                    },
                    {
                      title: 'Repeated early stop-outs',
                      items: anomalies.early_stop_out_hotspots?.map((item) => ({
                        key: item.symbol,
                        primary: item.symbol,
                        secondary: `${item.deals} stop-outs | avg hold ${formatHold(item.avg_hold_ms)}`,
                        value: `${formatPct(item.avg_mae_pct)} MAE`,
                        filterPatch: {
                          symbol: item.symbol,
                          status: 'CLOSED',
                        },
                        filterNote: `Filtered deals to early stop-out hotspot ${item.symbol}.`,
                        scanNote: `AI scan started for early stop-out hotspot ${item.symbol}.`,
                      })),
                    },
                    {
                      title: 'Outsized leverage / sizing losses',
                      items: anomalies.oversized_loss_hotspots?.map((item) => ({
                        key: item.symbol,
                        primary: item.symbol,
                        secondary: `${item.deals} losses | planned risk ${formatPct(item.avg_planned_risk_pct)}`,
                        value: `${formatScore(item.avg_risk_sizing_score)} sizing`,
                        filterPatch: {
                          symbol: item.symbol,
                          status: 'CLOSED',
                        },
                        filterNote: `Filtered deals to oversized-loss hotspot ${item.symbol}.`,
                        scanNote: `AI scan started for oversized-loss hotspot ${item.symbol}.`,
                      })),
                    },
                    {
                      title: 'Close-reason quality',
                      items: anomalies.close_reason_quality?.map((item) => ({
                        key: item.reason,
                        primary: item.reason,
                        secondary: `${item.deals} closes | give-back ${formatPct(item.avg_give_back_pct)}`,
                        value: `${formatScore(item.avg_exit_efficiency_score)} exit`,
                        filterPatch: {
                          close_reason: item.reason,
                          status: 'CLOSED',
                        },
                        filterNote: `Filtered deals to close-reason quality slice ${item.reason}.`,
                        scanNote: `AI scan started for close-reason quality slice ${item.reason}.`,
                      })),
                    },
                  ].map((section) => (
                    <div key={section.title}>
                      <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                        {section.title}
                      </div>
                      <div className="space-y-2">
                        {(section.items || []).length === 0 ? (
                          <div className="text-sm text-nofx-text-muted">
                            Nothing notable in this slice yet.
                          </div>
                        ) : (
                          section.items?.slice(0, 4).map((item) => (
                            <div
                              key={item.key}
                              className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 flex items-start justify-between gap-4"
                            >
                              <div>
                                <div className="font-medium">
                                  {item.primary}
                                </div>
                                <div className="text-xs text-nofx-text-muted mt-1">
                                  {item.secondary}
                                </div>
                                <div className="flex flex-wrap gap-2 mt-3">
                                  <button
                                    onClick={() =>
                                      applyDrilldownFilters(
                                        item.filterPatch,
                                        item.filterNote
                                      )
                                    }
                                    className="px-2.5 py-1 rounded-lg border border-white/10 bg-white/5 text-xs font-medium"
                                  >
                                    Filter
                                  </button>
                                  <button
                                    onClick={() =>
                                      void runAIScan(
                                        item.filterPatch,
                                        item.scanNote
                                      )
                                    }
                                    disabled={!selectedTraderId || runningScan}
                                    className="px-2.5 py-1 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 text-nofx-gold text-xs font-medium disabled:opacity-50"
                                  >
                                    Scan this cohort
                                  </button>
                                </div>
                              </div>
                              <div className="text-sm font-semibold text-rose-400">
                                {item.value}
                              </div>
                            </div>
                          ))
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h2 className="font-semibold text-lg">AI scan</h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    Run a model against the current filtered dataset, validate
                    the evidence gate, then either apply directly or launch a
                    timed challenger compare.
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={exportScansJSON}
                    disabled={!selectedTraderId || scans.length === 0}
                    className="h-10 px-4 rounded-lg border border-white/10 bg-black/20 text-sm disabled:opacity-40"
                  >
                    Export scans
                  </button>
                  <button
                    onClick={() => void runAIScan()}
                    disabled={!selectedTraderId || runningScan}
                    className="h-10 px-4 rounded-lg bg-nofx-gold text-black font-semibold disabled:opacity-50"
                  >
                    {runningScan ? 'Running…' : 'Run scan'}
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-4">
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={selectedModelId}
                    onChange={setSelectedModelId}
                    options={models
                      .filter((item) => item.enabled)
                      .map((item) => ({
                        value: item.id,
                        label: `${item.name} (${item.provider})`,
                      }))}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={selectedRemoteModel}
                    onChange={setSelectedRemoteModel}
                    options={(availableRemoteModels.length > 0
                      ? availableRemoteModels
                      : [
                          {
                            id: '',
                            label: 'Configured default',
                            provider: '',
                            available: true,
                          },
                        ]
                    ).map((item) => ({
                      value: item.id,
                      label: item.label,
                    }))}
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mt-3">
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={launchMode}
                    onChange={setLaunchMode}
                    options={challengerModeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={launchExchangeId}
                    onChange={setLaunchExchangeId}
                    options={exchanges.map((item) => ({
                      value: item.id,
                      label: `${item.name} / ${item.account_name}${item.testnet ? ' · testnet' : ''}`,
                    }))}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={launchWindowHours}
                    onChange={setLaunchWindowHours}
                    options={challengerWindowOptions}
                  />
                </div>
              </div>

              <div className="text-xs text-nofx-text-muted mt-2">
                `paper / simulation` currently uses a selected testnet wallet.
                `isolated_live` requires a different live wallet than the
                incumbent. `shared_live` can use the same live wallet.
              </div>

              <div className="space-y-4 mt-5">
                {scans.length === 0 ? (
                  <div className="text-sm text-nofx-text-muted">
                    No AI scans saved yet.
                  </div>
                ) : (
                  scans.map((scan) => {
                    const validation = scan.validation
                    const validationStatus =
                      validation?.status || scan.scan.validation_status || 'pending'
                    const validationPassed = validationStatus === 'passed'
                    const relatedCompare = getLatestCompareForScan(
                      scan.scan.id,
                      challengerCompares
                    )
                    const promotionState = getScanPromotionState(
                      scan,
                      relatedCompare
                    )
                    const hasPatch =
                      !!scan.strategy_patch &&
                      Object.keys(scan.strategy_patch).length > 0
                    return (
                      <div
                        key={scan.scan.id}
                        className="rounded-xl border border-white/10 bg-black/20 p-4"
                      >
                        <div className="flex items-start justify-between gap-4">
                          <div>
                            <div className="text-xs text-nofx-gold">
                              {scan.scan.provider} / {scan.scan.model_name}
                            </div>
                            <div className="font-semibold mt-1">
                              {scan.result.executive_summary || scan.scan.summary}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-2">
                              {new Date(scan.scan.created_at).toLocaleString()} |{' '}
                              {scan.scan.dataset_count} deals
                            </div>
                            <div className="flex flex-wrap items-center gap-2 mt-3">
                              <span
                                className={`px-2 py-1 rounded-full text-[11px] uppercase tracking-[0.18em] ${
                                  validationPassed
                                    ? 'bg-emerald-500/15 text-emerald-300'
                                    : validationStatus === 'failed'
                                      ? 'bg-rose-500/15 text-rose-300'
                                      : 'bg-amber-500/15 text-amber-200'
                                }`}
                              >
                                {validationStatus}
                              </span>
                              <span className="text-xs text-nofx-text-muted">
                                {scan.scan.validation_summary ||
                                  'Validation required before apply or challenger launch.'}
                              </span>
                            </div>
                            <div className="flex flex-wrap items-center gap-2 mt-2">
                              <span
                                className={`px-2 py-1 rounded-full text-[11px] uppercase tracking-[0.18em] ${
                                  promotionState.tone === 'emerald'
                                    ? 'bg-emerald-500/15 text-emerald-300'
                                    : promotionState.tone === 'rose'
                                      ? 'bg-rose-500/15 text-rose-300'
                                      : promotionState.tone === 'sky'
                                        ? 'bg-sky-500/15 text-sky-300'
                                        : 'bg-amber-500/15 text-amber-200'
                                }`}
                              >
                                {promotionState.label}
                              </span>
                              <span className="text-xs text-nofx-text-muted">
                                {promotionState.detail}
                              </span>
                            </div>
                          </div>
                          <div className="flex flex-wrap justify-end gap-2">
                            <button
                              onClick={() => validateScan(scan)}
                              disabled={validatingScanId === scan.scan.id}
                              className="h-9 px-3 rounded-lg border border-white/15 text-white disabled:opacity-50"
                            >
                              {validatingScanId === scan.scan.id
                                ? 'Validating…'
                                : validationStatus === 'passed'
                                  ? 'Revalidate'
                                  : 'Validate'}
                            </button>
                            <button
                              onClick={() => launchChallenger(scan)}
                              disabled={
                                launchingScanId === scan.scan.id ||
                                !validationPassed ||
                                !hasPatch ||
                                !launchExchangeId
                              }
                              className="h-9 px-3 rounded-lg border border-sky-400/30 text-sky-300 disabled:opacity-50"
                            >
                              {launchingScanId === scan.scan.id
                                ? 'Launching…'
                                : 'Launch challenger'}
                            </button>
                            <button
                              onClick={() => applyScan(scan)}
                              disabled={
                                applyingScanId === scan.scan.id ||
                                !validationPassed ||
                                !hasPatch
                              }
                              className="h-9 px-3 rounded-lg border border-nofx-gold/30 text-nofx-gold disabled:opacity-50"
                            >
                              {applyingScanId === scan.scan.id
                                ? 'Applying…'
                                : 'Apply patch'}
                            </button>
                          </div>
                        </div>

                        {validation && (
                          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 mt-4 text-sm">
                            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                Evidence gate
                              </div>
                              <div className="space-y-1 text-nofx-text-muted">
                                <div>
                                  Closed deals: {validation.closed_deal_count} /{' '}
                                  {validation.min_closed_deal_count}
                                </div>
                                <div>
                                  Train / holdout / recent: {validation.training_closed_deal_count} /{' '}
                                  {validation.holdout_closed_deal_count} /{' '}
                                  {validation.recent_closed_deal_count || 0}
                                </div>
                                <div>
                                  Config valid:{' '}
                                  {validation.config_valid ? 'yes' : 'no'}
                                </div>
                                {validation.recent_slice_label && (
                                  <div>
                                    Recent slice:{' '}
                                    {validation.recent_slice_label}
                                  </div>
                                )}
                              </div>
                            </div>
                            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                Gate output
                              </div>
                              {(validation.blocking_issues || []).length === 0 ? (
                                <div className="text-sm text-emerald-300">
                                  Ready for direct apply or challenger launch.
                                </div>
                              ) : (
                                <div className="space-y-1 text-sm text-rose-300">
                                  {validation.blocking_issues?.map((item) => (
                                    <div key={item}>• {item}</div>
                                  ))}
                                </div>
                              )}
                            </div>
                            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                Replay gate
                              </div>
                              {!validation.replay?.supported ? (
                                <div className="text-sm text-nofx-text-muted">
                                  No supported historical replay for this patch
                                  yet.
                                </div>
                              ) : (
                                <div className="space-y-1 text-nofx-text-muted">
                                  <div>
                                    Holdout delta:{' '}
                                    <span
                                      className={
                                        validation.replay.holdout_net_pnl_delta >= 0
                                          ? 'text-emerald-300'
                                          : 'text-rose-300'
                                      }
                                    >
                                      {formatMoney(
                                        validation.replay.holdout_net_pnl_delta
                                      )}
                                    </span>
                                  </div>
                                  <div>
                                    Training delta:{' '}
                                    <span
                                      className={
                                        validation.replay.training_net_pnl_delta >= 0
                                          ? 'text-emerald-300'
                                          : 'text-rose-300'
                                      }
                                    >
                                      {formatMoney(
                                        validation.replay.training_net_pnl_delta
                                      )}
                                    </span>
                                  </div>
                                  <div>
                                    Recent delta:{' '}
                                    <span
                                      className={
                                        validation.replay.recent_net_pnl_delta >= 0
                                          ? 'text-emerald-300'
                                          : 'text-rose-300'
                                      }
                                    >
                                      {formatMoney(
                                        validation.replay.recent_net_pnl_delta
                                      )}
                                    </span>
                                  </div>
                                  <div>
                                    Covered fields:{' '}
                                    {(validation.replay.supported_paths || []).join(
                                      ', '
                                    ) || '-'}
                                  </div>
                                </div>
                              )}
                            </div>
                            <div className="rounded-lg border border-white/10 bg-black/20 p-3 xl:col-span-2">
                              <div className="flex items-center justify-between gap-3 mb-2">
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                  Metric gates
                                </div>
                                <div className="text-xs text-nofx-text-muted">
                                  {(validation.checks || []).filter((item) => item.passed).length} /{' '}
                                  {(validation.checks || []).length} passed
                                </div>
                              </div>
                              {!(validation.checks || []).length ? (
                                <div className="text-sm text-nofx-text-muted">
                                  No metric checks recorded yet.
                                </div>
                              ) : (
                                <div className="space-y-2 max-h-72 overflow-auto pr-1">
                                  {[...(validation.checks || [])]
                                    .sort((left, right) => Number(left.passed) - Number(right.passed))
                                    .map((item) => (
                                      <div
                                        key={item.key}
                                        className="rounded-lg border border-white/8 bg-black/20 p-3"
                                      >
                                        <div className="flex items-start justify-between gap-3">
                                          <div>
                                            <div className="font-medium text-white">
                                              {item.label}
                                            </div>
                                            <div className="text-xs text-nofx-text-muted mt-1">
                                              {formatValidationScope(item.scope)} |{' '}
                                              {item.source || 'baseline'}
                                            </div>
                                          </div>
                                          <span
                                            className={`px-2 py-1 rounded-full text-[11px] uppercase tracking-[0.18em] ${
                                              item.passed
                                                ? 'bg-emerald-500/15 text-emerald-300'
                                                : 'bg-rose-500/15 text-rose-300'
                                            }`}
                                          >
                                            {item.passed ? 'pass' : 'block'}
                                          </span>
                                        </div>
                                        <div className="text-xs text-nofx-text-muted mt-2">
                                          {formatValidationCheckSummary(item)}
                                        </div>
                                        {typeof item.baseline === 'number' &&
                                          item.comparator === 'delta_gte' && (
                                            <div className="text-xs text-nofx-text-muted mt-1">
                                              baseline{' '}
                                              {formatValidationMetricValue(
                                                item.metric,
                                                item.baseline
                                              )}
                                              {' | '}projected{' '}
                                              {formatValidationMetricValue(
                                                item.metric,
                                                item.actual
                                              )}
                                            </div>
                                          )}
                                        {!item.passed && (
                                          <div className="text-xs text-rose-300 mt-2">
                                            {item.message}
                                          </div>
                                        )}
                                      </div>
                                    ))}
                                </div>
                              )}
                            </div>
                          </div>
                        )}

                        {scan.result.weaknesses &&
                          scan.result.weaknesses.length > 0 && (
                            <div className="mt-4">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                Weaknesses
                              </div>
                              <div className="space-y-2 text-sm text-nofx-text-muted">
                                {scan.result.weaknesses
                                  .slice(0, 3)
                                  .map((item) => (
                                    <div key={item}>• {item}</div>
                                  ))}
                              </div>
                            </div>
                          )}

                        {scan.result.immediate_actions &&
                          scan.result.immediate_actions.length > 0 && (
                            <div className="mt-4">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                Immediate actions
                              </div>
                              <div className="space-y-3">
                                {scan.result.immediate_actions
                                  .slice(0, 2)
                                  .map((item) => (
                                    <div
                                      key={item.title}
                                      className="rounded-lg border border-white/10 p-3"
                                    >
                                      <div className="font-medium">
                                        {item.title}
                                      </div>
                                      <div className="text-sm text-nofx-text-muted mt-1">
                                        {item.rationale}
                                      </div>
                                    </div>
                                  ))}
                              </div>
                            </div>
                          )}
                      </div>
                    )
                  })
                )}
              </div>

              <div className="mt-6 rounded-xl border border-white/10 bg-black/20 p-4">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <div className="font-semibold">Scan compare</div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      Compare two saved AI scans on filters, overlaps and patch
                      deltas.
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {compareLoading && (
                      <div className="text-xs text-nofx-text-muted">Loading…</div>
                    )}
                    <button
                      onClick={exportScanCompareJSON}
                      disabled={!compareResult}
                      className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs disabled:opacity-40"
                    >
                      Export compare
                    </button>
                  </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-4">
                  <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                    <NofxSelect
                      value={compareLeftScanId}
                      onChange={setCompareLeftScanId}
                      options={scans.map((item) => ({
                        value: item.scan.id,
                        label: `${item.scan.model_name} · ${new Date(item.scan.created_at).toLocaleDateString()}`,
                      }))}
                    />
                  </div>
                  <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                    <NofxSelect
                      value={compareRightScanId}
                      onChange={setCompareRightScanId}
                      options={scans.map((item) => ({
                        value: item.scan.id,
                        label: `${item.scan.model_name} · ${new Date(item.scan.created_at).toLocaleDateString()}`,
                      }))}
                    />
                  </div>
                </div>

                {!compareResult ? (
                  <div className="text-sm text-nofx-text-muted mt-4">
                    Select two different scans to compare.
                  </div>
                ) : (
                  <div className="space-y-4 mt-4">
                    <div className="grid grid-cols-2 gap-3 text-sm">
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">Left</div>
                        <div className="font-medium mt-1">
                          {compareResult.left.scan.provider} /{' '}
                          {compareResult.left.scan.model_name}
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">
                          Right
                        </div>
                        <div className="font-medium mt-1">
                          {compareResult.right.scan.provider} /{' '}
                          {compareResult.right.scan.model_name}
                        </div>
                      </div>
                    </div>

                    <div className="rounded-xl border border-white/10 bg-black/30 p-4">
                      <div className="flex flex-wrap items-start justify-between gap-3">
                        <div>
                          <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                            Disagreement score
                          </div>
                          <div
                            className={`mt-2 text-2xl font-semibold ${compareLevelClasses(compareResult.disagreement_level)}`}
                          >
                            {compareResult.conflict_score.toFixed(0)} / 100
                          </div>
                          <div className="text-sm text-nofx-text-muted mt-2 max-w-2xl">
                            {compareResult.disagreement_summary ||
                              'No disagreement summary available.'}
                          </div>
                        </div>
                        <div className="text-sm">
                          <span className="text-nofx-text-muted">Filters:</span>{' '}
                          <span
                            className={
                              compareResult.same_filters
                                ? 'text-emerald-400'
                                : 'text-amber-300'
                            }
                          >
                            {compareResult.same_filters
                              ? 'Same dataset'
                              : 'Different dataset filters'}
                          </span>
                        </div>
                      </div>

                      {compareResult.flags && compareResult.flags.length > 0 && (
                        <div className="flex flex-wrap gap-2 mt-4">
                          {compareResult.flags.map((flag) => (
                            <span
                              key={flag.code}
                              className={`px-2 py-1 rounded-full text-xs border ${compareFlagClasses(flag.tone)}`}
                              title={flag.note}
                            >
                              {formatCompareFlagTitle(flag.code, flag.title)}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>

                    <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
                      <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                        <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                          Recommendation overlap
                        </div>
                        {!compareResult.recommendation_overlap ? (
                          <div className="text-sm text-nofx-text-muted">
                            No actionable recommendations recorded in either
                            scan.
                          </div>
                        ) : (
                          <div className="space-y-3">
                            <div className="text-2xl font-semibold text-white">
                              {compareResult.recommendation_overlap.overlap_score.toFixed(
                                0
                              )}
                              %
                            </div>
                            <div className="space-y-2 text-sm">
                              <div>
                                <div className="text-nofx-text-muted mb-1">
                                  Shared
                                </div>
                                <div className="space-y-1">
                                  {(compareResult.recommendation_overlap.shared ||
                                    []
                                  )
                                    .slice(0, 4)
                                    .map((item) => (
                                      <div key={item} className="text-white">
                                        {item}
                                      </div>
                                    ))}
                                  {(!compareResult.recommendation_overlap
                                    .shared ||
                                    compareResult.recommendation_overlap.shared
                                      .length === 0) && (
                                    <div className="text-nofx-text-muted">
                                      No shared recommendation.
                                    </div>
                                  )}
                                </div>
                              </div>
                              <div className="grid grid-cols-2 gap-3 text-xs text-nofx-text-muted">
                                <div>
                                  Left-only:{' '}
                                  {(
                                    compareResult.recommendation_overlap
                                      .left_only || []
                                  ).length}
                                </div>
                                <div>
                                  Right-only:{' '}
                                  {(
                                    compareResult.recommendation_overlap
                                      .right_only || []
                                  ).length}
                                </div>
                              </div>
                            </div>
                          </div>
                        )}
                      </div>

                      <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                        <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                          Shared evidence
                        </div>
                        {!compareResult.shared_evidence ? (
                          <div className="text-sm text-nofx-text-muted">
                            No strong shared evidence in strengths, weaknesses,
                            or patterns.
                          </div>
                        ) : (
                          <div className="space-y-3">
                            <div className="text-2xl font-semibold text-white">
                              {compareResult.shared_evidence.total_shared}
                            </div>
                            <div className="space-y-2 text-sm text-nofx-text-muted">
                              {(compareResult.shared_evidence.shared_strengths ||
                                []
                              )
                                .slice(0, 2)
                                .map((item) => (
                                  <div key={`strength-${item}`}>
                                    Strength: <span className="text-white">{item}</span>
                                  </div>
                                ))}
                              {(compareResult.shared_evidence.shared_weaknesses ||
                                []
                              )
                                .slice(0, 2)
                                .map((item) => (
                                  <div key={`weakness-${item}`}>
                                    Weakness: <span className="text-white">{item}</span>
                                  </div>
                                ))}
                              {(compareResult.shared_evidence.shared_patterns ||
                                []
                              )
                                .slice(0, 2)
                                .map((item) => (
                                  <div key={`pattern-${item}`}>
                                    Pattern: <span className="text-white">{item}</span>
                                  </div>
                                ))}
                            </div>
                          </div>
                        )}
                      </div>

                      <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                        <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                          Target cohorts
                        </div>
                        {!compareResult.target_cohorts ? (
                          <div className="text-sm text-nofx-text-muted">
                            No explicit cohort tags detected yet.
                          </div>
                        ) : (
                          <div className="space-y-3 text-sm">
                            <div>
                              <div className="text-nofx-text-muted mb-1">
                                Shared tags
                              </div>
                              <div className="flex flex-wrap gap-2">
                                {(compareResult.target_cohorts.shared_tags || [])
                                  .slice(0, 4)
                                  .map((item) => (
                                    <span
                                      key={`shared-${item}`}
                                      className="px-2 py-1 rounded-full border border-emerald-400/20 bg-emerald-500/10 text-emerald-300 text-xs"
                                    >
                                      {item}
                                    </span>
                                  ))}
                                {(!compareResult.target_cohorts.shared_tags ||
                                  compareResult.target_cohorts.shared_tags
                                    .length === 0) && (
                                  <span className="text-nofx-text-muted text-xs">
                                    No shared cohort tags.
                                  </span>
                                )}
                              </div>
                            </div>
                            <div>
                              <div className="text-nofx-text-muted mb-1">
                                Conflicts
                              </div>
                              <div className="flex flex-wrap gap-2">
                                {(
                                  compareResult.target_cohorts
                                    .conflicting_dimensions || []
                                ).map((item) => (
                                  <span
                                    key={`conflict-${item}`}
                                    className="px-2 py-1 rounded-full border border-rose-400/20 bg-rose-500/10 text-rose-300 text-xs"
                                  >
                                    {item}
                                  </span>
                                ))}
                                {(!compareResult.target_cohorts
                                  .conflicting_dimensions ||
                                  compareResult.target_cohorts
                                    .conflicting_dimensions.length === 0) && (
                                  <span className="text-nofx-text-muted text-xs">
                                    No conflicting cohort dimension detected.
                                  </span>
                                )}
                              </div>
                            </div>
                          </div>
                        )}
                      </div>
                    </div>

                    {compareResult.filter_differences &&
                      compareResult.filter_differences.length > 0 && (
                        <div>
                          <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                            Filter differences
                          </div>
                          <div className="grid grid-cols-1 xl:grid-cols-2 gap-2">
                            {compareResult.filter_differences
                              .slice(0, 6)
                              .map((item) => (
                                <div
                                  key={`filter-${item.path}`}
                                  className="rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-xs"
                                >
                                  <div className="text-nofx-gold">
                                    {item.path}
                                  </div>
                                  <div className="text-nofx-text-muted mt-1">
                                    Left: {item.left}
                                  </div>
                                  <div className="text-nofx-text-muted">
                                    Right: {item.right}
                                  </div>
                                </div>
                              ))}
                          </div>
                        </div>
                      )}

                    {compareResult.patch_differences &&
                      compareResult.patch_differences.length > 0 && (
                        <div>
                          <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                            Patch differences
                          </div>
                          <div className="grid grid-cols-1 xl:grid-cols-2 gap-2">
                            {compareResult.patch_differences
                              .slice(0, 8)
                              .map((item) => (
                                <div
                                  key={item.path}
                                  className="rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-xs"
                                >
                                  <div className="text-nofx-gold">
                                    {item.path}
                                  </div>
                                  <div className="text-nofx-text-muted mt-1">
                                    Left: {item.left}
                                  </div>
                                  <div className="text-nofx-text-muted">
                                    Right: {item.right}
                                  </div>
                                </div>
                              ))}
                          </div>
                        </div>
                      )}

                    {compareResult.model_leaderboards &&
                      compareResult.model_leaderboards.length > 0 && (
                        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                          <div className="font-semibold">Model usefulness by cohort</div>
                          <div className="text-xs text-nofx-text-muted mt-1">
                            Aggregated from saved scans plus observed apply /
                            challenger outcomes.
                          </div>
                          <div className="space-y-4 mt-4">
                            {compareResult.model_leaderboards.map((group) => (
                              <div key={group.cohort_key}>
                                <div className="flex items-center gap-2 mb-2">
                                  <div className="text-sm font-medium">
                                    {group.cohort_label}
                                  </div>
                                  {group.relevant && (
                                    <span className="px-2 py-1 rounded-full border border-nofx-gold/30 bg-nofx-gold/10 text-[11px] text-nofx-gold">
                                      Relevant to this compare
                                    </span>
                                  )}
                                </div>
                                <div className="space-y-2">
                                  {(group.entries || []).map((entry) => (
                                    <div
                                      key={`${group.cohort_key}-${entry.model_key}`}
                                      className="rounded-lg border border-white/10 bg-black/30 px-3 py-3 flex items-start justify-between gap-4"
                                    >
                                      <div>
                                        <div className="font-medium">
                                          {entry.model_label}
                                        </div>
                                        <div className="text-xs text-nofx-text-muted mt-1">
                                          {entry.scan_count} scans |{' '}
                                          {entry.promotion_ready_count} ready |{' '}
                                          {entry.applied_count} applied | W/L{' '}
                                          {entry.challenger_win_count}/
                                          {entry.challenger_loss_count}
                                        </div>
                                      </div>
                                      <div className="text-right">
                                        <div className="text-sm font-semibold text-white">
                                          Score{' '}
                                          {entry.usefulness_score >= 0 ? '+' : ''}
                                          {entry.usefulness_score.toFixed(2)}
                                        </div>
                                        <div className="text-xs text-nofx-text-muted mt-1">
                                          Avg delta{' '}
                                          <span
                                            className={
                                              entry.avg_net_pnl_delta >= 0
                                                ? 'text-emerald-300'
                                                : 'text-rose-300'
                                            }
                                          >
                                            {formatMoney(entry.avg_net_pnl_delta)}
                                          </span>
                                        </div>
                                      </div>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}
                  </div>
                )}
              </div>
            </div>

            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h2 className="font-semibold text-lg">Challenger history</h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    Timed old-vs-new comparisons launched from validated AI
                    scans.
                  </p>
                </div>
              </div>

              <div className="space-y-3 mt-4">
                {challengerCompares.length === 0 ? (
                  <div className="text-sm text-nofx-text-muted">
                    No challenger compares started yet.
                  </div>
                ) : (
                  challengerCompares.map((item) => (
                    <div
                      key={item.compare.id}
                      className={`rounded-xl border p-4 cursor-pointer transition-colors ${
                        selectedCompareId === item.compare.id
                          ? 'border-sky-400/40 bg-sky-500/10'
                          : 'border-white/10 bg-black/20 hover:border-white/20'
                      }`}
                      onClick={() => setSelectedCompareId(item.compare.id)}
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div>
                          <div className="text-xs text-sky-300 uppercase tracking-[0.18em]">
                            {formatCompareMode(item.compare.mode)} ·{' '}
                            {formatCompareStatus(item.compare.status)}
                          </div>
                          <div className="font-medium mt-1">
                            {item.incumbent_trader_name || 'Incumbent'} vs{' '}
                            {item.challenger_trader_name || 'Challenger'}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-2">
                            {item.challenger_exchange_name || 'Wallet unknown'} ·
                            window {item.compare.window_hours}h
                            {item.compare.extension_count > 0
                              ? ` · +${item.compare.extension_count} extensions`
                              : ''}
                          </div>
                        </div>
                        <div className="text-right text-sm">
                          <div className="text-nofx-text-muted">
                            Ends {new Date(item.compare.ends_at).toLocaleString()}
                          </div>
                          {item.compare.resolved_at && (
                            <div className="text-xs text-nofx-text-muted mt-1">
                              Resolved{' '}
                              {new Date(
                                item.compare.resolved_at
                              ).toLocaleString()}
                            </div>
                          )}
                          {item.compare.winner_trader_id && (
                            <div className="text-xs text-emerald-300 mt-1">
                              Winner:{' '}
                              {item.compare.winner_trader_id ===
                              item.compare.incumbent_trader_id
                                ? item.incumbent_trader_name || 'Incumbent'
                                : item.challenger_trader_name || 'Challenger'}
                            </div>
                          )}
                        </div>
                      </div>

                      {item.metrics && (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-4 text-sm">
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              Incumbent
                            </div>
                            <div className="font-semibold mt-1">
                              {formatMoney(item.metrics.incumbent_pnl)}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {item.metrics.incumbent_trade_count} closed trades
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              Challenger
                            </div>
                            <div className="font-semibold mt-1">
                              {formatMoney(item.metrics.challenger_pnl)}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {item.metrics.challenger_trade_count} closed trades
                            </div>
                          </div>
                        </div>
                      )}

                      <div className="text-sm text-nofx-text-muted mt-4">
                        {item.compare.summary}
                      </div>

                      <div className="mt-4">
                        <button
                          onClick={(event) => {
                            event.stopPropagation()
                            setSelectedCompareId(item.compare.id)
                          }}
                          className="h-9 px-3 rounded-lg border border-white/15 text-white"
                        >
                          Open detail
                        </button>
                      </div>
                    </div>
                  ))
                )}
              </div>

              {(selectedCompareDetail || compareDetailLoading) && (
                <div
                  ref={compareDetailRef}
                  className="rounded-xl border border-white/10 bg-black/20 p-5 mt-5"
                >
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                    <div>
                      <div className="text-xs uppercase tracking-[0.2em] text-sky-300">
                        Compare detail
                      </div>
                      <h3 className="font-semibold text-lg mt-1">
                        {selectedCompareDetail?.incumbent_trader_name ||
                          'Incumbent'}{' '}
                        vs{' '}
                        {selectedCompareDetail?.challenger_trader_name ||
                          'Challenger'}
                      </h3>
                      <div className="text-sm text-nofx-text-muted mt-2">
                        {selectedCompareDetail
                          ? `${formatCompareMode(
                              selectedCompareDetail.compare.mode
                            )} · ${selectedCompareDetail.challenger_exchange_name || 'Wallet unknown'} · window ${selectedCompareDetail.compare.window_hours}h`
                          : 'Loading compare detail…'}
                      </div>
                      {selectedCompareDetail?.source_scan_summary && (
                        <div className="text-sm text-nofx-text-muted mt-2">
                          Source scan: {selectedCompareDetail.source_scan_summary}
                        </div>
                      )}
                    </div>

                    {selectedCompareDetail && (
                      <div className="flex flex-wrap gap-2">
                        {isActiveCompareStatus(
                          selectedCompareDetail.compare.status
                        ) && (
                          <>
                            <button
                              onClick={() => stopCompare(selectedCompareDetail)}
                              disabled={
                                compareActionKey ===
                                `stop:${selectedCompareDetail.compare.id}`
                              }
                              className="h-9 px-3 rounded-lg border border-rose-400/30 text-rose-300 disabled:opacity-50"
                            >
                              {compareActionKey ===
                              `stop:${selectedCompareDetail.compare.id}`
                                ? 'Stopping…'
                                : 'Stop compare'}
                            </button>
                            <button
                              onClick={() =>
                                resolveCompare(
                                  selectedCompareDetail,
                                  selectedCompareDetail.compare.incumbent_trader_id,
                                  'incumbent'
                                )
                              }
                              disabled={
                                compareActionKey ===
                                `resolve:${selectedCompareDetail.compare.id}:${selectedCompareDetail.compare.incumbent_trader_id}`
                              }
                              className="h-9 px-3 rounded-lg border border-white/15 text-white disabled:opacity-50"
                            >
                              {compareActionKey ===
                              `resolve:${selectedCompareDetail.compare.id}:${selectedCompareDetail.compare.incumbent_trader_id}`
                                ? 'Resolving…'
                                : 'Keep incumbent'}
                            </button>
                            <button
                              onClick={() =>
                                resolveCompare(
                                  selectedCompareDetail,
                                  selectedCompareDetail.compare.challenger_trader_id,
                                  'challenger'
                                )
                              }
                              disabled={
                                compareActionKey ===
                                `resolve:${selectedCompareDetail.compare.id}:${selectedCompareDetail.compare.challenger_trader_id}`
                              }
                              className="h-9 px-3 rounded-lg border border-sky-400/30 text-sky-300 disabled:opacity-50"
                            >
                              {compareActionKey ===
                              `resolve:${selectedCompareDetail.compare.id}:${selectedCompareDetail.compare.challenger_trader_id}`
                                ? 'Resolving…'
                                : 'Keep challenger'}
                            </button>
                          </>
                        )}
                      </div>
                    )}
                  </div>

                  {compareDetailLoading ? (
                    <div className="text-sm text-nofx-text-muted mt-4">
                      Loading compare detail…
                    </div>
                  ) : selectedCompareDetail ? (
                    <>
                      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 mt-5 text-sm">
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            Status
                          </div>
                          <div className="font-semibold mt-1 capitalize">
                            {formatCompareStatus(
                              selectedCompareDetail.compare.status
                            )}
                          </div>
                          {selectedCompareDetail.compare.resolved_at && (
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {new Date(
                                selectedCompareDetail.compare.resolved_at
                              ).toLocaleString()}
                            </div>
                          )}
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            Incumbent PnL
                          </div>
                          <div className="font-semibold mt-1">
                            {formatMoney(
                              selectedCompareDetail.metrics?.incumbent_pnl || 0
                            )}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-1">
                            {selectedCompareDetail.metrics?.incumbent_trade_count ||
                              0}{' '}
                            trades · fees{' '}
                            {formatMoney(
                              selectedCompareDetail.metrics?.incumbent_fees || 0
                            )}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            Challenger PnL
                          </div>
                          <div className="font-semibold mt-1">
                            {formatMoney(
                              selectedCompareDetail.metrics?.challenger_pnl || 0
                            )}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-1">
                            {selectedCompareDetail.metrics
                              ?.challenger_trade_count || 0}{' '}
                            trades · fees{' '}
                            {formatMoney(
                              selectedCompareDetail.metrics?.challenger_fees || 0
                            )}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            Winner
                          </div>
                          <div className="font-semibold mt-1">
                            {selectedCompareDetail.compare.winner_trader_id
                              ? selectedCompareDetail.compare.winner_trader_id ===
                                selectedCompareDetail.compare.incumbent_trader_id
                                ? selectedCompareDetail.incumbent_trader_name ||
                                  'Incumbent'
                                : selectedCompareDetail.challenger_trader_name ||
                                  'Challenger'
                              : 'Not resolved'}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-1">
                            {selectedCompareDetail.compare.extension_count > 0
                              ? `Extensions: ${selectedCompareDetail.compare.extension_count}`
                              : 'No extensions'}
                          </div>
                        </div>
                      </div>

                      <div className="rounded-lg border border-white/10 bg-black/20 p-4 mt-4">
                        <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                          Summary
                        </div>
                        <div className="text-sm text-nofx-text-muted">
                          {selectedCompareDetail.compare.summary}
                        </div>
                        {selectedCompareDetail.compare.error_message && (
                          <div className="text-sm text-rose-300 mt-3">
                            {selectedCompareDetail.compare.error_message}
                          </div>
                        )}
                      </div>

                      <div className="rounded-lg border border-white/10 bg-black/20 p-4 mt-4">
                        <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                          Compare protocol
                        </div>
                        {!selectedCompareDetail.protocol ||
                        selectedCompareDetail.protocol.length === 0 ? (
                          <div className="text-sm text-nofx-text-muted">
                            No protocol entries recorded yet.
                          </div>
                        ) : (
                          <div className="space-y-3">
                            {selectedCompareDetail.protocol.map((event, index) => (
                              <div
                                key={`${event.timestamp}-${event.type}-${index}`}
                                className="rounded-lg border border-white/10 bg-black/30 px-3 py-3"
                              >
                                <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-2">
                                  <div>
                                    <div className="text-xs uppercase tracking-[0.18em] text-sky-300">
                                      {formatCompareProtocolType(event.type)}
                                      {event.actor
                                        ? ` · ${event.actor}`
                                        : ''}
                                    </div>
                                    <div className="text-sm mt-1">
                                      {event.message}
                                    </div>
                                  </div>
                                  <div className="text-xs text-nofx-text-muted">
                                    {new Date(event.timestamp).toLocaleString()}
                                  </div>
                                </div>
                                {buildCompareProtocolMetricSummary(event) && (
                                  <div className="text-xs text-nofx-text-muted mt-2">
                                    {buildCompareProtocolMetricSummary(event)}
                                  </div>
                                )}
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    </>
                  ) : null}
                </div>
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h2 className="font-semibold text-lg">Strategy history</h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    Applied AI patches and rollback points for the current
                    trader strategy.
                  </p>
                </div>
              </div>

              <div className="space-y-3 mt-4">
                {versions.length === 0 ? (
                  <div className="text-sm text-nofx-text-muted">
                    No strategy versions captured yet.
                  </div>
                ) : (
                  versions.map((version) => {
                    const relatedCompare = challengerCompares.find(
                      (item) =>
                        item.compare.id === version.version.source_compare_id
                    )
                    const hasCompareLink = Boolean(
                      version.version.source_compare_id?.trim()
                    )
                    const compareNote = relatedCompare
                      ? relatedCompare.compare.summary
                      : hasCompareLink
                        ? 'Linked challenger compare available.'
                        : ''
                    return (
                      <div
                        key={version.version.id}
                        className={`rounded-xl border p-4 cursor-pointer transition-colors ${
                          selectedVersionId === version.version.id
                            ? 'border-nofx-gold/40 bg-nofx-gold/10'
                            : 'border-white/10 bg-black/20 hover:border-white/20'
                        }`}
                        onClick={() => setSelectedVersionId(version.version.id)}
                      >
                        <div className="flex items-start justify-between gap-4">
                          <div>
                            <div className="text-xs text-nofx-gold uppercase">
                              {formatStrategyVersionSourceType(
                                version.version.source_type
                              )}
                            </div>
                            <div className="font-medium mt-1">
                              {version.version.summary || 'Strategy change'}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-2">
                              Applied{' '}
                              {new Date(
                                version.version.applied_at ||
                                  version.version.created_at
                              ).toLocaleString()}
                            </div>
                            {version.version.expected_effect && (
                              <div className="text-xs text-nofx-text-muted mt-2">
                                Intended effect: {version.version.expected_effect}
                              </div>
                            )}
                            {hasCompareLink && (
                              <div className="text-xs text-sky-300 mt-2">
                                Compare note: {compareNote}
                              </div>
                            )}
                            {version.attribution?.warnings &&
                              version.attribution.warnings.length > 0 && (
                                <div className="text-xs text-rose-300 mt-2">
                                  {version.attribution.warnings[0]}
                                </div>
                              )}
                          </div>
                          <div className="flex flex-wrap justify-end gap-2">
                            <button
                              onClick={(event) => {
                                event.stopPropagation()
                                openVersionDetail(version.version.id)
                              }}
                              className="h-9 px-3 rounded-lg border border-nofx-gold/30 text-nofx-gold"
                            >
                              Open detail
                            </button>
                            {hasCompareLink && (
                              <button
                                onClick={(event) => {
                                  event.stopPropagation()
                                  void openVersionCompare(version)
                                }}
                                disabled={
                                  openingCompareId ===
                                  version.version.source_compare_id
                                }
                                className="h-9 px-3 rounded-lg border border-sky-400/30 text-sky-300 disabled:opacity-50"
                              >
                                {openingCompareId ===
                                version.version.source_compare_id
                                  ? 'Opening…'
                                  : 'Open compare'}
                              </button>
                            )}
                            <button
                              onClick={(event) => {
                                event.stopPropagation()
                                void rollbackVersion(version)
                              }}
                              disabled={rollingBackVersionId === version.version.id}
                              className="h-9 px-3 rounded-lg border border-white/15 text-white disabled:opacity-50"
                            >
                              {rollingBackVersionId === version.version.id
                                ? 'Rolling back…'
                                : 'Rollback'}
                            </button>
                          </div>
                        </div>
                      </div>
                    )
                  })
                )}
              </div>

              {selectedVersion && (
                <div
                  ref={versionDetailRef}
                  className="rounded-xl border border-white/10 bg-black/20 p-5 mt-5"
                >
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                    <div>
                      <div className="text-xs uppercase tracking-[0.2em] text-nofx-gold">
                        Strategy version detail
                      </div>
                      <h3 className="font-semibold text-lg mt-1">
                        {selectedVersion.version.summary || 'Strategy change'}
                      </h3>
                      <div className="text-sm text-nofx-text-muted mt-2">
                        {formatStrategyVersionSourceType(
                          selectedVersion.version.source_type
                        )}{' '}
                        · applied{' '}
                        {new Date(
                          selectedVersion.version.applied_at ||
                            selectedVersion.version.created_at
                        ).toLocaleString()}
                      </div>
                      {selectedVersion.version.expected_effect && (
                        <div className="text-sm text-nofx-text-muted mt-2">
                          Intended effect:{' '}
                          {selectedVersion.version.expected_effect}
                        </div>
                      )}
                      {selectedVersion.source_scan_summary && (
                        <div className="text-sm text-nofx-text-muted mt-2">
                          Source scan: {selectedVersion.source_scan_summary}
                        </div>
                      )}
                      {selectedVersion.compare_summary && (
                        <div className="text-sm text-sky-300 mt-2">
                          Linked compare: {selectedVersion.compare_summary}
                        </div>
                      )}
                    </div>

                    <div className="flex flex-wrap gap-2">
                      {selectedVersion.version.source_compare_id?.trim() && (
                        <button
                          onClick={() => openVersionCompare(selectedVersion)}
                          disabled={
                            openingCompareId ===
                            selectedVersion.version.source_compare_id
                          }
                          className="h-9 px-3 rounded-lg border border-sky-400/30 text-sky-300 disabled:opacity-50"
                        >
                          {openingCompareId ===
                          selectedVersion.version.source_compare_id
                            ? 'Opening…'
                            : 'Open compare'}
                        </button>
                      )}
                      <button
                        onClick={() => rollbackVersion(selectedVersion)}
                        disabled={
                          rollingBackVersionId === selectedVersion.version.id
                        }
                        className="h-9 px-3 rounded-lg border border-white/15 text-white disabled:opacity-50"
                      >
                        {rollingBackVersionId === selectedVersion.version.id
                          ? 'Rolling back…'
                          : 'Rollback'}
                      </button>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 mt-5 text-sm">
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        Target cohort
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatVersionCohort(selectedVersion.target_cohort)}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        Full before
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatDatasetMini(
                          selectedVersion.attribution?.full_before_summary
                        )}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        Full after
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatDatasetMini(
                          selectedVersion.attribution?.full_after_summary
                        )}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        Rollback suggestion
                      </div>
                      <div
                        className={`font-semibold mt-1 ${
                          selectedVersion.attribution?.rollback_suggested
                            ? 'text-rose-300'
                            : 'text-emerald-300'
                        }`}
                      >
                        {selectedVersion.attribution?.rollback_suggested
                          ? 'Suggested'
                          : 'Not suggested'}
                      </div>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3 mt-4 text-sm">
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        Target before
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatDatasetMini(
                          selectedVersion.attribution?.target_before_summary
                        )}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        Target after
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatDatasetMini(
                          selectedVersion.attribution?.target_after_summary
                        )}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        Non-target after
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatDatasetMini(
                          selectedVersion.attribution?.non_target_after_summary
                        )}
                      </div>
                    </div>
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4 mt-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                      Observation window
                    </div>
                    <div className="text-sm text-nofx-text-muted">
                      Before:{' '}
                      {selectedVersion.attribution?.before_start
                        ? new Date(
                            selectedVersion.attribution.before_start
                          ).toLocaleString()
                        : '-'}{' '}
                      to{' '}
                      {selectedVersion.attribution?.before_end
                        ? new Date(
                            selectedVersion.attribution.before_end
                          ).toLocaleString()
                        : '-'}
                    </div>
                    <div className="text-sm text-nofx-text-muted mt-1">
                      After:{' '}
                      {selectedVersion.attribution?.after_start
                        ? new Date(
                            selectedVersion.attribution.after_start
                          ).toLocaleString()
                        : '-'}{' '}
                      to{' '}
                      {selectedVersion.attribution?.after_end
                        ? new Date(
                            selectedVersion.attribution.after_end
                          ).toLocaleString()
                        : '-'}
                    </div>
                    {selectedVersion.attribution?.note && (
                      <div className="text-sm text-nofx-text-muted mt-3">
                        {selectedVersion.attribution.note}
                      </div>
                    )}
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4 mt-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                      Regression warnings
                    </div>
                    {!selectedVersion.attribution?.warnings ||
                    selectedVersion.attribution.warnings.length === 0 ? (
                      <div className="text-sm text-emerald-300">
                        No regression warning triggered for the observed window.
                      </div>
                    ) : (
                      <div className="space-y-1 text-sm text-rose-300">
                        {selectedVersion.attribution.warnings.map((item) => (
                          <div key={item}>• {item}</div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </DeepVoidBackground>
  )
}
