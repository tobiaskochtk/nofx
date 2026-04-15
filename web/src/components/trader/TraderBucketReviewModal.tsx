import { Loader2, RefreshCw, X } from 'lucide-react'
import type { Language } from '../../i18n/translations'
import type { TraderAI500BucketReview } from '../../types'

interface TraderBucketReviewModalProps {
  isOpen: boolean
  traderName: string
  language: Language
  review?: TraderAI500BucketReview | null
  loading?: boolean
  error?: string | null
  onClose: () => void
  onRefresh: () => void
}

function getCopy(language: Language) {
  switch (language) {
    case 'zh':
      return {
        title: 'AI500 24小时桶复盘',
        subtitle: '基于已保存 decision records 的实时聚合',
        refresh: '刷新',
        ready: '24h 已就绪',
        warming: '数据预热中',
        cycles: '有效周期',
        candidates: '候选币总数',
        openDecisions: '开仓决策',
        noCandidates: '无候选周期',
        loading: '正在加载复盘数据...',
        noData: '还没有可复盘的数据。',
        note: '这个视图不是单独批处理生成，而是实时从已保存的 cycle 记录聚合出来。达到 24 小时后会自动可评估。',
        legacy: '检测到更早的旧记录不含 bucket 元数据。24h 就绪状态只基于新记录计算。',
        buckets: 'Bucket 汇总',
        recentCycles: '最近周期',
        candidateCount: '候选',
        openSymbols: '开仓',
      }
    case 'id':
      return {
        title: 'Review Bucket AI500 24 Jam',
        subtitle: 'Agregasi live dari decision records yang tersimpan',
        refresh: 'Muat Ulang',
        ready: '24j siap',
        warming: 'Masih warming-up',
        cycles: 'Siklus valid',
        candidates: 'Total kandidat',
        openDecisions: 'Keputusan open',
        noCandidates: 'Siklus tanpa kandidat',
        loading: 'Memuat data review...',
        noData: 'Belum ada data yang bisa direview.',
        note: 'Tampilan ini tidak dibuat oleh batch terpisah. Ia dihitung live dari cycle yang sudah tersimpan. Setelah 24 jam, evaluasi langsung tersedia.',
        legacy: 'Ada record lama tanpa metadata bucket. Status kesiapan 24j hanya dihitung dari record baru.',
        buckets: 'Ringkasan Bucket',
        recentCycles: 'Siklus terbaru',
        candidateCount: 'Kandidat',
        openSymbols: 'Open',
      }
    default:
      return {
        title: 'AI500 24h Bucket Review',
        subtitle: 'Live aggregation from persisted decision records',
        refresh: 'Refresh',
        ready: '24h ready',
        warming: 'Warming up',
        cycles: 'Valid cycles',
        candidates: 'Total candidates',
        openDecisions: 'Open decisions',
        noCandidates: 'No-candidate cycles',
        loading: 'Loading review data...',
        noData: 'No reviewable data yet.',
        note: 'This view is computed live from saved cycle records. It becomes review-ready automatically once 24 hours of post-deploy data exist.',
        legacy: 'Older records without bucket metadata were detected. The 24h readiness check only uses the new records.',
        buckets: 'Bucket summary',
        recentCycles: 'Recent cycles',
        candidateCount: 'Candidates',
        openSymbols: 'Open',
      }
  }
}

function bucketColor(name: string): string {
  switch (name) {
    case 'primary':
      return 'rgba(14, 203, 129, 0.2)'
    case 'adaptive':
      return 'rgba(240, 185, 11, 0.2)'
    case 'fallback_eligible':
      return 'rgba(96, 165, 250, 0.2)'
    case 'exploration':
      return 'rgba(192, 132, 252, 0.2)'
    default:
      return 'rgba(132, 142, 156, 0.2)'
  }
}

function formatLocalTime(value?: string): string {
  if (!value) return '--'
  return new Date(value).toLocaleString()
}

