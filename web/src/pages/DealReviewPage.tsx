import { useEffect, useMemo, useRef, useState } from 'react'
import { mutate } from 'swr'
import { api } from '../lib/api'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { NofxSelect } from '../components/ui/select'
import { DealReviewTimelineChart } from '../components/trader/DealReviewTimelineChart'
import { useLanguage } from '../contexts/LanguageContext'
import { toDateTimeLocale } from '../i18n/locale'
import { confirmToast, notify } from '../lib/notify'
import type { Language } from '../i18n/translations'
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
  DealReviewLearnedPattern,
  DealReviewLearnedPatternLiveGuardEvent,
  DealReviewLearnedPatternLiveGuardEventSummary,
  DealReviewLearnedPatternLiveGuardStatus,
  DealReviewLearnedPatternSummary,
  DealReviewCaseListItem,
  DealReviewDatasetSummary,
  DealReviewSymbolBehaviorLiveGuardEvent,
  DealReviewSymbolBehaviorLiveGuardEventSummary,
  DealReviewSymbolBehaviorLiveGuardStatus,
  DealReviewSymbolBehaviorPrior,
  DealReviewSymbolBehaviorPriorSummary,
  DealReviewStrategyVersionDetail,
  DealReviewValidationCheck,
  Exchange,
  RemoteModelInfo,
  SemanticMemorySearchHit,
  TraderInfo,
} from '../types'

interface DealReviewPageProps {
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
}

function pickDealReviewText(language: Language, en: string, de: string): string {
  return language === 'de' ? de : en
}

