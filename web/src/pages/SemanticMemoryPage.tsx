import { useEffect, useMemo, useState } from 'react'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { NofxSelect } from '../components/ui/select'
import { api } from '../lib/api'
import { notify } from '../lib/notify'
import type {
  SemanticMemoryBenchmarkRun,
  SemanticMemoryCorpusStatus,
  SemanticMemoryQueryResult,
  SemanticMemorySearchPresetDetail,
  TraderInfo,
} from '../types'

interface SemanticMemoryPageProps {
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
}

const docScopeOptions = [
  { value: 'all', label: 'All corpora' },
  { value: 'deal_review_case', label: 'Deal review cases' },
  { value: 'autonomous_optimizer_run', label: 'Optimizer runs' },
  { value: 'autonomous_optimizer_backlog_item', label: 'Optimizer backlog' },
  { value: 'strategy_version', label: 'Strategy versions' },
]

const outcomeOptions = [
  { value: '', label: 'Any outcome' },
  { value: 'profit', label: 'Profit' },
  { value: 'loss', label: 'Loss' },
  { value: 'flat', label: 'Flat' },
  { value: 'open', label: 'Open' },
]

const runStatusOptions = [
  { value: '', label: 'Any run status' },
  { value: 'blocked_by_gate', label: 'Blocked by gate' },
  { value: 'deferred_for_next_window', label: 'Deferred' },
  { value: 'auto_applied', label: 'Auto applied' },
  { value: 'monitoring', label: 'Monitoring' },
  { value: 'rollback_pending', label: 'Rollback pending' },
  { value: 'rolled_back', label: 'Rolled back' },
  { value: 'kept', label: 'Kept' },
  { value: 'backlog_only', label: 'Backlog only' },
  { value: 'insufficient_evidence', label: 'Insufficient evidence' },
]

function normalizeTime(value?: string): string {
  if (!value || value.startsWith('0001-01-01')) return '-'
  return new Date(value).toLocaleString()
}

function formatPct(part: number, total: number): string {
  if (!total) return '0%'
  return `${Math.round((part / total) * 100)}%`
}

function formatInteger(value?: number): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return value.toLocaleString()
}

function formatUsd(value?: number): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: value < 1 ? 4 : 2,
    maximumFractionDigits: value < 1 ? 4 : 2,
  }).format(value)
}

function formatDocType(value?: string): string {
  if (!value) return '-'
  return value
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase())
}

function formatSemanticSimilarityScore(value?: number): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return `${Math.round(Math.max(0, Math.min(1, value)) * 100)}% match`
}

function formatRatio(value?: number): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return `${Math.round(Math.max(0, Math.min(1, value)) * 100)}%`
}

