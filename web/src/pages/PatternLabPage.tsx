import { useEffect, useState } from 'react'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { NofxSelect } from '../components/ui/select'
import { api } from '../lib/api'
import { notify } from '../lib/notify'
import type {
  DealReviewLearnedPattern,
  DealReviewLearnedPatternSummary,
  TraderInfo,
} from '../types'

interface PatternLabPageProps {
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
}

interface PatternLabFilters {
  patternId: string
  symbol: string
  side: string
  scopeType: string
  patternClass: string
  validationLabel: string
  feature: string
  limit: string
}

const sideOptions = [
  { value: '', label: 'All sides' },
  { value: 'LONG', label: 'LONG' },
  { value: 'SHORT', label: 'SHORT' },
]

const scopeOptions = [
  { value: '', label: 'All scopes' },
  { value: 'trader_local', label: 'Trader local' },
  { value: 'symbol', label: 'Symbol override' },
]

const patternClassOptions = [
  { value: '', label: 'All pattern classes' },
  { value: 'positive_edge', label: 'Positive edge' },
  { value: 'negative_edge', label: 'Anti-edge' },
]

const validationLabelOptions = [
  { value: '', label: 'All labels' },
  { value: 'confirmed', label: 'Confirmed' },
  { value: 'candidate', label: 'Candidate' },
  { value: 'insufficient_evidence', label: 'Need evidence' },
  { value: 'false_positive', label: 'False positive' },
  { value: 'reverse_risk', label: 'Reverse risk' },
  { value: 'drifting', label: 'Drifting' },
  { value: 'expired', label: 'Expired' },
]

const limitOptions = [
  { value: '24', label: '24 items' },
  { value: '50', label: '50 items' },
  { value: '100', label: '100 items' },
]

function createDefaultFilters(): PatternLabFilters {
  return {
    patternId: '',
    symbol: '',
    side: '',
    scopeType: '',
    patternClass: '',
    validationLabel: '',
    feature: '',
    limit: '50',
  }
}