function formatMoney(value: number): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}`
}

function formatPct(value: number): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
}

function formatSemanticSimilarityScore(
  value?: number,
  language: Language = 'en'
): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return `${Math.round(Math.max(0, Math.min(1, value)) * 100)}% ${pickDealReviewText(language, 'match', 'Treffer')}`
}

function formatValidationScope(value: string, language: Language = 'en'): string {
  if (!value) return '-'
  if (value === 'recent') {
    return pickDealReviewText(language, 'Recent live-like', 'Aktuell live-aehnlich')
  }
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

function formatValidationCheckSummary(
  check: DealReviewValidationCheck,
  language: Language = 'en'
): string {
  if (check.comparator === 'delta_gte') {
    return `${pickDealReviewText(language, 'delta', 'Delta')} ${formatValidationMetricValue(check.metric, check.delta)} | ${pickDealReviewText(language, 'floor', 'Untergrenze')} ${formatValidationMetricValue(check.metric, check.threshold)}`
  }
  if (check.comparator === 'lte') {
    return `${pickDealReviewText(language, 'actual', 'Ist')} ${formatValidationMetricValue(check.metric, check.actual)} | ${pickDealReviewText(language, 'ceiling', 'Obergrenze')} ${formatValidationMetricValue(check.metric, check.threshold)}`
  }
  return `${pickDealReviewText(language, 'actual', 'Ist')} ${formatValidationMetricValue(check.metric, check.actual)} | ${pickDealReviewText(language, 'floor', 'Untergrenze')} ${formatValidationMetricValue(check.metric, check.threshold)}`
}

function formatSymbolBehaviorAction(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'penalize_setup':
      return pickDealReviewText(language, 'Penalize setup', 'Setup bestrafen')
    case 'favor_setup':
      return pickDealReviewText(language, 'Favor setup', 'Setup bevorzugen')
    case 'observe':
      return pickDealReviewText(language, 'Observe', 'Beobachten')
    default:
      return value || '-'
  }
}

function formatSymbolBehaviorValidationLabel(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'confirmed':
      return pickDealReviewText(language, 'Confirmed', 'Bestaetigt')
    case 'candidate':
      return pickDealReviewText(language, 'Candidate', 'Kandidat')
    case 'insufficient_evidence':
      return pickDealReviewText(language, 'Need evidence', 'Mehr Belege noetig')
    case 'drifting':
      return pickDealReviewText(language, 'Drifting', 'Abdriftend')
    case 'false_positive':
      return pickDealReviewText(language, 'False positive', 'Falsch positiv')
    case 'false_negative_risk':
      return pickDealReviewText(language, 'False-negative risk', 'Falsch-negativ-Risiko')
    default:
      return value || '-'
  }
}

function symbolBehaviorValidationClasses(value?: string): string {
  switch (value) {
    case 'confirmed':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'false_positive':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'false_negative_risk':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'drifting':
      return 'border-orange-400/20 bg-orange-500/10 text-orange-200'
    case 'insufficient_evidence':
      return 'border-white/10 bg-white/5 text-white'
    default:
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
  }
}

function symbolBehaviorToneClasses(value?: string): string {
  switch (value) {
    case 'negative':
      return 'border-rose-400/25 bg-rose-500/10 text-rose-200'
    case 'positive':
      return 'border-emerald-400/25 bg-emerald-500/10 text-emerald-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternClass(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'positive_edge':
      return pickDealReviewText(language, 'Positive edge', 'Positiver Vorteil')
    case 'negative_edge':
      return pickDealReviewText(language, 'Anti-edge', 'Anti-Muster')
    default:
      return value || '-'
  }
}

function learnedPatternClassClasses(value?: string): string {
  switch (value) {
    case 'positive_edge':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'negative_edge':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternValidationLabel(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'confirmed':
      return pickDealReviewText(language, 'Confirmed', 'Bestaetigt')
    case 'candidate':
      return pickDealReviewText(language, 'Candidate', 'Kandidat')
    case 'insufficient_evidence':
      return pickDealReviewText(language, 'Need evidence', 'Mehr Belege noetig')
    case 'false_positive':
      return pickDealReviewText(language, 'False positive', 'Falsch positiv')
    case 'reverse_risk':
      return pickDealReviewText(language, 'Reverse risk', 'Umkehrrisiko')
    case 'drifting':
      return pickDealReviewText(language, 'Drifting', 'Abdriftend')
    case 'expired':
      return pickDealReviewText(language, 'Expired', 'Abgelaufen')
    default:
      return value || '-'
  }
}

function learnedPatternValidationClasses(value?: string): string {
  switch (value) {
    case 'confirmed':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'false_positive':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'reverse_risk':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'drifting':
      return 'border-orange-400/20 bg-orange-500/10 text-orange-200'
    case 'expired':
      return 'border-white/10 bg-white/5 text-nofx-text-muted'
    case 'insufficient_evidence':
      return 'border-white/10 bg-white/5 text-white'
    default:
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
  }
}

function formatLearnedPatternUse(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'review_hint':
      return pickDealReviewText(language, 'Review hint', 'Review-Hinweis')
    case 'prompt_hint':
      return pickDealReviewText(language, 'Prompt hint', 'Prompt-Hinweis')
    case 'config_candidate':
      return pickDealReviewText(language, 'Config candidate', 'Konfig-Kandidat')
    case 'monitoring_rule':
      return pickDealReviewText(language, 'Monitoring rule', 'Monitoring-Regel')
    case 'monitor_only':
      return pickDealReviewText(language, 'Monitor only', 'Nur beobachten')
    case 'expired_do_not_use':
      return pickDealReviewText(language, 'Do not use', 'Nicht verwenden')
    default:
      return value || '-'
  }
}

function formatLearnedPatternLifecycleStatus(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'active':
      return pickDealReviewText(language, 'Active', 'Aktiv')
    case 'degrading':
      return pickDealReviewText(language, 'Degrading', 'Verschlechternd')
    case 'rollback_watch':
      return pickDealReviewText(language, 'Rollback watch', 'Rollback-Watch')
    case 'expired':
      return pickDealReviewText(language, 'Expired', 'Abgelaufen')
    default:
      return value || '-'
  }
}

function learnedPatternLifecycleClasses(value?: string): string {
  switch (value) {
    case 'active':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'degrading':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'rollback_watch':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'expired':
      return 'border-white/10 bg-white/5 text-nofx-text-muted'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternScope(
  pattern?: DealReviewLearnedPattern | null,
  language: Language = 'en'
): string {
  if (!pattern) return '-'
  if (pattern.scope_type === 'global') {
    return pickDealReviewText(language, 'Global', 'Global')
  }
  if (pattern.scope_type === 'regime_local') {
    return pattern.regime_signature
      ? `${pickDealReviewText(language, 'Regime', 'Regime')} ${pattern.regime_signature}`
      : pickDealReviewText(language, 'Regime-local', 'Regime-lokal')
  }
  if (pattern.scope_type === 'symbol' && pattern.symbol) {
    return pickDealReviewText(language, `${pattern.symbol} override`, `${pattern.symbol} Override`)
  }
  if (pattern.scope_type === 'trader_local') {
    return pickDealReviewText(language, 'Trader-local', 'Trader-lokal')
  }
  return pattern.scope_type || '-'
}

function symbolBehaviorTabClasses(value: string, active: boolean): string {
  const base =
    'inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors'
  if (!active) {
    return `${base} border-white/10 bg-black/20 text-nofx-text-muted hover:border-white/20 hover:text-white`
  }
  switch (value) {
    case 'confirmed':
      return `${base} border-emerald-400/25 bg-emerald-500/15 text-emerald-200`
    case 'false_positive':
      return `${base} border-rose-400/25 bg-rose-500/15 text-rose-200`
    case 'false_negative_risk':
      return `${base} border-amber-400/25 bg-amber-500/15 text-amber-200`
    case 'drifting':
      return `${base} border-orange-400/25 bg-orange-500/15 text-orange-200`
    default:
      return `${base} border-sky-400/25 bg-sky-500/15 text-sky-200`
  }
}

function formatSymbolBehaviorLiveGuardMode(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'hard_block':
      return pickDealReviewText(language, 'Hard block', 'Harter Block')
    case 'monitor':
      return pickDealReviewText(language, 'Monitor', 'Beobachten')
    default:
      return value || '-'
  }
}

function formatSymbolBehaviorLiveGuardEffect(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'hard_blocked':
      return pickDealReviewText(language, 'Hard blocked', 'Hart blockiert')
    case 'monitor_only':
      return pickDealReviewText(language, 'Monitor only', 'Nur beobachten')
    case 'matched_not_qualified':
      return pickDealReviewText(language, 'Matched, not qualified', 'Getroffen, nicht qualifiziert')
    case 'no_match':
      return pickDealReviewText(language, 'No match', 'Kein Treffer')
    default:
      return value || '-'
  }
}

function symbolBehaviorLiveGuardEffectClasses(value?: string): string {
  switch (value) {
    case 'hard_blocked':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'monitor_only':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'matched_not_qualified':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    case 'no_match':
      return 'border-white/10 bg-white/5 text-white'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternLiveGuardAttributionStatus(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'correctly_blocked':
      return pickDealReviewText(language, 'Correctly blocked', 'Korrekt blockiert')
    case 'overblocked':
      return pickDealReviewText(language, 'Overblocked', 'Ueberblockiert')
    case 'warning_confirmed':
      return pickDealReviewText(language, 'Warning confirmed', 'Warnung bestaetigt')
    case 'warning_not_confirmed':
      return pickDealReviewText(language, 'Warning not confirmed', 'Warnung nicht bestaetigt')
    case 'threshold_missed_loss':
      return pickDealReviewText(language, 'Threshold missed loss', 'Schwellenwert verpasster Verlust')
    case 'threshold_missed_profit':
      return pickDealReviewText(language, 'Threshold missed profit', 'Schwellenwert verpasster Gewinn')
    case 'followup_open':
      return pickDealReviewText(language, 'Follow-up open', 'Follow-up offen')
    case 'pending':
      return pickDealReviewText(language, 'Pending', 'Ausstehend')
    default:
      return value || '-'
  }
}

function learnedPatternLiveGuardAttributionClasses(value?: string): string {
  switch (value) {
    case 'correctly_blocked':
    case 'warning_confirmed':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'overblocked':
    case 'warning_not_confirmed':
    case 'threshold_missed_profit':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'threshold_missed_loss':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'followup_open':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    case 'pending':
      return 'border-white/10 bg-white/5 text-white'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternRollupLabel(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'protective':
      return pickDealReviewText(language, 'Protective', 'Schuetzend')
    case 'overblocking':
      return pickDealReviewText(language, 'Overblocking', 'Ueberblockierend')
    case 'mixed':
      return pickDealReviewText(language, 'Mixed', 'Gemischt')
    case 'pending':
      return pickDealReviewText(language, 'Pending', 'Ausstehend')
    default:
      return value || '-'
  }
}

function learnedPatternRollupClasses(value?: string): string {
  switch (value) {
    case 'protective':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'overblocking':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'mixed':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'pending':
      return 'border-white/10 bg-white/5 text-white'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternRollupTrendLabel(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'improving':
      return pickDealReviewText(language, 'Improving', 'Verbessernd')
    case 'stable':
      return pickDealReviewText(language, 'Stable', 'Stabil')
    case 'degrading':
      return pickDealReviewText(language, 'Degrading', 'Verschlechternd')
    case 'newly_overblocking':
      return pickDealReviewText(language, 'Newly overblocking', 'Neu ueberblockierend')
    case 'insufficient_evidence':
      return pickDealReviewText(language, 'Need evidence', 'Mehr Belege noetig')
    default:
      return value || '-'
  }
}

function learnedPatternRollupTrendClasses(value?: string): string {
  switch (value) {
    case 'improving':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'stable':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    case 'degrading':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'newly_overblocking':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'insufficient_evidence':
      return 'border-white/10 bg-white/5 text-white'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatTimestampLabel(value?: string, language: Language = 'en'): string {
  if (!value) return '-'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return '-'
  return parsed.toLocaleString(toDateTimeLocale(language))
}

function formatCompactTimestampLabel(
  value?: string,
  language: Language = 'en'
): string {
  if (!value) return '-'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return '-'
  return parsed.toLocaleString(toDateTimeLocale(language), {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatLifecycleSnapshotSource(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'live_guard_event':
      return pickDealReviewText(language, 'guard', 'Guard')
    case 'rebuild':
      return pickDealReviewText(language, 'rebuild', 'Rebuild')
    default:
      return value || pickDealReviewText(language, 'snapshot', 'Snapshot')
  }
}

function formatLearnedPatternManualControlState(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'live':
      return pickDealReviewText(language, 'Live', 'Live')
    case 'suppressed':
      return pickDealReviewText(language, 'Suppressed', 'Unterdrueckt')
    case 'retired':
      return pickDealReviewText(language, 'Retired', 'Stillgelegt')
    default:
      return value || '-'
  }
}

function learnedPatternManualControlClasses(value?: string): string {
  switch (value) {
    case 'live':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'suppressed':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'retired':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternManualControlAction(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'acknowledge_live':
      return pickDealReviewText(language, 'Keep live', 'Live lassen')
    case 'suppress':
      return pickDealReviewText(language, 'Suppress', 'Unterdruecken')
    case 'retire':
      return pickDealReviewText(language, 'Retire', 'Stilllegen')
    case 'rearm':
      return pickDealReviewText(language, 'Re-arm', 'Reaktivieren')
    default:
      return value || '-'
  }
}

function formatLearnedPatternActionHintAction(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'suppress':
      return pickDealReviewText(language, 'Suggest suppress', 'Unterdruecken empfehlen')
    case 'retire':
      return pickDealReviewText(language, 'Suggest retire', 'Stilllegung empfehlen')
    case 'rearm':
      return pickDealReviewText(language, 'Suggest re-arm', 'Reaktivierung empfehlen')
    case 'acknowledge_live':
      return pickDealReviewText(language, 'Suggest keep live', 'Live lassen empfehlen')
    default:
      return value || '-'
  }
}

function learnedPatternActionHintActionClasses(value?: string): string {
  switch (value) {
    case 'suppress':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'retire':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'rearm':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    case 'acknowledge_live':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternActionHintPriority(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'critical':
      return pickDealReviewText(language, 'Critical', 'Kritisch')
    case 'warning':
      return pickDealReviewText(language, 'Warning', 'Warnung')
    case 'info':
      return pickDealReviewText(language, 'Info', 'Info')
    default:
      return value || '-'
  }
}

function learnedPatternActionHintPriorityClasses(value?: string): string {
  switch (value) {
    case 'critical':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'warning':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'info':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternLiveActionKind(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'suppression':
      return pickDealReviewText(
        language,
        'Direct suppression candidate',
        'Direkter Unterdrueckungs-Kandidat'
      )
    case 'rollback':
      return pickDealReviewText(language, 'Rollback-grade candidate', 'Rollback-Kandidat')
    default:
      return value || '-'
  }
}

function learnedPatternLiveActionKindClasses(value?: string): string {
  switch (value) {
    case 'suppression':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'rollback':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternInterventionEventType(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'suggested':
      return pickDealReviewText(language, 'Suggested', 'Vorgeschlagen')
    case 'manual_action':
      return pickDealReviewText(language, 'Manual action', 'Manuelle Aktion')
    default:
      return value || '-'
  }
}

function learnedPatternInterventionEventTypeClasses(value?: string): string {
  switch (value) {
    case 'suggested':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    case 'manual_action':
      return 'border-nofx-gold/30 bg-nofx-gold/10 text-nofx-gold'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLearnedPatternInterventionStatus(
  value?: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'open':
      return pickDealReviewText(language, 'Open', 'Offen')
    case 'accepted':
      return pickDealReviewText(language, 'Accepted', 'Akzeptiert')
    case 'overridden':
      return pickDealReviewText(language, 'Overridden', 'Ueberschrieben')
    case 'superseded':
      return pickDealReviewText(language, 'Superseded', 'Ersetzt')
    case 'cleared':
      return pickDealReviewText(language, 'Cleared', 'Bereinigt')
    case 'standalone':
      return pickDealReviewText(language, 'Standalone', 'Eigenstaendig')
    default:
      return value || '-'
  }
}

function learnedPatternInterventionStatusClasses(value?: string): string {
  switch (value) {
    case 'open':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'accepted':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'overridden':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'superseded':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    case 'cleared':
      return 'border-white/10 bg-white/5 text-nofx-text-muted'
    case 'standalone':
      return 'border-white/10 bg-black/20 text-white/80'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function learnedPatternInterventionTimestampLabel(event: {
  resolved_at?: string
  last_seen_at?: string
  first_seen_at?: string
  created_at?: string
}): string {
  return (
    event.resolved_at ||
    event.last_seen_at ||
    event.first_seen_at ||
    event.created_at ||
    ''
  )
}

function normalizeLearnedPatternLiveActionKind(value?: string): string {
  switch (value) {
    case 'suppression':
      return 'suppression'
    case 'rollback':
      return 'rollback'
    default:
      return ''
  }
}

function normalizeLearnedPatternInterventionState(value?: string): string {
  switch (value) {
    case 'open':
      return 'open'
    case 'accepted':
    case 'overridden':
    case 'superseded':
    case 'cleared':
    case 'standalone':
      return 'resolved'
    default:
      return ''
  }
}

function learnedPatternHasOpenIntervention(
  pattern?: DealReviewLearnedPattern | null
): boolean {
  return Boolean(
    pattern?.intervention_history?.some(
      (event) => normalizeLearnedPatternInterventionState(event.event_status) === 'open'
    )
  )
}

function learnedPatternHasResolvedIntervention(
  pattern?: DealReviewLearnedPattern | null
): boolean {
  return Boolean(
    pattern?.intervention_history?.some(
      (event) =>
        normalizeLearnedPatternInterventionState(event.event_status) === 'resolved'
    )
  )
}

function learnedPatternStrongestLiveActionKind(
  pattern?: DealReviewLearnedPattern | null,
  openOnly = false
): string {
  const directKinds = (pattern?.intervention_history || [])
    .filter((event) => {
      if (!event.direct_live_action_candidate) return false
      if (!openOnly) return true
      return normalizeLearnedPatternInterventionState(event.event_status) === 'open'
    })
    .map((event) => normalizeLearnedPatternLiveActionKind(event.direct_live_action_kind))
    .filter(Boolean)

  if (directKinds.includes('rollback')) return 'rollback'
  if (directKinds.includes('suppression')) return 'suppression'
  if (!openOnly) {
    return normalizeLearnedPatternLiveActionKind(
      pattern?.live_action_hint?.candidate_kind
    )
  }
  return ''
}

function learnedPatternHasDirectLiveActionCandidate(
  pattern?: DealReviewLearnedPattern | null
): boolean {
  if (normalizeLearnedPatternLiveActionKind(pattern?.live_action_hint?.candidate_kind)) {
    return true
  }
  return Boolean(
    pattern?.intervention_history?.some(
      (event) =>
        event.direct_live_action_candidate ||
        Boolean(normalizeLearnedPatternLiveActionKind(event.direct_live_action_kind))
    )
  )
}

function learnedPatternMatchesInterventionFilter(
  pattern: DealReviewLearnedPattern,
  value: string
): boolean {
  switch (value) {
    case '':
      return true
    case 'open':
      return learnedPatternHasOpenIntervention(pattern)
    case 'resolved':
      return learnedPatternHasResolvedIntervention(pattern)
    case 'none':
      return !pattern.intervention_history || pattern.intervention_history.length === 0
    default:
      return true
  }
}

function learnedPatternReviewPriorityScore(
  pattern: DealReviewLearnedPattern
): number {
  let score = Math.max(0, pattern.composite_score || 0) * 100
  score += Math.max(0, pattern.confidence_score || 0) * 35
  score += Math.max(0, pattern.match_score || 0) * 25
  if (learnedPatternHasOpenIntervention(pattern)) score += 120
  if (learnedPatternHasResolvedIntervention(pattern)) score += 20
  if (learnedPatternHasDirectLiveActionCandidate(pattern)) score += 80

  switch (learnedPatternStrongestLiveActionKind(pattern, true)) {
    case 'rollback':
      score += 200
      break
    case 'suppression':
      score += 120
      break
    default:
      switch (learnedPatternStrongestLiveActionKind(pattern, false)) {
        case 'rollback':
          score += 40
          break
        case 'suppression':
          score += 20
          break
      }
  }

  return score
}

function formatVersionCohort(
  targetCohort?: Record<string, unknown>,
  language: Language = 'en'
): string {
  if (!targetCohort || Object.keys(targetCohort).length === 0) {
    return pickDealReviewText(language, 'No explicit target cohort stored', 'Keine explizite Zielkohorte gespeichert')
  }
  return Object.entries(targetCohort)
    .map(([key, value]) => `${key}: ${String(value)}`)
    .join(' | ')
}

function formatDatasetMini(
  summary?: DealReviewDatasetSummary,
  language: Language = 'en'
): string {
  if (!summary) return pickDealReviewText(language, 'No closed deals', 'Keine geschlossenen Deals')
  return `${summary.closed_deals} ${pickDealReviewText(language, 'deals', 'Deals')} | ${formatMoney(summary.net_pnl)} | ${summary.win_rate.toFixed(1)}% ${pickDealReviewText(language, 'win', 'Gewinn')} | PF ${summary.profit_factor.toFixed(2)}`
}

function formatClassifierIssueType(
  issueType?: string,
  language: Language = 'en'
): string {
  switch (issueType) {
    case 'likely_bad_trade':
      return pickDealReviewText(language, 'Likely bad trade', 'Wahrscheinlich schlechter Trade')
    case 'likely_bad_exit':
      return pickDealReviewText(language, 'Likely bad exit', 'Wahrscheinlich schlechter Exit')
    case 'likely_avoidable_loss':
      return pickDealReviewText(language, 'Likely avoidable loss', 'Wahrscheinlich vermeidbarer Verlust')
    case 'likely_regime_mismatch':
      return pickDealReviewText(language, 'Likely regime mismatch', 'Wahrscheinlich Regime-Fehlanpassung')
    case 'other_review_signal':
      return pickDealReviewText(language, 'Other review signal', 'Sonstiges Review-Signal')
    default:
      return pickDealReviewText(language, 'Review signal', 'Review-Signal')
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

function formatDate(ms: number, language: Language = 'en'): string {
  if (!ms) return '-'
  return new Date(ms).toLocaleString(toDateTimeLocale(language))
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

function formatHoursCompact(value?: number): string {
  if (typeof value !== 'number' || Number.isNaN(value) || value <= 0) return '-'
  if (value < 24) return `${value.toFixed(value < 10 ? 1 : 0)}h`
  const days = value / 24
  return `${days.toFixed(days < 10 ? 1 : 0)}d`
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

function buildQualityNarrative(
  caseRec: DealReviewCase,
  language: Language = 'en'
): string {
  if (
    caseRec.entry_timing_score >= 70 &&
    caseRec.max_favorable_excursion > 0.05 &&
    caseRec.exit_efficiency_score < 40
  ) {
    return pickDealReviewText(
      language,
      `Entry was valid, but exit captured only ${Math.round(caseRec.mfe_captured_pct || 0)}% of available MFE.`,
      `Der Einstieg war valide, aber der Exit hat nur ${Math.round(caseRec.mfe_captured_pct || 0)}% der verfuegbaren MFE eingefangen.`
    )
  }
  if (caseRec.realized_pnl < 0 && caseRec.profit_given_back > 0.05) {
    return pickDealReviewText(
      language,
      `This loss was avoidable: the trade gave back ${formatMoney(caseRec.profit_given_back)} after being in profit.`,
      `Dieser Verlust war vermeidbar: Der Trade hat ${formatMoney(caseRec.profit_given_back)} abgegeben, nachdem er bereits im Gewinn war.`
    )
  }
  if (
    caseRec.realized_pnl > 0 &&
    caseRec.entry_timing_score < 35
  ) {
    return pickDealReviewText(
      language,
      'Weak entry recovered into profit. Treat this as a lucky exit, not a clean edge.',
      'Ein schwacher Einstieg hat sich noch in Gewinn gedreht. Das ist eher ein Gluecks-Exit als ein sauberer Vorteil.'
    )
  }
  if (caseRec.risk_sizing_score > 0 && caseRec.risk_sizing_score < 35) {
    return pickDealReviewText(
      language,
      `Risk sizing was aggressive for this path: planned risk was ${formatPct(caseRec.planned_risk_pct || 0)}.`,
      `Das Risikosizing war auf diesem Pfad aggressiv: Das geplante Risiko lag bei ${formatPct(caseRec.planned_risk_pct || 0)}.`
    )
  }
  return ''
}

function buildQualityBadges(
  caseRec: DealReviewCase,
  language: Language = 'en'
): Array<{
  label: string
  tone: 'good' | 'warn' | 'bad'
}> {
  const badges: Array<{ label: string; tone: 'good' | 'warn' | 'bad' }> = []

  if (
    caseRec.entry_timing_score >= 70 &&
    caseRec.max_favorable_excursion > 0.05 &&
    caseRec.exit_efficiency_score < 40
  ) {
    badges.push({
      label: pickDealReviewText(language, 'Strong entry / weak exit', 'Starker Einstieg / schwacher Exit'),
      tone: 'warn',
    })
  } else if (caseRec.entry_timing_score >= 70) {
    badges.push({ label: pickDealReviewText(language, 'Strong entry', 'Starker Einstieg'), tone: 'good' })
  } else if (caseRec.entry_timing_score < 35) {
    badges.push({ label: pickDealReviewText(language, 'Weak entry', 'Schwacher Einstieg'), tone: 'bad' })
  }

  if (caseRec.max_favorable_excursion > 0.05) {
    if (caseRec.exit_efficiency_score >= 75) {
      badges.push({ label: pickDealReviewText(language, 'Strong exit', 'Starker Exit'), tone: 'good' })
    } else if (caseRec.exit_efficiency_score < 35) {
      badges.push({ label: pickDealReviewText(language, 'Weak exit', 'Schwacher Exit'), tone: 'bad' })
    }
  }

  if (caseRec.realized_pnl < 0 && caseRec.profit_given_back > 0.05) {
    badges.push({ label: pickDealReviewText(language, 'Avoidable loss', 'Vermeidbarer Verlust'), tone: 'warn' })
  }

  if (
    caseRec.realized_pnl > 0 &&
    caseRec.entry_timing_score < 35
  ) {
    badges.push({
      label: pickDealReviewText(language, 'Weak entry / lucky exit', 'Schwacher Einstieg / Gluecks-Exit'),
      tone: 'warn',
    })
  }

  if (caseRec.risk_sizing_score > 0 && caseRec.risk_sizing_score < 35) {
    badges.push({ label: pickDealReviewText(language, 'Oversized risk', 'Ueberzogenes Risiko'), tone: 'bad' })
  }

  return badges.slice(0, 4)
}

function formatCompareMode(value: string, language: Language = 'en'): string {
  switch (value) {
    case 'shared_live':
      return pickDealReviewText(language, 'Shared live wallet', 'Geteilte Live-Wallet')
    case 'isolated_live':
      return pickDealReviewText(language, 'Isolated live wallet', 'Isolierte Live-Wallet')
    case 'paper':
      return pickDealReviewText(language, 'Paper / simulation', 'Paper / Simulation')
    default:
      return value || '-'
  }
}

function formatCompareStatus(value: string, language: Language = 'en'): string {
  if (!value) return '-'
  switch (value) {
    case 'starting':
      return pickDealReviewText(language, 'Starting', 'Startet')
    case 'running':
      return pickDealReviewText(language, 'Running', 'Laeuft')
    case 'resolved':
      return pickDealReviewText(language, 'Resolved', 'Abgeschlossen')
    case 'stopped':
      return pickDealReviewText(language, 'Stopped', 'Gestoppt')
    case 'failed':
      return pickDealReviewText(language, 'Failed', 'Fehlgeschlagen')
    default:
      break
  }
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

function formatCompareFlagTitle(
  code?: string,
  fallback?: string,
  language: Language = 'en'
): string {
  switch (code) {
    case 'strong_consensus':
      return pickDealReviewText(language, 'Strong consensus', 'Starker Konsens')
    case 'mixed_recommendation':
      return pickDealReviewText(language, 'Mixed recommendation', 'Gemischte Empfehlung')
    case 'low_confidence_disagreement':
      return pickDealReviewText(language, 'Low-confidence disagreement', 'Widerspruch mit niedriger Konfidenz')
    default:
      return fallback || pickDealReviewText(language, 'Compare signal', 'Vergleichssignal')
  }
}

function isActiveCompareStatus(value: string): boolean {
  return value === 'starting' || value === 'running'
}

function formatCompareProtocolType(
  value: string,
  language: Language = 'en'
): string {
  if (!value) return pickDealReviewText(language, 'Event', 'Ereignis')
  return value.replace(/_/g, ' ')
}

function formatStrategyVersionSourceType(
  value: string,
  language: Language = 'en'
): string {
  switch (value) {
    case 'ai_apply':
      return pickDealReviewText(language, 'AI apply', 'KI-Uebernahme')
    case 'ai_challenger_candidate':
      return pickDealReviewText(language, 'Challenger candidate', 'Challenger-Kandidat')
    case 'rollback':
      return pickDealReviewText(language, 'Rollback', 'Rollback')
    default:
      return value.replace(/_/g, ' ') || pickDealReviewText(language, 'Strategy change', 'Strategieaenderung')
  }
}

function buildCompareProtocolMetricSummary(
  event?: DealReviewChallengerProtocolEvent,
  language: Language = 'en'
): string {
  const metrics = event?.metrics
  if (!metrics) return ''
  return `${pickDealReviewText(language, 'Incumbent', 'Inkumbent')} ${formatMoney(metrics.incumbent_pnl)} (${metrics.incumbent_trade_count} ${pickDealReviewText(language, 'trades', 'Trades')}) | ${pickDealReviewText(language, 'Challenger', 'Challenger')} ${formatMoney(metrics.challenger_pnl)} (${metrics.challenger_trade_count} ${pickDealReviewText(language, 'trades', 'Trades')})`
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

function getPatchOutcomeLabel(
  delta: number | null,
  rollbackSuggested?: boolean,
  language: Language = 'en'
): string {
  if (rollbackSuggested) return pickDealReviewText(language, 'Regression risk', 'Regressionsrisiko')
  if (typeof delta === 'number' && delta < -0.01) {
    return pickDealReviewText(language, 'Weaker', 'Schwaecher')
  }
  if (typeof delta === 'number' && delta > 0.01) {
    return pickDealReviewText(language, 'Improved', 'Verbessert')
  }
  return pickDealReviewText(language, 'Mixed / flat', 'Gemischt / flach')
}

function normalizePresetText(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function buildQueueLabel(queueMode: string, language: Language = 'en'): string {
  switch (queueMode) {
    case 'unlabeled_losses':
      return pickDealReviewText(language, 'Unlabeled losses', 'Unbeschriftete Verluste')
    case 'biggest_giveback':
      return pickDealReviewText(language, 'Biggest give-back exits', 'Groesste Gewinnabgaben')
    case 'regime_mismatch':
      return pickDealReviewText(language, 'Regime mismatch candidates', 'Regime-Fehlanpassungs-Kandidaten')
    default:
      return pickDealReviewText(language, 'All filtered deals', 'Alle gefilterten Deals')
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

function formatDealReasonPreview(
  value?: string,
  language: Language = 'en'
): string {
  const text = (value || '').trim()
  if (!text) {
    return pickDealReviewText(
      language,
      'No linked rationale snapshot.',
      'Kein verknuepfter Begruendungs-Snapshot.'
    )
  }
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
  relatedCompare?: DealReviewChallengerCompareDetail,
  language: Language = 'en'
): { label: string; tone: string; detail: string } {
  const validationStatus =
    scan.validation?.status || scan.scan.validation_status || 'pending'
  if (validationStatus !== 'passed') {
    return {
      label:
        validationStatus === 'failed'
          ? pickDealReviewText(language, 'Blocked by validation', 'Durch Validierung blockiert')
          : pickDealReviewText(language, 'Validation pending', 'Validierung ausstehend'),
      tone: validationStatus === 'failed' ? 'rose' : 'amber',
      detail:
        validationStatus === 'failed'
          ? pickDealReviewText(
              language,
              'This scan cannot be promoted until the blocking validation checks pass.',
              'Dieser Scan kann erst uebernommen werden, wenn die blockierenden Validierungspruefungen bestehen.'
            )
          : pickDealReviewText(
              language,
              'Validation has not completed yet.',
              'Die Validierung ist noch nicht abgeschlossen.'
            ),
    }
  }

  if (!relatedCompare) {
    return {
      label: pickDealReviewText(language, 'Validated, challenger not started', 'Validiert, Challenger nicht gestartet'),
      tone: 'amber',
      detail:
        pickDealReviewText(
          language,
          'Validation passed. The patch is ready for direct apply or for a challenger launch.',
          'Die Validierung ist bestanden. Der Patch ist bereit fuer direkte Uebernahme oder einen Challenger-Start.'
        ),
    }
  }

  switch (relatedCompare.compare.status) {
    case 'starting':
    case 'running':
      return {
        label: pickDealReviewText(language, 'Challenger running', 'Challenger laeuft'),
        tone: 'sky',
        detail:
          relatedCompare.compare.summary ||
          pickDealReviewText(
            language,
            'Incumbent and challenger are currently in the comparison window.',
            'Inkumbent und Challenger befinden sich aktuell im Vergleichsfenster.'
          ),
      }
    case 'completed':
      if (
        relatedCompare.compare.winner_trader_id &&
        relatedCompare.compare.winner_trader_id ===
          relatedCompare.compare.challenger_trader_id
      ) {
        return {
          label: pickDealReviewText(language, 'Winner promoted', 'Sieger uebernommen'),
          tone: 'emerald',
          detail:
            relatedCompare.compare.summary ||
            pickDealReviewText(
              language,
              'The challenger won on realized PnL and the incumbent was deactivated automatically.',
              'Der Challenger gewann beim realisierten PnL und der Inkumbent wurde automatisch deaktiviert.'
            ),
        }
      }
      if (
        relatedCompare.compare.winner_trader_id &&
        relatedCompare.compare.winner_trader_id ===
          relatedCompare.compare.incumbent_trader_id
      ) {
        return {
          label: pickDealReviewText(language, 'Challenger rejected', 'Challenger abgelehnt'),
          tone: 'rose',
          detail:
            relatedCompare.compare.summary ||
            pickDealReviewText(
              language,
              'The incumbent stayed ahead on realized PnL and the challenger was deactivated automatically.',
              'Der Inkumbent blieb beim realisierten PnL vorne und der Challenger wurde automatisch deaktiviert.'
            ),
        }
      }
      return {
        label: pickDealReviewText(language, 'Challenger finished', 'Challenger abgeschlossen'),
        tone: 'emerald',
        detail:
          relatedCompare.compare.summary ||
          pickDealReviewText(
            language,
            'The challenger comparison finished.',
            'Der Challenger-Vergleich wurde abgeschlossen.'
          ),
      }
    case 'stopped':
      return {
        label: pickDealReviewText(language, 'Challenger finished', 'Challenger abgeschlossen'),
        tone: 'amber',
        detail:
          relatedCompare.compare.summary ||
          pickDealReviewText(
            language,
            'The challenger comparison was stopped manually before auto resolution.',
            'Der Challenger-Vergleich wurde vor der automatischen Aufloesung manuell gestoppt.'
          ),
      }
    case 'failed':
      return {
        label: pickDealReviewText(language, 'Challenger finished', 'Challenger abgeschlossen'),
        tone: 'rose',
        detail:
          relatedCompare.compare.summary ||
          relatedCompare.compare.error_message ||
          pickDealReviewText(
            language,
            'The challenger workflow failed before completion.',
            'Der Challenger-Workflow ist vor dem Abschluss fehlgeschlagen.'
          ),
      }
    default:
      return {
        label: pickDealReviewText(language, 'Validated, challenger not started', 'Validiert, Challenger nicht gestartet'),
        tone: 'amber',
        detail:
          pickDealReviewText(
            language,
            'Validation passed. The patch is ready for direct apply or for a challenger launch.',
            'Die Validierung ist bestanden. Der Patch ist bereit fuer direkte Uebernahme oder einen Challenger-Start.'
          ),
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

function getDealReviewCloseExitOrigin(detail?: DealReviewCaseDetail | null): string {
  return firstNonEmpty(detail?.close?.event?.exit_origin, detail?.case.exit_origin)
}

function getDealReviewCloseExitReasonQuality(
  detail?: DealReviewCaseDetail | null
): string {
  return firstNonEmpty(
    detail?.close?.event?.exit_reason_quality,
    detail?.case.exit_reason_quality
  )
}

function getDealReviewCloseExitEvidenceSummary(
  detail?: DealReviewCaseDetail | null
): string {
  return firstNonEmpty(
    detail?.close?.event?.exit_evidence_summary,
    detail?.case.exit_evidence_summary
  )
}

function getDealReviewCloseExitEvidence(detail?: DealReviewCaseDetail | null) {
  return detail?.close?.event?.exit_evidence || detail?.case.exit_evidence
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

const exitReasonQualityOptions = [
  { value: '', label: 'All exit evidence' },
  { value: 'explicit', label: 'Explicit' },
  { value: 'high_confidence_inferred', label: 'High-confidence inferred' },
  { value: 'low_confidence_inferred', label: 'Low-confidence inferred' },
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
  const { language } = useLanguage()
  const pickText = (en: string, de: string) => pickDealReviewText(language, en, de)
  const [symbol, setSymbol] = useState('')
  const [side, setSide] = useState('')
  const [status, setStatus] = useState('CLOSED')
  const [outcome, setOutcome] = useState('')
  const [openSelectionBucket, setOpenSelectionBucket] = useState('')
  const [closeReason, setCloseReason] = useState('')
  const [exitReasonQuality, setExitReasonQuality] = useState('')
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
  const [customFromTime, setCustomFromTime] = useState<number | null>(null)
  const [customToTime, setCustomToTime] = useState<number | null>(null)
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
  const [similarCases, setSimilarCases] = useState<SemanticMemorySearchHit[]>([])
  const [similarCasesLoading, setSimilarCasesLoading] = useState(false)
  const [similarCasesError, setSimilarCasesError] = useState<string | null>(null)
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
  const [controlNotes, setControlNotes] = useState<Record<string, string>>({})
  const [controlBusyKey, setControlBusyKey] = useState('')
  const [detailPatternDirectCandidateFilter, setDetailPatternDirectCandidateFilter] =
    useState('')
  const [detailPatternLiveActionKindFilter, setDetailPatternLiveActionKindFilter] =
    useState('')
  const [detailPatternInterventionStateFilter, setDetailPatternInterventionStateFilter] =
    useState('')
  const [anomalies, setAnomalies] = useState<DealReviewAnomalySummary | null>(
    null
  )
  const [symbolBehaviorPriors, setSymbolBehaviorPriors] = useState<
    DealReviewSymbolBehaviorPrior[]
  >([])
  const [symbolBehaviorLiveGuardEvents, setSymbolBehaviorLiveGuardEvents] =
    useState<DealReviewSymbolBehaviorLiveGuardEvent[]>([])
  const [symbolBehaviorLiveGuardSummary, setSymbolBehaviorLiveGuardSummary] =
    useState<DealReviewSymbolBehaviorLiveGuardEventSummary | null>(null)
  const [symbolBehaviorLiveGuardStatus, setSymbolBehaviorLiveGuardStatus] =
    useState<DealReviewSymbolBehaviorLiveGuardStatus | null>(null)
  const [symbolBehaviorPriorSummary, setSymbolBehaviorPriorSummary] = useState<
    DealReviewSymbolBehaviorPriorSummary | null
  >(null)
  const [learnedPatternLiveGuardEvents, setLearnedPatternLiveGuardEvents] =
    useState<DealReviewLearnedPatternLiveGuardEvent[]>([])
  const [learnedPatternLiveGuardSummary, setLearnedPatternLiveGuardSummary] =
    useState<DealReviewLearnedPatternLiveGuardEventSummary | null>(null)
  const [learnedPatternLiveGuardStatus, setLearnedPatternLiveGuardStatus] =
    useState<DealReviewLearnedPatternLiveGuardStatus | null>(null)
  const [learnedPatternSummary, setLearnedPatternSummary] =
    useState<DealReviewLearnedPatternSummary | null>(null)
  const [selectedSymbolPriorId, setSelectedSymbolPriorId] = useState<string | null>(
    null
  )
  const [symbolBehaviorClusterFilter, setSymbolBehaviorClusterFilter] =
    useState('')
  const [symbolBehaviorPriorTab, setSymbolBehaviorPriorTab] = useState<
    'all' | 'confirmed' | 'false_positive' | 'false_negative_risk' | 'drifting'
  >('all')
  const [versions, setVersions] = useState<DealReviewStrategyVersionDetail[]>(
    []
  )
  const [selectedVersionId, setSelectedVersionId] = useState<string | null>(null)
  const [similarVersions, setSimilarVersions] = useState<SemanticMemorySearchHit[]>([])
  const [similarVersionsLoading, setSimilarVersionsLoading] = useState(false)
  const [similarVersionsError, setSimilarVersionsError] = useState<string | null>(null)
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
  const previousTraderIdRef = useRef<string | undefined>(undefined)
  const currentTraderIdRef = useRef<string | undefined>(selectedTraderId)
  const detailRequestKeyRef = useRef(0)
  const similarCasesRequestKeyRef = useRef(0)
  const similarVersionsRequestKeyRef = useRef(0)
  const comparePeerRequestKeyRef = useRef(0)
  const traderChanged = previousTraderIdRef.current !== selectedTraderId

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
  const selectedCaseVisible = Boolean(
    selectedCaseId && items.some((item) => item.case.id === selectedCaseId)
  )
  const detailLearnedPatterns = detail?.learned_patterns || []
  const visibleDetailLearnedPatterns = useMemo(() => {
    const filtered = detailLearnedPatterns.filter((pattern) => {
      if (
        detailPatternDirectCandidateFilter === 'only' &&
        !learnedPatternHasDirectLiveActionCandidate(pattern)
      ) {
        return false
      }
      if (
        detailPatternLiveActionKindFilter &&
        learnedPatternStrongestLiveActionKind(pattern, false) !==
          detailPatternLiveActionKindFilter
      ) {
        return false
      }
      if (
        !learnedPatternMatchesInterventionFilter(
          pattern,
          detailPatternInterventionStateFilter
        )
      ) {
        return false
      }
      return true
    })

    return [...filtered].sort((left, right) => {
      const leftScore = learnedPatternReviewPriorityScore(left)
      const rightScore = learnedPatternReviewPriorityScore(right)
      if (leftScore !== rightScore) return rightScore - leftScore
      if ((left.match_score || 0) !== (right.match_score || 0)) {
        return (right.match_score || 0) - (left.match_score || 0)
      }
      if ((left.composite_score || 0) !== (right.composite_score || 0)) {
        return (right.composite_score || 0) - (left.composite_score || 0)
      }
      return (right.sample_count || 0) - (left.sample_count || 0)
    })
  }, [
    detailLearnedPatterns,
    detailPatternDirectCandidateFilter,
    detailPatternInterventionStateFilter,
    detailPatternLiveActionKindFilter,
  ])
  const comparePeerCaseVisible = Boolean(
    comparePeerCaseId && items.some((item) => item.case.id === comparePeerCaseId)
  )
  const selectedVersionVisible = Boolean(
    selectedVersionId &&
      versions.some((item) => item.version.id === selectedVersionId)
  )
  const selectedCompareVisible = Boolean(
    selectedCompareId &&
      challengerCompares.some((item) => item.compare.id === selectedCompareId)
  )
  const compareScanSelectionValid = Boolean(
    compareLeftScanId &&
      compareRightScanId &&
      scans.some((item) => item.scan.id === compareLeftScanId) &&
      scans.some((item) => item.scan.id === compareRightScanId)
  )

  const localizedSideOptions = useMemo(
    () =>
      sideOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All Sides', 'Alle Richtungen')
            : item.label,
      })),
    [language]
  )
  const localizedStatusOptions = useMemo(
    () =>
      statusOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All Statuses', 'Alle Status')
            : item.label,
      })),
    [language]
  )
  const localizedOutcomeOptions = useMemo(
    () =>
      outcomeOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All Outcomes', 'Alle Ergebnisse')
            : item.value === 'profit'
              ? pickText('Profit', 'Gewinn')
              : item.value === 'loss'
                ? pickText('Loss', 'Verlust')
                : item.value === 'flat'
                  ? pickText('Flat', 'Flat')
                  : item.value === 'open'
                    ? pickText('Open', 'Offen')
                    : item.label,
      })),
    [language]
  )
  const localizedRangeOptions = useMemo(
    () =>
      rangeOptions.map((item) => ({
        ...item,
        label:
          item.value === 'all'
            ? pickText('All time', 'Gesamter Zeitraum')
            : item.label.replace('Last ', pickText('Last ', 'Letzte ')),
      })),
    [language]
  )
  const localizedTrendRegimeOptions = useMemo(
    () =>
      trendRegimeOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All trend regimes', 'Alle Trend-Regime')
            : item.value === 'uptrend'
              ? pickText('Uptrend', 'Aufwaertstrend')
              : item.value === 'downtrend'
                ? pickText('Downtrend', 'Abwaertstrend')
                : item.value === 'chop'
                  ? pickText('Chop', 'Seitwaerts')
                  : item.value === 'mixed'
                    ? pickText('Mixed', 'Gemischt')
                    : item.label,
      })),
    [language]
  )
  const localizedVolatilityRegimeOptions = useMemo(
    () =>
      volatilityRegimeOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All vol regimes', 'Alle Volatilitaets-Regime')
            : item.value === 'high_vol'
              ? pickText('High vol', 'Hohe Volatilitaet')
              : item.value === 'low_vol'
                ? pickText('Low vol', 'Niedrige Volatilitaet')
                : item.label === 'Normal'
                  ? pickText('Normal', 'Normal')
                  : item.label,
      })),
    [language]
  )
  const localizedBtcStrengthOptions = useMemo(
    () =>
      btcStrengthOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All BTC-relative states', 'Alle BTC-relativen Zustaende')
            : item.value === 'outperform'
              ? pickText('Outperforming BTC', 'Staerker als BTC')
              : item.value === 'lagging'
                ? pickText('Lagging BTC', 'Schwaecher als BTC')
                : item.value === 'neutral'
                  ? pickText('Neutral vs BTC', 'Neutral gegenueber BTC')
                  : item.label,
      })),
    [language]
  )
  const localizedFundingRegimeOptions = useMemo(
    () =>
      fundingRegimeOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All funding states', 'Alle Funding-Zustaende')
            : item.value === 'extreme_longs'
              ? pickText('Extreme longs', 'Extreme Longs')
              : item.value === 'longs_pay'
                ? pickText('Longs pay', 'Longs zahlen')
                : item.value === 'neutral'
                  ? pickText('Neutral funding', 'Neutrales Funding')
                  : item.value === 'shorts_pay'
                    ? pickText('Shorts pay', 'Shorts zahlen')
                    : item.value === 'extreme_shorts'
                      ? pickText('Extreme shorts', 'Extreme Shorts')
                      : item.label,
      })),
    [language]
  )
  const localizedOiRegimeOptions = useMemo(
    () =>
      oiRegimeOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All OI states', 'Alle OI-Zustaende')
            : item.value === 'oi_surge'
              ? pickText('OI surge', 'OI-Schub')
              : item.value === 'oi_rising'
                ? pickText('OI rising', 'Steigende OI')
                : item.value === 'oi_flat'
                  ? pickText('OI flat', 'Flache OI')
                  : item.value === 'oi_falling'
                    ? pickText('OI falling', 'Fallende OI')
                    : item.value === 'oi_flush'
                      ? pickText('OI flush', 'OI-Spuelung')
                      : item.label,
      })),
    [language]
  )
  const localizedSessionOptions = useMemo(
    () =>
      sessionOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All sessions', 'Alle Sessions')
            : item.value === 'off_hours'
              ? pickText('Off hours', 'Randzeiten')
              : item.label,
      })),
    [language]
  )
  const localizedWeekdayOptions = useMemo(
    () =>
      weekdayOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All weekdays', 'Alle Wochentage')
            : item.value === 'monday'
              ? pickText('Monday', 'Montag')
              : item.value === 'tuesday'
                ? pickText('Tuesday', 'Dienstag')
                : item.value === 'wednesday'
                  ? pickText('Wednesday', 'Mittwoch')
                  : item.value === 'thursday'
                    ? pickText('Thursday', 'Donnerstag')
                    : item.value === 'friday'
                      ? pickText('Friday', 'Freitag')
                      : item.value === 'saturday'
                        ? pickText('Saturday', 'Samstag')
                        : item.value === 'sunday'
                          ? pickText('Sunday', 'Sonntag')
                          : item.label,
      })),
    [language]
  )
  const localizedVenueTierOptions = useMemo(
    () =>
      venueTierOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All venue states', 'Alle Boersen-Zustaende')
            : item.value === 'tradable'
              ? pickText('Tradable', 'Handelbar')
              : item.value === 'thin_book'
                ? pickText('Thin book', 'Duennes Orderbuch')
                : item.value === 'restricted'
                  ? pickText('Restricted', 'Eingeschraenkt')
                  : item.value === 'unsupported'
                    ? pickText('Unsupported', 'Nicht unterstuetzt')
                    : item.value === 'unknown'
                      ? pickText('Unknown', 'Unbekannt')
                      : item.label,
      })),
    [language]
  )
  const localizedLiquidityTierOptions = useMemo(
    () =>
      liquidityTierOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All liquidity tiers', 'Alle Liquiditaetsstufen')
            : item.value === 'high'
              ? pickText('High liquidity', 'Hohe Liquiditaet')
              : item.value === 'medium'
                ? pickText('Medium liquidity', 'Mittlere Liquiditaet')
                : item.value === 'low'
                  ? pickText('Low liquidity', 'Niedrige Liquiditaet')
                  : item.label,
      })),
    [language]
  )
  const localizedExecutionBucketOptions = useMemo(
    () =>
      executionBucketOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All execution buckets', 'Alle Ausfuehrungs-Buckets')
            : item.value === 'tight'
              ? pickText('Tight', 'Eng')
              : item.value === 'normal'
                ? pickText('Normal', 'Normal')
                : item.value === 'wide'
                  ? pickText('Wide', 'Weit')
                  : item.value === 'extreme'
                    ? pickText('Extreme', 'Extrem')
                    : item.label,
      })),
    [language]
  )
  const localizedExitReasonQualityOptions = useMemo(
    () =>
      exitReasonQualityOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All exit evidence', 'Alle Exit-Belege')
            : item.value === 'explicit'
              ? pickText('Explicit', 'Explizit')
              : item.value === 'high_confidence_inferred'
                ? pickText('High-confidence inferred', 'Mit hoher Konfidenz abgeleitet')
                : item.value === 'low_confidence_inferred'
                  ? pickText('Low-confidence inferred', 'Mit niedriger Konfidenz abgeleitet')
                  : item.label,
      })),
    [language]
  )
  const localizedReviewQueueOptions = useMemo(
    () =>
      reviewQueueOptions.map((item) => ({
        ...item,
        label:
          item.value === ''
            ? pickText('All filtered deals', 'Alle gefilterten Deals')
            : item.value === 'unlabeled_losses'
              ? pickText('Review queue: unlabeled losses', 'Review-Warteschlange: unbeschriftete Verluste')
              : item.value === 'biggest_giveback'
                ? pickText('Review queue: biggest give-back exits', 'Review-Warteschlange: groesste Gewinnabgaben')
                : item.value === 'regime_mismatch'
                  ? pickText('Review queue: regime mismatch candidates', 'Review-Warteschlange: Regime-Fehlanpassungs-Kandidaten')
                  : item.label,
      })),
    [language]
  )
  const localizedChallengerModeOptions = useMemo(
    () =>
      challengerModeOptions.map((item) => ({
        ...item,
        label:
          item.value === 'shared_live'
            ? pickText('Shared live wallet', 'Geteilte Live-Wallet')
            : item.value === 'isolated_live'
              ? pickText('Isolated live wallet', 'Isolierte Live-Wallet')
              : item.value === 'paper'
                ? pickText('Paper / testnet', 'Paper / Testnet')
                : item.label,
      })),
    [language]
  )

  const openPatternLabWithFilters = (params?: Record<string, string>) => {
    const url = new URL(window.location.href)
    url.pathname = '/pattern-lab'
    url.search = ''
    Object.entries(params || {}).forEach(([key, value]) => {
      const normalized = value.trim()
      if (normalized) {
        url.searchParams.set(key, normalized)
      }
    })
    window.history.pushState({}, '', url.toString())
    window.dispatchEvent(new PopStateEvent('popstate'))
  }
  const symbolBehaviorPriorTabCounts = useMemo(() => {
    const counts = {
      all: symbolBehaviorPriors.length,
      confirmed: 0,
      false_positive: 0,
      false_negative_risk: 0,
      drifting: 0,
    }
    symbolBehaviorPriors.forEach((prior) => {
      switch (prior.validation_label) {
        case 'confirmed':
          counts.confirmed += 1
          break
        case 'false_positive':
          counts.false_positive += 1
          break
        case 'false_negative_risk':
          counts.false_negative_risk += 1
          break
        case 'drifting':
          counts.drifting += 1
          break
        default:
          break
      }
    })
    return counts
  }, [symbolBehaviorPriors])
  const symbolBehaviorPriorTabs = useMemo(
    () => [
      { value: 'all', label: 'All priors', count: symbolBehaviorPriorTabCounts.all },
      {
        value: 'confirmed',
        label: 'Confirmed',
        count: symbolBehaviorPriorTabCounts.confirmed,
      },
      {
        value: 'false_positive',
        label: 'False positives',
        count: symbolBehaviorPriorTabCounts.false_positive,
      },
      {
        value: 'false_negative_risk',
        label: 'Reverse-edge risk',
        count: symbolBehaviorPriorTabCounts.false_negative_risk,
      },
      {
        value: 'drifting',
        label: 'Drifting',
        count: symbolBehaviorPriorTabCounts.drifting,
      },
    ] as const,
    [symbolBehaviorPriorTabCounts]
  )
  const visibleSymbolBehaviorPriors = useMemo(() => {
    if (symbolBehaviorPriors.length === 0) {
      return []
    }
    const filtered =
      symbolBehaviorPriorTab === 'all'
        ? symbolBehaviorPriors
        : symbolBehaviorPriors.filter(
            (item) => item.validation_label === symbolBehaviorPriorTab
          )
    const ordered = [...filtered]
    if (selectedSymbolPriorId) {
      const selectedIndex = ordered.findIndex((item) => item.id === selectedSymbolPriorId)
      if (selectedIndex > 0) {
        const [selected] = ordered.splice(selectedIndex, 1)
        if (selected) {
          ordered.unshift(selected)
        }
      }
    }
    return ordered.slice(0, 6)
  }, [symbolBehaviorPriors, selectedSymbolPriorId, symbolBehaviorPriorTab])

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
      exit_reason_quality: exitReasonQuality || undefined,
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
    if (customFromTime) {
      filter.from_time = customFromTime
    } else {
      const rangeStart = getDateRangeStart(dateRange)
      if (rangeStart !== undefined) {
        filter.from_time = rangeStart
      }
    }
    if (customToTime) {
      filter.to_time = customToTime
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
    exit_reason_quality: exitReasonQuality,
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
    setCustomFromTime(null)
    setCustomToTime(null)
    setSymbol(normalizePresetText(filters?.symbol).toUpperCase())
    setSide(normalizePresetText(filters?.side))
    setStatus(hasStatus ? normalizePresetText(filters?.status) : 'CLOSED')
    setOutcome(normalizePresetText(filters?.outcome))
    setOpenSelectionBucket(normalizePresetText(filters?.open_selection_bucket))
    setCloseReason(normalizePresetText(filters?.close_reason))
    setExitReasonQuality(normalizePresetText(filters?.exit_reason_quality))
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
    setCustomFromTime(null)
    setCustomToTime(null)
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
    if (typeof patch.exit_reason_quality === 'string') {
      setExitReasonQuality(patch.exit_reason_quality)
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

  const loadCases = async (
    overrides?: Record<string, string | number | undefined>
  ) => {
    const traderId = selectedTraderId
    if (!traderId) return
    setLoading(true)
    setError(null)
    try {
      const result = await api.getDealReviewCases(
        traderId,
        buildFilterPayload(overrides)
      )
      if (currentTraderIdRef.current !== traderId) return
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
      if (currentTraderIdRef.current !== traderId) return
      setError(
        err instanceof Error ? err.message : 'Failed to fetch deal review cases'
      )
    } finally {
      if (currentTraderIdRef.current !== traderId) return
      setLoading(false)
    }
  }

  const loadDetail = async () => {
    if (!selectedTraderId || !selectedCaseId || !selectedCaseVisible) {
      setDetail(null)
      return
    }
    const requestKey = detailRequestKeyRef.current + 1
    detailRequestKeyRef.current = requestKey
    setDetailLoading(true)
    try {
      const result = await api.getDealReviewCaseDetail(
        selectedTraderId,
        selectedCaseId
      )
      if (detailRequestKeyRef.current !== requestKey) return
      setDetail(result)
    } catch (err) {
      if (detailRequestKeyRef.current !== requestKey) return
      setDetail(null)
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch deal detail'
      )
    } finally {
      if (detailRequestKeyRef.current !== requestKey) return
      setDetailLoading(false)
    }
  }

  const patternControlKey = (pattern: DealReviewLearnedPattern): string =>
    pattern.stable_key || pattern.id

  const canManagePattern = (pattern: DealReviewLearnedPattern): boolean =>
    pattern.base_recommended_use === 'monitoring_rule' ||
    pattern.recommended_use === 'monitoring_rule' ||
    Boolean(pattern.manual_control)

  const applyPatternControl = async (
    pattern: DealReviewLearnedPattern,
    action: 'acknowledge_live' | 'suppress' | 'retire' | 'rearm',
    forcedNote?: string
  ) => {
    if (!selectedTraderId) return
    const key = patternControlKey(pattern)
    const note = (forcedNote || controlNotes[key] || '').trim()
    if (!note) {
      notify.error('A short analyst note is required.')
      return
    }
    setControlBusyKey(`${key}:${action}`)
    try {
      await api.applyDealReviewLearnedPatternControl(selectedTraderId, {
        pattern_id: pattern.id || undefined,
        stable_key: pattern.stable_key || undefined,
        action,
        note,
      })
      notify.success(`${formatLearnedPatternManualControlAction(action)} saved`)
      setControlNotes((current) => ({ ...current, [key]: '' }))
      await Promise.all([loadDetail(), mutate(`status-${selectedTraderId}`)])
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to apply learned-pattern control'
      )
    } finally {
      setControlBusyKey('')
    }
  }

  const applySuggestedPatternAction = async (pattern: DealReviewLearnedPattern) => {
    const action = pattern.action_hint?.recommended_action
    const autoNote = (
      pattern.action_hint?.auto_note ||
      pattern.live_action_hint?.summary ||
      ''
    ).trim()
    if (
      action !== 'acknowledge_live' &&
      action !== 'suppress' &&
      action !== 'retire' &&
      action !== 'rearm'
    ) {
      notify.error('No suggested action is available for this pattern.')
      return
    }
    await applyPatternControl(pattern, action, autoNote)
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
    notify.success(
      pickText(
        'Filtered deal dataset exported as JSON',
        'Gefilterter Deal-Datensatz als JSON exportiert'
      )
    )
  }

  const exportFilteredDealsCSV = () => {
    if (!selectedTraderId || displayedItems.length === 0) return
    downloadTextFile(
      `deal-review-${selectedTraderId}-${Date.now()}.csv`,
      buildDealReviewCasesCSV(displayedItems),
      'text/csv;charset=utf-8'
    )
    notify.success(
      pickText(
        'Filtered deal dataset exported as CSV',
        'Gefilterter Deal-Datensatz als CSV exportiert'
      )
    )
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
    notify.success(
      pickText(
        'Saved scan outputs exported as JSON',
        'Gespeicherte Scan-Ausgaben als JSON exportiert'
      )
    )
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
    notify.success(
      pickText('Scan compare exported as JSON', 'Scan-Vergleich als JSON exportiert')
    )
  }

  const loadScanHistory = async () => {
    const traderId = selectedTraderId
    if (!traderId) return
    try {
      const result = await api.getDealReviewAIScans(traderId, 8)
      if (currentTraderIdRef.current !== traderId) return
      setScans(result)
    } catch (err) {
      if (currentTraderIdRef.current !== traderId) return
      notify.error(
        err instanceof Error
          ? err.message
          : pickText('Failed to fetch AI scans', 'KI-Scans konnten nicht geladen werden')
      )
    }
  }

  const loadAnomalies = async (
    overrides?: Record<string, string | number | undefined>
  ) => {
    const traderId = selectedTraderId
    if (!traderId) return
    try {
      const result = await api.getDealReviewAnomalies(
        traderId,
        buildFilterPayload(overrides)
      )
      if (currentTraderIdRef.current !== traderId) return
      setAnomalies(result)
    } catch (err) {
      if (currentTraderIdRef.current !== traderId) return
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch anomalies'
      )
    }
  }

  const loadSymbolBehaviorPriors = async () => {
    const traderId = selectedTraderId
    if (!traderId) return
    try {
      const result = await api.getDealReviewSymbolBehaviorPriors(
        traderId,
        {
          symbol: symbol || undefined,
          side: side || undefined,
          signal_cluster: symbolBehaviorClusterFilter || undefined,
          limit: 100,
        }
      )
      if (currentTraderIdRef.current !== traderId) return
      setSymbolBehaviorPriors(result.items || [])
      setSymbolBehaviorPriorSummary(result.summary || null)
    } catch (err) {
      if (currentTraderIdRef.current !== traderId) return
      setSymbolBehaviorPriors([])
      setSymbolBehaviorPriorSummary(null)
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to fetch symbol behavior priors'
      )
    }
  }

  const loadSymbolBehaviorLiveGuardEvents = async () => {
    const traderId = selectedTraderId
    if (!traderId) {
      setSymbolBehaviorLiveGuardEvents([])
      setSymbolBehaviorLiveGuardSummary(null)
      setSymbolBehaviorLiveGuardStatus(null)
      return
    }
    try {
      const result = await api.getDealReviewSymbolBehaviorLiveGuardEvents(
        traderId,
        {
          symbol: symbol || undefined,
          side: side || undefined,
          limit: 12,
        }
      )
      if (currentTraderIdRef.current !== traderId) return
      setSymbolBehaviorLiveGuardEvents(result.items || [])
      setSymbolBehaviorLiveGuardSummary(result.summary || null)
      setSymbolBehaviorLiveGuardStatus(result.guard || null)
    } catch (err) {
      if (currentTraderIdRef.current !== traderId) return
      setSymbolBehaviorLiveGuardEvents([])
      setSymbolBehaviorLiveGuardSummary(null)
      setSymbolBehaviorLiveGuardStatus(null)
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to fetch symbol-prior live guard events'
      )
    }
  }

  const loadLearnedPatterns = async () => {
    const traderId = selectedTraderId
    if (!traderId) {
      setLearnedPatternLiveGuardEvents([])
      setLearnedPatternLiveGuardSummary(null)
      setLearnedPatternLiveGuardStatus(null)
      setLearnedPatternSummary(null)
      return
    }
    try {
      const [result, guardResult] = await Promise.all([
        api.getDealReviewLearnedPatterns(traderId, {
          symbol: symbol || undefined,
          side: side || undefined,
          limit: 60,
        }),
        api.getDealReviewLearnedPatternLiveGuardEvents(traderId, {
          symbol: symbol || undefined,
          side: side || undefined,
          limit: 12,
        }),
      ])
      if (currentTraderIdRef.current !== traderId) return
      setLearnedPatternSummary(result.summary || null)
      setLearnedPatternLiveGuardEvents(guardResult.items || [])
      setLearnedPatternLiveGuardSummary(guardResult.summary || null)
      setLearnedPatternLiveGuardStatus(guardResult.guard || null)
    } catch (err) {
      if (currentTraderIdRef.current !== traderId) return
      setLearnedPatternLiveGuardEvents([])
      setLearnedPatternLiveGuardSummary(null)
      setLearnedPatternLiveGuardStatus(null)
      setLearnedPatternSummary(null)
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch learned patterns'
      )
    }
  }

  const loadFilterPresets = async () => {
    const traderId = selectedTraderId
    if (!traderId) return
    try {
      const result = await api.getDealReviewFilterPresets(traderId)
      if (currentTraderIdRef.current !== traderId) return
      setFilterPresets(result)
    } catch (err) {
      if (currentTraderIdRef.current !== traderId) return
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch review presets'
      )
    }
  }

  const loadVersions = async () => {
    const traderId = selectedTraderId
    if (!traderId) return
    try {
      const result = await api.getDealReviewStrategyVersions(traderId, 20)
      if (currentTraderIdRef.current !== traderId) return
      setVersions(result)
    } catch (err) {
      if (currentTraderIdRef.current !== traderId) return
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
    const traderId = selectedTraderId
    if (!traderId) return
    try {
      const result = await api.getDealReviewChallengerCompares(traderId, 8)
      if (currentTraderIdRef.current !== traderId) return
      setChallengerCompares(result)
    } catch (err) {
      if (currentTraderIdRef.current !== traderId) return
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to fetch challenger history'
      )
    }
  }

  const loadChallengerCompareDetail = async () => {
    if (
      traderChanged ||
      !selectedTraderId ||
      !selectedCompareId ||
      !selectedCompareVisible
    ) {
      setSelectedCompareDetail(null)
      setCompareDetailLoading(false)
      return
    }
    const traderId = selectedTraderId
    setCompareDetailLoading(true)
    try {
      const result = await api.getDealReviewChallengerCompare(
        traderId,
        selectedCompareId
      )
      if (currentTraderIdRef.current !== traderId) return
      setSelectedCompareDetail(result)
    } catch (err) {
      if (currentTraderIdRef.current !== traderId) return
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch challenger detail'
      )
    } finally {
      if (currentTraderIdRef.current !== traderId) return
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
    previousTraderIdRef.current = selectedTraderId
  }, [selectedTraderId])

  useEffect(() => {
    currentTraderIdRef.current = selectedTraderId
  }, [selectedTraderId])

  useEffect(() => {
    if (!traderChanged) {
      return
    }
    setItems([])
    setSummary(null)
    setSelectedCaseId(null)
    setDetail(null)
    setSimilarCases([])
    setSimilarCasesError(null)
    setSimilarVersions([])
    setSimilarVersionsError(null)
    setComparePeerCaseId('')
    setComparePeerDetail(null)
    setScans([])
    setCompareLeftScanId('')
    setCompareRightScanId('')
    setCompareResult(null)
    setVersions([])
    setSelectedVersionId(null)
    setChallengerCompares([])
    setSelectedCompareId(null)
    setSelectedCompareDetail(null)
    setFilterPresets([])
    setSelectedPresetId('')
    setPresetName('')
    setAnomalies(null)
    setSymbolBehaviorPriors([])
    setSymbolBehaviorPriorSummary(null)
    setSymbolBehaviorLiveGuardEvents([])
    setSymbolBehaviorLiveGuardSummary(null)
    setSymbolBehaviorLiveGuardStatus(null)
    setLearnedPatternSummary(null)
    setLearnedPatternLiveGuardEvents([])
    setLearnedPatternLiveGuardSummary(null)
    setLearnedPatternLiveGuardStatus(null)
    setSelectedSymbolPriorId(null)
    setReviewQueueMode('')
    setDetailPatternDirectCandidateFilter('')
    setDetailPatternLiveActionKindFilter('')
    setDetailPatternInterventionStateFilter('')
    setError(null)
  }, [selectedTraderId, traderChanged])

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
    exitReasonQuality,
    dateRange,
    customFromTime,
    customToTime,
    minPnl,
    maxPnl,
  ])

  useEffect(() => {
    if (traderChanged) {
      return
    }
    void loadDetail()
  }, [selectedTraderId, selectedCaseId, selectedCaseVisible, traderChanged])

  useEffect(() => {
    if (!selectedTraderId || !selectedCaseId || !selectedCaseVisible || traderChanged) {
      setSimilarCases([])
      setSimilarCasesError(null)
      setSimilarCasesLoading(false)
      return
    }
    const requestKey = similarCasesRequestKeyRef.current + 1
    similarCasesRequestKeyRef.current = requestKey
    setSimilarCasesLoading(true)
    setSimilarCasesError(null)
    void (async () => {
      try {
        const result = await api.getDealReviewCaseSimilar(
          selectedTraderId,
          selectedCaseId,
          { limit: 6 }
        )
        if (similarCasesRequestKeyRef.current !== requestKey) return
        setSimilarCases(result.items || [])
      } catch (err) {
        if (similarCasesRequestKeyRef.current !== requestKey) return
        setSimilarCases([])
        setSimilarCasesError(
          err instanceof Error ? err.message : 'Failed to fetch similar historical deals'
        )
      } finally {
        if (similarCasesRequestKeyRef.current !== requestKey) return
        setSimilarCasesLoading(false)
      }
    })()
  }, [selectedTraderId, selectedCaseId, selectedCaseVisible, traderChanged])

  useEffect(() => {
    if (!selectedCaseId || !comparePeerCaseId) return
    if (selectedCaseId === comparePeerCaseId) {
      setComparePeerCaseId('')
      setComparePeerDetail(null)
    }
  }, [selectedCaseId, comparePeerCaseId])

  useEffect(() => {
    if (!selectedTraderId || !comparePeerCaseId || !comparePeerCaseVisible) {
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
    const requestKey = comparePeerRequestKeyRef.current + 1
    comparePeerRequestKeyRef.current = requestKey
    setComparePeerLoading(true)
    void (async () => {
      try {
        if (detail?.case.id === comparePeerCaseId) {
          if (active && comparePeerRequestKeyRef.current === requestKey) {
            setComparePeerDetail(detail)
          }
          return
        }
        const result = await api.getDealReviewCaseDetail(
          selectedTraderId,
          comparePeerCaseId
        )
        if (active && comparePeerRequestKeyRef.current === requestKey) {
          setComparePeerDetail(result)
        }
      } catch (err) {
        if (active && comparePeerRequestKeyRef.current === requestKey) {
          setComparePeerDetail(null)
          notify.error(
            err instanceof Error
              ? err.message
              : 'Failed to load compare deal detail'
          )
        }
      } finally {
        if (active && comparePeerRequestKeyRef.current === requestKey) {
          setComparePeerLoading(false)
        }
      }
    })()

    return () => {
      active = false
    }
  }, [
    selectedTraderId,
    selectedCaseId,
    comparePeerCaseId,
    comparePeerCaseVisible,
    detail,
  ])

  useEffect(() => {
    void loadScanHistory()
  }, [selectedTraderId])

  useEffect(() => {
    setSymbolBehaviorClusterFilter('')
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
    exitReasonQuality,
    dateRange,
    customFromTime,
    customToTime,
    minPnl,
    maxPnl,
  ])

  useEffect(() => {
    void loadSymbolBehaviorPriors()
  }, [selectedTraderId, symbol, side, symbolBehaviorClusterFilter])

  useEffect(() => {
    void loadSymbolBehaviorLiveGuardEvents()
  }, [selectedTraderId, symbol, side])

  useEffect(() => {
    void loadLearnedPatterns()
  }, [selectedTraderId, symbol, side])

  useEffect(() => {
    if (!selectedSymbolPriorId) {
      return
    }
    const selectedPrior = symbolBehaviorPriors.find(
      (item) => item.id === selectedSymbolPriorId
    )
    switch (selectedPrior?.validation_label) {
      case 'confirmed':
      case 'false_positive':
      case 'false_negative_risk':
      case 'drifting':
        setSymbolBehaviorPriorTab(selectedPrior.validation_label)
        break
      default:
        break
    }
  }, [selectedSymbolPriorId, symbolBehaviorPriors])

  useEffect(() => {
    if (!selectedTraderId || window.location.pathname !== '/deal-review') {
      return
    }
    const params = new URLSearchParams(window.location.search)
    const caseId = params.get('case_id')?.trim() || ''
    const fromTime = Number(params.get('optimizer_from_time') || '')
    const toTime = Number(params.get('optimizer_to_time') || '')
    const versionId =
      params.get('strategy_version_id')?.trim() ||
      params.get('optimizer_version_id')?.trim() ||
      ''
    const symbolParam = params.get('symbol')?.trim().toUpperCase() || ''
    const sideParam = params.get('side')?.trim().toUpperCase() || ''
    const statusParam = params.get('status')?.trim().toUpperCase() || ''
    const selectionBucketParam = params.get('open_selection_bucket')?.trim() || ''
    const trendParam = params.get('open_trend_regime')?.trim() || ''
    const volatilityParam = params.get('open_volatility_regime')?.trim() || ''
    const btcStrengthParam = params.get('open_btc_strength_regime')?.trim() || ''
    const fundingParam = params.get('open_funding_regime')?.trim() || ''
    const oiParam = params.get('open_oi_regime')?.trim() || ''
    const sessionParam = params.get('open_session_bucket')?.trim() || ''
    const weekdayParam = params.get('open_weekday_bucket')?.trim() || ''
    const venueTierParam = params.get('open_venue_tier')?.trim() || ''
    const liquidityTierParam = params.get('open_liquidity_tier')?.trim() || ''
    const spreadParam = params.get('open_spread_bucket')?.trim() || ''
    const slippageParam = params.get('open_slippage_bucket')?.trim() || ''
    const symbolPriorId = params.get('symbol_prior_id')?.trim() || ''

    if (caseId) {
      setSelectedCaseId(caseId)
    }
    if (Number.isFinite(fromTime) && fromTime > 0) {
      setSelectedPresetId('')
      setCustomFromTime(fromTime)
    }
    if (Number.isFinite(toTime) && toTime > 0) {
      setSelectedPresetId('')
      setCustomToTime(toTime)
    }
    if (versionId) {
      setSelectedVersionId(versionId)
    }
    if (symbolParam) {
      setSelectedPresetId('')
      setSymbol(symbolParam)
    }
    if (sideParam) {
      setSelectedPresetId('')
      setSide(sideParam)
    }
    if (statusParam) {
      setSelectedPresetId('')
      setStatus(statusParam)
    }
    if (selectionBucketParam) {
      setSelectedPresetId('')
      setOpenSelectionBucket(selectionBucketParam)
    }
    if (trendParam) {
      setSelectedPresetId('')
      setOpenTrendRegime(trendParam)
    }
    if (volatilityParam) {
      setSelectedPresetId('')
      setOpenVolatilityRegime(volatilityParam)
    }
    if (btcStrengthParam) {
      setSelectedPresetId('')
      setOpenBTCStrengthRegime(btcStrengthParam)
    }
    if (fundingParam) {
      setSelectedPresetId('')
      setOpenFundingRegime(fundingParam)
    }
    if (oiParam) {
      setSelectedPresetId('')
      setOpenOIRegime(oiParam)
    }
    if (sessionParam) {
      setSelectedPresetId('')
      setOpenSessionBucket(sessionParam)
    }
    if (weekdayParam) {
      setSelectedPresetId('')
      setOpenWeekdayBucket(weekdayParam)
    }
    if (venueTierParam) {
      setSelectedPresetId('')
      setOpenVenueTier(venueTierParam)
    }
    if (liquidityTierParam) {
      setSelectedPresetId('')
      setOpenLiquidityTier(liquidityTierParam)
    }
    if (spreadParam) {
      setSelectedPresetId('')
      setOpenSpreadBucket(spreadParam)
    }
    if (slippageParam) {
      setSelectedPresetId('')
      setOpenSlippageBucket(slippageParam)
    }
    setSelectedSymbolPriorId(symbolPriorId || null)
  }, [selectedTraderId])

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
  }, [selectedTraderId, selectedCompareId, selectedCompareVisible, traderChanged])

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
    if (
      !selectedTraderId ||
      !selectedVersionId ||
      !selectedVersionVisible ||
      traderChanged
    ) {
      setSimilarVersions([])
      setSimilarVersionsError(null)
      setSimilarVersionsLoading(false)
      return
    }
    const requestKey = similarVersionsRequestKeyRef.current + 1
    similarVersionsRequestKeyRef.current = requestKey
    setSimilarVersionsLoading(true)
    setSimilarVersionsError(null)
    void (async () => {
      try {
        const result = await api.getDealReviewStrategyVersionSimilar(
          selectedTraderId,
          selectedVersionId,
          { limit: 5 }
        )
        if (similarVersionsRequestKeyRef.current !== requestKey) return
        setSimilarVersions(result.items || [])
      } catch (err) {
        if (similarVersionsRequestKeyRef.current !== requestKey) return
        setSimilarVersions([])
        setSimilarVersionsError(
          err instanceof Error ? err.message : 'Failed to fetch similar strategy versions'
        )
      } finally {
        if (similarVersionsRequestKeyRef.current !== requestKey) return
        setSimilarVersionsLoading(false)
      }
    })()
  }, [selectedTraderId, selectedVersionId, selectedVersionVisible, traderChanged])

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
      traderChanged ||
      !selectedTraderId ||
      !compareScanSelectionValid ||
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
          err instanceof Error
            ? err.message
            : pickText('Failed to compare AI scans', 'KI-Scans konnten nicht verglichen werden')
        )
        setCompareResult(null)
      })
      .finally(() => setCompareLoading(false))
  }, [
    selectedTraderId,
    compareLeftScanId,
    compareRightScanId,
    compareScanSelectionValid,
    traderChanged,
  ])

  const runAIScan = async (
    overrides?: Record<string, string | number | undefined>,
    successMessage: string = pickText('AI scan completed', 'KI-Scan abgeschlossen')
  ) => {
    if (!selectedTraderId) return
    if (items.length === 0) {
      notify.error(
        pickText(
          'No deals matched the selected filters',
          'Keine Deals passten zu den ausgewaehlten Filtern'
        )
      )
      return
    }
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
      notify.error(
        err instanceof Error ? err.message : pickText('AI scan failed', 'KI-Scan fehlgeschlagen')
      )
    } finally {
      setRunningScan(false)
    }
  }

  const saveFilterPreset = async () => {
    if (!selectedTraderId) return
    const trimmedName = presetName.trim()
    if (!trimmedName) {
      notify.error(
        pickText('Preset name is required', 'Ein Preset-Name ist erforderlich')
      )
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
          ? pickText('Review preset updated', 'Review-Preset aktualisiert')
          : pickText('Review preset saved', 'Review-Preset gespeichert')
      )
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : pickText('Failed to save review preset', 'Review-Preset konnte nicht gespeichert werden')
      )
    } finally {
      setSavingPreset(false)
    }
  }

  const deleteFilterPreset = async () => {
    if (!selectedTraderId || !selectedPreset) return
    const confirmed = await confirmToast(
      pickText(
        `Delete the preset "${selectedPreset.preset.name}"?`,
        `Preset "${selectedPreset.preset.name}" loeschen?`
      ),
      {
        title: pickText('Delete Preset', 'Preset loeschen'),
        okText: pickText('Delete', 'Loeschen'),
        cancelText: pickText('Cancel', 'Abbrechen'),
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
      notify.success(pickText('Review preset deleted', 'Review-Preset geloescht'))
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : pickText('Failed to delete review preset', 'Review-Preset konnte nicht geloescht werden')
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
          ? pickText('AI scan validation passed', 'KI-Scan-Validierung bestanden')
          : pickText('AI scan validation updated', 'KI-Scan-Validierung aktualisiert')
      )
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : pickText('Validation failed', 'Validierung fehlgeschlagen')
      )
    } finally {
      setValidatingScanId(null)
    }
  }

  const applyScan = async (scan: DealReviewAIScanDetail) => {
    if (!selectedTraderId) return
    const confirmed = await confirmToast(
      pickText(
        'Apply the strategy patch from this AI scan and reload the trader configuration?',
        'Diesen Strategie-Patch aus dem KI-Scan uebernehmen und die Trader-Konfiguration neu laden?'
      ),
      {
        title: pickText('Apply AI Patch', 'KI-Patch uebernehmen'),
        okText: pickText('Apply', 'Uebernehmen'),
        cancelText: pickText('Cancel', 'Abbrechen'),
      }
    )
    if (!confirmed) return

    setApplyingScanId(scan.scan.id)
    try {
      await api.applyDealReviewAIScan(selectedTraderId, scan.scan.id)
      notify.success(pickText('Strategy patch applied', 'Strategie-Patch uebernommen'))
      await loadScanHistory()
      await loadVersions()
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : pickText('Failed to apply AI patch', 'KI-Patch konnte nicht uebernommen werden')
      )
    } finally {
      setApplyingScanId(null)
    }
  }

  const launchChallenger = async (scan: DealReviewAIScanDetail) => {
    if (!selectedTraderId || !launchExchangeId) return
    const confirmed = await confirmToast(
      pickText(
        `Launch a challenger trader in ${launchMode} mode for ${launchWindowHours}h using the selected wallet?`,
        `Einen Challenger-Trader im Modus ${launchMode} fuer ${launchWindowHours}h mit der ausgewaehlten Wallet starten?`
      ),
      {
        title: pickText('Launch Challenger', 'Challenger starten'),
        okText: pickText('Launch', 'Starten'),
        cancelText: pickText('Cancel', 'Abbrechen'),
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
      notify.success(
        pickText('Challenger compare started', 'Challenger-Vergleich gestartet')
      )
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

  const openVersionDetail = (
    versionId: string,
    detailOverride?: DealReviewStrategyVersionDetail | null
  ) => {
    if (detailOverride) {
      setVersions((current) => {
        const withoutCurrent = current.filter(
          (item) => item.version.id !== detailOverride.version.id
        )
        return [detailOverride, ...withoutCurrent]
      })
    }
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
            <div className="w-full lg:w-80 space-y-2">
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
              <button
                onClick={() =>
                  openPatternLabWithFilters({
                    symbol: symbol.trim().toUpperCase(),
                    side: side.trim().toUpperCase(),
                  })
                }
                className="w-full h-10 rounded-lg border border-white/10 bg-black/20 text-sm font-medium"
              >
                Open Pattern Lab
              </button>
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
                      {
                        value: '',
                        label: pickText(
                          'Load saved cohort preset',
                          'Gespeichertes Kohorten-Preset laden'
                        ),
                      },
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
                  placeholder={pickText('Preset name', 'Preset-Name')}
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <button
                  onClick={saveFilterPreset}
                  disabled={!selectedTraderId || savingPreset}
                  className="h-11 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 text-nofx-gold font-semibold disabled:opacity-50"
                >
                  {savingPreset
                    ? pickText('Saving…', 'Speichere…')
                    : selectedPreset &&
                        selectedPreset.preset.name.trim().toLowerCase() ===
                          presetName.trim().toLowerCase()
                      ? pickText('Update preset', 'Preset aktualisieren')
                      : pickText('Save preset', 'Preset speichern')}
                </button>
                <button
                  onClick={deleteFilterPreset}
                  disabled={!selectedPreset || deletingPresetId === selectedPreset?.preset.id}
                  className="h-11 rounded-lg border border-white/10 bg-black/20 font-semibold disabled:opacity-50"
                >
                  {deletingPresetId === selectedPreset?.preset.id
                    ? pickText('Deleting…', 'Loesche…')
                    : pickText('Delete preset', 'Preset loeschen')}
                </button>
              </div>

              <div className="flex flex-wrap gap-2 mt-3">
                {[
                  {
                    label: pickText('Trend + high vol', 'Trend + hohe Volatilitaet'),
                    patch: {
                      open_trend_regime: 'uptrend',
                      open_volatility_regime: 'high_vol',
                    },
                  },
                  {
                    label: pickText('Low-vol chop', 'Low-Vol Seitwaerts'),
                    patch: {
                      open_trend_regime: 'chop',
                      open_volatility_regime: 'low_vol',
                    },
                  },
                  {
                    label: pickText(
                      'BTC-leading alt weakness',
                      'BTC-fuehrende Altcoin-Schwaeche'
                    ),
                    patch: { open_btc_strength_regime: 'lagging' },
                  },
                  {
                    label: pickText('Funding extreme longs', 'Funding: extreme Longs'),
                    patch: { open_funding_regime: 'extreme_longs' },
                  },
                  {
                    label: pickText('Asia session', 'Asien-Session'),
                    patch: { open_session_bucket: 'asia' },
                  },
                  {
                    label: pickText('EU session', 'EU-Session'),
                    patch: { open_session_bucket: 'eu' },
                  },
                  {
                    label: pickText('US session', 'US-Session'),
                    patch: { open_session_bucket: 'us' },
                  },
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
                  {pickText('Reset regime filters', 'Regime-Filter zuruecksetzen')}
                </button>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 xl:grid-cols-8 gap-3 mt-3">
                <input
                  value={symbol}
                  onChange={(event) => setSymbol(event.target.value)}
                  placeholder={pickText('Symbol', 'Symbol')}
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={side}
                    onChange={setSide}
                    options={localizedSideOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={status}
                    onChange={setStatus}
                    options={localizedStatusOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={outcome}
                    onChange={setOutcome}
                    options={localizedOutcomeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={dateRange}
                    onChange={(value) => {
                      setDateRange(value)
                      setCustomFromTime(null)
                      setCustomToTime(null)
                    }}
                    options={localizedRangeOptions}
                  />
                </div>
                <input
                  value={openSelectionBucket}
                  onChange={(event) =>
                    setOpenSelectionBucket(event.target.value)
                  }
                  placeholder={pickText('Selection bucket', 'Auswahl-Bucket')}
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <input
                  value={closeReason}
                  onChange={(event) => setCloseReason(event.target.value)}
                  placeholder={pickText('Close reason', 'Schliessungsgrund')}
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={exitReasonQuality}
                    onChange={setExitReasonQuality}
                    options={localizedExitReasonQualityOptions}
                  />
                </div>
                <button
                  onClick={() => {
                    void loadCases()
                    void loadAnomalies()
                  }}
                  className="h-11 rounded-lg bg-nofx-gold text-black font-semibold hover:opacity-90 transition"
                >
                  {pickText('Refresh', 'Aktualisieren')}
                </button>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-6 gap-3 mt-3">
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openTrendRegime}
                    onChange={setOpenTrendRegime}
                    options={localizedTrendRegimeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openVolatilityRegime}
                    onChange={setOpenVolatilityRegime}
                    options={localizedVolatilityRegimeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openBTCStrengthRegime}
                    onChange={setOpenBTCStrengthRegime}
                    options={localizedBtcStrengthOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openFundingRegime}
                    onChange={setOpenFundingRegime}
                    options={localizedFundingRegimeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openOIRegime}
                    onChange={setOpenOIRegime}
                    options={localizedOiRegimeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openSessionBucket}
                    onChange={setOpenSessionBucket}
                    options={localizedSessionOptions}
                  />
                </div>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3 mt-3">
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openWeekdayBucket}
                    onChange={setOpenWeekdayBucket}
                    options={localizedWeekdayOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openVenueTier}
                    onChange={setOpenVenueTier}
                    options={localizedVenueTierOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openLiquidityTier}
                    onChange={setOpenLiquidityTier}
                    options={localizedLiquidityTierOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openSpreadBucket}
                    onChange={setOpenSpreadBucket}
                    options={localizedExecutionBucketOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={openSlippageBucket}
                    onChange={setOpenSlippageBucket}
                    options={localizedExecutionBucketOptions}
                  />
                </div>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mt-3">
                <input
                  value={minPnl}
                  onChange={(event) => setMinPnl(event.target.value)}
                  placeholder={pickText('Min PnL', 'Min. PnL')}
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <input
                  value={maxPnl}
                  onChange={(event) => setMaxPnl(event.target.value)}
                  placeholder={pickText('Max PnL', 'Max. PnL')}
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={reviewQueueMode}
                    onChange={setReviewQueueMode}
                    options={localizedReviewQueueOptions}
                  />
                </div>
              </div>
              {(customFromTime || customToTime) && (
                <div className="mt-3 rounded-lg border border-sky-400/20 bg-sky-500/10 p-3 flex flex-col md:flex-row md:items-center md:justify-between gap-3">
                  <div className="text-sm text-sky-200">
                    {pickText(
                      'Exact optimizer review window active:',
                      'Exaktes Optimizer-Review-Fenster aktiv:'
                    )}{' '}
                    {customFromTime
                      ? new Date(customFromTime).toLocaleString(
                          toDateTimeLocale(language)
                        )
                      : '-'}{' '}
                    {pickText('to', 'bis')}{' '}
                    {customToTime
                      ? new Date(customToTime).toLocaleString(
                          toDateTimeLocale(language)
                        )
                      : '-'}
                  </div>
                  <button
                    onClick={() => {
                      setCustomFromTime(null)
                      setCustomToTime(null)
                    }}
                    className="h-9 px-3 rounded-lg border border-sky-300/30 text-sky-200"
                  >
                    {pickText('Clear exact window', 'Exaktes Fenster loeschen')}
                  </button>
                </div>
              )}
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">{pickText('Deals', 'Deals')}</div>
                <div className="text-2xl font-semibold">
                  {summary?.total_deals || 0}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">{pickText('Win Rate', 'Trefferquote')}</div>
                <div className="text-2xl font-semibold">
                  {summary ? `${summary.win_rate.toFixed(1)}%` : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">{pickText('Net PnL', 'Netto-PnL')}</div>
                <div
                  className={`text-2xl font-semibold ${summary && summary.net_pnl >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}
                >
                  {summary ? formatMoney(summary.net_pnl) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">{pickText('Avg Hold', 'Ø Haltedauer')}</div>
                <div className="text-2xl font-semibold">
                  {summary ? formatHold(summary.avg_hold_ms) : '-'}
                </div>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3">
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">{pickText('Bad entries', 'Schlechte Einstiege')}</div>
                <div className="text-2xl font-semibold mt-1">
                  {summary?.bad_entry_deals || 0}
                </div>
                <div className="text-xs text-nofx-text-muted mt-2">
                  {pickText('Avg entry score', 'Ø Einstiegsscore')}{' '}
                  {summary ? formatScore(summary.avg_entry_timing_score) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">{pickText('Bad exits', 'Schlechte Exits')}</div>
                <div className="text-2xl font-semibold mt-1">
                  {summary?.bad_exit_deals || 0}
                </div>
                <div className="text-xs text-nofx-text-muted mt-2">
                  {pickText('Avg exit score', 'Ø Exit-Score')}{' '}
                  {summary ? formatScore(summary.avg_exit_efficiency_score) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">{pickText('Avoidable losses', 'Vermeidbare Verluste')}</div>
                <div className="text-2xl font-semibold mt-1">
                  {summary?.avoidable_loss_deals || 0}
                </div>
                <div className="text-xs text-nofx-text-muted mt-2">
                  {pickText('Avg give-back', 'Ø Gewinnabgabe')}{' '}
                  {summary ? formatPct(summary.avg_profit_given_back_pct) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">{pickText('Strong entry / weak exit', 'Starker Einstieg / schwacher Exit')}</div>
                <div className="text-2xl font-semibold mt-1">
                  {summary?.strong_entry_weak_exit_deals || 0}
                </div>
                <div className="text-xs text-nofx-text-muted mt-2">
                  {pickText('Avg MFE capture', 'Ø MFE-Erfassung')}{' '}
                  {summary ? formatPct(summary.avg_mfe_captured_pct) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">{pickText('Weak entry / lucky exit', 'Schwacher Einstieg / Gluecks-Exit')}</div>
                <div className="text-2xl font-semibold mt-1">
                  {summary?.weak_entry_lucky_exit_deals || 0}
                </div>
                <div className="text-xs text-nofx-text-muted mt-2">
                  {pickText('Avg sizing score', 'Ø Sizing-Score')}{' '}
                  {summary ? formatScore(summary.avg_risk_sizing_score) : '-'}
                </div>
              </div>
            </div>

            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h2 className="font-semibold text-lg">
                    {pickText(
                      'What changed after last patch?',
                      'Was hat sich nach dem letzten Patch geaendert?'
                    )}
                  </h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    {pickText(
                      'Fast readout from the most recent strategy version and its observed attribution window.',
                      'Schnelle Einordnung der juengsten Strategieversion und ihres beobachteten Attributionsfensters.'
                    )}
                  </p>
                </div>
                {latestVersion && (
                  <button
                    onClick={() => openVersionDetail(latestVersion.version.id)}
                    className="h-10 px-4 rounded-lg border border-nofx-gold/30 text-nofx-gold font-semibold"
                  >
                    {pickText('Open patch detail', 'Patch-Details oeffnen')}
                  </button>
                )}
              </div>

              {!latestVersion ? (
                <div className="text-sm text-nofx-text-muted mt-4">
                  {pickText(
                    'No strategy patch has been applied yet for this trader.',
                    'Fuer diesen Trader wurde noch kein Strategie-Patch uebernommen.'
                  )}
                </div>
              ) : (
                <div className="space-y-4 mt-4">
                  <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-gold">
                      {formatStrategyVersionSourceType(
                        latestVersion.version.source_type,
                        language
                      )}
                    </div>
                    <div className="font-semibold mt-2">
                      {latestVersion.version.summary ||
                        pickText('Strategy change', 'Strategieaenderung')}
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-2">
                      {pickText('Applied', 'Uebernommen')}{' '}
                      {new Date(
                        latestVersion.version.applied_at ||
                          latestVersion.version.created_at
                      ).toLocaleString(toDateTimeLocale(language))}
                    </div>
                    {latestVersion.version.expected_effect && (
                      <div className="text-sm text-nofx-text-muted mt-3">
                        {pickText('Intended effect:', 'Beabsichtigter Effekt:')}{' '}
                        {latestVersion.version.expected_effect}
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
                      <div className="text-xs text-nofx-text-muted">{pickText('Status', 'Status')}</div>
                      <div
                        className={`text-lg font-semibold mt-1 ${getPatchOutcomeTone(
                          latestFullDelta,
                          latestVersion.attribution?.rollback_suggested
                        )}`}
                      >
                        {getPatchOutcomeLabel(
                          latestFullDelta,
                          latestVersion.attribution?.rollback_suggested,
                          language
                        )}
                      </div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Full-strategy delta', 'Delta Gesamtstrategie')}
                      </div>
                      <div
                        className={`text-lg font-semibold mt-1 ${getPatchOutcomeTone(
                          latestFullDelta
                        )}`}
                      >
                        {latestFullDelta === null
                          ? pickText('No observation yet', 'Noch keine Beobachtung')
                          : formatMoney(latestFullDelta)}
                      </div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Target-cohort delta', 'Delta Zielkohorte')}
                      </div>
                      <div
                        className={`text-lg font-semibold mt-1 ${getPatchOutcomeTone(
                          latestTargetDelta
                        )}`}
                      >
                        {latestTargetDelta === null
                          ? pickText('Not cohort-scoped', 'Nicht kohortenbezogen')
                          : formatMoney(latestTargetDelta)}
                      </div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Rollback suggestion', 'Rollback-Empfehlung')}
                      </div>
                      <div
                        className={`text-lg font-semibold mt-1 ${
                          latestVersion.attribution?.rollback_suggested
                            ? 'text-rose-300'
                            : 'text-emerald-300'
                        }`}
                      >
                        {latestVersion.attribution?.rollback_suggested
                          ? pickText('Suggested', 'Empfohlen')
                          : pickText('Not suggested', 'Nicht empfohlen')}
                      </div>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Full before', 'Gesamt davor')}
                      </div>
                      <div className="text-sm text-nofx-text-muted mt-1">
                        {formatDatasetMini(
                          latestVersion.attribution?.full_before_summary,
                          language
                        )}
                      </div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Full after', 'Gesamt danach')}
                      </div>
                      <div className="text-sm text-nofx-text-muted mt-1">
                        {formatDatasetMini(
                          latestVersion.attribution?.full_after_summary,
                          language
                        )}
                      </div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Target cohort', 'Zielkohorte')}
                      </div>
                      <div className="text-sm text-nofx-text-muted mt-1">
                        {formatVersionCohort(latestVersion.target_cohort, language)}
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
                  <h2 className="font-semibold text-lg">
                    {pickText('Filtered deals', 'Gefilterte Deals')}
                  </h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    {selectedTrader
                      ? `${pickText('Trader', 'Trader')}: ${selectedTrader.trader_name}`
                      : pickText('Select a trader', 'Trader auswaehlen')}
                  </p>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    {pickText('Queue:', 'Warteschlange:')}{' '}
                    {buildQueueLabel(reviewQueueMode, language)} · {displayedItems.length}{' '}
                    {pickText('visible', 'sichtbar')} / {items.length}{' '}
                    {pickText('loaded', 'geladen')}
                  </p>
                </div>
                <div className="text-right">
                  <div className="flex items-center justify-end gap-2 mb-2">
                    <button
                      onClick={exportFilteredDealsJSON}
                      disabled={displayedItems.length === 0}
                      className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs disabled:opacity-40"
                    >
                      {pickText('Export JSON', 'JSON exportieren')}
                    </button>
                    <button
                      onClick={exportFilteredDealsCSV}
                      disabled={displayedItems.length === 0}
                      className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs disabled:opacity-40"
                    >
                      {pickText('Export CSV', 'CSV exportieren')}
                    </button>
                  </div>
                  <div className="text-xs text-nofx-text-muted">
                    {pickText('Hotkeys:', 'Hotkeys:')} `J` / `K` {pickText('or', 'oder')} `↑` / `↓`
                  </div>
                  {loading && (
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {pickText('Loading…', 'Laedt…')}
                    </div>
                  )}
                </div>
              </div>
              {error ? (
                <div className="p-5 text-rose-400">{error}</div>
              ) : displayedItems.length === 0 ? (
                <div className="p-5 text-nofx-text-muted">
                  {reviewQueueMode
                    ? pickText(
                        'No deals matched the current review queue.',
                        'Keine Deals passten zur aktuellen Review-Warteschlange.'
                      )
                    : pickText(
                        'No deals matched the current filters.',
                        'Keine Deals passten zu den aktuellen Filtern.'
                      )}
                </div>
              ) : (
                <div className="divide-y divide-white/5">
                  {displayedItems.map((item) => {
                    const topSuggestion = item.classifier_assist?.suggestions?.[0]
                    const assistClasses = classifierToneClasses(
                      item.classifier_assist?.highlight_level
                    )
                    const giveBackPct = getDealGiveBackPct(item.case)
                    const qualityBadges = buildQualityBadges(item.case, language)
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
                              {pickText('Open:', 'Open:')} {formatDate(item.case.entry_time_ms, language)} |{' '}
                              {pickText('Close:', 'Close:')} {formatDate(item.case.exit_time_ms, language)}
                            </div>
                            <div className="text-sm text-nofx-text-muted mt-2 line-clamp-2">
                              {item.open_reasoning ||
                                pickText(
                                  'No open reasoning snapshot linked yet.',
                                  'Noch kein Snapshot der Open-Begruendung verknuepft.'
                                )}
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
                                    topSuggestion.issue_type,
                                    language
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
                                  {pickText('Give-back', 'Gewinnabgabe')}{' '}
                                  {formatPct(giveBackPct)}
                                  {' · '}
                                  {pickText('Peak', 'Peak')}{' '}
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
                  <h2 className="font-semibold text-lg">
                    {pickText('Deal detail', 'Deal-Details')}
                  </h2>
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
                    {pickText('Previous', 'Zurueck')}
                  </button>
                  <button
                    onClick={() => goToReviewCase(1)}
                    disabled={
                      selectedCaseIndex < 0 ||
                      selectedCaseIndex >= displayedItems.length - 1
                    }
                    className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-sm disabled:opacity-40"
                  >
                    {pickText('Next', 'Weiter')}
                  </button>
                </div>
              </div>
              {detailLoading ? (
                <div className="text-nofx-text-muted">
                  {pickText('Loading detail…', 'Details werden geladen…')}
                </div>
              ) : !detail ? (
                <div className="text-nofx-text-muted">
                  {pickText(
                    'Select a deal to inspect its open and close rationale.',
                    'Waehle einen Deal aus, um seine Open- und Close-Begruendung zu pruefen.'
                  )}
                </div>
              ) : (
                <div className="space-y-5">
                  <div className="grid grid-cols-2 gap-3 text-sm">
                    <div>
                      <span className="text-nofx-text-muted">{pickText('Symbol', 'Symbol')}</span>
                      <div className="font-semibold mt-1">
                        {detail.case.symbol}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">{pickText('Side', 'Richtung')}</span>
                      <div className="font-semibold mt-1">
                        {detail.case.side}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">{pickText('Outcome', 'Ergebnis')}</span>
                      <div className="font-semibold mt-1">
                        {detail.case.outcome}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">{pickText('Hold', 'Haltedauer')}</span>
                      <div className="font-semibold mt-1">
                        {formatHold(detail.case.hold_duration_ms)}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">{pickText('Entry', 'Einstieg')}</span>
                      <div className="font-semibold mt-1">
                        {formatMoney(detail.case.entry_price)} @{' '}
                        {formatDate(detail.case.entry_time_ms, language)}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">{pickText('Exit', 'Exit')}</span>
                      <div className="font-semibold mt-1">
                        {detail.case.exit_time_ms
                          ? `${formatMoney(detail.case.exit_price)} @ ${formatDate(detail.case.exit_time_ms, language)}`
                          : '-'}
                      </div>
                    </div>
                    <div className="col-span-2">
                      <span className="text-nofx-text-muted">{pickText('PnL', 'PnL')}</span>
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
                        <div className="font-semibold">
                          {pickText('Quality readout', 'Qualitaetsauswertung')}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          {pickText(
                            'Entry, exit, and sizing quality derived from the full deal path.',
                            'Einstiegs-, Exit- und Sizing-Qualitaet abgeleitet aus dem vollstaendigen Deal-Verlauf.'
                          )}
                        </div>
                      </div>
                      <div className="text-right text-xs text-nofx-text-muted">
                        {pickText('MFE capture', 'MFE-Erfassung')}{' '}
                        {formatPct(detail.case.mfe_captured_pct || 0)}
                      </div>
                    </div>

                    {buildQualityNarrative(detail.case, language) && (
                      <div className="rounded-lg border border-amber-400/20 bg-amber-500/10 px-3 py-2 text-sm text-amber-100">
                        {buildQualityNarrative(detail.case, language)}
                      </div>
                    )}

                    <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Entry timing score', 'Einstiegs-Timing-Score')}
                        </div>
                        <div className="text-lg font-semibold mt-1">
                          {formatScore(detail.case.entry_timing_score)}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-2">
                          {pickText('First profit', 'Erster Gewinn')}{' '}
                          {formatHold(detail.case.time_to_first_profit_ms)}
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Exit efficiency score', 'Exit-Effizienz-Score')}
                        </div>
                        <div className="text-lg font-semibold mt-1">
                          {formatScore(detail.case.exit_efficiency_score)}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-2">
                          {pickText('Give-back', 'Gewinnabgabe')}{' '}
                          {formatMoney(detail.case.profit_given_back)} /{' '}
                          {formatPct(detail.case.profit_given_back_pct)}
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Risk sizing score', 'Risiko-Sizing-Score')}
                        </div>
                        <div className="text-lg font-semibold mt-1">
                          {formatScore(detail.case.risk_sizing_score)}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-2">
                          {pickText('Planned risk', 'Geplantes Risiko')}{' '}
                          {formatPct(detail.case.planned_risk_pct)}
                        </div>
                      </div>
                    </div>

                    <div className="grid grid-cols-2 xl:grid-cols-4 gap-3 text-sm">
                      <div>
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Max favorable excursion', 'Maximale positive Auslenkung')}
                        </div>
                        <div className="font-semibold mt-1">
                          {formatMoney(detail.case.max_favorable_excursion)} /{' '}
                          {formatPct(detail.case.max_favorable_excursion_pct)}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Max adverse excursion', 'Maximale negative Auslenkung')}
                        </div>
                        <div className="font-semibold mt-1 text-rose-300">
                          {formatMoney(detail.case.max_adverse_excursion)} /{' '}
                          {formatPct(detail.case.max_adverse_excursion_pct)}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Time to first profit', 'Zeit bis zum ersten Gewinn')}
                        </div>
                        <div className="font-semibold mt-1">
                          {detail.case.time_to_first_profit_ms
                            ? formatHold(detail.case.time_to_first_profit_ms)
                            : pickText('Never', 'Nie')}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Time to max drawdown', 'Zeit bis zum maximalen Drawdown')}
                        </div>
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
                        <div className="font-semibold">
                          {pickText('Matching symbol priors', 'Passende Symbol-Priors')}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          {pickText(
                            'Learned symbol+side+regime patterns from prior closed deals for this trader.',
                            'Gelernte Symbol+Richtung+Regime-Muster aus frueheren geschlossenen Deals dieses Traders.'
                          )}
                        </div>
                      </div>
                      <div className="text-xs text-nofx-text-muted">
                        {detail.symbol_behavior_priors?.length || 0}{' '}
                        {pickText('matches', 'Treffer')}
                      </div>
                    </div>

                    {!detail.symbol_behavior_priors ||
                    detail.symbol_behavior_priors.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No learned symbol prior matched this deal yet.',
                          'Bisher passt noch kein gelerntes Symbol-Prior zu diesem Deal.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-3">
                        {detail.symbol_behavior_priors.map((prior) => (
                          <div
                            key={prior.id}
                            className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3"
                          >
                            <div className="flex flex-wrap items-start justify-between gap-3">
                              <div>
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="font-semibold">
                                    {prior.symbol} · {prior.side}
                                  </span>
                                  <span
                                    className={`px-2 py-1 rounded-full text-[11px] border ${symbolBehaviorToneClasses(
                                      prior.behavior_bias
                                    )}`}
                                  >
                                    {prior.behavior_bias ||
                                      pickText('mixed', 'gemischt')}
                                  </span>
                                <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                  {prior.status}
                                </span>
                                <span
                                  className={`px-2 py-1 rounded-full text-[11px] border ${symbolBehaviorValidationClasses(
                                    prior.validation_label
                                  )}`}
                                >
                                  {formatSymbolBehaviorValidationLabel(
                                    prior.validation_label,
                                    language
                                  )}
                                </span>
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                {prior.regime_signature}
                              </div>
                              </div>
                              <div className="text-right text-xs text-nofx-text-muted">
                                <div>
                                  {pickText('Match', 'Treffer')}{' '}
                                  {formatSemanticSimilarityScore(prior.match_score, language)}
                                </div>
                                <div className="mt-1">
                                  {pickText('Action', 'Aktion')}{' '}
                                  {formatSymbolBehaviorAction(prior.recommended_action, language)}
                                </div>
                              </div>
                            </div>

                            <div className="text-sm text-nofx-text-muted">
                              {prior.summary}
                            </div>

                            {prior.validation_summary && (
                              <div className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-2 text-xs text-amber-100">
                                {prior.validation_summary}
                              </div>
                            )}

                            {prior.validation_alert && (
                              <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-xs text-nofx-text-muted">
                                {prior.validation_alert}
                              </div>
                            )}

                            <div className="grid grid-cols-2 md:grid-cols-5 gap-3 text-xs">
                              <div>
                                Samples:{' '}
                                <span className="text-white">
                                  {prior.sample_count}
                                </span>
                              </div>
                              <div>
                                Win rate:{' '}
                                <span className="text-white">
                                  {formatPct(prior.win_rate * 100)}
                                </span>
                              </div>
                              <div>
                                Avg PnL:{' '}
                                <span
                                  className={
                                    prior.avg_pnl >= 0
                                      ? 'text-emerald-300'
                                      : 'text-rose-300'
                                  }
                                >
                                  {formatMoney(prior.avg_pnl)} /{' '}
                                  {formatPct(prior.avg_pnl_pct)}
                                </span>
                              </div>
                              <div>
                                Confidence:{' '}
                                <span className="text-white">
                                  {formatPct(prior.confidence_score * 100)}
                                </span>
                              </div>
                              <div>
                                Stability:{' '}
                                <span className="text-white">
                                  {formatPct(prior.stability_score * 100)}
                                </span>
                              </div>
                            </div>

                            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
                              <div>
                                Train / holdout:{' '}
                                <span className="text-white">
                                  {prior.training_sample_count || 0} /{' '}
                                  {prior.validation_sample_count || 0}
                                </span>
                              </div>
                              <div>
                                Holdout support:{' '}
                                <span className="text-white">
                                  {prior.validation_support_count || 0} /{' '}
                                  {prior.validation_sample_count || 0}
                                </span>
                                <span className="text-nofx-text-muted">
                                  {' '}
                                  ({formatPct(
                                    (prior.validation_support_score || 0) * 100
                                  )})
                                </span>
                              </div>
                              <div>
                                Recent support:{' '}
                                <span className="text-white">
                                  {prior.recent_support_count || 0} /{' '}
                                  {prior.recent_sample_count || 0}
                                </span>
                                <span className="text-nofx-text-muted">
                                  {' '}
                                  ({formatPct(
                                    (prior.recent_support_score || 0) * 100
                                  )})
                                </span>
                              </div>
                              <div>
                                Drift:{' '}
                                <span
                                  className={
                                    (prior.drift_score || 0) >= 0.5
                                      ? 'text-amber-300'
                                      : 'text-white'
                                  }
                                >
                                  {formatPct((prior.drift_score || 0) * 100)}
                                </span>
                              </div>
                            </div>

                            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
                              <div>
                                FP score:{' '}
                                <span className="text-rose-300">
                                  {formatPct(
                                    (prior.false_positive_score || 0) * 100
                                  )}
                                </span>
                              </div>
                              <div>
                                FN risk:{' '}
                                <span className="text-amber-300">
                                  {formatPct(
                                    (prior.false_negative_score || 0) * 100
                                  )}
                                </span>
                              </div>
                              <div>
                                Validation label:{' '}
                                <span className="text-white">
                                  {formatSymbolBehaviorValidationLabel(
                                    prior.validation_label
                                  )}
                                </span>
                              </div>
                              <div>
                                Recent avg:{' '}
                                <span className="text-white">
                                  {formatPct(prior.recent_avg_pnl_pct || 0)}
                                </span>
                              </div>
                            </div>

                            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
                              <div>
                                Decision cycles:{' '}
                                <span className="text-white">
                                  {prior.decision_cycle_count || 0}
                                </span>
                              </div>
                              <div>
                                Open signals:{' '}
                                <span className="text-white">
                                  {prior.decision_open_count || 0}
                                </span>
                              </div>
                              <div>
                                Avg decision conf:{' '}
                                <span className="text-white">
                                  {formatPct(prior.avg_decision_confidence || 0)}
                                </span>
                              </div>
                              <div>
                                Contradiction:{' '}
                                <span
                                  className={
                                    (prior.contradiction_score || 0) >= 0.55
                                      ? 'text-rose-300'
                                      : 'text-white'
                                  }
                                >
                                  {formatPct((prior.contradiction_score || 0) * 100)}
                                </span>
                              </div>
                            </div>

                            {prior.signal_cluster_key && (
                              <div className="text-xs text-nofx-text-muted">
                                Normalized cluster:{' '}
                                <span className="text-white">
                                  {prior.signal_cluster_key}
                                </span>
                              </div>
                            )}

                            {prior.signal_clusters &&
                              prior.signal_clusters.length > 0 && (
                                <div className="flex flex-wrap gap-2">
                                  {prior.signal_clusters.map((cluster) => (
                                    <span
                                      key={`${prior.id}-cluster-${cluster}`}
                                      className="px-2 py-1 rounded-full text-[11px] border border-emerald-400/20 bg-emerald-500/10 text-emerald-200"
                                    >
                                      {cluster}
                                    </span>
                                  ))}
                                </div>
                              )}

                            {prior.signal_tags && prior.signal_tags.length > 0 && (
                              <div className="flex flex-wrap gap-2">
                                {prior.signal_tags.map((tag) => (
                                  <span
                                    key={`${prior.id}-signal-${tag}`}
                                    className="px-2 py-1 rounded-full text-[11px] border border-sky-400/20 bg-sky-500/10 text-sky-200"
                                  >
                                    {tag}
                                  </span>
                                ))}
                              </div>
                            )}

                            {prior.evidence && prior.evidence.length > 0 && (
                              <div className="space-y-2">
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                  Evidence
                                </div>
                                <div className="space-y-2">
                                  {prior.evidence.map((evidence) => (
                                    <div
                                      key={`${prior.id}-${evidence.case_id}`}
                                      className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 flex flex-wrap items-center justify-between gap-3"
                                    >
                                      <div className="text-xs text-nofx-text-muted">
                                        <span className="text-white">
                                          {evidence.case_id}
                                        </span>{' '}
                                        · {evidence.outcome || '-'} ·{' '}
                                        {formatMoney(evidence.realized_pnl)} /{' '}
                                        {formatPct(evidence.realized_pnl_pct)}
                                      </div>
                                      <button
                                        onClick={() =>
                                          setSelectedCaseId(evidence.case_id)
                                        }
                                        className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs"
                                      >
                                        Focus evidence
                                      </button>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            )}

                            {prior.decision_evidence &&
                              prior.decision_evidence.length > 0 && (
                                <div className="space-y-2">
                                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                    Decision evidence
                                  </div>
                                  <div className="space-y-2">
                                    {prior.decision_evidence.map((evidence) => (
                                      <div
                                        key={`${prior.id}-decision-${evidence.cycle_number}`}
                                        className="rounded-lg border border-white/10 bg-black/20 px-3 py-2"
                                      >
                                        <div className="flex flex-wrap items-center justify-between gap-3">
                                          <div className="text-xs text-nofx-text-muted">
                                            Cycle{' '}
                                            <span className="text-white">
                                              {evidence.cycle_number}
                                            </span>{' '}
                                            · {evidence.action} ·{' '}
                                            {formatPct(evidence.confidence || 0)}
                                          </div>
                                          <div className="text-xs text-nofx-text-muted">
                                            {evidence.timestamp
                                              ? new Date(
                                                  evidence.timestamp
                                                ).toLocaleString()
                                              : '-'}
                                          </div>
                                        </div>
                                        {evidence.reasoning && (
                                          <div className="text-sm text-nofx-text-muted mt-2">
                                        {evidence.reasoning}
                                          </div>
                                        )}
                                        {evidence.signal_cluster_key && (
                                          <div className="text-xs text-nofx-text-muted mt-2">
                                            Normalized cluster:{' '}
                                            <span className="text-white">
                                              {evidence.signal_cluster_key}
                                            </span>
                                          </div>
                                        )}
                                        {((evidence.signal_tags &&
                                          evidence.signal_tags.length > 0) ||
                                          (evidence.signal_clusters &&
                                            evidence.signal_clusters.length >
                                              0) ||
                                          (evidence.candidate_sources &&
                                            evidence.candidate_sources.length >
                                              0)) && (
                                          <div className="flex flex-wrap gap-2 mt-2">
                                            {evidence.signal_clusters?.map(
                                              (cluster) => (
                                                <span
                                                  key={`${prior.id}-decision-cluster-${evidence.cycle_number}-${cluster}`}
                                                  className="px-2 py-1 rounded-full text-[11px] border border-emerald-400/20 bg-emerald-500/10 text-emerald-200"
                                                >
                                                  {cluster}
                                                </span>
                                              )
                                            )}
                                            {evidence.signal_tags?.map((tag) => (
                                              <span
                                                key={`${prior.id}-decision-tag-${evidence.cycle_number}-${tag}`}
                                                className="px-2 py-1 rounded-full text-[11px] border border-sky-400/20 bg-sky-500/10 text-sky-200"
                                              >
                                                {tag}
                                              </span>
                                            ))}
                                            {evidence.candidate_sources?.map(
                                              (source) => (
                                                <span
                                                  key={`${prior.id}-decision-source-${evidence.cycle_number}-${source}`}
                                                  className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white"
                                                >
                                                  {source}
                                                </span>
                                              )
                                            )}
                                          </div>
                                        )}
                                      </div>
                                    ))}
                                  </div>
                                </div>
                              )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <div className="font-semibold">
                          {pickText('Deal compare', 'Deal-Vergleich')}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          {pickText(
                            'Compare the current deal against another loaded case side by side.',
                            'Vergleiche den aktuellen Deal direkt mit einem anderen geladenen Fall.'
                          )}
                        </div>
                      </div>
                      <div className="w-full max-w-xs h-10 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                        <NofxSelect
                          value={comparePeerCaseId}
                          onChange={setComparePeerCaseId}
                          options={[
                            {
                              value: '',
                              label: pickText(
                                'Select compare deal',
                                'Vergleichsdeal auswaehlen'
                              ),
                            },
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
                        {pickText(
                          'Choose another visible deal to compare it against the currently selected one.',
                          'Waehle einen anderen sichtbaren Deal aus, um ihn mit dem aktuell ausgewaehlten zu vergleichen.'
                        )}
                      </div>
                    ) : comparePeerLoading ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText('Loading compare deal…', 'Vergleichsdeal wird geladen…')}
                      </div>
                    ) : !comparePeerDetail ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText('No compare deal loaded yet.', 'Noch kein Vergleichsdeal geladen.')}
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
                                    {index === 0
                                      ? pickText('Current deal', 'Aktueller Deal')
                                      : pickText('Compare deal', 'Vergleichsdeal')}
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
                                    {pickText('Focus deal', 'Deal fokussieren')}
                                  </button>
                                )}
                              </div>

                              <div className="grid grid-cols-2 gap-3 text-xs">
                                <div>
                                  {pickText('Outcome:', 'Ergebnis:')}{' '}
                                  <span className="text-white">
                                    {entry.case.outcome}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Hold:', 'Haltedauer:')}{' '}
                                  <span className="text-white">
                                    {formatHold(entry.case.hold_duration_ms)}
                                  </span>
                                </div>
                                <div>
                                  {pickText('PnL:', 'PnL:')}{' '}
                                  <span className={tone}>
                                    {formatMoney(entry.case.realized_pnl)} /{' '}
                                    {formatPct(entry.case.realized_pnl_pct)}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Close reason:', 'Schliessungsgrund:')}{' '}
                                  <span className="text-white">
                                    {entry.case.close_reason || '-'}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Entry:', 'Einstieg:')}{' '}
                                  <span className="text-white">
                                    {formatMoney(entry.case.entry_price)}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Exit:', 'Exit:')}{' '}
                                  <span className="text-white">
                                    {entry.case.exit_price
                                      ? formatMoney(entry.case.exit_price)
                                      : '-'}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Peak MFE:', 'Peak-MFE:')}{' '}
                                  <span className="text-emerald-300">
                                    {formatPct(entry.case.max_favorable_excursion_pct || 0)}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Max adverse:', 'Max negativ:')}{' '}
                                  <span className="text-rose-300">
                                    {formatPct(entry.case.max_adverse_excursion_pct || 0)}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Give-back:', 'Gewinnabgabe:')}{' '}
                                  <span className="text-amber-300">
                                    {formatPct(maxGiveBackPct)}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Exit efficiency:', 'Exit-Effizienz:')}{' '}
                                  <span className="text-white">
                                    {formatScore(entry.case.exit_efficiency_score)}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Entry timing:', 'Einstiegs-Timing:')}{' '}
                                  <span className="text-white">
                                    {formatScore(entry.case.entry_timing_score)}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Risk sizing:', 'Risiko-Sizing:')}{' '}
                                  <span className="text-white">
                                    {formatScore(entry.case.risk_sizing_score)}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Path points:', 'Pfadpunkte:')}{' '}
                                  <span className="text-white">
                                    {entry.price_timeline?.summary.point_count || 0}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Cycle / platform:', 'Zyklus / Plattform:')}{' '}
                                  <span className="text-white">
                                    {entry.price_timeline?.summary.cycle_samples || 0} /{' '}
                                    {entry.price_timeline?.summary.platform_samples || 0}
                                  </span>
                                </div>
                                <div>
                                  {pickText('Labels:', 'Labels:')}{' '}
                                  <span className="text-white">
                                    {(entry.labels || []).length
                                      ? entry.labels?.join(', ')
                                      : '-'}
                                  </span>
                                </div>
                              </div>

                              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                  {pickText('Open rationale', 'Open-Begruendung')}
                                </div>
                                <div className="text-sm text-nofx-text-muted mt-2">
                                  {formatDealReasonPreview(
                                    entry.open?.event.reasoning,
                                    language
                                  )}
                                </div>
                              </div>

                              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                  {pickText('Close rationale', 'Close-Begruendung')}
                                </div>
                                <div className="text-sm text-nofx-text-muted mt-2">
                                  {formatDealReasonPreview(
                                    entry.close?.event.reasoning,
                                    language
                                  )}
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
                          {pickText('Open sources', 'Open-Quellen')}
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
                        <div className="font-semibold">
                          {pickText('Learned review assist', 'Gelernte Review-Hilfe')}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          {pickText(
                            'Label-memory heuristic trained from previously reviewed deals for this trader.',
                            'Label-Memory-Heuristik, trainiert auf zuvor geprueften Deals dieses Traders.'
                          )}
                        </div>
                      </div>
                      {detail.classifier_assist?.highlight_level && (
                        <span
                          className={`px-2 py-1 rounded-full text-[11px] border ${classifierToneClasses(
                            detail.classifier_assist.highlight_level
                          )}`}
                        >
                          {detail.classifier_assist.highlight_level}{' '}
                          {pickText('signal', 'Signal')}
                        </span>
                      )}
                    </div>

                    <div className="text-sm text-nofx-text-muted">
                      {detail.classifier_assist?.summary ||
                        pickText(
                          'No learned review signal yet. Add more labels to give the heuristic model better examples.',
                          'Noch kein gelerntes Review-Signal vorhanden. Fuege mehr Labels hinzu, damit die Heuristik bessere Beispiele erhaelt.'
                        )}
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
                                        suggestion.issue_type,
                                        language
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
                                      {suggestion.evidence_count}{' '}
                                      {pickText('matches', 'Treffer')}
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
                                        ? pickText('Applying…', 'Wende an…')
                                        : pickText('Accept + label', 'Akzeptieren + labeln')}
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
                                        ? pickText('Saving…', 'Speichere…')
                                        : pickText('Reject', 'Ablehnen')}
                                    </button>
                                    {(suggestion.accepted_count > 0 ||
                                      suggestion.rejected_count > 0) && (
                                      <span className="text-xs text-nofx-text-muted">
                                        {pickText('Accepted', 'Akzeptiert')}{' '}
                                        {suggestion.accepted_count} ·{' '}
                                        {pickText('Rejected', 'Abgelehnt')}{' '}
                                        {suggestion.rejected_count}
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
                        <div className="font-semibold">
                          {pickText('Matching learned patterns', 'Passende gelernte Muster')}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          Learned feature combinations replayed from prior closed
                          deals for this trader.
                        </div>
                      </div>
                      <div className="text-xs text-nofx-text-muted">
                        {visibleDetailLearnedPatterns.length} / {detailLearnedPatterns.length}{' '}
                        {pickText('visible', 'sichtbar')}
                      </div>
                    </div>

                    {detailLearnedPatterns.length > 0 && (
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                        <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                          <NofxSelect
                            value={detailPatternDirectCandidateFilter}
                            onChange={setDetailPatternDirectCandidateFilter}
                            options={[
                              {
                                value: '',
                                label: pickText(
                                  'All candidate states',
                                  'Alle Kandidatenzustaende'
                                ),
                              },
                              {
                                value: 'only',
                                label: pickText(
                                  'Only direct candidates',
                                  'Nur direkte Kandidaten'
                                ),
                              },
                            ]}
                          />
                        </div>
                        <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                          <NofxSelect
                            value={detailPatternLiveActionKindFilter}
                            onChange={setDetailPatternLiveActionKindFilter}
                            options={[
                              {
                                value: '',
                                label: pickText(
                                  'All live-action kinds',
                                  'Alle Live-Aktionsarten'
                                ),
                              },
                              {
                                value: 'suppression',
                                label: formatLearnedPatternLiveActionKind(
                                  'suppression',
                                  language
                                ),
                              },
                              {
                                value: 'rollback',
                                label: formatLearnedPatternLiveActionKind(
                                  'rollback',
                                  language
                                ),
                              },
                            ]}
                          />
                        </div>
                        <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                          <NofxSelect
                            value={detailPatternInterventionStateFilter}
                            onChange={setDetailPatternInterventionStateFilter}
                            options={[
                              {
                                value: '',
                                label: pickText(
                                  'All intervention states',
                                  'Alle Interventionszustaende'
                                ),
                              },
                              {
                                value: 'open',
                                label: pickText(
                                  'Open interventions',
                                  'Offene Interventionen'
                                ),
                              },
                              {
                                value: 'resolved',
                                label: pickText(
                                  'Resolved interventions',
                                  'Abgeschlossene Interventionen'
                                ),
                              },
                              {
                                value: 'none',
                                label: pickText(
                                  'No interventions',
                                  'Keine Interventionen'
                                ),
                              },
                            ]}
                          />
                        </div>
                      </div>
                    )}

                    {detailLearnedPatterns.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        No learned pattern matched this deal yet.
                      </div>
                    ) : visibleDetailLearnedPatterns.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No learned pattern matches the current local review filters.',
                          'Kein gelerntes Muster passt auf die aktuellen lokalen Review-Filter.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-3">
                        {visibleDetailLearnedPatterns.map((pattern) => (
                          <div
                            key={pattern.id}
                            className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3"
                          >
                            <div className="flex flex-wrap items-start justify-between gap-3">
                              <div>
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="font-semibold">
                                    {pattern.pattern_signature}
                                  </span>
                                  <span
                                    className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternClassClasses(
                                      pattern.pattern_class
                                    )}`}
                                  >
                                    {formatLearnedPatternClass(pattern.pattern_class)}
                                  </span>
                                  <span
                                    className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternValidationClasses(
                                      pattern.validation_label
                                    )}`}
                                  >
                                    {formatLearnedPatternValidationLabel(
                                      pattern.validation_label
                                    )}
                                  </span>
                                  <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                    {formatLearnedPatternScope(pattern)}
                                  </span>
                                  {learnedPatternHasDirectLiveActionCandidate(
                                    pattern
                                  ) && (
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternLiveActionKindClasses(
                                        learnedPatternStrongestLiveActionKind(
                                          pattern,
                                          false
                                        )
                                      )}`}
                                    >
                                      {formatLearnedPatternLiveActionKind(
                                        learnedPatternStrongestLiveActionKind(
                                          pattern,
                                          false
                                        )
                                      )}
                                    </span>
                                  )}
                                  {learnedPatternHasOpenIntervention(pattern) && (
                                    <span className="px-2 py-1 rounded-full text-[11px] border border-amber-400/20 bg-amber-500/10 text-amber-200">
                                      {pickText('Open interventions', 'Offene Interventionen')}
                                    </span>
                                  )}
                                  {!learnedPatternHasOpenIntervention(pattern) &&
                                    learnedPatternHasResolvedIntervention(pattern) && (
                                      <span className="px-2 py-1 rounded-full text-[11px] border border-emerald-400/20 bg-emerald-500/10 text-emerald-200">
                                        {pickText(
                                          'Resolved interventions',
                                          'Abgeschlossene Interventionen'
                                        )}
                                      </span>
                                    )}
                                  {pattern.manual_control?.control_state && (
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternManualControlClasses(
                                        pattern.manual_control.control_state
                                      )}`}
                                    >
                                      Analyst{' '}
                                      {formatLearnedPatternManualControlState(
                                        pattern.manual_control.control_state
                                      )}
                                    </span>
                                  )}
                                  {pattern.lifecycle?.status && (
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternLifecycleClasses(
                                        pattern.lifecycle.status
                                      )}`}
                                    >
                                      {formatLearnedPatternLifecycleStatus(
                                        pattern.lifecycle.status
                                      )}
                                    </span>
                                  )}
                                </div>
                                <div className="text-xs text-nofx-text-muted mt-1">
                                  {pattern.regime_signature || 'No regime signature stored'}
                                </div>
                              </div>
                              <div className="text-right text-xs text-nofx-text-muted">
                                <div>
                                  {formatSemanticSimilarityScore(pattern.match_score)}
                                </div>
                                <div className="mt-1">
                                  {formatLearnedPatternUse(pattern.recommended_use)}
                                </div>
                              </div>
                            </div>

                            <div className="text-sm text-nofx-text-muted">
                              {pattern.summary}
                            </div>

                            {pattern.validation_alert && (
                              <div className="rounded-lg border border-sky-400/15 bg-sky-500/5 px-3 py-2 text-xs text-sky-100">
                                {pattern.validation_alert}
                              </div>
                            )}

                            {pattern.lifecycle?.summary && (
                              <div
                                className={`rounded-lg border px-3 py-2 text-xs ${learnedPatternLifecycleClasses(
                                  pattern.lifecycle.status
                                )}`}
                              >
                                <div className="font-medium">
                                  {formatLearnedPatternLifecycleStatus(
                                    pattern.lifecycle.status
                                  )}
                                </div>
                                <div className="mt-1">
                                  {pattern.lifecycle.summary}
                                </div>
                              </div>
                            )}

                            {pattern.lifecycle_trend?.summary && (
                              <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs text-nofx-text-muted">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="font-medium text-white/90">
                                    Lifecycle trend
                                  </span>
                                  {pattern.lifecycle_trend.fragile && (
                                    <span className="px-2 py-1 rounded-full text-[11px] border border-amber-400/20 bg-amber-500/10 text-amber-200">
                                      Fragile
                                    </span>
                                  )}
                                  <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                    {pattern.lifecycle_trend.snapshot_count || 0} snapshots
                                  </span>
                                  <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                    span {formatHoursCompact(pattern.lifecycle_trend.span_hours)}
                                  </span>
                                </div>
                                <div className="mt-2">{pattern.lifecycle_trend.summary}</div>
                                <div className="mt-2 flex flex-wrap gap-3">
                                  <span>
                                    active {formatHoursCompact(pattern.lifecycle_trend.active_hours)}
                                  </span>
                                  <span>
                                    degrading{' '}
                                    {formatHoursCompact(
                                      pattern.lifecycle_trend.degrading_hours
                                    )}
                                  </span>
                                  <span>
                                    rollback{' '}
                                    {formatHoursCompact(
                                      pattern.lifecycle_trend.rollback_watch_hours
                                    )}
                                  </span>
                                  <span>
                                    stale lag{' '}
                                    {pattern.lifecycle_trend.stale_guard_snapshot_count || 0}
                                  </span>
                                </div>
                              </div>
                            )}

                            {pattern.live_guard_attribution?.summary && (
                              <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs text-nofx-text-muted">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="font-medium text-white/90">
                                    Live-guard attribution
                                  </span>
                                  <span
                                    className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternRollupClasses(
                                      pattern.live_guard_attribution.attribution_label
                                    )}`}
                                  >
                                    {formatLearnedPatternRollupLabel(
                                      pattern.live_guard_attribution.attribution_label
                                    )}
                                  </span>
                                  <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                    {pattern.live_guard_attribution.event_count || 0} events
                                  </span>
                                  <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                    {pattern.live_guard_attribution.resolved_event_count || 0} resolved
                                  </span>
                                </div>
                                <div className="mt-2">
                                  {pattern.live_guard_attribution.summary}
                                </div>
                                <div className="mt-2 flex flex-wrap gap-3">
                                  <span>
                                    protective{' '}
                                    {formatPct(
                                      (pattern.live_guard_attribution.protective_rate || 0) *
                                        100
                                    )}
                                  </span>
                                  <span>
                                    overblocking{' '}
                                    {formatPct(
                                      (pattern.live_guard_attribution.overblocking_rate || 0) *
                                        100
                                    )}
                                  </span>
                                  <span>
                                    pending{' '}
                                    {pattern.live_guard_attribution.pending_count || 0}
                                  </span>
                                  <span>
                                    follow-up open{' '}
                                    {pattern.live_guard_attribution.followup_open_count || 0}
                                  </span>
                                </div>
                              </div>
                            )}

                            {pattern.live_guard_attribution_delta?.summary && (
                              <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-xs text-nofx-text-muted">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="font-medium text-white/90">
                                    Live-guard trend
                                  </span>
                                  <span
                                    className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternRollupTrendClasses(
                                      pattern.live_guard_attribution_delta.trend_label
                                    )}`}
                                  >
                                    {formatLearnedPatternRollupTrendLabel(
                                      pattern.live_guard_attribution_delta.trend_label
                                    )}
                                  </span>
                                  <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                    recent{' '}
                                    {pattern.live_guard_attribution_delta
                                      .recent_resolved_event_count || 0}{' '}
                                    resolved
                                  </span>
                                  <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                    prior{' '}
                                    {pattern.live_guard_attribution_delta
                                      .prior_resolved_event_count || 0}{' '}
                                    resolved
                                  </span>
                                </div>
                                <div className="mt-2">
                                  {pattern.live_guard_attribution_delta.summary}
                                </div>
                                <div className="mt-2 flex flex-wrap gap-3">
                                  <span>
                                    recent overblocking{' '}
                                    {formatPct(
                                      (pattern.live_guard_attribution_delta
                                        .recent_overblocking_rate || 0) * 100
                                    )}
                                  </span>
                                  <span>
                                    prior overblocking{' '}
                                    {formatPct(
                                      (pattern.live_guard_attribution_delta
                                        .prior_overblocking_rate || 0) * 100
                                    )}
                                  </span>
                                  <span>
                                    delta{' '}
                                    {formatPct(
                                      (pattern.live_guard_attribution_delta
                                        .overblocking_rate_delta || 0) * 100
                                    )}
                                  </span>
                                  <span>
                                    confidence{' '}
                                    {formatPct(
                                      (pattern.live_guard_attribution_delta
                                        .confidence_score || 0) * 100
                                    )}
                                  </span>
                                </div>
                              </div>
                            )}

                            {pattern.action_hint?.summary && (
                              <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 space-y-2">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="font-medium text-white/90">
                                    Suggested analyst action
                                  </span>
                                  {pattern.action_hint.recommended_action && (
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternActionHintActionClasses(
                                        pattern.action_hint.recommended_action
                                      )}`}
                                    >
                                      {formatLearnedPatternActionHintAction(
                                        pattern.action_hint.recommended_action
                                      )}
                                    </span>
                                  )}
                                  {pattern.action_hint.priority_label && (
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternActionHintPriorityClasses(
                                        pattern.action_hint.priority_label
                                      )}`}
                                    >
                                      {formatLearnedPatternActionHintPriority(
                                        pattern.action_hint.priority_label
                                      )}
                                    </span>
                                  )}
                                  <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                    confidence{' '}
                                    {formatPct(
                                      (pattern.action_hint.confidence_score || 0) *
                                        100
                                    )}
                                  </span>
                                </div>
                                <div className="text-xs text-nofx-text-muted">
                                  {pattern.action_hint.summary}
                                </div>
                                {pattern.action_hint.auto_note && (
                                  <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs text-nofx-text-muted">
                                    {pattern.action_hint.auto_note}
                                  </div>
                                )}
                                {pattern.action_hint.recommended_action && (
                                  <div className="flex flex-wrap gap-2">
                                    <button
                                      onClick={() =>
                                        void applySuggestedPatternAction(pattern)
                                      }
                                      disabled={controlBusyKey !== ''}
                                      className="h-9 px-3 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 text-sm text-nofx-gold disabled:opacity-50"
                                    >
                                      {controlBusyKey ===
                                      `${patternControlKey(pattern)}:${pattern.action_hint.recommended_action}`
                                        ? 'Saving...'
                                        : formatLearnedPatternActionHintAction(
                                            pattern.action_hint.recommended_action
                                          )}
                                    </button>
                                    <button
                                      onClick={() =>
                                        setControlNotes((current) => ({
                                          ...current,
                                          [patternControlKey(pattern)]:
                                            pattern.action_hint?.auto_note || '',
                                        }))
                                      }
                                      className="h-9 px-3 rounded-lg border border-white/10 bg-white/5 text-sm text-white"
                                    >
                                      Use suggested note
                                    </button>
                                  </div>
                                )}
                              </div>
                            )}

                            {pattern.live_action_hint?.summary && (
                              <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3 space-y-2 text-xs text-nofx-text-muted">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="font-medium text-white/90">
                                    Direct live action
                                  </span>
                                  <span
                                    className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternLiveActionKindClasses(
                                      pattern.live_action_hint.candidate_kind
                                    )}`}
                                  >
                                    {formatLearnedPatternLiveActionKind(
                                      pattern.live_action_hint.candidate_kind
                                    )}
                                  </span>
                                  {pattern.live_action_hint.recommended_action && (
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternActionHintActionClasses(
                                        pattern.live_action_hint.recommended_action
                                      )}`}
                                    >
                                      {formatLearnedPatternActionHintAction(
                                        pattern.live_action_hint.recommended_action
                                      )}
                                    </span>
                                  )}
                                  <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                    confidence{' '}
                                    {formatPct(
                                      (pattern.live_action_hint.confidence_score ||
                                        0) * 100
                                    )}
                                  </span>
                                </div>
                                <div>{pattern.live_action_hint.summary}</div>
                              </div>
                            )}

                            {pattern.lifecycle_history &&
                              pattern.lifecycle_history.length > 0 && (
                                <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-2">
                                  <div className="text-[11px] uppercase tracking-[0.18em] text-nofx-text-muted">
                                    Lifecycle trail
                                  </div>
                                  <div className="mt-2 flex flex-wrap gap-2">
                                    {pattern.lifecycle_history.map((snapshot) => (
                                      <div
                                        key={snapshot.id}
                                        className={`rounded-full border px-2 py-1 text-[11px] ${learnedPatternLifecycleClasses(
                                          snapshot.lifecycle_status
                                        )}`}
                                        title={snapshot.summary || undefined}
                                      >
                                        {formatLearnedPatternLifecycleStatus(
                                          snapshot.lifecycle_status
                                        )}{' '}
                                        ·{' '}
                                        {formatCompactTimestampLabel(
                                          snapshot.captured_at
                                        )}{' '}
                                        ·{' '}
                                        {formatLifecycleSnapshotSource(
                                          snapshot.snapshot_source
                                        )}
                                      </div>
                                    ))}
                                  </div>
                                </div>
                              )}

                            {pattern.manual_control && (
                              <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-xs text-nofx-text-muted">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span
                                    className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternManualControlClasses(
                                      pattern.manual_control.control_state
                                    )}`}
                                  >
                                    {formatLearnedPatternManualControlState(
                                      pattern.manual_control.control_state
                                    )}
                                  </span>
                                  <span>
                                    {formatLearnedPatternManualControlAction(
                                      pattern.manual_control.last_action
                                    )}{' '}
                                    · {formatTimestampLabel(pattern.manual_control.applied_at)}
                                  </span>
                                </div>
                                {pattern.manual_control.note && (
                                  <div className="mt-1 text-white/85">
                                    {pattern.manual_control.note}
                                  </div>
                                )}
                              </div>
                            )}

                            {pattern.manual_control_history &&
                              pattern.manual_control_history.length > 0 && (
                                <div className="flex flex-wrap gap-2">
                                  {pattern.manual_control_history.map((event) => (
                                    <div
                                      key={event.id}
                                      className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-[11px] text-white/80"
                                      title={event.note || undefined}
                                    >
                                      {formatLearnedPatternManualControlAction(
                                        event.action
                                      )}{' '}
                                      · {formatCompactTimestampLabel(event.applied_at)}
                                    </div>
                                  ))}
                                </div>
                              )}

                            {pattern.intervention_history &&
                              pattern.intervention_history.length > 0 && (
                                <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 space-y-3">
                                  <div className="text-[11px] uppercase tracking-[0.18em] text-nofx-text-muted">
                                    Intervention history
                                  </div>
                                  <div className="space-y-2">
                                    {pattern.intervention_history.map((event) => (
                                      <div
                                        key={event.id}
                                        className="rounded-lg border border-white/10 bg-white/[0.03] px-3 py-2 text-xs text-nofx-text-muted"
                                      >
                                        <div className="flex flex-wrap items-center gap-2">
                                          <span
                                            className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternInterventionEventTypeClasses(
                                              event.event_type
                                            )}`}
                                          >
                                            {formatLearnedPatternInterventionEventType(
                                              event.event_type
                                            )}
                                          </span>
                                          <span
                                            className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternInterventionStatusClasses(
                                              event.event_status
                                            )}`}
                                          >
                                            {formatLearnedPatternInterventionStatus(
                                              event.event_status
                                            )}
                                          </span>
                                          {event.suggested_action && (
                                            <span
                                              className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternActionHintActionClasses(
                                                event.suggested_action
                                              )}`}
                                            >
                                              {formatLearnedPatternActionHintAction(
                                                event.suggested_action
                                              )}
                                            </span>
                                          )}
                                          {event.applied_action && (
                                            <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                              Applied{' '}
                                              {formatLearnedPatternManualControlAction(
                                                event.applied_action
                                              )}
                                            </span>
                                          )}
                                          <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/70">
                                            {formatCompactTimestampLabel(
                                              learnedPatternInterventionTimestampLabel(
                                                event
                                              )
                                            )}
                                          </span>
                                          {event.event_type === 'suggested' &&
                                            (event.seen_count || 0) > 1 && (
                                              <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/70">
                                                seen {event.seen_count}x
                                              </span>
                                            )}
                                        </div>
                                        {event.summary && (
                                          <div className="mt-2 text-white/85">
                                            {event.summary}
                                          </div>
                                        )}
                                        {event.note && (
                                          <div className="mt-2 rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-white/75">
                                            {event.note}
                                          </div>
                                        )}
                                        {event.direct_live_action_candidate && (
                                          <div className="mt-2 rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-2 text-white/80">
                                            <div className="flex flex-wrap items-center gap-2">
                                              <span
                                                className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternLiveActionKindClasses(
                                                  event.direct_live_action_kind
                                                )}`}
                                              >
                                                {formatLearnedPatternLiveActionKind(
                                                  event.direct_live_action_kind
                                                )}
                                              </span>
                                              <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/70">
                                                confidence{' '}
                                                {formatPct(
                                                  (event.direct_live_action_confidence ||
                                                    0) * 100
                                                )}
                                              </span>
                                            </div>
                                            {event.direct_live_action_summary && (
                                              <div className="mt-2">
                                                {event.direct_live_action_summary}
                                              </div>
                                            )}
                                          </div>
                                        )}
                                      </div>
                                    ))}
                                  </div>
                                </div>
                              )}

                            {canManagePattern(pattern) && (
                              <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 space-y-3">
                                <div className="flex flex-wrap items-center justify-between gap-2">
                                  <div>
                                    <div className="text-[11px] uppercase tracking-[0.18em] text-nofx-text-muted">
                                      Analyst control
                                    </div>
                                    <div className="text-xs text-nofx-text-muted mt-1">
                                      Base {formatLearnedPatternUse(pattern.base_recommended_use)}{' '}
                                      · effective{' '}
                                      {formatLearnedPatternUse(pattern.recommended_use)}
                                    </div>
                                  </div>
                                  {pattern.manual_control?.control_state ? (
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternManualControlClasses(
                                        pattern.manual_control.control_state
                                      )}`}
                                    >
                                      {formatLearnedPatternManualControlState(
                                        pattern.manual_control.control_state
                                      )}
                                    </span>
                                  ) : (
                                    <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                      Inherit
                                    </span>
                                  )}
                                </div>

                                <textarea
                                  value={controlNotes[patternControlKey(pattern)] || ''}
                                  onChange={(event) =>
                                    setControlNotes((current) => ({
                                      ...current,
                                      [patternControlKey(pattern)]: event.target.value,
                                    }))
                                  }
                                  placeholder="Why should this rule stay live, be suppressed, retired, or re-armed?"
                                  className="w-full min-h-[84px] rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm text-white placeholder:text-nofx-text-muted/70"
                                />

                                <div className="flex flex-wrap gap-2">
                                  <button
                                    onClick={() =>
                                      void applyPatternControl(
                                        pattern,
                                        'acknowledge_live'
                                      )
                                    }
                                    disabled={controlBusyKey !== ''}
                                    className="h-9 px-3 rounded-lg border border-emerald-400/20 bg-emerald-500/10 text-sm text-emerald-200 disabled:opacity-50"
                                  >
                                    {controlBusyKey ===
                                    `${patternControlKey(pattern)}:acknowledge_live`
                                      ? 'Refreshing...'
                                      : 'Keep live'}
                                  </button>
                                  <button
                                    onClick={() =>
                                      void applyPatternControl(pattern, 'suppress')
                                    }
                                    disabled={controlBusyKey !== ''}
                                    className="h-9 px-3 rounded-lg border border-amber-400/20 bg-amber-500/10 text-sm text-amber-200 disabled:opacity-50"
                                  >
                                    {controlBusyKey ===
                                    `${patternControlKey(pattern)}:suppress`
                                      ? 'Refreshing...'
                                      : 'Suppress'}
                                  </button>
                                  <button
                                    onClick={() =>
                                      void applyPatternControl(pattern, 'retire')
                                    }
                                    disabled={controlBusyKey !== ''}
                                    className="h-9 px-3 rounded-lg border border-rose-400/20 bg-rose-500/10 text-sm text-rose-200 disabled:opacity-50"
                                  >
                                    {controlBusyKey ===
                                    `${patternControlKey(pattern)}:retire`
                                      ? 'Refreshing...'
                                      : 'Retire'}
                                  </button>
                                  {(pattern.manual_control?.control_state ===
                                    'suppressed' ||
                                    pattern.manual_control?.control_state ===
                                      'retired') && (
                                    <button
                                      onClick={() =>
                                        void applyPatternControl(pattern, 'rearm')
                                      }
                                      disabled={controlBusyKey !== ''}
                                      className="h-9 px-3 rounded-lg border border-sky-400/20 bg-sky-500/10 text-sm text-sky-200 disabled:opacity-50"
                                    >
                                      {controlBusyKey ===
                                      `${patternControlKey(pattern)}:rearm`
                                        ? 'Refreshing...'
                                        : 'Re-arm'}
                                    </button>
                                  )}
                                </div>
                              </div>
                            )}

                            {pattern.feature_set && pattern.feature_set.length > 0 && (
                              <div className="flex flex-wrap gap-2">
                                {pattern.feature_set.map((feature) => (
                                  <span
                                    key={`${pattern.id}-${feature}`}
                                    className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white"
                                  >
                                    {feature}
                                  </span>
                                ))}
                              </div>
                            )}

                            <div className="grid grid-cols-2 md:grid-cols-5 gap-3 text-xs">
                              <div>
                                Samples:{' '}
                                <span className="text-white">
                                  {pattern.sample_count}
                                </span>
                              </div>
                              <div>
                                Support:{' '}
                                <span className="text-white">
                                  {pattern.support_count} / {pattern.sample_count}
                                </span>
                              </div>
                              <div>
                                Avg PnL:{' '}
                                <span
                                  className={
                                    pattern.avg_pnl >= 0
                                      ? 'text-emerald-300'
                                      : 'text-rose-300'
                                  }
                                >
                                  {formatMoney(pattern.avg_pnl)} /{' '}
                                  {formatPct(pattern.avg_pnl_pct)}
                                </span>
                              </div>
                              <div>
                                Lift:{' '}
                                <span
                                  className={
                                    pattern.lift_avg_pnl_pct >= 0
                                      ? 'text-emerald-300'
                                      : 'text-rose-300'
                                  }
                                >
                                  {formatPct(pattern.lift_avg_pnl_pct)}
                                </span>
                              </div>
                              <div>
                                Confidence:{' '}
                                <span className="text-white">
                                  {formatPct((pattern.confidence_score || 0) * 100)}
                                </span>
                              </div>
                            </div>

                            <div className="grid grid-cols-2 md:grid-cols-5 gap-3 text-xs">
                              <div>
                                Train / holdout:{' '}
                                <span className="text-white">
                                  {pattern.training_sample_count || 0} /{' '}
                                  {pattern.validation_sample_count || 0}
                                </span>
                              </div>
                              <div>
                                Holdout support:{' '}
                                <span className="text-white">
                                  {pattern.validation_support_count || 0} /{' '}
                                  {pattern.validation_sample_count || 0}
                                </span>
                              </div>
                              <div>
                                Recent support:{' '}
                                <span className="text-white">
                                  {pattern.recent_support_count || 0} /{' '}
                                  {pattern.recent_sample_count || 0}
                                </span>
                              </div>
                              <div>
                                Drift:{' '}
                                <span
                                  className={
                                    (pattern.drift_score || 0) >= 0.5
                                      ? 'text-amber-300'
                                      : 'text-white'
                                  }
                                >
                                  {formatPct((pattern.drift_score || 0) * 100)}
                                </span>
                              </div>
                              <div>
                                Recency:{' '}
                                <span className="text-white">
                                  {formatPct((pattern.recency_weight || 0) * 100)}
                                </span>
                              </div>
                            </div>

                            <div className="grid grid-cols-2 md:grid-cols-5 gap-3 text-xs">
                              <div>
                                Stability:{' '}
                                <span className="text-white">
                                  {formatPct((pattern.stability_score || 0) * 100)}
                                </span>
                              </div>
                              <div>
                                Composite:{' '}
                                <span className="text-white">
                                  {formatPct((pattern.composite_score || 0) * 100)}
                                </span>
                              </div>
                              <div>
                                False-positive:{' '}
                                <span className="text-rose-300">
                                  {formatPct((pattern.false_positive_score || 0) * 100)}
                                </span>
                              </div>
                              <div>
                                Reverse-risk:{' '}
                                <span className="text-amber-300">
                                  {formatPct((pattern.reverse_risk_score || 0) * 100)}
                                </span>
                              </div>
                              <div>
                                Avg hold:{' '}
                                <span className="text-white">
                                  {formatHold(pattern.avg_hold_ms || 0)}
                                </span>
                              </div>
                            </div>

                            {pattern.evidence && pattern.evidence.length > 0 && (
                              <div className="space-y-2">
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                  Evidence
                                </div>
                                <div className="space-y-2">
                                  {pattern.evidence.map((evidence) => (
                                    <div
                                      key={`${pattern.id}-${evidence.case_id}`}
                                      className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 flex flex-wrap items-center justify-between gap-3"
                                    >
                                      <div className="text-xs text-nofx-text-muted">
                                        <span className="text-white">
                                          {evidence.symbol} · {evidence.side}
                                        </span>{' '}
                                        · {evidence.outcome || '-'} ·{' '}
                                        {formatMoney(evidence.realized_pnl)} /{' '}
                                        {formatPct(evidence.realized_pnl_pct)} ·{' '}
                                        {formatHold(evidence.hold_duration_ms || 0)}
                                      </div>
                                      <button
                                        onClick={() =>
                                          setSelectedCaseId(evidence.case_id)
                                        }
                                        className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs"
                                      >
                                        Focus evidence
                                      </button>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <div className="font-semibold">
                          {pickText('Similar historical deals', 'Aehnliche historische Deals')}
                        </div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          Semantic matches from the internal review corpus for this trader.
                        </div>
                      </div>
                    </div>

                    {similarCasesLoading ? (
                      <div className="text-sm text-nofx-text-muted">
                        Loading similar historical deals…
                      </div>
                    ) : similarCasesError ? (
                      <div className="text-sm text-amber-200">
                        {similarCasesError}
                      </div>
                    ) : similarCases.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        No similar historical deals were found yet.
                      </div>
                    ) : (
                      <div className="space-y-3">
                        {similarCases.map((hit) => (
                          <div
                            key={`${hit.document.id}-${hit.document.source_id}`}
                            className="rounded-xl border border-white/10 bg-black/20 p-4"
                          >
                            <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-3">
                              <div>
                                <div className="font-semibold">
                                  {hit.document.title || hit.document.source_id}
                                </div>
                                <div className="text-xs text-nofx-text-muted mt-1">
                                  {formatSemanticSimilarityScore(hit.similarity_score)} · updated{' '}
                                  {hit.document.source_updated_at
                                    ? new Date(hit.document.source_updated_at).toLocaleString()
                                    : '-'}
                                </div>
                              </div>
                              <button
                                onClick={() => setSelectedCaseId(hit.document.source_id)}
                                className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-sm"
                              >
                                Focus deal
                              </button>
                            </div>
                            <div className="text-sm text-nofx-text-muted mt-3">
                              {hit.document.summary || 'No summary stored'}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <div className="font-semibold">
                          {pickText('AI review assist', 'KI-Review-Hilfe')}
                        </div>
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
                        <div className="font-semibold">
                          {pickText('Analyst review', 'Analysten-Review')}
                        </div>
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
                    const closeExitOrigin = getDealReviewCloseExitOrigin(detail)
                    const closeExitReasonQuality =
                      getDealReviewCloseExitReasonQuality(detail)
                    const closeExitEvidenceSummary =
                      getDealReviewCloseExitEvidenceSummary(detail)
                    const closeExitEvidence = getDealReviewCloseExitEvidence(detail)

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
                            <div>
                              Exit origin:{' '}
                              <span className="text-white">
                                {formatMarketContextToken(closeExitOrigin)}
                              </span>
                            </div>
                            <div>
                              Evidence quality:{' '}
                              <span className="text-white">
                                {formatMarketContextToken(closeExitReasonQuality)}
                              </span>
                            </div>
                          </>
                        )}
                      </div>

                      {section.label === 'Close' &&
                        (closeExitEvidenceSummary || closeExitEvidence) && (
                          <div className="rounded-lg border border-amber-400/20 bg-amber-500/5 p-3 space-y-3">
                            <div className="text-xs uppercase tracking-[0.2em] text-amber-200/80">
                              Exit evidence
                            </div>
                            <div className="text-sm text-nofx-text-muted whitespace-pre-wrap">
                              {closeExitEvidenceSummary || 'No exit evidence summary stored.'}
                            </div>
                            {closeExitEvidence && (
                              <div className="grid grid-cols-2 gap-3 text-xs">
                                <div>
                                  Matched by:{' '}
                                  <span className="text-white">
                                    {formatMarketContextToken(
                                      closeExitEvidence.matched_by
                                    )}
                                  </span>
                                </div>
                                <div>
                                  Exchange order:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.exchange_order_id || '-'}
                                  </span>
                                </div>
                                <div>
                                  Client order:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.client_order_id || '-'}
                                  </span>
                                </div>
                                <div>
                                  Order type:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.order_type || '-'}
                                  </span>
                                </div>
                                <div>
                                  Venue order:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.venue_order_type || '-'}
                                  </span>
                                </div>
                                <div>
                                  Trigger subtype:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.trigger_subtype || '-'}
                                  </span>
                                </div>
                                <div>
                                  Trigger source:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.trigger_source || '-'}
                                  </span>
                                </div>
                                <div>
                                  Order action:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.order_action || '-'}
                                  </span>
                                </div>
                                <div>
                                  Order status:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.order_status || '-'}
                                  </span>
                                </div>
                                <div>
                                  Trigger price:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.trigger_price || '-'}
                                  </span>
                                </div>
                                <div>
                                  Fill price:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.fill_price || '-'}
                                  </span>
                                </div>
                                <div>
                                  Fill qty:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.fill_quantity || '-'}
                                  </span>
                                </div>
                                <div>
                                  Decision cycle:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.decision_cycle_number || '-'}
                                  </span>
                                </div>
                                <div>
                                  Stop-loss distance:{' '}
                                  <span className="text-white">
                                    {formatMarketContextValue(
                                      closeExitEvidence.stop_loss_distance_bps,
                                      'bps'
                                    )}
                                  </span>
                                </div>
                                <div>
                                  Take-profit distance:{' '}
                                  <span className="text-white">
                                    {formatMarketContextValue(
                                      closeExitEvidence.take_profit_distance_bps,
                                      'bps'
                                    )}
                                  </span>
                                </div>
                                <div>
                                  Trailing update:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.trailing_update_id || '-'}
                                  </span>
                                </div>
                                <div>
                                  Previous stop:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.previous_stop_price || '-'}
                                  </span>
                                </div>
                                <div>
                                  New stop:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.new_stop_price || '-'}
                                  </span>
                                </div>
                                <div>
                                  Trailing mode:{' '}
                                  <span className="text-white">
                                    {formatMarketContextToken(
                                      closeExitEvidence.trailing_mode
                                    )}
                                  </span>
                                </div>
                                <div>
                                  Trailing tier:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.trailing_tier_index ?? '-'}
                                  </span>
                                </div>
                                <div>
                                  Trailing updated:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.trailing_updated_at_ms
                                      ? formatDate(
                                          closeExitEvidence.trailing_updated_at_ms
                                        )
                                      : '-'}
                                  </span>
                                </div>
                                <div>
                                  Tier trigger:{' '}
                                  <span className="text-white">
                                    {formatMarketContextValue(
                                      closeExitEvidence.trailing_trigger_profit_pct,
                                      'pct'
                                    )}
                                  </span>
                                </div>
                                <div>
                                  Locked profit:{' '}
                                  <span className="text-white">
                                    {formatMarketContextValue(
                                      closeExitEvidence.trailing_lock_profit_pct,
                                      'pct'
                                    )}
                                  </span>
                                </div>
                                <div>
                                  Trail offset:{' '}
                                  <span className="text-white">
                                    {formatMarketContextValue(
                                      closeExitEvidence.trailing_offset_pct,
                                      'pct'
                                    )}
                                  </span>
                                </div>
                                <div>
                                  Intent ID:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.intent_id || '-'}
                                  </span>
                                </div>
                                <div>
                                  Intent type:{' '}
                                  <span className="text-white">
                                    {formatMarketContextToken(
                                      closeExitEvidence.intent_type
                                    )}
                                  </span>
                                </div>
                                <div>
                                  Intent source:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.intent_source_module || '-'}
                                  </span>
                                </div>
                                <div>
                                  Intent confidence:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.intent_confidence || '-'}
                                  </span>
                                </div>
                                <div>
                                  Intent created:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.intent_created_at_ms
                                      ? formatDate(
                                          closeExitEvidence.intent_created_at_ms
                                        )
                                      : '-'}
                                  </span>
                                </div>
                                <div className="col-span-2">
                                  Intent summary:{' '}
                                  <span className="text-white">
                                    {closeExitEvidence.intent_summary || '-'}
                                  </span>
                                </div>
                                <div className="col-span-2">
                                  Intent reasoning:{' '}
                                  <span className="text-white whitespace-pre-wrap">
                                    {closeExitEvidence.intent_reasoning || '-'}
                                  </span>
                                </div>
                              </div>
                            )}
                          </div>
                        )}

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
                  <h2 className="font-semibold text-lg">
                    {pickText('Anomaly scan', 'Anomalie-Scan')}
                  </h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    {pickText(
                      'Heuristic hotspots from the currently filtered closed deals.',
                      'Heuristische Hotspots aus den aktuell gefilterten geschlossenen Deals.'
                    )}
                  </p>
                </div>
                <div className="text-xs text-nofx-text-muted">
                  {anomalies?.closed_deals || 0} {pickText('closed deals', 'geschlossene Deals')}
                </div>
              </div>

              {!anomalies ? (
                <div className="text-sm text-nofx-text-muted mt-4">
                  {pickText('No anomaly data available.', 'Keine Anomaliedaten verfuegbar.')}
                </div>
              ) : (
                <div className="space-y-4 mt-4">
                  <div>
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                      {pickText('Learned symbol priors', 'Gelernte Symbol-Priors')}
                    </div>
                    <div className="space-y-3">
                      {(symbolBehaviorLiveGuardStatus ||
                        symbolBehaviorLiveGuardEvents.length > 0) && (
                        <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 space-y-3">
                          <div className="flex flex-wrap items-start justify-between gap-3">
                            <div>
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                {pickText('Live guard', 'Live-Guard')}
                              </div>
                              <div className="text-sm mt-1">
                                {symbolBehaviorLiveGuardStatus?.strategy_name ||
                                  pickText('Strategy config', 'Strategie-Konfiguration')}
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                {symbolBehaviorLiveGuardStatus?.config.enabled
                                  ? pickText(
                                      'Live symbol-prior gating is active for this trader.',
                                      'Das Live-Gating fuer Symbol-Priors ist fuer diesen Trader aktiv.'
                                    )
                                  : pickText(
                                      'Live symbol-prior gating is currently disabled for this trader.',
                                      'Das Live-Gating fuer Symbol-Priors ist fuer diesen Trader derzeit deaktiviert.'
                                    )}
                              </div>
                            </div>
                            <div className="flex flex-wrap items-center gap-2">
                              <span
                                className={`px-2 py-1 rounded-full text-[11px] border ${
                                  symbolBehaviorLiveGuardStatus?.config.enabled
                                    ? 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
                                    : 'border-white/10 bg-white/5 text-white'
                                }`}
                              >
                                {symbolBehaviorLiveGuardStatus?.config.enabled
                                  ? pickText('Enabled', 'Aktiviert')
                                  : pickText('Disabled', 'Deaktiviert')}
                              </span>
                              {symbolBehaviorLiveGuardStatus?.config.mode && (
                                <span className="px-2 py-1 rounded-full text-[11px] border border-sky-400/20 bg-sky-500/10 text-sky-200">
                                  {formatSymbolBehaviorLiveGuardMode(
                                    symbolBehaviorLiveGuardStatus.config.mode,
                                    language
                                  )}
                                </span>
                              )}
                            </div>
                          </div>

                          {symbolBehaviorLiveGuardSummary && (
                            <div className="grid grid-cols-2 xl:grid-cols-5 gap-3 text-xs">
                              <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3">
                                <div className="text-nofx-text-muted">{pickText('Recent checks', 'Juengste Pruefungen')}</div>
                                <div className="text-lg font-semibold mt-1">
                                  {symbolBehaviorLiveGuardSummary.total_visible || 0}
                                </div>
                              </div>
                              <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3">
                                <div className="text-nofx-text-muted">{pickText('Hard blocked', 'Hart blockiert')}</div>
                                <div className="text-lg font-semibold mt-1 text-rose-300">
                                  {symbolBehaviorLiveGuardSummary.hard_blocked_count || 0}
                                </div>
                              </div>
                              <div className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-3">
                                <div className="text-nofx-text-muted">{pickText('Monitor only', 'Nur beobachten')}</div>
                                <div className="text-lg font-semibold mt-1 text-amber-200">
                                  {symbolBehaviorLiveGuardSummary.monitor_only_count || 0}
                                </div>
                              </div>
                              <div className="rounded-lg border border-sky-400/15 bg-sky-500/5 px-3 py-3">
                                <div className="text-nofx-text-muted">
                                  Matched, not qualified
                                </div>
                                <div className="text-lg font-semibold mt-1 text-sky-200">
                                  {symbolBehaviorLiveGuardSummary.matched_unqualified_count ||
                                    0}
                                </div>
                              </div>
                              <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-3">
                                <div className="text-nofx-text-muted">
                                  {pickText('Last check', 'Letzte Pruefung')}
                                </div>
                                <div className="text-xs font-medium mt-1">
                                  {formatTimestampLabel(
                                    symbolBehaviorLiveGuardSummary.latest_decision_timestamp
                                  )}
                                </div>
                              </div>
                            </div>
                          )}

                          {symbolBehaviorLiveGuardStatus?.config.enabled && (
                            <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs text-nofx-text-muted">
                              Thresholds: confidence ≥{' '}
                              {formatPct(
                                (symbolBehaviorLiveGuardStatus.config
                                  .min_confidence_score || 0) * 100
                              )}{' '}
                              · samples ≥{' '}
                              {symbolBehaviorLiveGuardStatus.config.min_sample_count || 0}{' '}
                              · match ≥{' '}
                              {formatPct(
                                (symbolBehaviorLiveGuardStatus.config.min_match_score || 0) *
                                  100
                              )}{' '}
                              · contradiction ≥{' '}
                              {formatPct(
                                (symbolBehaviorLiveGuardStatus.config
                                  .min_contradiction_score || 0) * 100
                              )}{' '}
                              · FP ≤{' '}
                              {formatPct(
                                (symbolBehaviorLiveGuardStatus.config
                                  .max_false_positive_score || 0) * 100
                              )}{' '}
                              · drift ≤{' '}
                              {formatPct(
                                (symbolBehaviorLiveGuardStatus.config.max_drift_score || 0) *
                                  100
                              )}
                            </div>
                          )}

                          {symbolBehaviorLiveGuardEvents.length === 0 ? (
                            <div className="text-sm text-nofx-text-muted">
                              No live guard evaluations recorded yet for this trader
                              slice. The panel will populate once the trader reaches
                              new open-decision checks.
                            </div>
                          ) : (
                            <div className="space-y-2">
                              {symbolBehaviorLiveGuardEvents.map((event) => (
                                <div
                                  key={event.id}
                                  className={`rounded-lg border bg-black/20 px-3 py-3 ${
                                    event.matched_prior_id &&
                                    selectedSymbolPriorId === event.matched_prior_id
                                      ? 'border-nofx-gold/40 ring-1 ring-nofx-gold/20'
                                      : 'border-white/10'
                                  }`}
                                >
                                  <div className="flex flex-wrap items-start justify-between gap-3">
                                    <div>
                                      <div className="flex flex-wrap items-center gap-2">
                                        <div className="font-medium">
                                          {event.symbol} · {event.side}
                                        </div>
                                        <span
                                          className={`px-2 py-1 rounded-full text-[11px] border ${symbolBehaviorLiveGuardEffectClasses(
                                            event.effect
                                          )}`}
                                        >
                                          {formatSymbolBehaviorLiveGuardEffect(event.effect)}
                                        </span>
                                        {event.policy_mode && (
                                          <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                            {formatSymbolBehaviorLiveGuardMode(
                                              event.policy_mode
                                            )}
                                          </span>
                                        )}
                                      </div>
                                      <div className="text-xs text-nofx-text-muted mt-1">
                                        {event.action || 'open'} · cycle{' '}
                                        {event.cycle_number || '-'} ·{' '}
                                        {event.selection_bucket || 'unknown'} ·{' '}
                                        {event.trend_regime || 'unknown'} /{' '}
                                        {event.volatility_regime || 'unknown'} /{' '}
                                        {event.oi_regime || 'unknown'}
                                      </div>
                                    </div>
                                    <div className="text-right text-xs text-nofx-text-muted">
                                      <div>{formatTimestampLabel(event.decision_timestamp)}</div>
                                      <div className="mt-1">
                                        match {formatPct((event.match_score || 0) * 100)}
                                      </div>
                                      <div className="mt-1">
                                        conf {event.decision_confidence || 0}%
                                      </div>
                                    </div>
                                  </div>

                                  <div className="text-sm text-nofx-text-muted mt-2">
                                    {event.summary || event.block_reason || 'No summary stored'}
                                  </div>

                                  {event.matched_prior_key && (
                                    <div className="text-xs text-nofx-text-muted mt-2">
                                      Prior: {event.matched_prior_key}
                                      {event.matched_prior_validation_label
                                        ? ` · ${formatSymbolBehaviorValidationLabel(
                                            event.matched_prior_validation_label
                                          )}`
                                        : ''}
                                    </div>
                                  )}

                                  <div className="flex flex-wrap gap-2 mt-3">
                                    {event.matched_prior_id && (
                                      <button
                                        onClick={() =>
                                          setSelectedSymbolPriorId(event.matched_prior_id || null)
                                        }
                                        className="px-2.5 py-1 rounded-lg border border-white/10 bg-white/5 text-xs font-medium"
                                      >
                                        Focus prior
                                      </button>
                                    )}
                                    <button
                                      onClick={() =>
                                        applyDrilldownFilters(
                                          {
                                            symbol: event.symbol,
                                            side: event.side,
                                          },
                                          `Filtered review to live guard event ${event.symbol} ${event.side}.`
                                        )
                                      }
                                      className="px-2.5 py-1 rounded-lg border border-white/10 bg-white/5 text-xs font-medium"
                                    >
                                      Filter deals
                                    </button>
                                  </div>
                                </div>
                              ))}
                            </div>
                          )}
                        </div>
                      )}

                      {symbolBehaviorPriorSummary && (
                        <div className="grid grid-cols-2 xl:grid-cols-5 gap-3 text-xs">
                          <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Tracked priors', 'Verfolgte Priors')}
                            </div>
                            <div className="text-lg font-semibold mt-1">
                              {symbolBehaviorPriorSummary.total_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-emerald-400/15 bg-emerald-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Confirmed', 'Bestaetigt')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-emerald-300">
                              {symbolBehaviorPriorSummary.confirmed_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('False positives', 'Falsch positive')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-rose-300">
                              {symbolBehaviorPriorSummary.false_positive_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Reverse-edge risk', 'Umkehr-Vorteils-Risiko')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-amber-300">
                              {symbolBehaviorPriorSummary.false_negative_risk_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-orange-400/15 bg-orange-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Drifting', 'Abdriftend')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-orange-300">
                              {symbolBehaviorPriorSummary.drifting_count || 0}
                            </div>
                          </div>
                        </div>
                      )}

                      {symbolBehaviorPriorSummary?.notes &&
                        symbolBehaviorPriorSummary.notes.length > 0 && (
                          <div className="space-y-2">
                            {symbolBehaviorPriorSummary.notes.map((note) => (
                              <div
                                key={`prior-note-${note}`}
                                className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-2 text-xs text-amber-100"
                              >
                                {note}
                              </div>
                            ))}
                          </div>
                        )}

                      {symbolBehaviorPriorSummary?.top_signal_clusters &&
                        symbolBehaviorPriorSummary.top_signal_clusters.length >
                          0 && (
                          <div className="rounded-lg border border-emerald-400/15 bg-emerald-500/5 px-3 py-3 space-y-3">
                            <div className="flex flex-wrap items-center justify-between gap-3">
                              <div className="text-xs uppercase tracking-[0.2em] text-emerald-200">
                                Normalized Signal Clusters
                              </div>
                              {symbolBehaviorClusterFilter && (
                                <button
                                  onClick={() => setSymbolBehaviorClusterFilter('')}
                                  className="px-2.5 py-1 rounded-lg border border-white/10 bg-black/20 text-xs font-medium"
                                >
                                  Clear cluster filter
                                </button>
                              )}
                            </div>
                            <div className="space-y-2">
                              {symbolBehaviorPriorSummary.top_signal_clusters.map(
                                (cluster) => (
                                  <button
                                    key={`cluster-rollup-${cluster.cluster_label}`}
                                    onClick={() => {
                                      setSymbolBehaviorClusterFilter(
                                        cluster.cluster_label
                                      )
                                      setSymbolBehaviorPriorTab('all')
                                      setSelectedSymbolPriorId(null)
                                    }}
                                    className={`w-full text-left rounded-lg border px-3 py-3 flex items-start justify-between gap-4 ${
                                      symbolBehaviorClusterFilter ===
                                      cluster.cluster_label
                                        ? 'border-emerald-300/40 bg-emerald-500/10 ring-1 ring-emerald-300/20'
                                        : 'border-white/10 bg-black/20'
                                    }`}
                                  >
                                    <div>
                                      <div className="flex flex-wrap items-center gap-2">
                                        <span className="font-medium">
                                          {cluster.cluster_label}
                                        </span>
                                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                          {cluster.prior_count} priors
                                        </span>
                                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                          {cluster.symbol_count} symbols
                                        </span>
                                      </div>
                                      <div className="text-xs text-nofx-text-muted mt-1">
                                        {cluster.sample_count} deals ·{' '}
                                        {cluster.decision_open_count} decision
                                        opens · confirmed {cluster.confirmed_count}{' '}
                                        · FP {cluster.false_positive_count} · FN{' '}
                                        {cluster.false_negative_risk_count} ·
                                        drifting {cluster.drifting_count}
                                      </div>
                                      {cluster.top_symbols &&
                                        cluster.top_symbols.length > 0 && (
                                          <div className="flex flex-wrap gap-2 mt-2">
                                            {cluster.top_symbols.map((symbol) => (
                                              <span
                                                key={`${cluster.cluster_label}-${symbol}`}
                                                className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white"
                                              >
                                                {symbol}
                                              </span>
                                            ))}
                                          </div>
                                        )}
                                    </div>
                                    <div className="text-right text-xs text-nofx-text-muted">
                                      <div>
                                        avg pnl{' '}
                                        <span
                                          className={
                                            (cluster.avg_pnl_pct || 0) >= 0
                                              ? 'text-emerald-300'
                                              : 'text-rose-300'
                                          }
                                        >
                                          {formatPct(cluster.avg_pnl_pct || 0)}
                                        </span>
                                      </div>
                                      <div className="mt-1">
                                        contradiction{' '}
                                        {formatPct(
                                          (cluster.avg_contradiction_score || 0) *
                                            100
                                        )}
                                      </div>
                                      <div className="mt-1">
                                        support{' '}
                                        {formatPct(
                                          (cluster.avg_validation_support_score ||
                                            0) * 100
                                        )}
                                      </div>
                                    </div>
                                  </button>
                                )
                              )}
                            </div>
                          </div>
                        )}

                      {symbolBehaviorPriorSummary?.top_false_positives &&
                        symbolBehaviorPriorSummary.top_false_positives.length >
                          0 && (
                          <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3 space-y-2">
                            <div className="text-xs uppercase tracking-[0.2em] text-rose-200">
                              Top False Positives
                            </div>
                            <div className="space-y-2">
                              {symbolBehaviorPriorSummary.top_false_positives.map(
                                (prior) => (
                                  <button
                                    key={`fp-${prior.id}`}
                                    onClick={() => {
                                      setSymbolBehaviorPriorTab('false_positive')
                                      setSelectedSymbolPriorId(prior.id)
                                    }}
                                    className="w-full text-left rounded-lg border border-white/10 bg-black/20 px-3 py-2 flex items-start justify-between gap-3"
                                  >
                                    <div>
                                      <div className="font-medium">
                                        {prior.symbol} · {prior.side}
                                      </div>
                                      <div className="text-xs text-nofx-text-muted mt-1">
                                        {prior.validation_alert ||
                                          prior.validation_summary}
                                      </div>
                                    </div>
                                    <div className="text-right text-xs text-rose-200">
                                      FP {formatPct((prior.false_positive_score || 0) * 100)}
                                    </div>
                                  </button>
                                )
                              )}
                            </div>
                          </div>
                        )}

                      {symbolBehaviorPriorSummary?.top_false_negative_risks &&
                        symbolBehaviorPriorSummary.top_false_negative_risks
                          .length > 0 && (
                          <div className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-3 space-y-2">
                            <div className="text-xs uppercase tracking-[0.2em] text-amber-200">
                              Top Reverse-Edge Risks
                            </div>
                            <div className="space-y-2">
                              {symbolBehaviorPriorSummary.top_false_negative_risks.map(
                                (prior) => (
                                  <button
                                    key={`fn-${prior.id}`}
                                    onClick={() => {
                                      setSymbolBehaviorPriorTab('false_negative_risk')
                                      setSelectedSymbolPriorId(prior.id)
                                    }}
                                    className="w-full text-left rounded-lg border border-white/10 bg-black/20 px-3 py-2 flex items-start justify-between gap-3"
                                  >
                                    <div>
                                      <div className="font-medium">
                                        {prior.symbol} · {prior.side}
                                      </div>
                                      <div className="text-xs text-nofx-text-muted mt-1">
                                        {prior.validation_alert ||
                                          prior.validation_summary}
                                      </div>
                                    </div>
                                    <div className="text-right text-xs text-amber-200">
                                      FN {formatPct((prior.false_negative_score || 0) * 100)}
                                    </div>
                                  </button>
                                )
                              )}
                            </div>
                          </div>
                        )}

                      <div className="space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          {symbolBehaviorPriorTabs.map((tab) => (
                            <button
                              key={`symbol-prior-tab-${tab.value}`}
                              onClick={() => setSymbolBehaviorPriorTab(tab.value)}
                              className={symbolBehaviorTabClasses(
                                tab.value,
                                symbolBehaviorPriorTab === tab.value
                              )}
                            >
                              <span>{tab.label}</span>
                              <span className="rounded-full bg-black/20 px-2 py-0.5 text-[11px]">
                                {tab.count}
                              </span>
                            </button>
                          ))}
                        </div>
                        <div className="text-xs text-nofx-text-muted">
                          {symbolBehaviorClusterFilter
                            ? `Cluster filter active: ${symbolBehaviorClusterFilter}. `
                            : ''}
                          {symbolBehaviorPriorTab === 'all'
                            ? `Showing ${visibleSymbolBehaviorPriors.length} of ${symbolBehaviorPriorTabCounts.all} priors in this trader slice.`
                            : `Showing ${visibleSymbolBehaviorPriors.length} of ${
                                symbolBehaviorPriorTabCounts[symbolBehaviorPriorTab]
                              } ${formatSymbolBehaviorValidationLabel(
                                symbolBehaviorPriorTab
                              ).toLowerCase()} priors.`}
                        </div>
                      </div>

                      {symbolBehaviorPriors.length === 0 ? (
                        <div className="text-sm text-nofx-text-muted">
                          No symbol priors available yet for this trader slice.
                        </div>
                      ) : visibleSymbolBehaviorPriors.length === 0 ? (
                        <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 text-sm text-nofx-text-muted flex items-center justify-between gap-3">
                          <div>
                            No{' '}
                            {formatSymbolBehaviorValidationLabel(
                              symbolBehaviorPriorTab
                            ).toLowerCase()}{' '}
                            priors matched the current trader slice.
                          </div>
                          <button
                            onClick={() => setSymbolBehaviorPriorTab('all')}
                            className="px-2.5 py-1 rounded-lg border border-white/10 bg-white/5 text-xs font-medium"
                          >
                            Show all
                          </button>
                        </div>
                      ) : (
                        visibleSymbolBehaviorPriors.map((prior) => (
                          <div
                            key={prior.id}
                            className={`rounded-lg border bg-black/20 px-3 py-3 flex items-start justify-between gap-4 ${
                              selectedSymbolPriorId === prior.id
                                ? 'border-nofx-gold/40 ring-1 ring-nofx-gold/20'
                                : 'border-white/10'
                            }`}
                          >
                            <div>
                              <div className="flex flex-wrap items-center gap-2">
                                <div className="font-medium">
                                  {prior.symbol} · {prior.side}
                                </div>
                                <span
                                  className={`px-2 py-1 rounded-full text-[11px] border ${symbolBehaviorToneClasses(
                                    prior.behavior_bias
                                  )}`}
                                >
                                  {prior.behavior_bias || 'mixed'}
                                </span>
                                <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                  {prior.status}
                                </span>
                                <span
                                  className={`px-2 py-1 rounded-full text-[11px] border ${symbolBehaviorValidationClasses(
                                    prior.validation_label
                                  )}`}
                                >
                                  {formatSymbolBehaviorValidationLabel(
                                    prior.validation_label
                                  )}
                                </span>
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                {prior.regime_signature}
                              </div>
                              <div className="text-sm text-nofx-text-muted mt-2">
                                {prior.summary}
                              </div>
                              {prior.validation_alert && (
                                <div className="text-xs text-nofx-text-muted mt-2">
                                  {prior.validation_alert}
                                </div>
                              )}
                              <div className="text-xs text-nofx-text-muted mt-2">
                                {prior.decision_open_count || 0} opens across{' '}
                                {prior.decision_cycle_count || 0} cycles · avg
                                decision conf{' '}
                                {formatPct(prior.avg_decision_confidence || 0)} ·
                                contradiction{' '}
                                {formatPct((prior.contradiction_score || 0) * 100)}
                              </div>
                              {prior.signal_cluster_key && (
                                <div className="text-xs text-nofx-text-muted mt-2">
                                  Normalized cluster:{' '}
                                  <span className="text-white">
                                    {prior.signal_cluster_key}
                                  </span>
                                </div>
                              )}
                              {prior.signal_clusters &&
                                prior.signal_clusters.length > 0 && (
                                  <div className="flex flex-wrap gap-2 mt-2">
                                    {prior.signal_clusters
                                      .slice(0, 6)
                                      .map((cluster) => (
                                        <span
                                          key={`${prior.id}-top-cluster-${cluster}`}
                                          className="px-2 py-1 rounded-full text-[11px] border border-emerald-400/20 bg-emerald-500/10 text-emerald-200"
                                        >
                                          {cluster}
                                        </span>
                                      ))}
                                  </div>
                                )}
                              {prior.signal_tags && prior.signal_tags.length > 0 && (
                                <div className="flex flex-wrap gap-2 mt-2">
                                  {prior.signal_tags.slice(0, 5).map((tag) => (
                                    <span
                                      key={`${prior.id}-top-signal-${tag}`}
                                      className="px-2 py-1 rounded-full text-[11px] border border-sky-400/20 bg-sky-500/10 text-sky-200"
                                    >
                                      {tag}
                                    </span>
                                  ))}
                                </div>
                              )}
                              <div className="flex flex-wrap gap-2 mt-3">
                                <button
                                  onClick={() =>
                                    applyDrilldownFilters(
                                      {
                                        symbol: prior.symbol,
                                        side: prior.side,
                                        status: 'CLOSED',
                                        open_selection_bucket:
                                          prior.open_selection_bucket || undefined,
                                        open_trend_regime:
                                          prior.open_trend_regime || undefined,
                                        open_volatility_regime:
                                          prior.open_volatility_regime || undefined,
                                        open_oi_regime:
                                          prior.open_oi_regime || undefined,
                                      },
                                      `Filtered deals to learned prior ${prior.symbol} ${prior.side}.`
                                    )
                                  }
                                  className="px-2.5 py-1 rounded-lg border border-white/10 bg-white/5 text-xs font-medium"
                                >
                                  Filter
                                </button>
                                <button
                                  onClick={() =>
                                    void runAIScan(
                                      {
                                        symbol: prior.symbol,
                                        side: prior.side,
                                        status: 'CLOSED',
                                        open_selection_bucket:
                                          prior.open_selection_bucket || undefined,
                                        open_trend_regime:
                                          prior.open_trend_regime || undefined,
                                        open_volatility_regime:
                                          prior.open_volatility_regime || undefined,
                                        open_oi_regime:
                                          prior.open_oi_regime || undefined,
                                      },
                                      `AI scan started for learned prior ${prior.symbol} ${prior.side}.`
                                    )
                                  }
                                  disabled={
                                    !selectedTraderId ||
                                    runningScan ||
                                    items.length === 0
                                  }
                                  className="px-2.5 py-1 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 text-nofx-gold text-xs font-medium disabled:opacity-50"
                                >
                                  Scan this prior
                                </button>
                              </div>
                            </div>
                            <div className="text-right text-sm">
                              <div
                                className={
                                  prior.avg_pnl >= 0
                                    ? 'font-semibold text-emerald-400'
                                    : 'font-semibold text-rose-400'
                                }
                              >
                                {formatMoney(prior.avg_pnl)} /{' '}
                                {formatPct(prior.avg_pnl_pct)}
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-2">
                                {prior.sample_count} deals · {formatPct(prior.win_rate * 100)} win
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                Holdout {prior.validation_support_count || 0}/
                                {prior.validation_sample_count || 0} · drift{' '}
                                {formatPct((prior.drift_score || 0) * 100)}
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                FP {formatPct((prior.false_positive_score || 0) * 100)} · FN{' '}
                                {formatPct((prior.false_negative_score || 0) * 100)}
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                {formatSymbolBehaviorAction(prior.recommended_action)}
                              </div>
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  </div>

                  <div>
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                      Learned patterns
                    </div>
                    {!learnedPatternSummary ? (
                      <div className="text-sm text-nofx-text-muted">
                        No learned pattern data available yet for this trader slice.
                      </div>
                    ) : (
                      <div className="space-y-3">
                        <div className="grid grid-cols-2 xl:grid-cols-5 gap-3 text-xs">
                          <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Tracked patterns', 'Verfolgte Muster')}
                            </div>
                            <div className="text-lg font-semibold mt-1">
                              {learnedPatternSummary.total_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-emerald-400/15 bg-emerald-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Positive edges', 'Positive Vorteile')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-emerald-300">
                              {learnedPatternSummary.positive_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Anti-edges', 'Anti-Muster')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-rose-300">
                              {learnedPatternSummary.negative_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-sky-400/15 bg-sky-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Confirmed', 'Bestaetigt')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-sky-200">
                              {learnedPatternSummary.confirmed_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Reverse / drifting', 'Umkehr / Drift')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-amber-200">
                              {(learnedPatternSummary.reverse_risk_count || 0) +
                                (learnedPatternSummary.drifting_count || 0)}
                            </div>
                          </div>
                        </div>

                        <div className="grid grid-cols-2 xl:grid-cols-6 gap-3 text-xs">
                          <div className="rounded-lg border border-sky-400/15 bg-sky-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Monitoring rules', 'Monitoring-Regeln')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-sky-200">
                              {learnedPatternSummary.monitoring_rule_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-orange-400/15 bg-orange-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Degrading / rollback', 'Verschlechterung / Rollback')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-orange-200">
                              {(learnedPatternSummary.lifecycle_degrading_count || 0) +
                                (learnedPatternSummary.lifecycle_rollback_watch_count || 0)}
                            </div>
                          </div>
                          <div className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Fragile live rules', 'Fragile Live-Regeln')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-amber-200">
                              {learnedPatternSummary.lifecycle_fragile_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Lagging guard trails', 'Nachlaufende Guard-Spuren')}
                            </div>
                            <div className="text-lg font-semibold mt-1">
                              {learnedPatternSummary.lifecycle_lagging_guard_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-emerald-400/15 bg-emerald-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Protective live rules', 'Schuetzende Live-Regeln')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-emerald-300">
                              {learnedPatternSummary.live_guard_protective_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Overblocking live rules', 'Ueberblockierende Live-Regeln')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-rose-300">
                              {learnedPatternSummary.live_guard_overblocking_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-emerald-400/15 bg-emerald-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Improving live rules', 'Verbessernde Live-Regeln')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-emerald-300">
                              {learnedPatternSummary.live_guard_improving_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Degrading live rules', 'Verschlechternde Live-Regeln')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-amber-200">
                              {learnedPatternSummary.live_guard_degrading_count || 0}
                            </div>
                          </div>
                          <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3">
                            <div className="text-nofx-text-muted">
                              {pickText('Newly overblocking', 'Neu ueberblockierend')}
                            </div>
                            <div className="text-lg font-semibold mt-1 text-rose-300">
                              {learnedPatternSummary.live_guard_newly_overblocking_count || 0}
                            </div>
                          </div>
                        </div>

                        {learnedPatternSummary.notes &&
                          learnedPatternSummary.notes.length > 0 && (
                            <div className="space-y-2">
                              {learnedPatternSummary.notes.map((note) => (
                                <div
                                  key={`learned-pattern-note-${note}`}
                                  className="rounded-lg border border-sky-400/15 bg-sky-500/5 px-3 py-2 text-xs text-sky-100"
                                >
                                  {note}
                                </div>
                              ))}
                            </div>
                          )}

                        {(learnedPatternLiveGuardStatus ||
                          learnedPatternLiveGuardEvents.length > 0) && (
                          <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 space-y-3">
                            <div className="flex flex-wrap items-start justify-between gap-3">
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                  Live guard
                                </div>
                                <div className="text-sm mt-1">
                                  {learnedPatternLiveGuardStatus?.strategy_name ||
                                    'Strategy config'}
                                </div>
                                <div className="text-xs text-nofx-text-muted mt-1">
                                  {learnedPatternLiveGuardStatus?.config.enabled
                                    ? 'Live learned-pattern gating is active for this trader.'
                                    : 'Live learned-pattern gating is currently disabled for this trader.'}
                                </div>
                              </div>
                              <div className="flex flex-wrap items-center gap-2">
                                <span
                                  className={`px-2 py-1 rounded-full text-[11px] border ${
                                    learnedPatternLiveGuardStatus?.config.enabled
                                      ? 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
                                      : 'border-white/10 bg-white/5 text-white'
                                  }`}
                                >
                                  {learnedPatternLiveGuardStatus?.config.enabled
                                    ? 'Enabled'
                                    : 'Disabled'}
                                </span>
                                {learnedPatternLiveGuardStatus?.config.mode && (
                                  <span className="px-2 py-1 rounded-full text-[11px] border border-sky-400/20 bg-sky-500/10 text-sky-200">
                                    {formatSymbolBehaviorLiveGuardMode(
                                      learnedPatternLiveGuardStatus.config.mode
                                    )}
                                  </span>
                                )}
                              </div>
                            </div>

                            {learnedPatternLiveGuardSummary && (
                              <div className="space-y-3">
                                <div className="grid grid-cols-2 xl:grid-cols-5 gap-3 text-xs">
                                  <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3">
                                    <div className="text-nofx-text-muted">
                                      {pickText('Recent checks', 'Juengste Pruefungen')}
                                    </div>
                                    <div className="text-lg font-semibold mt-1">
                                      {learnedPatternLiveGuardSummary.total_visible || 0}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3">
                                    <div className="text-nofx-text-muted">
                                      {pickText('Hard blocked', 'Hart blockiert')}
                                    </div>
                                    <div className="text-lg font-semibold mt-1 text-rose-300">
                                      {learnedPatternLiveGuardSummary.hard_blocked_count || 0}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-3">
                                    <div className="text-nofx-text-muted">
                                      {pickText('Monitor only', 'Nur beobachten')}
                                    </div>
                                    <div className="text-lg font-semibold mt-1 text-amber-200">
                                      {learnedPatternLiveGuardSummary.monitor_only_count || 0}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-sky-400/15 bg-sky-500/5 px-3 py-3">
                                    <div className="text-nofx-text-muted">
                                      Matched, not qualified
                                    </div>
                                    <div className="text-lg font-semibold mt-1 text-sky-200">
                                      {learnedPatternLiveGuardSummary.matched_unqualified_count ||
                                        0}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-3">
                                    <div className="text-nofx-text-muted">
                                      {pickText('Last check', 'Letzte Pruefung')}
                                    </div>
                                    <div className="text-xs font-medium mt-1">
                                      {formatTimestampLabel(
                                        learnedPatternLiveGuardSummary.latest_decision_timestamp
                                      )}
                                    </div>
                                  </div>
                                </div>

                                <div className="grid grid-cols-2 xl:grid-cols-5 gap-3 text-xs">
                                  <div className="rounded-lg border border-emerald-400/15 bg-emerald-500/5 px-3 py-3">
                                    <div className="text-nofx-text-muted">
                                      {pickText('Correctly blocked', 'Korrekt blockiert')}
                                    </div>
                                    <div className="text-lg font-semibold mt-1 text-emerald-200">
                                      {learnedPatternLiveGuardSummary.correctly_blocked_count || 0}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3">
                                    <div className="text-nofx-text-muted">
                                      {pickText('Overblocked', 'Ueberblockiert')}
                                    </div>
                                    <div className="text-lg font-semibold mt-1 text-rose-300">
                                      {learnedPatternLiveGuardSummary.overblocked_count || 0}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-emerald-400/15 bg-emerald-500/5 px-3 py-3">
                                    <div className="text-nofx-text-muted">
                                      {pickText('Warning confirmed', 'Warnung bestaetigt')}
                                    </div>
                                    <div className="text-lg font-semibold mt-1 text-emerald-200">
                                      {learnedPatternLiveGuardSummary.warning_confirmed_count || 0}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3">
                                    <div className="text-nofx-text-muted">
                                      {pickText('Warning not confirmed', 'Warnung nicht bestaetigt')}
                                    </div>
                                    <div className="text-lg font-semibold mt-1 text-rose-300">
                                      {learnedPatternLiveGuardSummary.warning_not_confirmed_count ||
                                        0}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-3">
                                    <div className="text-nofx-text-muted">
                                      {pickText('Pending / open', 'Ausstehend / offen')}
                                    </div>
                                    <div className="text-lg font-semibold mt-1">
                                      {(learnedPatternLiveGuardSummary.attribution_pending_count ||
                                        0) +
                                        (learnedPatternLiveGuardSummary.followup_open_count || 0)}
                                    </div>
                                  </div>
                                </div>

                                {(learnedPatternLiveGuardSummary.threshold_missed_loss_count > 0 ||
                                  learnedPatternLiveGuardSummary.threshold_missed_profit_count >
                                    0) && (
                                  <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs text-nofx-text-muted">
                                    Threshold misses: loss{' '}
                                    {learnedPatternLiveGuardSummary.threshold_missed_loss_count ||
                                      0}{' '}
                                    · profit{' '}
                                    {learnedPatternLiveGuardSummary.threshold_missed_profit_count ||
                                      0}
                                  </div>
                                )}
                              </div>
                            )}

                            {learnedPatternLiveGuardStatus?.config.enabled && (
                              <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs text-nofx-text-muted">
                                Thresholds: composite ≥{' '}
                                {formatPct(
                                  (learnedPatternLiveGuardStatus.config
                                    .min_composite_score || 0) * 100
                                )}{' '}
                                · confidence ≥{' '}
                                {formatPct(
                                  (learnedPatternLiveGuardStatus.config
                                    .min_confidence_score || 0) * 100
                                )}{' '}
                                · validation support ≥{' '}
                                {formatPct(
                                  (learnedPatternLiveGuardStatus.config
                                    .min_validation_support_score || 0) * 100
                                )}{' '}
                                · samples ≥{' '}
                                {learnedPatternLiveGuardStatus.config.min_sample_count ||
                                  0}{' '}
                                · match ≥{' '}
                                {formatPct(
                                  (learnedPatternLiveGuardStatus.config
                                    .min_match_score || 0) * 100
                                )}{' '}
                                · FP ≤{' '}
                                {formatPct(
                                  (learnedPatternLiveGuardStatus.config
                                    .max_false_positive_score || 0) * 100
                                )}{' '}
                                · drift ≤{' '}
                                {formatPct(
                                  (learnedPatternLiveGuardStatus.config.max_drift_score ||
                                    0) * 100
                                )}{' '}
                                · only `monitoring_rule` anti-patterns can qualify for
                                live monitor / block effects
                              </div>
                            )}

                            {learnedPatternLiveGuardEvents.length === 0 ? (
                              <div className="text-sm text-nofx-text-muted">
                                No learned-pattern live guard evaluations recorded yet
                                for this trader slice. The panel will populate once
                                the trader reaches new open-decision checks.
                              </div>
                            ) : (
                              <div className="space-y-2">
                                {learnedPatternLiveGuardEvents.map((event) => (
                                  <div
                                    key={event.id}
                                    className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                                  >
                                    <div className="flex flex-wrap items-start justify-between gap-3">
                                      <div>
                                        <div className="flex flex-wrap items-center gap-2">
                                          <div className="font-medium">
                                            {event.symbol} · {event.side}
                                          </div>
                                          <span
                                            className={`px-2 py-1 rounded-full text-[11px] border ${symbolBehaviorLiveGuardEffectClasses(
                                              event.effect
                                            )}`}
                                          >
                                            {formatSymbolBehaviorLiveGuardEffect(
                                              event.effect
                                            )}
                                          </span>
                                          {event.policy_mode && (
                                            <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                              {formatSymbolBehaviorLiveGuardMode(
                                                event.policy_mode
                                              )}
                                            </span>
                                          )}
                                          {event.matched_pattern_class && (
                                            <span
                                              className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternClassClasses(
                                                event.matched_pattern_class
                                              )}`}
                                            >
                                              {formatLearnedPatternClass(
                                                event.matched_pattern_class
                                              )}
                                            </span>
                                          )}
                                          {event.matched_pattern_validation_label && (
                                            <span
                                              className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternValidationClasses(
                                                event.matched_pattern_validation_label
                                              )}`}
                                            >
                                              {formatLearnedPatternValidationLabel(
                                                event.matched_pattern_validation_label
                                              )}
                                            </span>
                                          )}
                                        </div>
                                        <div className="text-xs text-nofx-text-muted mt-1">
                                          {event.action || 'open'} · cycle{' '}
                                          {event.cycle_number || '-'} ·{' '}
                                          {event.selection_bucket || 'unknown'} ·{' '}
                                          {event.trend_regime || 'unknown'} /{' '}
                                          {event.volatility_regime || 'unknown'} /{' '}
                                          {event.oi_regime || 'unknown'}
                                        </div>
                                      </div>
                                      <div className="text-right text-xs text-nofx-text-muted">
                                        <div>{formatTimestampLabel(event.decision_timestamp)}</div>
                                        <div className="mt-1">
                                          match {formatPct((event.match_score || 0) * 100)}
                                        </div>
                                        <div className="mt-1">
                                          conf {event.decision_confidence || 0}%
                                        </div>
                                      </div>
                                    </div>

                                    <div className="text-sm text-nofx-text-muted mt-2">
                                      {event.summary || 'No summary stored.'}
                                    </div>

                                    {event.attribution && (
                                      <div className="mt-3 rounded-lg border border-white/10 bg-white/5 px-3 py-3">
                                        <div className="flex flex-wrap items-center gap-2">
                                          <span
                                            className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternLiveGuardAttributionClasses(
                                              event.attribution.status
                                            )}`}
                                          >
                                            {formatLearnedPatternLiveGuardAttributionStatus(
                                              event.attribution.status
                                            )}
                                          </span>
                                          {typeof event.attribution.horizon_hours === 'number' && (
                                            <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                              {event.attribution.horizon_hours}h window
                                            </span>
                                          )}
                                          {event.attribution.followup_case_outcome && (
                                            <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                              outcome {event.attribution.followup_case_outcome}
                                            </span>
                                          )}
                                        </div>
                                        <div className="mt-2 text-xs text-nofx-text-muted">
                                          {event.attribution.summary || 'No follow-up attribution stored yet.'}
                                        </div>
                                        <div className="mt-2 flex flex-wrap gap-3 text-xs text-nofx-text-muted">
                                          {event.attribution.followup_entry_delay_ms ? (
                                            <span>
                                              Follow-up delay{' '}
                                              {formatHold(event.attribution.followup_entry_delay_ms)}
                                            </span>
                                          ) : null}
                                          {event.attribution.next_attempt_cycle_number ? (
                                            <span>
                                              Next attempt cycle{' '}
                                              {event.attribution.next_attempt_cycle_number}
                                            </span>
                                          ) : null}
                                          {event.attribution.next_attempt_terminal_status ? (
                                            <span>
                                              Next attempt{' '}
                                              {event.attribution.next_attempt_terminal_status}
                                            </span>
                                          ) : null}
                                          {typeof event.attribution.followup_realized_pnl ===
                                          'number' &&
                                          (event.attribution.followup_case_id ||
                                            event.attribution.followup_realized_pnl !== 0 ||
                                            event.attribution.followup_realized_pnl_pct !== 0) ? (
                                            <span
                                              className={
                                                (event.attribution.followup_realized_pnl || 0) >= 0
                                                  ? 'text-emerald-300'
                                                  : 'text-rose-300'
                                              }
                                            >
                                              Follow-up PnL{' '}
                                              {formatMoney(
                                                event.attribution.followup_realized_pnl || 0
                                              )}{' '}
                                              /{' '}
                                              {formatPct(
                                                event.attribution.followup_realized_pnl_pct || 0
                                              )}
                                            </span>
                                          ) : null}
                                        </div>
                                      </div>
                                    )}

                                    {(event.block_reason ||
                                      event.matched_pattern_signature ||
                                      event.matched_pattern_recommended_use) && (
                                      <div className="flex flex-wrap items-center gap-2 mt-2 text-xs">
                                        {event.matched_pattern_signature && (
                                          <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1">
                                            {event.matched_pattern_signature}
                                          </span>
                                        )}
                                        {event.matched_pattern_recommended_use && (
                                          <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1">
                                            {formatLearnedPatternUse(
                                              event.matched_pattern_recommended_use
                                            )}
                                          </span>
                                        )}
                                        {event.block_reason && (
                                          <span className="text-rose-300">
                                            Block reason: {event.block_reason}
                                          </span>
                                        )}
                                      </div>
                                    )}

                                    <div className="flex flex-wrap gap-2 mt-3">
                                      {(event.matched_pattern_id ||
                                        event.matched_pattern_stable_key) && (
                                        <button
                                          onClick={() =>
                                            openPatternLabWithFilters({
                                              pattern_id: event.matched_pattern_id || '',
                                              stable_key:
                                                event.matched_pattern_stable_key || '',
                                              symbol: event.symbol || '',
                                              side: event.side || '',
                                            })
                                          }
                                          className="h-8 px-3 rounded-lg border border-white/10 bg-white/5 text-xs"
                                        >
                                          Open pattern
                                        </button>
                                      )}
                                      <button
                                        onClick={() => {
                                          if (event.symbol) setSymbol(event.symbol)
                                          if (event.side) setSide(event.side)
                                        }}
                                        className="h-8 px-3 rounded-lg border border-white/10 bg-white/5 text-xs"
                                      >
                                        Filter review to this setup
                                      </button>
                                      {event.attribution?.followup_case_id && (
                                        <button
                                          onClick={() => {
                                            if (event.symbol) setSymbol(event.symbol)
                                            if (event.side) setSide(event.side)
                                            setSelectedCaseId(event.attribution?.followup_case_id || null)
                                          }}
                                          className="h-8 px-3 rounded-lg border border-white/10 bg-white/5 text-xs"
                                        >
                                          Open follow-up deal
                                        </button>
                                      )}
                                    </div>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        )}

                        <div className="grid xl:grid-cols-2 gap-3">
                          <div className="rounded-lg border border-emerald-400/15 bg-emerald-500/5 px-3 py-3 space-y-2">
                            <div className="text-xs uppercase tracking-[0.2em] text-emerald-200">
                              Top Positive Patterns
                            </div>
                            {!learnedPatternSummary.top_positive_patterns ||
                            learnedPatternSummary.top_positive_patterns.length === 0 ? (
                              <div className="text-sm text-nofx-text-muted">
                                No positive patterns surfaced yet.
                              </div>
                            ) : (
                              learnedPatternSummary.top_positive_patterns.map((pattern) => (
                                <div
                                  key={`learned-pattern-positive-${pattern.id}`}
                                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                                >
                                  <div className="flex flex-wrap items-center gap-2">
                                    <span className="font-medium">
                                      {pattern.pattern_signature}
                                    </span>
                                    <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                      {formatLearnedPatternScope(pattern)}
                                    </span>
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternValidationClasses(
                                        pattern.validation_label
                                      )}`}
                                    >
                                      {formatLearnedPatternValidationLabel(
                                        pattern.validation_label
                                      )}
                                    </span>
                                  </div>
                                  <div className="text-xs text-nofx-text-muted mt-1">
                                    {pattern.sample_count} deals · avg{' '}
                                    {formatPct(pattern.avg_pnl_pct)} · lift{' '}
                                    {formatPct(pattern.lift_avg_pnl_pct)}
                                  </div>
                                </div>
                              ))
                            )}
                          </div>

                          <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3 space-y-2">
                            <div className="text-xs uppercase tracking-[0.2em] text-rose-200">
                              Top Anti-Patterns
                            </div>
                            {!learnedPatternSummary.top_negative_patterns ||
                            learnedPatternSummary.top_negative_patterns.length === 0 ? (
                              <div className="text-sm text-nofx-text-muted">
                                No anti-patterns surfaced yet.
                              </div>
                            ) : (
                              learnedPatternSummary.top_negative_patterns.map((pattern) => (
                                <div
                                  key={`learned-pattern-negative-${pattern.id}`}
                                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                                >
                                  <div className="flex flex-wrap items-center gap-2">
                                    <span className="font-medium">
                                      {pattern.pattern_signature}
                                    </span>
                                    <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                      {formatLearnedPatternScope(pattern)}
                                    </span>
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternValidationClasses(
                                        pattern.validation_label
                                      )}`}
                                    >
                                      {formatLearnedPatternValidationLabel(
                                        pattern.validation_label
                                      )}
                                    </span>
                                  </div>
                                  <div className="text-xs text-nofx-text-muted mt-1">
                                    {pattern.sample_count} deals · avg{' '}
                                    {formatPct(pattern.avg_pnl_pct)} · lift{' '}
                                    {formatPct(pattern.lift_avg_pnl_pct)}
                                  </div>
                                </div>
                              ))
                            )}
                          </div>

                          <div className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-3 space-y-2">
                            <div className="text-xs uppercase tracking-[0.2em] text-amber-200">
                              Top Symbol Overrides
                            </div>
                            {!learnedPatternSummary.top_symbol_overrides ||
                            learnedPatternSummary.top_symbol_overrides.length === 0 ? (
                              <div className="text-sm text-nofx-text-muted">
                                No symbol-specific overrides surfaced yet.
                              </div>
                            ) : (
                              learnedPatternSummary.top_symbol_overrides.map((pattern) => (
                                <div
                                  key={`learned-pattern-override-${pattern.id}`}
                                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                                >
                                  <div className="flex flex-wrap items-center gap-2">
                                    <span className="font-medium">
                                      {pattern.symbol || 'Trader-local'} ·{' '}
                                      {pattern.pattern_signature}
                                    </span>
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternClassClasses(
                                        pattern.pattern_class
                                      )}`}
                                    >
                                      {formatLearnedPatternClass(pattern.pattern_class)}
                                    </span>
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternValidationClasses(
                                        pattern.validation_label
                                      )}`}
                                    >
                                      {formatLearnedPatternValidationLabel(
                                        pattern.validation_label
                                      )}
                                    </span>
                                  </div>
                                  <div className="text-xs text-nofx-text-muted mt-1">
                                    {pattern.side} · lift {formatPct(pattern.lift_avg_pnl_pct)} ·{' '}
                                    {pattern.sample_count} deals
                                  </div>
                                </div>
                              ))
                            )}
                          </div>
                        </div>

                        {((learnedPatternSummary.top_expiring_monitoring_rules &&
                          learnedPatternSummary.top_expiring_monitoring_rules.length > 0) ||
                          (learnedPatternSummary.top_rollback_watch_patterns &&
                            learnedPatternSummary.top_rollback_watch_patterns.length > 0) ||
                          (learnedPatternSummary.top_improving_monitoring_rules &&
                            learnedPatternSummary.top_improving_monitoring_rules.length > 0) ||
                          (learnedPatternSummary.top_degrading_monitoring_rules &&
                            learnedPatternSummary.top_degrading_monitoring_rules.length > 0) ||
                          (learnedPatternSummary.top_overblocking_monitoring_rules &&
                            learnedPatternSummary.top_overblocking_monitoring_rules.length > 0) ||
                          (learnedPatternSummary.top_fragile_monitoring_rules &&
                            learnedPatternSummary.top_fragile_monitoring_rules.length > 0)) && (
                          <div className="grid xl:grid-cols-3 2xl:grid-cols-6 gap-3">
                            <div className="rounded-lg border border-orange-400/15 bg-orange-500/5 px-3 py-3 space-y-2">
                              <div className="text-xs uppercase tracking-[0.2em] text-orange-200">
                                Expiring Monitoring Rules
                              </div>
                              {!learnedPatternSummary.top_expiring_monitoring_rules ||
                              learnedPatternSummary.top_expiring_monitoring_rules.length ===
                                0 ? (
                                <div className="text-sm text-nofx-text-muted">
                                  No monitoring rules are expiring for this slice.
                                </div>
                              ) : (
                                learnedPatternSummary.top_expiring_monitoring_rules.map(
                                  (pattern) => (
                                    <div
                                      key={`learned-pattern-expiring-${pattern.id}`}
                                      className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                                    >
                                      <div className="flex flex-wrap items-center gap-2">
                                        <span className="font-medium">
                                          {pattern.pattern_signature}
                                        </span>
                                        <span
                                          className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternLifecycleClasses(
                                            pattern.lifecycle?.status
                                          )}`}
                                        >
                                          {formatLearnedPatternLifecycleStatus(
                                            pattern.lifecycle?.status
                                          )}
                                        </span>
                                      </div>
                                      <div className="text-xs text-nofx-text-muted mt-1">
                                        expiry{' '}
                                        {formatPct(
                                          (pattern.lifecycle?.expiry_score || 0) * 100
                                        )}{' '}
                                        · validation{' '}
                                        {formatPct(
                                          (pattern.validation_support_score || 0) * 100
                                        )}{' '}
                                        · recent {pattern.recent_support_count || 0}/
                                        {pattern.recent_sample_count || 0}
                                      </div>
                                      {pattern.lifecycle?.summary && (
                                        <div className="text-xs text-nofx-text-muted mt-2">
                                          {pattern.lifecycle.summary}
                                        </div>
                                      )}
                                    </div>
                                  )
                                )
                              )}
                            </div>

                            <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3 space-y-2">
                              <div className="text-xs uppercase tracking-[0.2em] text-emerald-200">
                                Improving Rules
                              </div>
                              {!learnedPatternSummary.top_improving_monitoring_rules ||
                              learnedPatternSummary.top_improving_monitoring_rules.length ===
                                0 ? (
                                <div className="text-sm text-nofx-text-muted">
                                  No monitoring rules are currently improving.
                                </div>
                              ) : (
                                learnedPatternSummary.top_improving_monitoring_rules.map(
                                  (pattern) => (
                                    <div
                                      key={`learned-pattern-improving-${pattern.id}`}
                                      className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                                    >
                                      <div className="flex flex-wrap items-center gap-2">
                                        <span className="font-medium">
                                          {pattern.pattern_signature}
                                        </span>
                                        <span
                                          className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternRollupTrendClasses(
                                            pattern.live_guard_attribution_delta?.trend_label
                                          )}`}
                                        >
                                          {formatLearnedPatternRollupTrendLabel(
                                            pattern.live_guard_attribution_delta?.trend_label
                                          )}
                                        </span>
                                      </div>
                                      <div className="text-xs text-nofx-text-muted mt-1">
                                        protective delta{' '}
                                        {formatPct(
                                          (pattern.live_guard_attribution_delta
                                            ?.protective_rate_delta || 0) * 100
                                        )}{' '}
                                        · recent{' '}
                                        {formatPct(
                                          (pattern.live_guard_attribution_delta
                                            ?.recent_protective_rate || 0) * 100
                                        )}{' '}
                                        · prior{' '}
                                        {formatPct(
                                          (pattern.live_guard_attribution_delta
                                            ?.prior_protective_rate || 0) * 100
                                        )}
                                      </div>
                                      {pattern.live_guard_attribution_delta?.summary && (
                                        <div className="text-xs text-nofx-text-muted mt-2">
                                          {pattern.live_guard_attribution_delta.summary}
                                        </div>
                                      )}
                                    </div>
                                  )
                                )
                              )}
                            </div>

                            <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3 space-y-2">
                              <div className="text-xs uppercase tracking-[0.2em] text-amber-200">
                                Degrading Rules
                              </div>
                              {!learnedPatternSummary.top_degrading_monitoring_rules ||
                              learnedPatternSummary.top_degrading_monitoring_rules.length ===
                                0 ? (
                                <div className="text-sm text-nofx-text-muted">
                                  No monitoring rules are currently degrading.
                                </div>
                              ) : (
                                learnedPatternSummary.top_degrading_monitoring_rules.map(
                                  (pattern) => (
                                    <div
                                      key={`learned-pattern-degrading-${pattern.id}`}
                                      className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                                    >
                                      <div className="flex flex-wrap items-center gap-2">
                                        <span className="font-medium">
                                          {pattern.pattern_signature}
                                        </span>
                                        <span
                                          className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternRollupTrendClasses(
                                            pattern.live_guard_attribution_delta?.trend_label
                                          )}`}
                                        >
                                          {formatLearnedPatternRollupTrendLabel(
                                            pattern.live_guard_attribution_delta?.trend_label
                                          )}
                                        </span>
                                      </div>
                                      <div className="text-xs text-nofx-text-muted mt-1">
                                        overblocking delta{' '}
                                        {formatPct(
                                          (pattern.live_guard_attribution_delta
                                            ?.overblocking_rate_delta || 0) * 100
                                        )}{' '}
                                        · recent{' '}
                                        {formatPct(
                                          (pattern.live_guard_attribution_delta
                                            ?.recent_overblocking_rate || 0) * 100
                                        )}{' '}
                                        · prior{' '}
                                        {formatPct(
                                          (pattern.live_guard_attribution_delta
                                            ?.prior_overblocking_rate || 0) * 100
                                        )}
                                      </div>
                                      {pattern.live_guard_attribution_delta?.summary && (
                                        <div className="text-xs text-nofx-text-muted mt-2">
                                          {pattern.live_guard_attribution_delta.summary}
                                        </div>
                                      )}
                                    </div>
                                  )
                                )
                              )}
                            </div>

                            <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3 space-y-2">
                              <div className="text-xs uppercase tracking-[0.2em] text-rose-200">
                                Overblocking Rules
                              </div>
                              {!learnedPatternSummary.top_overblocking_monitoring_rules ||
                              learnedPatternSummary.top_overblocking_monitoring_rules.length ===
                                0 ? (
                                <div className="text-sm text-nofx-text-muted">
                                  No monitoring rules currently look overblocking.
                                </div>
                              ) : (
                                learnedPatternSummary.top_overblocking_monitoring_rules.map(
                                  (pattern) => (
                                    <div
                                      key={`learned-pattern-overblocking-${pattern.id}`}
                                      className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                                    >
                                      <div className="flex flex-wrap items-center gap-2">
                                        <span className="font-medium">
                                          {pattern.pattern_signature}
                                        </span>
                                        <span
                                          className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternRollupClasses(
                                            pattern.live_guard_attribution?.attribution_label
                                          )}`}
                                        >
                                          {formatLearnedPatternRollupLabel(
                                            pattern.live_guard_attribution?.attribution_label
                                          )}
                                        </span>
                                      </div>
                                      <div className="text-xs text-nofx-text-muted mt-1">
                                        overblocking{' '}
                                        {formatPct(
                                          (pattern.live_guard_attribution?.overblocking_rate || 0) *
                                            100
                                        )}{' '}
                                        · resolved{' '}
                                        {pattern.live_guard_attribution?.resolved_event_count || 0}{' '}
                                        · confidence{' '}
                                        {formatPct(
                                          (pattern.live_guard_attribution?.confidence_score || 0) *
                                            100
                                        )}
                                      </div>
                                      {pattern.live_guard_attribution?.summary && (
                                        <div className="text-xs text-nofx-text-muted mt-2">
                                          {pattern.live_guard_attribution.summary}
                                        </div>
                                      )}
                                    </div>
                                  )
                                )
                              )}
                            </div>

                            <div className="rounded-lg border border-amber-400/15 bg-amber-500/5 px-3 py-3 space-y-2">
                              <div className="text-xs uppercase tracking-[0.2em] text-amber-200">
                                Fragile Monitoring Rules
                              </div>
                              {!learnedPatternSummary.top_fragile_monitoring_rules ||
                              learnedPatternSummary.top_fragile_monitoring_rules.length ===
                                0 ? (
                                <div className="text-sm text-nofx-text-muted">
                                  No monitoring rules currently look structurally fragile.
                                </div>
                              ) : (
                                learnedPatternSummary.top_fragile_monitoring_rules.map(
                                  (pattern) => (
                                    <div
                                      key={`learned-pattern-fragile-${pattern.id}`}
                                      className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                                    >
                                      <div className="flex flex-wrap items-center gap-2">
                                        <span className="font-medium">
                                          {pattern.pattern_signature}
                                        </span>
                                        {pattern.lifecycle?.status && (
                                          <span
                                            className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternLifecycleClasses(
                                              pattern.lifecycle.status
                                            )}`}
                                          >
                                            {formatLearnedPatternLifecycleStatus(
                                              pattern.lifecycle.status
                                            )}
                                          </span>
                                        )}
                                        <span className="px-2 py-1 rounded-full text-[11px] border border-amber-400/20 bg-amber-500/10 text-amber-200">
                                          Fragile
                                        </span>
                                      </div>
                                      <div className="text-xs text-nofx-text-muted mt-1">
                                        degrade{' '}
                                        {formatPct(
                                          ((pattern.lifecycle_trend?.degrading_share || 0) +
                                            (pattern.lifecycle_trend?.rollback_watch_share || 0)) *
                                            100
                                        )}{' '}
                                        · stale lag{' '}
                                        {pattern.lifecycle_trend?.stale_guard_snapshot_count || 0}
                                        {' '}· changes{' '}
                                        {pattern.lifecycle_trend?.status_change_count || 0}
                                      </div>
                                      {pattern.lifecycle_trend?.summary && (
                                        <div className="text-xs text-nofx-text-muted mt-2">
                                          {pattern.lifecycle_trend.summary}
                                        </div>
                                      )}
                                    </div>
                                  )
                                )
                              )}
                            </div>

                            <div className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3 space-y-2">
                              <div className="text-xs uppercase tracking-[0.2em] text-rose-200">
                                Rollback Watch
                              </div>
                              {!learnedPatternSummary.top_rollback_watch_patterns ||
                              learnedPatternSummary.top_rollback_watch_patterns.length ===
                                0 ? (
                                <div className="text-sm text-nofx-text-muted">
                                  No monitoring rules are currently on rollback watch.
                                </div>
                              ) : (
                                learnedPatternSummary.top_rollback_watch_patterns.map(
                                  (pattern) => (
                                    <div
                                      key={`learned-pattern-rollback-${pattern.id}`}
                                      className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                                    >
                                      <div className="flex flex-wrap items-center gap-2">
                                        <span className="font-medium">
                                          {pattern.pattern_signature}
                                        </span>
                                        <span
                                          className={`px-2 py-1 rounded-full text-[11px] border ${learnedPatternLifecycleClasses(
                                            pattern.lifecycle?.status
                                          )}`}
                                        >
                                          {formatLearnedPatternLifecycleStatus(
                                            pattern.lifecycle?.status
                                          )}
                                        </span>
                                      </div>
                                      <div className="text-xs text-nofx-text-muted mt-1">
                                        rollback{' '}
                                        {formatPct(
                                          (pattern.lifecycle?.rollback_score || 0) * 100
                                        )}{' '}
                                        · guard hits{' '}
                                        {pattern.lifecycle?.recent_guard_event_count || 0}{' '}
                                        · hard block{' '}
                                        {pattern.lifecycle?.recent_hard_blocked_count || 0}
                                      </div>
                                      {pattern.lifecycle?.summary && (
                                        <div className="text-xs text-nofx-text-muted mt-2">
                                          {pattern.lifecycle.summary}
                                        </div>
                                      )}
                                    </div>
                                  )
                                )
                              )}
                            </div>
                          </div>
                        )}
                      </div>
                    )}
                  </div>

                  <div>
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                      Repeated symbol edge failures
                    </div>
                    <div className="space-y-2">
                      {!anomalies.symbol_edge_failures ||
                      anomalies.symbol_edge_failures.length === 0 ? (
                        <div className="text-sm text-nofx-text-muted">
                          No repeated symbol-specific edge failures detected in
                          this filtered slice yet.
                        </div>
                      ) : (
                        anomalies.symbol_edge_failures.map((item) => (
                          <div
                            key={`${item.symbol}-${item.side}-${item.regime_signature}`}
                            className="rounded-lg border border-rose-400/15 bg-rose-500/5 px-3 py-3 flex items-start justify-between gap-4"
                          >
                            <div>
                              <div className="flex flex-wrap items-center gap-2">
                                <div className="font-medium">
                                  {item.symbol} · {item.side}
                                </div>
                                <span className="px-2 py-1 rounded-full text-[11px] border border-rose-400/20 bg-rose-500/10 text-rose-200">
                                  contradiction {formatPct(item.contradiction_score * 100)}
                                </span>
                                <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                                  {formatSymbolBehaviorAction(
                                    item.recommended_action
                                  )}
                                </span>
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                {item.regime_signature}
                              </div>
                              <div className="text-sm text-nofx-text-muted mt-2">
                                {item.summary ||
                                  `${item.symbol} ${item.side} repeatedly underperformed under this setup.`}
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-2">
                                {item.slice_deals} matching deals in current
                                slice · {item.historical_deals} historical
                                deals · {item.decision_open_count} decision
                                opens · avg decision conf{' '}
                                {formatPct(item.avg_decision_confidence || 0)}
                              </div>
                              {item.signal_tags && item.signal_tags.length > 0 && (
                                <div className="flex flex-wrap gap-2 mt-2">
                                  {item.signal_tags.slice(0, 6).map((tag) => (
                                    <span
                                      key={`${item.symbol}-${item.side}-${tag}`}
                                      className="px-2 py-1 rounded-full text-[11px] border border-rose-400/20 bg-rose-500/10 text-rose-100"
                                    >
                                      {tag}
                                    </span>
                                  ))}
                                </div>
                              )}
                              <div className="flex flex-wrap gap-2 mt-3">
                                <button
                                  onClick={() =>
                                    applyDrilldownFilters(
                                      {
                                        symbol: item.symbol,
                                        side: item.side,
                                        status: 'CLOSED',
                                        open_selection_bucket:
                                          item.open_selection_bucket || undefined,
                                        open_trend_regime:
                                          item.open_trend_regime || undefined,
                                        open_volatility_regime:
                                          item.open_volatility_regime || undefined,
                                        open_oi_regime:
                                          item.open_oi_regime || undefined,
                                      },
                                      `Filtered deals to repeated edge failure ${item.symbol} ${item.side}.`
                                    )
                                  }
                                  className="px-2.5 py-1 rounded-lg border border-white/10 bg-white/5 text-xs font-medium"
                                >
                                  Filter
                                </button>
                                <button
                                  onClick={() =>
                                    void runAIScan(
                                      {
                                        symbol: item.symbol,
                                        side: item.side,
                                        status: 'CLOSED',
                                        open_selection_bucket:
                                          item.open_selection_bucket || undefined,
                                        open_trend_regime:
                                          item.open_trend_regime || undefined,
                                        open_volatility_regime:
                                          item.open_volatility_regime || undefined,
                                        open_oi_regime:
                                          item.open_oi_regime || undefined,
                                      },
                                      `AI scan started for repeated edge failure ${item.symbol} ${item.side}.`
                                    )
                                  }
                                  disabled={
                                    !selectedTraderId ||
                                    runningScan ||
                                    items.length === 0
                                  }
                                  className="px-2.5 py-1 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 text-nofx-gold text-xs font-medium disabled:opacity-50"
                                >
                                  Scan this cohort
                                </button>
                              </div>
                            </div>
                            <div className="text-right text-sm">
                              <div className="font-semibold text-rose-400">
                                {formatMoney(item.slice_net_pnl)}
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-2">
                                current slice
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                historical avg {formatPct(item.avg_pnl_pct)}
                              </div>
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  </div>

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
                    {
                      title: 'Unknown / low-confidence exits',
                      items: anomalies.exit_uncertainty?.map((item) => ({
                        key: `${item.close_reason}:${item.exit_reason_quality}:${item.exit_origin}`,
                        primary: item.label,
                        secondary: `${item.deals} closes | ${item.exit_origin} | ${formatMoney(item.avg_pnl)} avg`,
                        value: `${formatPct(item.share_pct)} share`,
                        filterPatch: {
                          close_reason: item.close_reason || undefined,
                          exit_reason_quality:
                            item.exit_reason_quality || undefined,
                          status: 'CLOSED',
                        },
                        filterNote: `Filtered deals to uncertain exit cohort ${item.label}.`,
                        scanNote: `AI scan started for uncertain exit cohort ${item.label}.`,
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
                                    {pickText('Filter', 'Filtern')}
                                  </button>
                                  <button
                                    onClick={() =>
                                      void runAIScan(
                                        item.filterPatch,
                                        item.scanNote
                                      )
                                    }
                                    disabled={
                                      !selectedTraderId ||
                                      runningScan ||
                                      items.length === 0
                                    }
                                    className="px-2.5 py-1 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 text-nofx-gold text-xs font-medium disabled:opacity-50"
                                  >
                                    {pickText('Scan this cohort', 'Diese Kohorte scannen')}
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
                  <h2 className="font-semibold text-lg">
                    {pickText('AI scan', 'KI-Scan')}
                  </h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    {pickText(
                      'Run a model against the current filtered dataset, validate the evidence gate, then either apply directly or launch a timed challenger compare.',
                      'Fuehre ein Modell auf dem aktuell gefilterten Datensatz aus, pruefe das Evidence-Gate und uebernimm dann direkt oder starte einen zeitgesteuerten Challenger-Vergleich.'
                    )}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={exportScansJSON}
                    disabled={!selectedTraderId || scans.length === 0}
                    className="h-10 px-4 rounded-lg border border-white/10 bg-black/20 text-sm disabled:opacity-40"
                  >
                    {pickText('Export scans', 'Scans exportieren')}
                  </button>
                  <button
                    onClick={() => void runAIScan()}
                    disabled={
                      !selectedTraderId || runningScan || items.length === 0
                    }
                    className="h-10 px-4 rounded-lg bg-nofx-gold text-black font-semibold disabled:opacity-50"
                  >
                    {runningScan
                      ? pickText('Running…', 'Laeuft…')
                      : pickText('Run scan', 'Scan starten')}
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
                            label: pickText('Configured default', 'Konfigurierter Standard'),
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
                    options={localizedChallengerModeOptions}
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
                {pickText(
                  '`paper / simulation` currently uses a selected testnet wallet. `isolated_live` requires a different live wallet than the incumbent. `shared_live` can use the same live wallet.',
                  '`paper / simulation` nutzt derzeit die ausgewaehlte Testnet-Wallet. `isolated_live` benoetigt eine andere Live-Wallet als der Inkumbent. `shared_live` kann dieselbe Live-Wallet verwenden.'
                )}
              </div>

              <div className="space-y-4 mt-5">
                {scans.length === 0 ? (
                  <div className="text-sm text-nofx-text-muted">
                    {pickText('No AI scans saved yet.', 'Noch keine KI-Scans gespeichert.')}
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
                      relatedCompare,
                      language
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
                              {new Date(scan.scan.created_at).toLocaleString(
                                toDateTimeLocale(language)
                              )}{' '}
                              | {scan.scan.dataset_count} {pickText('deals', 'Deals')}
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
                                  pickText(
                                    'Validation required before apply or challenger launch.',
                                    'Validierung erforderlich, bevor uebernommen oder ein Challenger gestartet werden kann.'
                                  )}
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
                                ? pickText('Validating…', 'Validiere…')
                                : validationStatus === 'passed'
                                  ? pickText('Revalidate', 'Erneut validieren')
                                  : pickText('Validate', 'Validieren')}
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
                                ? pickText('Launching…', 'Starte…')
                                : pickText('Launch challenger', 'Challenger starten')}
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
                                ? pickText('Applying…', 'Wende an…')
                                : pickText('Apply patch', 'Patch anwenden')}
                            </button>
                          </div>
                        </div>

                        {validation && (
                          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 mt-4 text-sm">
                            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                {pickText('Evidence gate', 'Evidence-Gate')}
                              </div>
                              <div className="space-y-1 text-nofx-text-muted">
                                <div>
                                  {pickText('Closed deals:', 'Geschlossene Deals:')}{' '}
                                  {validation.closed_deal_count} /{' '}
                                  {validation.min_closed_deal_count}
                                </div>
                                <div>
                                  {pickText('Train / holdout / recent:', 'Train / Holdout / aktuell:')}{' '}
                                  {validation.training_closed_deal_count} /{' '}
                                  {validation.holdout_closed_deal_count} /{' '}
                                  {validation.recent_closed_deal_count || 0}
                                </div>
                                <div>
                                  {pickText('Config valid:', 'Config gueltig:')}{' '}
                                  {validation.config_valid
                                    ? pickText('yes', 'ja')
                                    : pickText('no', 'nein')}
                                </div>
                                {validation.recent_slice_label && (
                                  <div>
                                    {pickText('Recent slice:', 'Aktueller Ausschnitt:')}{' '}
                                    {validation.recent_slice_label}
                                  </div>
                                )}
                              </div>
                            </div>
                            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                {pickText('Gate output', 'Gate-Ausgabe')}
                              </div>
                              {(validation.blocking_issues || []).length === 0 ? (
                                <div className="text-sm text-emerald-300">
                                  {pickText(
                                    'Ready for direct apply or challenger launch.',
                                    'Bereit fuer direkte Uebernahme oder Challenger-Start.'
                                  )}
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
                                {pickText('Replay gate', 'Replay-Gate')}
                              </div>
                              {!validation.replay?.supported ? (
                                <div className="text-sm text-nofx-text-muted">
                                  {pickText(
                                    'No supported historical replay for this patch yet.',
                                    'Fuer diesen Patch gibt es noch kein unterstuetztes historisches Replay.'
                                  )}
                                </div>
                              ) : (
                                <div className="space-y-1 text-nofx-text-muted">
                                  <div>
                                    {pickText('Holdout delta:', 'Holdout-Delta:')}{' '}
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
                                    {pickText('Training delta:', 'Trainings-Delta:')}{' '}
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
                                    {pickText('Recent delta:', 'Aktuelles Delta:')}{' '}
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
                                    {pickText('Covered fields:', 'Abgedeckte Felder:')}{' '}
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
                                  {pickText('Metric gates', 'Metrik-Gates')}
                                </div>
                                <div className="text-xs text-nofx-text-muted">
                                  {(validation.checks || []).filter((item) => item.passed).length} /{' '}
                                  {(validation.checks || []).length}{' '}
                                  {pickText('passed', 'bestanden')}
                                </div>
                              </div>
                              {!(validation.checks || []).length ? (
                                <div className="text-sm text-nofx-text-muted">
                                  {pickText(
                                    'No metric checks recorded yet.',
                                    'Noch keine Metrik-Pruefungen gespeichert.'
                                  )}
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
                                              {formatValidationScope(item.scope, language)} |{' '}
                                              {item.source ||
                                                pickText('baseline', 'Baseline')}
                                            </div>
                                          </div>
                                          <span
                                            className={`px-2 py-1 rounded-full text-[11px] uppercase tracking-[0.18em] ${
                                              item.passed
                                                ? 'bg-emerald-500/15 text-emerald-300'
                                                : 'bg-rose-500/15 text-rose-300'
                                            }`}
                                          >
                                            {item.passed
                                              ? pickText('pass', 'pass')
                                              : pickText('block', 'block')}
                                          </span>
                                        </div>
                                        <div className="text-xs text-nofx-text-muted mt-2">
                                          {formatValidationCheckSummary(item, language)}
                                        </div>
                                        {typeof item.baseline === 'number' &&
                                          item.comparator === 'delta_gte' && (
                                            <div className="text-xs text-nofx-text-muted mt-1">
                                              {pickText('baseline', 'Baseline')}{' '}
                                              {formatValidationMetricValue(
                                                item.metric,
                                                item.baseline
                                              )}
                                              {' | '}
                                              {pickText('projected', 'projiziert')}{' '}
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
                                {pickText('Weaknesses', 'Schwaechen')}
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
                                {pickText('Immediate actions', 'Sofortmassnahmen')}
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
                    <div className="font-semibold">
                      {pickText('Scan compare', 'Scan-Vergleich')}
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {pickText(
                        'Compare two saved AI scans on filters, overlaps and patch deltas.',
                        'Vergleiche zwei gespeicherte KI-Scans anhand von Filtern, Ueberschneidungen und Patch-Deltas.'
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {compareLoading && (
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Loading…', 'Laedt…')}
                      </div>
                    )}
                    <button
                      onClick={exportScanCompareJSON}
                      disabled={!compareResult}
                      className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs disabled:opacity-40"
                    >
                      {pickText('Export compare', 'Vergleich exportieren')}
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
                        label: `${item.scan.model_name} · ${new Date(
                          item.scan.created_at
                        ).toLocaleDateString(toDateTimeLocale(language))}`,
                      }))}
                    />
                  </div>
                  <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                    <NofxSelect
                        value={compareRightScanId}
                        onChange={setCompareRightScanId}
                      options={scans.map((item) => ({
                        value: item.scan.id,
                        label: `${item.scan.model_name} · ${new Date(
                          item.scan.created_at
                        ).toLocaleDateString(toDateTimeLocale(language))}`,
                      }))}
                    />
                  </div>
                </div>

                {!compareResult ? (
                  <div className="text-sm text-nofx-text-muted mt-4">
                    {pickText(
                      'Select two different scans to compare.',
                      'Waehle zwei unterschiedliche Scans zum Vergleichen aus.'
                    )}
                  </div>
                ) : (
                  <div className="space-y-4 mt-4">
                    <div className="grid grid-cols-2 gap-3 text-sm">
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Left', 'Links')}
                        </div>
                        <div className="font-medium mt-1">
                          {compareResult.left.scan.provider} /{' '}
                          {compareResult.left.scan.model_name}
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Right', 'Rechts')}
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
                            {pickText('Disagreement score', 'Widerspruchs-Score')}
                          </div>
                          <div
                            className={`mt-2 text-2xl font-semibold ${compareLevelClasses(compareResult.disagreement_level)}`}
                          >
                            {compareResult.conflict_score.toFixed(0)} / 100
                          </div>
                          <div className="text-sm text-nofx-text-muted mt-2 max-w-2xl">
                            {compareResult.disagreement_summary ||
                              pickText(
                                'No disagreement summary available.',
                                'Keine Zusammenfassung der Widersprueche verfuegbar.'
                              )}
                          </div>
                        </div>
                        <div className="text-sm">
                          <span className="text-nofx-text-muted">
                            {pickText('Filters:', 'Filter:')}
                          </span>{' '}
                          <span
                            className={
                              compareResult.same_filters
                                ? 'text-emerald-400'
                                : 'text-amber-300'
                            }
                          >
                            {compareResult.same_filters
                              ? pickText('Same dataset', 'Gleicher Datensatz')
                              : pickText('Different dataset filters', 'Unterschiedliche Datensatz-Filter')}
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
                              {formatCompareFlagTitle(
                                flag.code,
                                flag.title,
                                language
                              )}
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
                          <div className="font-semibold">
                            {pickText('Model usefulness by cohort', 'Modell-Nutzen nach Kohorte')}
                          </div>
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
                  <h2 className="font-semibold text-lg">
                    {pickText('Challenger history', 'Challenger-Historie')}
                  </h2>
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
                  <h2 className="font-semibold text-lg">
                    {pickText('Strategy history', 'Strategiehistorie')}
                  </h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    {pickText(
                      'Applied AI patches and rollback points for the current trader strategy.',
                      'Uebernommene KI-Patches und Rollback-Punkte fuer die aktuelle Trader-Strategie.'
                    )}
                  </p>
                </div>
              </div>

              <div className="space-y-3 mt-4">
                {versions.length === 0 ? (
                  <div className="text-sm text-nofx-text-muted">
                    {pickText(
                      'No strategy versions captured yet.',
                      'Noch keine Strategieversionen erfasst.'
                    )}
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
                        ? pickText(
                            'Linked challenger compare available.',
                            'Verknuepfter Challenger-Vergleich verfuegbar.'
                          )
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
                                version.version.source_type,
                                language
                              )}
                            </div>
                            <div className="font-medium mt-1">
                              {version.version.summary ||
                                pickText('Strategy change', 'Strategieaenderung')}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-2">
                              {pickText('Applied', 'Uebernommen')}{' '}
                              {new Date(
                                version.version.applied_at ||
                                  version.version.created_at
                              ).toLocaleString(toDateTimeLocale(language))}
                            </div>
                            {version.version.expected_effect && (
                              <div className="text-xs text-nofx-text-muted mt-2">
                                {pickText('Intended effect:', 'Beabsichtigter Effekt:')}{' '}
                                {version.version.expected_effect}
                              </div>
                            )}
                            {hasCompareLink && (
                              <div className="text-xs text-sky-300 mt-2">
                                {pickText('Compare note:', 'Vergleichsnotiz:')}{' '}
                                {compareNote}
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
                              {pickText('Open detail', 'Details oeffnen')}
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
                                  ? pickText('Opening…', 'Oeffne…')
                                  : pickText('Open compare', 'Vergleich oeffnen')}
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
                                ? pickText('Rolling back…', 'Rollback laeuft…')
                                : pickText('Rollback', 'Rollback')}
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
                        {pickText('Strategy version detail', 'Details zur Strategieversion')}
                      </div>
                      <h3 className="font-semibold text-lg mt-1">
                        {selectedVersion.version.summary ||
                          pickText('Strategy change', 'Strategieaenderung')}
                      </h3>
                      <div className="text-sm text-nofx-text-muted mt-2">
                        {formatStrategyVersionSourceType(
                          selectedVersion.version.source_type,
                          language
                        )}{' '}
                        · {pickText('applied', 'uebernommen')}{' '}
                        {new Date(
                          selectedVersion.version.applied_at ||
                            selectedVersion.version.created_at
                        ).toLocaleString(toDateTimeLocale(language))}
                      </div>
                      {selectedVersion.version.expected_effect && (
                        <div className="text-sm text-nofx-text-muted mt-2">
                          {pickText('Intended effect:', 'Beabsichtigter Effekt:')}{' '}
                          {selectedVersion.version.expected_effect}
                        </div>
                      )}
                      {selectedVersion.source_scan_summary && (
                        <div className="text-sm text-nofx-text-muted mt-2">
                          {pickText('Source scan:', 'Quell-Scan:')}{' '}
                          {selectedVersion.source_scan_summary}
                        </div>
                      )}
                      {selectedVersion.compare_summary && (
                        <div className="text-sm text-sky-300 mt-2">
                          {pickText('Linked compare:', 'Verknuepfter Vergleich:')}{' '}
                          {selectedVersion.compare_summary}
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
                            ? pickText('Opening…', 'Oeffne…')
                            : pickText('Open compare', 'Vergleich oeffnen')}
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
                          ? pickText('Rolling back…', 'Rollback laeuft…')
                          : pickText('Rollback', 'Rollback')}
                      </button>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 mt-5 text-sm">
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Target cohort', 'Zielkohorte')}
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatVersionCohort(selectedVersion.target_cohort, language)}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Full before', 'Gesamt davor')}
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatDatasetMini(
                          selectedVersion.attribution?.full_before_summary,
                          language
                        )}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Full after', 'Gesamt danach')}
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatDatasetMini(
                          selectedVersion.attribution?.full_after_summary,
                          language
                        )}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Rollback suggestion', 'Rollback-Empfehlung')}
                      </div>
                      <div
                        className={`font-semibold mt-1 ${
                          selectedVersion.attribution?.rollback_suggested
                            ? 'text-rose-300'
                            : 'text-emerald-300'
                        }`}
                      >
                        {selectedVersion.attribution?.rollback_suggested
                          ? pickText('Suggested', 'Empfohlen')
                          : pickText('Not suggested', 'Nicht empfohlen')}
                      </div>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3 mt-4 text-sm">
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Target before', 'Ziel davor')}
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatDatasetMini(
                          selectedVersion.attribution?.target_before_summary,
                          language
                        )}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Target after', 'Ziel danach')}
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatDatasetMini(
                          selectedVersion.attribution?.target_after_summary,
                          language
                        )}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Non-target after', 'Nicht-Ziel danach')}
                      </div>
                      <div className="text-sm mt-1 text-nofx-text-muted">
                        {formatDatasetMini(
                          selectedVersion.attribution?.non_target_after_summary,
                          language
                        )}
                      </div>
                    </div>
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4 mt-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                      {pickText('Observation window', 'Beobachtungsfenster')}
                    </div>
                    <div className="text-sm text-nofx-text-muted">
                      {pickText('Before:', 'Davor:')}{' '}
                      {selectedVersion.attribution?.before_start
                        ? new Date(
                            selectedVersion.attribution.before_start
                          ).toLocaleString(toDateTimeLocale(language))
                        : '-'}{' '}
                      {pickText('to', 'bis')}{' '}
                      {selectedVersion.attribution?.before_end
                        ? new Date(
                            selectedVersion.attribution.before_end
                          ).toLocaleString(toDateTimeLocale(language))
                        : '-'}
                    </div>
                    <div className="text-sm text-nofx-text-muted mt-1">
                      {pickText('After:', 'Danach:')}{' '}
                      {selectedVersion.attribution?.after_start
                        ? new Date(
                            selectedVersion.attribution.after_start
                          ).toLocaleString(toDateTimeLocale(language))
                        : '-'}{' '}
                      {pickText('to', 'bis')}{' '}
                      {selectedVersion.attribution?.after_end
                        ? new Date(
                            selectedVersion.attribution.after_end
                          ).toLocaleString(toDateTimeLocale(language))
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
                      {pickText('Regression warnings', 'Regressionswarnungen')}
                    </div>
                    {!selectedVersion.attribution?.warnings ||
                    selectedVersion.attribution.warnings.length === 0 ? (
                      <div className="text-sm text-emerald-300">
                        {pickText(
                          'No regression warning triggered for the observed window.',
                          'Fuer das beobachtete Fenster wurde keine Regressionswarnung ausgeloest.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-1 text-sm text-rose-300">
                        {selectedVersion.attribution.warnings.map((item) => (
                          <div key={item}>• {item}</div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4 mt-4 space-y-4">
                    <div>
                      <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                        {pickText('Similar strategy versions', 'Aehnliche Strategieversionen')}
                      </div>
                      <div className="text-sm text-nofx-text-muted mt-2">
                        {pickText(
                          'Semantic matches from prior strategy patches for this trader.',
                          'Semantische Treffer aus frueheren Strategie-Patches dieses Traders.'
                        )}
                      </div>
                    </div>

                    {similarVersionsLoading ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText('Loading similar strategy versions…', 'Aehnliche Strategieversionen werden geladen…')}
                      </div>
                    ) : similarVersionsError ? (
                      <div className="text-sm text-amber-200">
                        {similarVersionsError}
                      </div>
                    ) : similarVersions.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No similar strategy versions were found yet.',
                          'Es wurden noch keine aehnlichen Strategieversionen gefunden.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-3">
                        {similarVersions.map((hit) => (
                          <div
                            key={`${hit.document.id}-${hit.document.source_id}`}
                            className="rounded-xl border border-white/10 bg-black/20 p-4"
                          >
                            <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-3">
                              <div>
                                <div className="font-semibold">
                                  {hit.document.title || hit.document.source_id}
                                </div>
                                <div className="text-xs text-nofx-text-muted mt-1">
                                  {formatSemanticSimilarityScore(
                                    hit.similarity_score,
                                    language
                                  )}{' '}
                                  · {pickText('updated', 'aktualisiert')}{' '}
                                  {hit.document.source_updated_at
                                    ? new Date(
                                        hit.document.source_updated_at
                                      ).toLocaleString(toDateTimeLocale(language))
                                    : '-'}
                                </div>
                              </div>
                              <button
                                onClick={() =>
                                  setSelectedVersionId(hit.document.source_id)
                                }
                                className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-sm"
                              >
                                {pickText('Focus version', 'Version fokussieren')}
                              </button>
                            </div>
                            <div className="text-sm text-nofx-text-muted mt-3">
                              {hit.document.summary ||
                                pickText('No summary stored', 'Keine Zusammenfassung gespeichert')}
                            </div>
                          </div>
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
