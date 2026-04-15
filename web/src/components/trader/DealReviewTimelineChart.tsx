import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { DealReviewPriceTimeline } from '../../types'

interface DealReviewTimelineChartProps {
  timeline?: DealReviewPriceTimeline | null
  entryPrice: number
  side: string
  stopLoss?: number
  takeProfit?: number
}

function formatTimelineMoney(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}`
}

function formatTimelinePct(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
}

function formatTimelinePrice(value: number): string {
  if (!Number.isFinite(value)) return '-'
  if (Math.abs(value) >= 1000) return value.toFixed(2)
  if (Math.abs(value) >= 1) return value.toFixed(4)
  return value.toFixed(6)
}

function formatTimelineTime(ms: number): string {
  if (!ms) return '-'
  return new Date(ms).toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function DealReviewTimelineChart({
  timeline,
  entryPrice,
  side,
  stopLoss,
  takeProfit,
}: DealReviewTimelineChartProps) {
  const points = timeline?.points || []
  const summary = timeline?.summary

  if (!timeline || points.length === 0) {
    return (
      <div className="rounded-xl border border-white/10 bg-black/20 p-4">
        <div className="font-semibold">Price path by decision cycle</div>
        <div className="text-sm text-nofx-text-muted mt-2">
          No cycle price history stored for this deal yet.
        </div>
      </div>
    )
  }

  const chartData = points.map((point) => ({
    ...point,
    time_label: formatTimelineTime(point.timestamp_ms),
    pnl_label: formatTimelinePct(point.unrealized_pnl_pct),
    price_label: formatTimelinePrice(point.mark_price),
  }))

  const tooltip = ({ active, payload }: any) => {
    if (!active || !payload || payload.length === 0) return null
    const point = payload[0]?.payload
    if (!point) return null
    return (
      <div className="rounded-lg border border-white/10 bg-[#0d1016] px-3 py-3 shadow-xl">
        <div className="text-xs text-nofx-text-muted">{point.time_label}</div>
        <div className="text-sm font-semibold text-white mt-1">
          {point.source === 'entry'
            ? 'Entry snapshot'
            : point.source === 'exit'
              ? 'Exit snapshot'
              : `Decision cycle ${point.decision_cycle_number || '-'}`}
        </div>
        <div className="text-xs text-nofx-text-muted mt-2">
          Price <span className="text-white">{point.price_label}</span>
        </div>
        <div className="text-xs text-nofx-text-muted">
          Unrealized PnL{' '}
          <span
            className={
              point.unrealized_pnl >= 0 ? 'text-emerald-400' : 'text-rose-400'
            }
          >
            {formatTimelineMoney(point.unrealized_pnl)}
          </span>{' '}
          <span className="text-white">({point.pnl_label})</span>
        </div>
        <div className="text-xs text-nofx-text-muted">
          Qty <span className="text-white">{point.quantity || '-'}</span>
        </div>
      </div>
    )
  }

  return (
    <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
      <div className="flex items-center justify-between gap-4">
        <div>
          <div className="font-semibold">Price path by decision cycle</div>
          <div className="text-xs text-nofx-text-muted mt-1">
            Entry/exit points are synthetic; cycle points come from stored
            decision snapshots.
          </div>
        </div>
        <div className="text-xs text-nofx-text-muted">{side} timeline</div>
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4 text-xs">
        <div className="rounded-lg border border-white/10 bg-black/30 px-3 py-3">
          <div className="text-nofx-text-muted">Cycle samples</div>
          <div className="mt-1 font-semibold text-white">
            {summary?.cycle_samples || 0}
          </div>
        </div>
        <div className="rounded-lg border border-white/10 bg-black/30 px-3 py-3">
          <div className="text-nofx-text-muted">Ever in profit</div>
          <div
            className={`mt-1 font-semibold ${summary?.ever_in_profit ? 'text-emerald-400' : 'text-rose-400'}`}
          >
            {summary?.ever_in_profit ? 'Yes' : 'No'}
          </div>
        </div>
        <div className="rounded-lg border border-white/10 bg-black/30 px-3 py-3">
          <div className="text-nofx-text-muted">Max favorable</div>
          <div className="mt-1 font-semibold text-emerald-400">
            {formatTimelineMoney(summary?.max_unrealized_pnl || 0)} /{' '}
            {formatTimelinePct(summary?.max_unrealized_pnl_pct || 0)}
          </div>
        </div>
        <div className="rounded-lg border border-white/10 bg-black/30 px-3 py-3">
          <div className="text-nofx-text-muted">Max adverse</div>
          <div className="mt-1 font-semibold text-rose-400">
            {formatTimelineMoney(summary?.min_unrealized_pnl || 0)} /{' '}
            {formatTimelinePct(summary?.min_unrealized_pnl_pct || 0)}
          </div>
        </div>
      </div>

      <div className="h-[320px]">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
            <CartesianGrid stroke="rgba(255,255,255,0.08)" strokeDasharray="3 3" />
            <XAxis
              dataKey="time_label"
              minTickGap={24}
              tick={{ fill: '#94A3B8', fontSize: 11 }}
            />
            <YAxis
              yAxisId="price"
              tick={{ fill: '#94A3B8', fontSize: 11 }}
              tickFormatter={(value) => formatTimelinePrice(Number(value))}
              width={78}
            />
            <YAxis
              yAxisId="pnl"
              orientation="right"
              tick={{ fill: '#94A3B8', fontSize: 11 }}
              tickFormatter={(value) => `${Number(value).toFixed(1)}%`}
              width={62}
            />
            <Tooltip content={tooltip} />
            <Legend wrapperStyle={{ fontSize: '12px' }} />

            {entryPrice > 0 && (
              <ReferenceLine
                yAxisId="price"
                y={entryPrice}
                stroke="rgba(245, 158, 11, 0.8)"
                strokeDasharray="4 4"
                ifOverflow="extendDomain"
                label={{ value: 'Entry', fill: '#f59e0b', fontSize: 11 }}
              />
            )}
            {stopLoss && stopLoss > 0 && (
              <ReferenceLine
                yAxisId="price"
                y={stopLoss}
                stroke="rgba(244, 63, 94, 0.7)"
                strokeDasharray="3 3"
                ifOverflow="extendDomain"
                label={{ value: 'SL', fill: '#f43f5e', fontSize: 11 }}
              />
            )}
            {takeProfit && takeProfit > 0 && (
              <ReferenceLine
                yAxisId="price"
                y={takeProfit}
                stroke="rgba(16, 185, 129, 0.7)"
                strokeDasharray="3 3"
                ifOverflow="extendDomain"
                label={{ value: 'TP', fill: '#10b981', fontSize: 11 }}
              />
            )}
            <ReferenceLine
              yAxisId="pnl"
              y={0}
              stroke="rgba(148, 163, 184, 0.45)"
              strokeDasharray="2 4"
            />

            <Line
              yAxisId="price"
              type="monotone"
              dataKey="mark_price"
              name="Price"
              stroke="#f59e0b"
              strokeWidth={2}
              dot={{ r: 2 }}
              activeDot={{ r: 4 }}
              isAnimationActive={false}
            />
            <Line
              yAxisId="pnl"
              type="monotone"
              dataKey="unrealized_pnl_pct"
              name="PnL %"
              stroke="#38bdf8"
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4 }}
              isAnimationActive={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>

      <div className="grid grid-cols-2 gap-3 text-xs lg:grid-cols-4">
        <div>
          <span className="text-nofx-text-muted">Highest mark</span>
          <div className="mt-1 text-white font-medium">
            {formatTimelinePrice(summary?.highest_mark_price || 0)}
          </div>
        </div>
        <div>
          <span className="text-nofx-text-muted">Lowest mark</span>
          <div className="mt-1 text-white font-medium">
            {formatTimelinePrice(summary?.lowest_mark_price || 0)}
          </div>
        </div>
        <div>
          <span className="text-nofx-text-muted">Stored points</span>
          <div className="mt-1 text-white font-medium">
            {summary?.point_count || 0}
          </div>
        </div>
        <div>
          <span className="text-nofx-text-muted">Profit zone</span>
          <div
            className={`mt-1 font-medium ${(summary?.max_unrealized_pnl || 0) > 0 ? 'text-emerald-400' : 'text-rose-400'}`}
          >
            {(summary?.max_unrealized_pnl || 0) > 0
              ? 'Reached above 0%'
              : 'Never above 0%'}
          </div>
        </div>
      </div>
    </div>
  )
}