export function SemanticMemoryPage({
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
}: SemanticMemoryPageProps) {
  const [status, setStatus] = useState<SemanticMemoryCorpusStatus | null>(null)
  const [searchResult, setSearchResult] = useState<SemanticMemoryQueryResult | null>(null)
  const [presets, setPresets] = useState<SemanticMemorySearchPresetDetail[]>([])
  const [benchmarks, setBenchmarks] = useState<SemanticMemoryBenchmarkRun[]>([])
  const [loadingStatus, setLoadingStatus] = useState(false)
  const [runningSearch, setRunningSearch] = useState(false)
  const [runningBenchmark, setRunningBenchmark] = useState(false)
  const [savingPreset, setSavingPreset] = useState(false)
  const [deletingPresetId, setDeletingPresetId] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [docScope, setDocScope] = useState('all')
  const [outcome, setOutcome] = useState('')
  const [runStatus, setRunStatus] = useState('')
  const [closeReason, setCloseReason] = useState('')
  const [limit, setLimit] = useState('12')
  const [selectedPresetId, setSelectedPresetId] = useState('')
  const [presetName, setPresetName] = useState('')
  const [error, setError] = useState<string | null>(null)

  const selectedTrader = traders?.find((item) => item.trader_id === selectedTraderId)
  const selectedPreset =
    presets.find((item) => item.preset.id === selectedPresetId) || null

  useEffect(() => {
    if (!selectedTraderId) {
      setStatus(null)
      return
    }
    setLoadingStatus(true)
    setError(null)
    void api
      .getTraderSemanticMemoryStatus(selectedTraderId)
      .then((result) => setStatus(result))
      .catch((err) => {
        const message =
          err instanceof Error ? err.message : 'Failed to fetch semantic memory status'
        setError(message)
      })
      .finally(() => setLoadingStatus(false))
  }, [selectedTraderId])

  useEffect(() => {
    setSearchResult(null)
    setError(null)
  }, [selectedTraderId])

  useEffect(() => {
    if (!selectedTraderId) {
      setPresets([])
      setBenchmarks([])
      setSelectedPresetId('')
      setPresetName('')
      return
    }
    void api
      .getTraderSemanticMemoryPresets(selectedTraderId)
      .then((result) => setPresets(result))
      .catch((err) => {
        const message =
          err instanceof Error ? err.message : 'Failed to fetch semantic memory presets'
        setError(message)
      })
    void api
      .getTraderSemanticMemoryBenchmarks(selectedTraderId)
      .then((result) => setBenchmarks(result))
      .catch((err) => {
        const message =
          err instanceof Error ? err.message : 'Failed to fetch semantic memory benchmarks'
        setError(message)
      })
  }, [selectedTraderId])

  useEffect(() => {
    if (selectedPreset) {
      setPresetName(selectedPreset.preset.name)
    }
  }, [selectedPreset])

  const totalEmbeddedPct = useMemo(() => {
    if (!status?.total_documents) return '0%'
    return formatPct(status.embedded_documents, status.total_documents)
  }, [status])

  const latestBenchmark = benchmarks[0] || null

  const docTypes = useMemo(() => {
    if (docScope === 'all') return undefined
    return [docScope]
  }, [docScope])

  const buildPresetConfig = () => ({
    query: query.trim(),
    doc_scope: docScope,
    doc_types: docTypes,
    outcome: outcome || '',
    status: runStatus || '',
    close_reason: closeReason.trim(),
    limit: Number(limit) || 12,
  })

  const applyPresetConfig = (config?: Record<string, unknown>) => {
    const nextQuery =
      typeof config?.query === 'string' ? config.query.trim() : ''
    const nextDocScope =
      typeof config?.doc_scope === 'string' && config.doc_scope.trim()
        ? config.doc_scope.trim()
        : Array.isArray(config?.doc_types) && config?.doc_types.length === 1
          ? String(config.doc_types[0] || '').trim() || 'all'
          : 'all'
    const nextOutcome =
      typeof config?.outcome === 'string' ? config.outcome.trim() : ''
    const nextRunStatus =
      typeof config?.status === 'string' ? config.status.trim() : ''
    const nextCloseReason =
      typeof config?.close_reason === 'string' ? config.close_reason : ''
    const nextLimit =
      typeof config?.limit === 'number'
        ? String(config.limit)
        : typeof config?.limit === 'string' && config.limit.trim()
          ? config.limit
          : '12'

    setQuery(nextQuery)
    setDocScope(nextDocScope || 'all')
    setOutcome(nextOutcome)
    setRunStatus(nextRunStatus)
    setCloseReason(nextCloseReason)
    setLimit(nextLimit)
    setSearchResult(null)
    setError(null)
  }

  const onPresetSelect = (presetId: string) => {
    setSelectedPresetId(presetId)
    const nextPreset = presets.find((item) => item.preset.id === presetId)
    setPresetName(nextPreset?.preset.name || '')
    applyPresetConfig((nextPreset?.config as Record<string, unknown>) || undefined)
  }

  const savePreset = async () => {
    if (!selectedTraderId) return
    if (!presetName.trim()) {
      notify.error('Enter a preset name first')
      return
    }
    if (!query.trim()) {
      notify.error('Semantic search presets need a query')
      return
    }
    setSavingPreset(true)
    try {
      const items = await api.saveTraderSemanticMemoryPreset(selectedTraderId, {
        id: selectedPresetId || undefined,
        name: presetName.trim(),
        config: buildPresetConfig(),
      })
      setPresets(items)
      const exactMatch =
        items.find(
          (item) =>
            item.preset.id === selectedPresetId ||
            item.preset.name === presetName.trim()
        ) || null
      if (exactMatch) {
        setSelectedPresetId(exactMatch.preset.id)
      }
      notify.success('Semantic search preset saved')
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Failed to save semantic memory preset'
      setError(message)
      notify.error(message)
    } finally {
      setSavingPreset(false)
    }
  }

  const deletePreset = async () => {
    if (!selectedTraderId || !selectedPresetId) return
    setDeletingPresetId(selectedPresetId)
    try {
      const items = await api.deleteTraderSemanticMemoryPreset(
        selectedTraderId,
        selectedPresetId
      )
      setPresets(items)
      setSelectedPresetId('')
      setPresetName('')
      notify.success('Semantic search preset deleted')
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Failed to delete semantic memory preset'
      setError(message)
      notify.error(message)
    } finally {
      setDeletingPresetId(null)
    }
  }

  const submitSearch = async () => {
    if (!selectedTraderId) return
    if (!query.trim()) {
      notify.error('Enter a semantic search query first')
      return
    }
    setRunningSearch(true)
    setError(null)
    try {
      const result = await api.searchTraderSemanticMemory(selectedTraderId, {
        query: query.trim(),
        doc_types: docTypes,
        limit: Number(limit) || 12,
        outcome: outcome || undefined,
        status: runStatus || undefined,
        close_reason: closeReason || undefined,
      })
      setSearchResult(result)
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Failed to search semantic memory'
      setError(message)
      notify.error(message)
    } finally {
      setRunningSearch(false)
    }
  }

  const runBenchmark = async () => {
    if (!selectedTraderId) return
    setRunningBenchmark(true)
    setError(null)
    try {
      const run = await api.runTraderSemanticMemoryBenchmark(selectedTraderId, {
        sample_per_doc_type: 12,
        top_k: 5,
      })
      setBenchmarks((current) => [run, ...current.filter((item) => item.id !== run.id)].slice(0, 8))
      notify.success('Semantic-memory benchmark completed')
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Failed to run semantic memory benchmark'
      setError(message)
      notify.error(message)
    } finally {
      setRunningBenchmark(false)
    }
  }

  const openSourceLink = (sourceLink?: string) => {
    if (!sourceLink) return
    const url = new URL(window.location.href)
    const target = new URL(sourceLink, window.location.origin)
    url.pathname = target.pathname
    url.search = target.search
    window.history.pushState({}, '', url.toString())
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  if (tradersError) {
    return <div className="p-8 text-red-400">Failed to load traders: {tradersError.message}</div>
  }

  return (
    <DeepVoidBackground className="min-h-screen pb-12" disableAnimation>
      <div className="w-full px-4 md:px-8 relative z-10 pt-6 space-y-6">
        <div className="nofx-glass rounded-xl p-6">
          <div className="flex flex-col lg:flex-row lg:items-end gap-4">
            <div className="flex-1">
              <div className="text-xs uppercase tracking-[0.24em] text-nofx-gold/80 mb-2">
                Semantic Memory
              </div>
              <h1 className="text-3xl font-semibold text-nofx-text-main">
                Analyst retrieval workspace
              </h1>
              <p className="text-sm text-nofx-text-muted mt-2 max-w-3xl">
                Search across the internal pgvector corpus for similar review cases,
                optimizer runs, and backlog findings. Use this to find prior edge
                failures, repeated gate blocks, and strategy lessons before changing
                a trader again.
              </p>
            </div>
            <div className="w-full lg:w-80">
              <label className="text-xs text-nofx-text-muted block mb-2">Trader</label>
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

        <div className="nofx-glass rounded-xl p-5 space-y-4">
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                Corpus Status
              </div>
              <div className="text-lg font-semibold mt-2">
                {selectedTrader?.trader_name || 'Select a trader'}
              </div>
            </div>
            {loadingStatus && <div className="text-sm text-nofx-text-muted">Refreshing…</div>}
          </div>

          {!status ? (
            <div className="text-sm text-nofx-text-muted">
              {selectedTraderId
                ? error || 'No semantic-memory status loaded yet.'
                : 'Select a trader to inspect the corpus.'}
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 md:grid-cols-4 gap-3 text-sm">
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs text-nofx-text-muted">Total documents</div>
                  <div className="text-lg font-semibold mt-1">{status.total_documents}</div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {formatInteger(status.total_token_estimate)} est. tokens
                  </div>
                </div>
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs text-nofx-text-muted">Embedded</div>
                  <div className="text-lg font-semibold mt-1">
                    {status.embedded_documents} <span className="text-xs text-nofx-text-muted">({totalEmbeddedPct})</span>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {formatInteger(status.embedded_token_estimate)} tokens embedded
                  </div>
                </div>
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs text-nofx-text-muted">Pending / failed</div>
                  <div className="text-lg font-semibold mt-1">
                    {status.pending_documents} / {status.failed_documents}
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {formatInteger(status.pending_token_estimate)} / {formatInteger(status.failed_token_estimate)} tokens
                  </div>
                </div>
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs text-nofx-text-muted">Vector runtime</div>
                  <div className="text-lg font-semibold mt-1">
                    {status.vector_available ? 'Ready' : 'Unavailable'}
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {status.vector_extension_name || '-'}
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    Embedding Cost
                  </div>
                  <div className="space-y-2 text-sm">
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Model</span>
                      <span>{status.cost_estimate.embedding_model || '-'}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Price / 1M tokens</span>
                      <span>{formatUsd(status.cost_estimate.price_per_1m_tokens_usd)}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Estimated corpus cost</span>
                      <span>{formatUsd(status.cost_estimate.estimated_total_cost_usd)}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Pending refresh cost</span>
                      <span>{formatUsd(status.cost_estimate.estimated_pending_cost_usd)}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Failed re-embed cost</span>
                      <span>{formatUsd(status.cost_estimate.estimated_failed_reembed_cost_usd)}</span>
                    </div>
                  </div>
                </div>

                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    Recent Embed Usage
                  </div>
                  <div className="space-y-2 text-sm">
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Recent runs</span>
                      <span>{formatInteger(status.embedding_usage.recent_runs)}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Embedded docs</span>
                      <span>{formatInteger(status.embedding_usage.recent_embedded_documents)}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Failed docs</span>
                      <span>{formatInteger(status.embedding_usage.recent_failed_documents)}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Prompt tokens</span>
                      <span>{formatInteger(status.embedding_usage.recent_prompt_tokens)}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Estimated cost</span>
                      <span>{formatUsd(status.embedding_usage.recent_estimated_cost_usd)}</span>
                    </div>
                    <div className="text-xs text-nofx-text-muted pt-1">
                      Last completed {normalizeTime(status.embedding_usage.last_completed_at)} via{' '}
                      {status.embedding_usage.last_embedding_provider || '-'} /{' '}
                      {status.embedding_usage.last_embedding_model || '-'}
                    </div>
                  </div>
                </div>

                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    Failure Summary
                  </div>
                  <div className="space-y-2 text-sm">
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Recent failed runs</span>
                      <span>{formatInteger(status.failure_summary.recent_failed_runs)}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-nofx-text-muted">Last failure scope</span>
                      <span>{formatDocType(status.failure_summary.last_failure_scope)}</span>
                    </div>
                    <div className="text-xs text-nofx-text-muted pt-1">
                      Last failed {normalizeTime(status.failure_summary.last_failed_at)}
                    </div>
                    <div className="rounded-lg border border-amber-400/20 bg-amber-500/5 p-3 text-xs text-amber-100">
                      {status.failure_summary.last_failure_message || 'No recent semantic-memory sync failures.'}
                    </div>
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    Corpus By Doc Type
                  </div>
                  <div className="space-y-3">
                    {status.doc_types.map((item) => (
                      <div key={item.doc_type} className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <div className="font-semibold">{formatDocType(item.doc_type)}</div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {item.total_documents} docs · {item.embedded_documents} embedded · model{' '}
                              {item.dominant_model || '-'}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {formatInteger(item.total_token_estimate)} tokens · est. total {formatUsd(item.estimated_total_cost_usd)} · pending {formatUsd(item.estimated_pending_cost_usd)}
                            </div>
                          </div>
                          <div className="text-right text-xs text-nofx-text-muted">
                            Updated {normalizeTime(item.last_updated_at)}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    Recent Sync Runs
                  </div>
                  {status.recent_sync_runs && status.recent_sync_runs.length > 0 ? (
                    <div className="space-y-3">
                      {status.recent_sync_runs.map((run) => (
                        <div key={run.id} className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="flex items-start justify-between gap-3">
                            <div>
                              <div className="font-semibold">{formatDocType(run.scope)}</div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                {run.summary || 'No summary stored'}
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                {formatInteger(run.total_documents)} docs · {formatInteger(run.prompt_tokens)} tokens · {formatUsd(run.estimated_cost_usd)} · {run.embedding_provider || '-'} / {run.embedding_model || '-'}
                              </div>
                            </div>
                            <div className="text-right text-xs text-nofx-text-muted">
                              <div>{formatDocType(run.status)}</div>
                              <div className="mt-1">{normalizeTime(run.started_at)}</div>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="text-sm text-nofx-text-muted">
                      No semantic-memory sync runs are recorded yet.
                    </div>
                  )}
                </div>
              </div>
            </>
          )}
        </div>

        <div className="nofx-glass rounded-xl p-5 space-y-4">
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                Retrieval Benchmarks
              </div>
              <div className="text-lg font-semibold mt-2">Semantic similarity quality</div>
              <div className="text-sm text-nofx-text-muted mt-2 max-w-3xl">
                Runs a reproducible similarity benchmark over the embedded corpora for this trader.
                Scores are heuristic and corpus-specific, meant to catch drift before broader rollout.
              </div>
            </div>
            <button
              onClick={runBenchmark}
              disabled={!selectedTraderId || runningBenchmark}
              className="h-11 px-5 rounded-lg border border-nofx-gold/30 text-nofx-gold disabled:opacity-50"
            >
              {runningBenchmark ? 'Running…' : 'Run benchmark'}
            </button>
          </div>

          {!latestBenchmark ? (
            <div className="text-sm text-nofx-text-muted">
              No benchmark run recorded yet for this trader.
            </div>
          ) : (
            <div className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-5 gap-3 text-sm">
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs text-nofx-text-muted">Status</div>
                  <div className="text-lg font-semibold mt-1">{formatDocType(latestBenchmark.status)}</div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {normalizeTime(latestBenchmark.completed_at || latestBenchmark.started_at)}
                  </div>
                </div>
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs text-nofx-text-muted">Avg top-1 relevance</div>
                  <div className="text-lg font-semibold mt-1">{formatRatio(latestBenchmark.avg_top1_relevance)}</div>
                </div>
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs text-nofx-text-muted">Hit@{latestBenchmark.config?.top_k || 5}</div>
                  <div className="text-lg font-semibold mt-1">{formatRatio(latestBenchmark.hit_rate_at_k)}</div>
                </div>
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs text-nofx-text-muted">Strong top-1</div>
                  <div className="text-lg font-semibold mt-1">{formatRatio(latestBenchmark.strong_top1_rate)}</div>
                </div>
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs text-nofx-text-muted">Queries / corpora</div>
                  <div className="text-lg font-semibold mt-1">
                    {formatInteger(latestBenchmark.query_count)} / {formatInteger(latestBenchmark.corpus_count)}
                  </div>
                </div>
              </div>

              <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm">
                <div className="font-semibold">{latestBenchmark.summary || 'No summary stored'}</div>
                {latestBenchmark.error_message && (
                  <div className="text-xs text-amber-200 mt-2">{latestBenchmark.error_message}</div>
                )}
              </div>

              {latestBenchmark.corpus_results && latestBenchmark.corpus_results.length > 0 && (
                <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
                  {latestBenchmark.corpus_results.map((result) => (
                    <div key={result.doc_type} className="rounded-lg border border-white/10 bg-black/20 p-4">
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="font-semibold">{formatDocType(result.doc_type)}</div>
                          <div className="text-xs text-nofx-text-muted mt-1">
                            {formatInteger(result.document_count)} docs · {formatInteger(result.evaluated_queries)} queries · {formatInteger(result.zero_hit_queries)} zero-hit
                          </div>
                        </div>
                        <div className="text-right text-xs text-nofx-text-muted">
                          hit@{latestBenchmark.config?.top_k || 5} {formatRatio(result.hit_rate_at_k)}
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-3 mt-4 text-sm">
                        <div>
                          <div className="text-xs text-nofx-text-muted">Top-1 relevance</div>
                          <div className="font-semibold mt-1">{formatRatio(result.avg_top1_relevance)}</div>
                        </div>
                        <div>
                          <div className="text-xs text-nofx-text-muted">Top-{latestBenchmark.config?.top_k || 5} relevance</div>
                          <div className="font-semibold mt-1">{formatRatio(result.avg_top_k_relevance)}</div>
                        </div>
                        <div>
                          <div className="text-xs text-nofx-text-muted">Top-1 similarity</div>
                          <div className="font-semibold mt-1">{formatRatio(result.avg_top1_similarity)}</div>
                        </div>
                        <div>
                          <div className="text-xs text-nofx-text-muted">Strong top-1</div>
                          <div className="font-semibold mt-1">{formatRatio(result.strong_top1_rate)}</div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}

              {benchmarks.length > 1 && (
                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    Recent Benchmark Runs
                  </div>
                  <div className="space-y-3">
                    {benchmarks.slice(1).map((run) => (
                      <div key={run.id} className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <div className="font-semibold">{run.summary || 'No summary stored'}</div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {formatInteger(run.query_count)} queries · avg top-1 relevance {formatRatio(run.avg_top1_relevance)} · hit@{run.config?.top_k || 5} {formatRatio(run.hit_rate_at_k)}
                            </div>
                          </div>
                          <div className="text-right text-xs text-nofx-text-muted">
                            <div>{formatDocType(run.status)}</div>
                            <div className="mt-1">{normalizeTime(run.completed_at || run.started_at)}</div>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        <div className="nofx-glass rounded-xl p-5 space-y-4">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
              Search
            </div>
            <div className="text-lg font-semibold mt-2">Free-text semantic retrieval</div>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-[1.2fr_1.2fr_auto_auto] gap-3">
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={selectedPresetId}
                onChange={onPresetSelect}
                options={[
                  { value: '', label: 'Saved searches' },
                  ...presets.map((item) => ({
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
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-4 text-sm outline-none focus:border-nofx-gold/40"
            />
            <button
              onClick={savePreset}
              disabled={!selectedTraderId || savingPreset}
              className="h-11 px-5 rounded-lg border border-nofx-gold/30 text-nofx-gold disabled:opacity-50"
            >
              {savingPreset ? 'Saving…' : selectedPresetId ? 'Update preset' : 'Save preset'}
            </button>
            <button
              onClick={deletePreset}
              disabled={!selectedTraderId || !selectedPresetId || deletingPresetId === selectedPresetId}
              className="h-11 px-5 rounded-lg border border-white/10 text-white disabled:opacity-50"
            >
              {deletingPresetId === selectedPresetId ? 'Deleting…' : 'Delete preset'}
            </button>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-[2fr_1fr_1fr_1fr_auto] gap-3">
            <textarea
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              rows={3}
              placeholder="Example: repeated long re-entries after losses in flat OI with poor exit efficiency"
              className="rounded-xl border border-white/10 bg-black/20 px-4 py-3 text-sm resize-none outline-none focus:border-nofx-gold/40"
            />
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect value={docScope} onChange={setDocScope} options={docScopeOptions} />
            </div>
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect value={outcome} onChange={setOutcome} options={outcomeOptions} />
            </div>
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect value={runStatus} onChange={setRunStatus} options={runStatusOptions} />
            </div>
            <button
              onClick={submitSearch}
              disabled={!selectedTraderId || runningSearch}
              className="h-11 px-5 rounded-lg border border-nofx-gold/30 text-nofx-gold disabled:opacity-50"
            >
              {runningSearch ? 'Searching…' : 'Search'}
            </button>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-3">
            <input
              value={closeReason}
              onChange={(event) => setCloseReason(event.target.value)}
              placeholder="Optional close reason filter"
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-4 text-sm outline-none focus:border-nofx-gold/40"
            />
            <input
              value={limit}
              onChange={(event) => setLimit(event.target.value)}
              placeholder="Result limit"
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-4 text-sm outline-none focus:border-nofx-gold/40"
            />
            <div className="text-xs text-nofx-text-muted flex items-center">
              Uses the internal pgvector corpus with the default OpenAI embedding account.
            </div>
          </div>

          {error && <div className="text-sm text-amber-200">{error}</div>}

          {!searchResult ? (
            <div className="text-sm text-nofx-text-muted">
              Run a query to retrieve semantically similar cases, optimizer runs, or backlog items.
            </div>
          ) : searchResult.items.length === 0 ? (
            <div className="text-sm text-nofx-text-muted">
              No semantic matches found for this query.
            </div>
          ) : (
            <div className="space-y-3">
              <div className="text-xs text-nofx-text-muted">
                Query embedded via {searchResult.embedding_provider} / {searchResult.embedding_model}
              </div>
              {searchResult.items.map((hit) => (
                <div
                  key={`${hit.document.id}-${hit.document.source_id}`}
                  className="rounded-xl border border-white/10 bg-black/20 p-4"
                >
                  <div className="flex flex-col xl:flex-row xl:items-start xl:justify-between gap-4">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="px-2 py-1 rounded-full text-[11px] bg-sky-500/10 border border-sky-400/20 text-sky-200">
                          {formatDocType(hit.document.doc_type)}
                        </span>
                        <span className="px-2 py-1 rounded-full text-[11px] bg-white/5 border border-white/10 text-nofx-text-muted">
                          {formatSemanticSimilarityScore(hit.similarity_score)}
                        </span>
                      </div>
                      <div className="font-semibold mt-3">{hit.document.title || hit.document.source_id}</div>
                      <div className="text-sm text-nofx-text-muted mt-2">
                        {hit.document.summary || 'No summary stored'}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-3">
                        Source {hit.document.source_id} · updated {normalizeTime(hit.document.source_updated_at)} · distance{' '}
                        {typeof hit.distance === 'number' ? hit.distance.toFixed(4) : '-'}
                      </div>
                    </div>
                    <div className="flex flex-col gap-2">
                      <button
                        onClick={() => openSourceLink(hit.source_link)}
                        className="h-10 px-4 rounded-lg border border-white/10 bg-black/20 text-sm"
                      >
                        Open source
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </DeepVoidBackground>
  )
}
