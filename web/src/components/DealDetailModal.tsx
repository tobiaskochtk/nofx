import { useEffect } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import type { Deal, DealEvent } from '../types'

export function DealDetailModal({ traderId, dealId, onClose }: { traderId: string; dealId: number; onClose: () => void }) {
  const { data } = useSWR<{ deal: Deal; events: DealEvent[] }>(dealId ? ['deal', traderId, dealId] : null, () => api.getDealById(traderId, dealId))

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  if (!data) return null
  const { deal, events } = data

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="w-[900px] max-h-[80vh] overflow-y-auto rounded-lg p-6" style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-bold">Deal #{deal.id} — {deal.symbol} {deal.side.toUpperCase()}</h2>
          <button onClick={onClose} className="px-3 py-1 rounded" style={{ background: '#2B3139' }}>Close</button>
        </div>

        {/* Summary */}
        <div className="grid grid-cols-2 gap-4 mb-4">
          <Info label="Open Time" value={new Date(deal.open_time).toLocaleString()} />
          <Info label="Status" value={deal.status} />
          <Info label="Leverage" value={`${deal.leverage}x`} />
          <Info label="Qty" value={Number.isFinite(Number(deal.quantity)) ? Number(deal.quantity).toFixed(6) : '-'} />
          <Info label="Open Price" value={Number.isFinite(Number(deal.open_price)) ? Number(deal.open_price).toFixed(6) : '-'} />
          <Info label="Close Price" value={deal.close_price === null || deal.close_price === undefined ? '-' : (Number.isFinite(Number(deal.close_price)) ? Number(deal.close_price).toFixed(6) : '-')} />
          <Info label="SL / TP" value={`${Number.isFinite(Number(deal.stop_loss)) ? Number(deal.stop_loss).toFixed(6) : '-'} / ${Number.isFinite(Number(deal.take_profit)) ? Number(deal.take_profit).toFixed(6) : '-'}`} />
          <Info label="PnL" value={deal.realized_pnl !== null && deal.realized_pnl !== undefined ? `${Number.isFinite(Number(deal.realized_pnl)) ? Number(deal.realized_pnl).toFixed(2) : '0.00'} (${Number.isFinite(Number(deal.realized_pnl_pct)) ? Number(deal.realized_pnl_pct).toFixed(2) : '0.00'}%)` : '-'} />
        </div>

        {/* Reasoning/Prompts (collapsible simple) */}
        {(deal.user_prompt || deal.system_prompt || deal.reasoning) && (
          <div className="mb-4">
            <h3 className="font-semibold mb-2">AI Context</h3>
            {deal.system_prompt && (<Pre label="System Prompt" text={deal.system_prompt} />)}
            {deal.user_prompt && (<Pre label="User Prompt" text={deal.user_prompt} />)}
            {deal.reasoning && (<Pre label="Reasoning" text={deal.reasoning} />)}
          </div>
        )}

        {/* Timeline */}
        <div>
          <h3 className="font-semibold mb-2">Timeline</h3>
          <div className="space-y-2">
            {events.map((e) => (
              <div key={e.id} className="p-3 rounded" style={{ background: '#0E1318', border: '1px solid #2B3139' }}>
                <div className="flex items-center justify-between mb-1">
                  <div className="font-mono text-sm" style={{ color: '#848E9C' }}>{new Date(e.created_at).toLocaleString()}</div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>#{e.id}</div>
                </div>
                <div className="text-sm">
                  <b>{e.type.split('_').join(' ')}</b> — {e.symbol} {e.side.toUpperCase()}
                  {e.quantity ? <> · Qty {e.quantity.toFixed(6)}</> : null}
                  {e.percentage ? <> · {e.percentage.toFixed(1)}%</> : null}
                  {e.price ? <> · Price {e.price.toFixed(6)}</> : null}
                  {e.order_id ? <> · Order {e.order_id}</> : null}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

function Info({ label, value }: { label: string; value: any }) {
  return (
    <div className="p-3 rounded" style={{ background: '#0E1318', border: '1px solid #2B3139' }}>
      <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{label}</div>
      <div className="font-mono">{value}</div>
    </div>
  )
}

function Pre({ label, text }: { label: string; text: string }) {
  return (
    <div className="mb-2">
      <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{label}</div>
      <pre className="p-3 rounded whitespace-pre-wrap overflow-x-auto" style={{ background: '#0E1318', border: '1px solid #2B3139', color: '#EAECEF' }}>{text}</pre>
    </div>
  )
}