function formatMoney(value: number): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}`
}

function formatPct(value: number): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
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

function formatTimestamp(value?: string): string {
  if (!value || value.startsWith('0001-01-01')) return '-'
  return new Date(value).toLocaleString()
}

function formatPatternClass(value?: string): string {
  switch (value) {
    case 'positive_edge':
      return 'Positive edge'
    case 'negative_edge':
      return 'Anti-edge'
    default:
      return value || '-'
  }
}

function patternClassClasses(value?: string): string {
  switch (value) {
    case 'positive_edge':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'negative_edge':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatValidationLabel(value?: string): string {
  switch (value) {
    case 'confirmed':
      return 'Confirmed'
    case 'candidate':
      return 'Candidate'
    case 'insufficient_evidence':
      return 'Need evidence'
    case 'false_positive':
      return 'False positive'
    case 'reverse_risk':
      return 'Reverse risk'
    case 'drifting':
      return 'Drifting'
    case 'expired':
      return 'Expired'
    default:
      return value || '-'
  }
}

function validationLabelClasses(value?: string): string {
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
    default:
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
  }
}

function formatPatternScope(pattern?: DealReviewLearnedPattern | null): string {
  if (!pattern) return '-'
  if (pattern.scope_type === 'symbol') {
    return pattern.symbol ? `Symbol ${pattern.symbol}` : 'Symbol'
  }
  return 'Trader local'
}

function formatRecommendedUse(value?: string): string {
  switch (value) {
    case 'review_hint':
      return 'Review hint'
    case 'prompt_hint':
      return 'Prompt hint'
    case 'config_candidate':
      return 'Config candidate'
    case 'monitor_only':
      return 'Monitor only'
    case 'expired_do_not_use':
      return 'Expired'
    default:
      return value || '-'
  }
}

function normalizeFilters(raw: PatternLabFilters): PatternLabFilters {
  return {
    patternId: raw.patternId.trim(),
    symbol: raw.symbol.trim().toUpperCase(),
    side: raw.side.trim().toUpperCase(),
    scopeType: raw.scopeType.trim(),
    patternClass: raw.patternClass.trim(),
    validationLabel: raw.validationLabel.trim(),
    feature: raw.feature.trim().toLowerCase(),
    limit: raw.limit.trim() || '50',
  }
}

function readFiltersFromURL(): PatternLabFilters {
  const params = new URLSearchParams(window.location.search)
  return normalizeFilters({
    patternId: params.get('pattern_id') || '',
    symbol: params.get('symbol') || '',
    side: params.get('side') || '',
    scopeType: params.get('scope_type') || '',
    patternClass: params.get('pattern_class') || '',
    validationLabel: params.get('validation_label') || '',
    feature: params.get('feature') || '',
    limit: params.get('limit') || '50',
  })
}

function writeFiltersToURL(filters: PatternLabFilters): void {
  const next = normalizeFilters(filters)
  const url = new URL(window.location.href)
  url.pathname = '/pattern-lab'
  url.search = ''
  if (next.patternId) url.searchParams.set('pattern_id', next.patternId)
  if (next.symbol) url.searchParams.set('symbol', next.symbol)
  if (next.side) url.searchParams.set('side', next.side)
  if (next.scopeType) url.searchParams.set('scope_type', next.scopeType)
  if (next.patternClass) url.searchParams.set('pattern_class', next.patternClass)
  if (next.validationLabel) {
    url.searchParams.set('validation_label', next.validationLabel)
  }
  if (next.feature) url.searchParams.set('feature', next.feature)
  if (next.limit && next.limit !== '50') url.searchParams.set('limit', next.limit)
  window.history.replaceState({}, '', url.toString())
}

function buildReviewQueryFromPattern(
  pattern: DealReviewLearnedPattern,
  evidenceCaseId?: string
): URLSearchParams {
  const params = new URLSearchParams()
  params.set('status', 'CLOSED')
  if (pattern.symbol) params.set('symbol', pattern.symbol)
  if (pattern.side) params.set('side', pattern.side)
  if (evidenceCaseId) params.set('case_id', evidenceCaseId)

  ;(pattern.feature_set || []).forEach((feature) => {
    const [prefix, ...rest] = feature.split(':')
    const value = rest.join(':')
    if (!prefix || !value) return
    switch (prefix) {
      case 'bucket':
        params.set('open_selection_bucket', value)
        break
      case 'trend':
        params.set('open_trend_regime', value)
        break
      case 'vol':
        params.set('open_volatility_regime', value)
        break
      case 'btc':
        params.set('open_btc_strength_regime', value)
        break
      case 'funding':
        params.set('open_funding_regime', value)
        break
      case 'oi':
        params.set('open_oi_regime', value)
        break
      case 'session':
        params.set('open_session_bucket', value)
        break
      case 'weekday':
        params.set('open_weekday_bucket', value)
        break
      case 'venue':
        params.set('open_venue_tier', value)
        break
      case 'liq':
        params.set('open_liquidity_tier', value)
        break
      case 'spread':
        params.set('open_spread_bucket', value)
        break
      case 'slip':
        params.set('open_slippage_bucket', value)
        break
      default:
        break
    }
  })
  return params
}

function openDealReview(pattern: DealReviewLearnedPattern, evidenceCaseId?: string): void {
  const url = new URL(window.location.href)
  url.pathname = '/deal-review'
  url.search = buildReviewQueryFromPattern(pattern, evidenceCaseId).toString()
  window.history.pushState({}, '', url.toString())
  window.dispatchEvent(new PopStateEvent('popstate'))
}

function strongestRiskScore(pattern: DealReviewLearnedPattern): number {
  return Math.max(
    pattern.drift_score || 0,
    pattern.reverse_risk_score || 0,
    pattern.false_positive_score || 0
  )
}

export function PatternLabPage({
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
}: PatternLabPageProps) {
  const [filters, setFilters] = useState<PatternLabFilters>(createDefaultFilters())
  const [appliedFilters, setAppliedFilters] = useState<PatternLabFilters>(
    createDefaultFilters()
  )
  const [items, setItems] = useState<DealReviewLearnedPattern[]>([])
  const [summary, setSummary] = useState<DealReviewLearnedPatternSummary | null>(
    null
  )
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [generatedAt, setGeneratedAt] = useState('')
  const selectedTrader = traders?.find((item) => item.trader_id === selectedTraderId)

  useEffect(() => {
    const next = readFiltersFromURL()
    setFilters(next)
    setAppliedFilters(next)

    const handlePopState = () => {
      const updated = readFiltersFromURL()
      setFilters(updated)
      setAppliedFilters(updated)
    }

    window.addEventListener('popstate', handlePopState)
    return () => {
      window.removeEventListener('popstate', handlePopState)
    }
  }, [])

  useEffect(() => {
    if (!selectedTraderId) {
      setItems([])
      setSummary(null)
      setGeneratedAt('')
      return
    }

    setLoading(true)
    setError(null)
    void api
      .getDealReviewLearnedPatterns(selectedTraderId, {
        pattern_id: appliedFilters.patternId || undefined,
        symbol: appliedFilters.symbol || undefined,
        side: appliedFilters.side || undefined,
        scope_type: appliedFilters.scopeType || undefined,
        pattern_class: appliedFilters.patternClass || undefined,
        validation_label: appliedFilters.validationLabel || undefined,
        feature: appliedFilters.feature || undefined,
        limit: Number(appliedFilters.limit) || 50,
      })
      .then((result) => {
        setItems(result.items || [])
        setSummary(result.summary || null)
        setGeneratedAt(result.generated_at || '')
      })
      .catch((err) => {
        const message =
          err instanceof Error ? err.message : 'Failed to fetch learned patterns'
        setError(message)
        setItems([])
        setSummary(null)
        setGeneratedAt('')
      })
      .finally(() => setLoading(false))
  }, [selectedTraderId, appliedFilters])

  const applyFilters = () => {
    const next = normalizeFilters(filters)
    setFilters(next)
    setAppliedFilters(next)
    writeFiltersToURL(next)
  }

  const resetFilters = () => {
    const next = createDefaultFilters()
    setFilters(next)
    setAppliedFilters(next)
    writeFiltersToURL(next)
  }

  const applyQuickFilter = (patch: Partial<PatternLabFilters>) => {
    const next = normalizeFilters({ ...createDefaultFilters(), ...filters, ...patch })
    setFilters(next)
    setAppliedFilters(next)
    writeFiltersToURL(next)
  }

  const topRiskPatterns = [...items]
    .filter((item) =>
      ['false_positive', 'reverse_risk', 'drifting'].includes(
        item.validation_label || ''
      )
    )
    .sort((left, right) => strongestRiskScore(right) - strongestRiskScore(left))
    .slice(0, 4)

  const topDriftingPatterns = [...items]
    .filter((item) => item.validation_label === 'drifting')
    .sort((left, right) => (right.drift_score || 0) - (left.drift_score || 0))
    .slice(0, 4)

  const visibleNotes = summary?.notes || []

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
                Pattern Lab
              </div>
              <h1 className="text-3xl font-semibold text-nofx-text-main">
                Learned pattern mining and drift watch
              </h1>
              <p className="text-sm text-nofx-text-muted mt-2">
                Inspect learned feature combinations separately from deal review:
                positive edges, anti-patterns, symbol overrides, reverse-risk, and
                drift under the currently selected trader.
              </p>
            </div>
            <div className="w-full lg:w-80 space-y-2">
              <label className="text-xs text-nofx-text-muted block">
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
                onClick={() => {
                  const url = new URL(window.location.href)
                  url.pathname = '/deal-review'
                  if (filters.symbol) url.searchParams.set('symbol', filters.symbol)
                  if (filters.side) url.searchParams.set('side', filters.side)
                  window.history.pushState({}, '', url.toString())
                  window.dispatchEvent(new PopStateEvent('popstate'))
                }}
                className="w-full h-10 rounded-lg border border-white/10 bg-black/20 text-sm font-medium"
              >
                Open Deal Review
              </button>
            </div>
          </div>
        </div>

        <div className="nofx-glass rounded-xl p-5 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold">Filters</div>
              <div className="text-xs text-nofx-text-muted mt-1">
                Filter the learned pattern corpus by symbol, side, scope, class,
                validation label, and feature token.
              </div>
            </div>
            <div className="text-xs text-nofx-text-muted">
              {selectedTrader ? selectedTrader.trader_name : 'No trader selected'}{' '}
              {generatedAt ? `· generated ${formatTimestamp(generatedAt)}` : ''}
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3">
            <input
              value={filters.symbol}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  symbol: event.target.value.toUpperCase(),
                }))
              }
              placeholder="Symbol"
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
            />
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.side}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, side: value }))
                }
                options={sideOptions}
              />
            </div>
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.scopeType}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, scopeType: value }))
                }
                options={scopeOptions}
              />
            </div>
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.patternClass}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, patternClass: value }))
                }
                options={patternClassOptions}
              />
            </div>
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.validationLabel}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, validationLabel: value }))
                }
                options={validationLabelOptions}
              />
            </div>
            <input
              value={filters.feature}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  feature: event.target.value.toLowerCase(),
                }))
              }
              placeholder="Feature token, e.g. bucket_breakout"
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
            />
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.limit}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, limit: value }))
                }
                options={limitOptions}
              />
            </div>
            <div className="flex items-center gap-3">
              <button
                onClick={applyFilters}
                className="flex-1 h-11 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 text-nofx-gold font-semibold"
              >
                Apply
              </button>
              <button
                onClick={resetFilters}
                className="flex-1 h-11 rounded-lg border border-white/10 bg-black/20 font-semibold"
              >
                Reset
              </button>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => applyQuickFilter(createDefaultFilters())}
              className="h-8 px-3 rounded-full border border-white/10 bg-black/20 text-xs"
            >
              All patterns
            </button>
            <button
              onClick={() => applyQuickFilter({ patternClass: 'positive_edge' })}
              className="h-8 px-3 rounded-full border border-emerald-400/20 bg-emerald-500/10 text-xs text-emerald-200"
            >
              Positive edges
            </button>
            <button
              onClick={() => applyQuickFilter({ patternClass: 'negative_edge' })}
              className="h-8 px-3 rounded-full border border-rose-400/20 bg-rose-500/10 text-xs text-rose-200"
            >
              Anti-patterns
            </button>
            <button
              onClick={() => applyQuickFilter({ validationLabel: 'confirmed' })}
              className="h-8 px-3 rounded-full border border-sky-400/20 bg-sky-500/10 text-xs text-sky-200"
            >
              Confirmed
            </button>
            <button
              onClick={() => applyQuickFilter({ validationLabel: 'reverse_risk' })}
              className="h-8 px-3 rounded-full border border-amber-400/20 bg-amber-500/10 text-xs text-amber-200"
            >
              Reverse risk
            </button>
            <button
              onClick={() => applyQuickFilter({ validationLabel: 'drifting' })}
              className="h-8 px-3 rounded-full border border-orange-400/20 bg-orange-500/10 text-xs text-orange-200"
            >
              Drifting
            </button>
            <button
              onClick={() => applyQuickFilter({ scopeType: 'symbol' })}
              className="h-8 px-3 rounded-full border border-white/10 bg-white/5 text-xs"
            >
              Symbol overrides
            </button>
          </div>

          {appliedFilters.patternId ? (
            <div className="rounded-lg border border-sky-400/20 bg-sky-500/10 px-3 py-2 text-xs text-sky-100">
              Direct pattern focus active: <span className="font-mono">{appliedFilters.patternId}</span>
            </div>
          ) : null}
        </div>

        <div className="grid grid-cols-2 xl:grid-cols-6 gap-3 text-xs">
          <div className="nofx-glass rounded-xl px-4 py-4">
            <div className="text-nofx-text-muted">Tracked patterns</div>
            <div className="text-2xl font-semibold mt-1">
              {summary?.total_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-emerald-400/15 bg-emerald-500/5">
            <div className="text-nofx-text-muted">Positive edges</div>
            <div className="text-2xl font-semibold mt-1 text-emerald-300">
              {summary?.positive_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-rose-400/15 bg-rose-500/5">
            <div className="text-nofx-text-muted">Anti-patterns</div>
            <div className="text-2xl font-semibold mt-1 text-rose-300">
              {summary?.negative_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-sky-400/15 bg-sky-500/5">
            <div className="text-nofx-text-muted">Confirmed</div>
            <div className="text-2xl font-semibold mt-1 text-sky-200">
              {summary?.confirmed_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-amber-400/15 bg-amber-500/5">
            <div className="text-nofx-text-muted">Reverse / drift</div>
            <div className="text-2xl font-semibold mt-1 text-amber-200">
              {(summary?.reverse_risk_count || 0) + (summary?.drifting_count || 0)}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-white/10 bg-white/5">
            <div className="text-nofx-text-muted">Loaded items</div>
            <div className="text-2xl font-semibold mt-1">{items.length}</div>
          </div>
        </div>

        {visibleNotes.length > 0 && (
          <div className="space-y-2">
            {visibleNotes.map((note) => (
              <div
                key={note}
                className="nofx-glass rounded-xl px-4 py-3 text-sm text-nofx-text-muted"
              >
                {note}
              </div>
            ))}
          </div>
        )}

        <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-emerald-200">
              Top Positive Patterns
            </div>
            {!summary?.top_positive_patterns ||
            summary.top_positive_patterns.length === 0 ? (
              <div className="text-sm text-nofx-text-muted">
                No positive patterns surfaced for this slice yet.
              </div>
            ) : (
              summary.top_positive_patterns.map((pattern) => (
                <div
                  key={`positive-${pattern.id}`}
                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{pattern.pattern_signature}</span>
                    <span
                      className={`px-2 py-1 rounded-full text-[11px] border ${validationLabelClasses(
                        pattern.validation_label
                      )}`}
                    >
                      {formatValidationLabel(pattern.validation_label)}
                    </span>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {formatPatternScope(pattern)} · avg {formatPct(pattern.avg_pnl_pct)} ·
                    lift {formatPct(pattern.lift_avg_pnl_pct)}
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-rose-200">
              Top Anti-Patterns
            </div>
            {!summary?.top_negative_patterns ||
            summary.top_negative_patterns.length === 0 ? (
              <div className="text-sm text-nofx-text-muted">
                No anti-patterns surfaced for this slice yet.
              </div>
            ) : (
              summary.top_negative_patterns.map((pattern) => (
                <div
                  key={`negative-${pattern.id}`}
                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{pattern.pattern_signature}</span>
                    <span
                      className={`px-2 py-1 rounded-full text-[11px] border ${validationLabelClasses(
                        pattern.validation_label
                      )}`}
                    >
                      {formatValidationLabel(pattern.validation_label)}
                    </span>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {formatPatternScope(pattern)} · avg {formatPct(pattern.avg_pnl_pct)} ·
                    lift {formatPct(pattern.lift_avg_pnl_pct)}
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-amber-200">
              Highest Risk Watchlist
            </div>
            {topRiskPatterns.length === 0 ? (
              <div className="text-sm text-nofx-text-muted">
                No false-positive, reverse-risk, or drifting patterns in this
                slice yet.
              </div>
            ) : (
              topRiskPatterns.map((pattern) => (
                <div
                  key={`risk-${pattern.id}`}
                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{pattern.pattern_signature}</span>
                    <span
                      className={`px-2 py-1 rounded-full text-[11px] border ${validationLabelClasses(
                        pattern.validation_label
                      )}`}
                    >
                      {formatValidationLabel(pattern.validation_label)}
                    </span>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    drift {formatPct((pattern.drift_score || 0) * 100)} · reverse{' '}
                    {formatPct((pattern.reverse_risk_score || 0) * 100)} · FP{' '}
                    {formatPct((pattern.false_positive_score || 0) * 100)}
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-sky-200">
              Symbol Overrides
            </div>
            {!summary?.top_symbol_overrides ||
            summary.top_symbol_overrides.length === 0 ? (
              <div className="text-sm text-nofx-text-muted">
                No symbol-specific overrides surfaced for this slice yet.
              </div>
            ) : (
              summary.top_symbol_overrides.map((pattern) => (
                <div
                  key={`override-${pattern.id}`}
                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">
                      {pattern.symbol} · {pattern.pattern_signature}
                    </span>
                    <span
                      className={`px-2 py-1 rounded-full text-[11px] border ${patternClassClasses(
                        pattern.pattern_class
                      )}`}
                    >
                      {formatPatternClass(pattern.pattern_class)}
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

        {topDriftingPatterns.length > 0 && (
          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-orange-200">
              Biggest Recent Drifts
            </div>
            <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
              {topDriftingPatterns.map((pattern) => (
                <div
                  key={`drift-${pattern.id}`}
                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{pattern.pattern_signature}</span>
                    <span
                      className={`px-2 py-1 rounded-full text-[11px] border ${validationLabelClasses(
                        pattern.validation_label
                      )}`}
                    >
                      {formatValidationLabel(pattern.validation_label)}
                    </span>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {formatPatternScope(pattern)} · drift{' '}
                    {formatPct((pattern.drift_score || 0) * 100)} · recent support{' '}
                    {pattern.recent_support_count || 0}/{pattern.recent_sample_count || 0}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        <div className="nofx-glass rounded-xl p-5 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold">Pattern corpus</div>
              <div className="text-xs text-nofx-text-muted mt-1">
                Full list for the current slice. Use these cards to inspect feature
                combinations and jump directly into deal review for the matching
                cohort.
              </div>
            </div>
            <div className="text-xs text-nofx-text-muted">
              {loading ? 'Refreshing…' : `${items.length} item(s) loaded`}
            </div>
          </div>

          {error && (
            <div className="rounded-lg border border-rose-400/20 bg-rose-500/10 px-3 py-2 text-sm text-rose-200">
              {error}
            </div>
          )}

          {!loading && items.length === 0 ? (
            <div className="text-sm text-nofx-text-muted">
              No learned patterns matched the current filter slice yet.
            </div>
          ) : (
            <div className="space-y-4">
              {items.map((pattern) => (
                <div
                  key={pattern.id}
                  className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4"
                >
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-semibold">{pattern.pattern_signature}</span>
                        <span
                          className={`px-2 py-1 rounded-full text-[11px] border ${patternClassClasses(
                            pattern.pattern_class
                          )}`}
                        >
                          {formatPatternClass(pattern.pattern_class)}
                        </span>
                        <span
                          className={`px-2 py-1 rounded-full text-[11px] border ${validationLabelClasses(
                            pattern.validation_label
                          )}`}
                        >
                          {formatValidationLabel(pattern.validation_label)}
                        </span>
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                          {formatPatternScope(pattern)}
                        </span>
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                          {formatRecommendedUse(pattern.recommended_use)}
                        </span>
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {pattern.regime_signature || 'No regime signature stored'}
                      </div>
                    </div>
                    <div className="text-right text-xs text-nofx-text-muted">
                      <div>
                        built {formatTimestamp(pattern.built_at)}
                      </div>
                      <div className="mt-1">
                        observed {formatTimestamp(pattern.last_observed_at)}
                      </div>
                    </div>
                  </div>

                  <div className="text-sm text-nofx-text-muted">{pattern.summary}</div>

                  {pattern.validation_alert && (
                    <div className="rounded-lg border border-sky-400/15 bg-sky-500/5 px-3 py-2 text-xs text-sky-100">
                      {pattern.validation_alert}
                    </div>
                  )}

                  {pattern.feature_set && pattern.feature_set.length > 0 && (
                    <div className="flex flex-wrap gap-2">
                      {pattern.feature_set.map((feature) => (
                        <button
                          key={`${pattern.id}-${feature}`}
                          onClick={() => {
                            const nextFeature = feature.replace(':', '_')
                            setFilters((current) => ({ ...current, feature: nextFeature }))
                            applyQuickFilter({ feature: nextFeature })
                          }}
                          className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white"
                        >
                          {feature}
                        </button>
                      ))}
                    </div>
                  )}

                  <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-6 gap-3 text-xs">
                    <div>
                      Samples:{' '}
                      <span className="text-white">{pattern.sample_count}</span>
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
                          pattern.avg_pnl >= 0 ? 'text-emerald-300' : 'text-rose-300'
                        }
                      >
                        {formatMoney(pattern.avg_pnl)} / {formatPct(pattern.avg_pnl_pct)}
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
                    <div>
                      Stability:{' '}
                      <span className="text-white">
                        {formatPct((pattern.stability_score || 0) * 100)}
                      </span>
                    </div>
                    <div>
                      Holdout:{' '}
                      <span className="text-white">
                        {pattern.validation_support_count || 0}/
                        {pattern.validation_sample_count || 0}
                      </span>
                    </div>
                    <div>
                      Recent:{' '}
                      <span className="text-white">
                        {pattern.recent_support_count || 0}/
                        {pattern.recent_sample_count || 0}
                      </span>
                    </div>
                    <div>
                      Drift:{' '}
                      <span
                        className={
                          (pattern.drift_score || 0) >= 0.5
                            ? 'text-orange-300'
                            : 'text-white'
                        }
                      >
                        {formatPct((pattern.drift_score || 0) * 100)}
                      </span>
                    </div>
                    <div>
                      Reverse:{' '}
                      <span className="text-amber-300">
                        {formatPct((pattern.reverse_risk_score || 0) * 100)}
                      </span>
                    </div>
                    <div>
                      FP:{' '}
                      <span className="text-rose-300">
                        {formatPct((pattern.false_positive_score || 0) * 100)}
                      </span>
                    </div>
                    <div>
                      Avg hold:{' '}
                      <span className="text-white">
                        {formatHold(pattern.avg_hold_ms || 0)}
                      </span>
                    </div>
                  </div>

                  <div className="flex flex-wrap gap-2">
                    <button
                      onClick={() => openDealReview(pattern)}
                      className="h-9 px-3 rounded-lg border border-white/10 bg-white/5 text-xs font-medium"
                    >
                      Open cohort in review
                    </button>
                    {pattern.symbol && (
                      <button
                        onClick={() =>
                          applyQuickFilter({
                            symbol: pattern.symbol,
                            side: pattern.side,
                            scopeType: pattern.scope_type,
                          })
                        }
                        className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-xs font-medium"
                      >
                        Focus {pattern.symbol}
                      </button>
                    )}
                    <button
                      onClick={() => {
                        navigator.clipboard
                          .writeText(pattern.pattern_signature)
                          .then(() => notify.success('Pattern signature copied'))
                          .catch(() => notify.error('Failed to copy pattern signature'))
                      }}
                      className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-xs font-medium"
                    >
                      Copy signature
                    </button>
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
                              · {evidence.outcome || '-'} · {formatMoney(evidence.realized_pnl)} /{' '}
                              {formatPct(evidence.realized_pnl_pct)} ·{' '}
                              {formatHold(evidence.hold_duration_ms || 0)}
                            </div>
                            <button
                              onClick={() => openDealReview(pattern, evidence.case_id)}
                              className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs"
                            >
                              Open deal
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
      </div>
    </DeepVoidBackground>
  )
}