export function TraderBucketReviewModal({
  isOpen,
  traderName,
  language,
  review,
  loading,
  error,
  onClose,
  onRefresh,
}: TraderBucketReviewModalProps) {
  const copy = getCopy(language)

  if (!isOpen) return null

  return (
    <div
      className="fixed inset-0 z-[80] flex items-center justify-center bg-black/70 backdrop-blur-sm p-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-6xl max-h-[90vh] overflow-hidden rounded-2xl border border-white/10 bg-[#10151D] shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-white/10 px-6 py-4 bg-[linear-gradient(135deg,rgba(99,102,241,0.18),rgba(15,23,42,0.95))]">
          <div>
            <div className="text-xl font-semibold text-white">{copy.title}</div>
            <div className="text-sm text-white/70">
              {traderName} · {copy.subtitle}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={onRefresh}
              className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white transition-colors hover:bg-white/10"
            >
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
              {copy.refresh}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg p-2 text-white/70 transition-colors hover:bg-white/10 hover:text-white"
            >
              <X className="h-5 w-5" />
            </button>
          </div>
        </div>

        <div className="overflow-y-auto px-6 py-5 space-y-5 max-h-[calc(90vh-80px)]">
          {loading && !review && (
            <div className="flex items-center justify-center rounded-xl border border-white/10 bg-white/5 px-6 py-14 text-white/70">
              <Loader2 className="mr-3 h-5 w-5 animate-spin" />
              {copy.loading}
            </div>
          )}

          {!loading && !review && !error && (
            <div className="rounded-xl border border-white/10 bg-white/5 px-6 py-10 text-center text-white/70">
              {copy.noData}
            </div>
          )}

          {error && !review && (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 px-6 py-5 text-red-200">
              {error}
            </div>
          )}

          {review && (
            <>
              <div className="rounded-xl border border-white/10 bg-white/5 p-4 text-sm text-white/75">
                {copy.note}
              </div>

              {review.legacy_record_count > 0 && (
                <div className="rounded-xl border border-amber-400/30 bg-amber-400/10 p-4 text-sm text-amber-100">
                  {copy.legacy} ({review.legacy_record_count})
                </div>
              )}

              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4">
                <div className="rounded-xl border border-white/10 bg-[#131A24] p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-white/45">
                    {review.has_full_window ? copy.ready : copy.warming}
                  </div>
                  <div className="mt-2 text-2xl font-semibold text-white">
                    {review.coverage_hours.toFixed(1)}h / {review.window_hours}h
                  </div>
                  <div className="mt-2 text-xs text-white/50">
                    {formatLocalTime(review.first_record_at)} {'->'} {formatLocalTime(review.last_record_at)}
                  </div>
                </div>
                <div className="rounded-xl border border-white/10 bg-[#131A24] p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-white/45">
                    {copy.cycles}
                  </div>
                  <div className="mt-2 text-2xl font-semibold text-white">
                    {review.record_count}
                  </div>
                  <div className="mt-2 text-xs text-white/50">
                    {copy.noCandidates}: {review.cycles_without_candidates}
                  </div>
                </div>
                <div className="rounded-xl border border-white/10 bg-[#131A24] p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-white/45">
                    {copy.candidates}
                  </div>
                  <div className="mt-2 text-2xl font-semibold text-white">
                    {review.total_candidates}
                  </div>
                  <div className="mt-2 text-xs text-white/50">
                    {review.cycles_with_candidates} cycles with candidates
                  </div>
                </div>
                <div className="rounded-xl border border-white/10 bg-[#131A24] p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-white/45">
                    {copy.openDecisions}
                  </div>
                  <div className="mt-2 text-2xl font-semibold text-white">
                    {review.total_open_decisions}
                  </div>
                  <div className="mt-2 text-xs text-white/50">
                    {review.cycles_with_open_decisions} cycles with openings
                  </div>
                </div>
              </div>

              <div>
                <div className="mb-3 text-sm font-semibold uppercase tracking-[0.2em] text-white/55">
                  {copy.buckets}
                </div>
                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  {review.buckets.map((bucket) => (
                    <div
                      key={bucket.name}
                      className="rounded-xl border border-white/10 bg-[#131A24] p-4"
                    >
                      <div className="flex items-center justify-between gap-3">
                        <div className="inline-flex items-center gap-2">
                          <span
                            className="inline-block h-2.5 w-2.5 rounded-full"
                            style={{ background: bucketColor(bucket.name) }}
                          />
                          <span className="font-semibold text-white">{bucket.name}</span>
                        </div>
                        <div className="text-xs text-white/50">
                          unique {bucket.unique_symbols}
                        </div>
                      </div>
                      <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                        <div className="rounded-lg bg-white/5 p-3">
                          <div className="text-xs uppercase tracking-[0.2em] text-white/45">
                            candidates
                          </div>
                          <div className="mt-1 text-xl font-semibold text-white">
                            {bucket.candidate_count}
                          </div>
                        </div>
                        <div className="rounded-lg bg-white/5 p-3">
                          <div className="text-xs uppercase tracking-[0.2em] text-white/45">
                            open decisions
                          </div>
                          <div className="mt-1 text-xl font-semibold text-white">
                            {bucket.open_decision_count}
                          </div>
                        </div>
                      </div>
                      {bucket.symbol_samples && bucket.symbol_samples.length > 0 && (
                        <div className="mt-4 flex flex-wrap gap-2">
                          {bucket.symbol_samples.map((symbol) => (
                            <span
                              key={symbol}
                              className="rounded-full border border-white/10 bg-white/5 px-2.5 py-1 text-xs text-white/75"
                            >
                              {symbol}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>

              <div>
                <div className="mb-3 text-sm font-semibold uppercase tracking-[0.2em] text-white/55">
                  {copy.recentCycles}
                </div>
                <div className="space-y-3">
                  {review.recent_cycles.map((cycle) => (
                    <div
                      key={`${cycle.cycle_number}-${cycle.timestamp}`}
                      className="rounded-xl border border-white/10 bg-[#131A24] p-4"
                    >
                      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                        <div>
                          <div className="text-white font-semibold">
                            Cycle #{cycle.cycle_number}
                          </div>
                          <div className="text-xs text-white/50">
                            {formatLocalTime(cycle.timestamp)}
                          </div>
                        </div>
                        <div className="flex flex-wrap gap-2 text-xs">
                          <span className="rounded-full border border-white/10 bg-white/5 px-2.5 py-1 text-white/75">
                            {copy.candidateCount}: {cycle.candidate_count}
                          </span>
                          {cycle.open_decision_symbols && cycle.open_decision_symbols.length > 0 && (
                            <span className="rounded-full border border-emerald-400/20 bg-emerald-400/10 px-2.5 py-1 text-emerald-200">
                              {copy.openSymbols}: {cycle.open_decision_symbols.join(', ')}
                            </span>
                          )}
                        </div>
                      </div>
                      <div className="mt-3 flex flex-wrap gap-2">
                        {cycle.buckets.map((bucket) => (
                          <span
                            key={`${cycle.cycle_number}-${bucket.name}`}
                            className="rounded-full border border-white/10 px-2.5 py-1 text-xs text-white/75"
                            style={{ background: bucketColor(bucket.name) }}
                          >
                            {bucket.name}: {bucket.count}
                            {bucket.symbols && bucket.symbols.length > 0 ? ` (${bucket.symbols.join(', ')})` : ''}
                          </span>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}

          {error && review && (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-200">
              {error}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
