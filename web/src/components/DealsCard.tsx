import { useEffect, useMemo, useState } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import type { Deal } from '../types'
import { DealDetailModal } from './DealDetailModal'

const PRESET_STORAGE_KEY = 'nofx_deal_presets_v1'

type DealsPreset = {
  name: string
  createdAt: number
  filters: {
    status: 'all' | 'open' | 'closed'
    side: 'all' | 'long' | 'short'
    range: '24h' | '7d' | '30d' | 'all'
    symbol: string
    q: string
    pnl: 'all' | 'win' | 'loss'
    pnlMin: string
    pnlMax: string
  }
  columns: string[]
  jsonlMode: 'full' | 'slim' | 'train'
  trainActions: 'all' | 'final'
}

export function DealsCard({ traderId }: { traderId: string }) {
  const [status, setStatus] = useState<'all' | 'open' | 'closed'>('all')
  const [side, setSide] = useState<'all' | 'long' | 'short'>('all')
  const [q, setQ] = useState('')
  const [pnl, setPnl] = useState<'all' | 'win' | 'loss'>('all')
  const [pnlMin, setPnlMin] = useState('')
  const [pnlMax, setPnlMax] = useState('')

  const [range, setRange] = useState<'24h' | '7d' | '30d' | 'all'>('7d')
  const [symbol, setSymbol] = useState('')
  const [jsonlMode, setJsonlMode] = useState<'full' | 'slim' | 'train'>('full')
  const [trainActions, setTrainActions] = useState<'all' | 'final'>('all')
  const [presets, setPresets] = useState<DealsPreset[]>([])
  const [presetsLoaded, setPresetsLoaded] = useState(false)
  const [selectedPresetName, setSelectedPresetName] = useState('')

  useEffect(() => {
    try {
      const raw = localStorage.getItem(PRESET_STORAGE_KEY)
      if (raw) {
        const parsed = JSON.parse(raw)
        if (Array.isArray(parsed)) {
          setPresets(parsed)
        }
      }
    } catch (error) {
      console.warn('Failed to load deal presets', error)
    } finally {
      setPresetsLoaded(true)
    }
  }, [])

  useEffect(() => {
    if (presetsLoaded) {
      localStorage.setItem(PRESET_STORAGE_KEY, JSON.stringify(presets))
    }
  }, [presets, presetsLoaded])

  const { from, to } = useMemo(() => {
    if (range === 'all') return { from: undefined as string | undefined, to: undefined as string | undefined }
    const now = new Date()
    let fromDate = new Date()
    if (range === '24h') fromDate = new Date(now.getTime() - 24 * 60 * 60 * 1000)
    if (range === '7d') fromDate = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)
    if (range === '30d') fromDate = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
    const fmt = (d: Date) => `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')} ${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}:${String(d.getSeconds()).padStart(2,'0')}`
    return { from: fmt(fromDate), to: fmt(now) }
  }, [range])

  const [pageSize, setPageSize] = useState(50)
  const [page, setPage] = useState(1)
  const parsePnlBound = (value: string) => {
    if (value === '') return undefined
    const num = Number(value)
    return Number.isNaN(num) ? undefined : num
  }
  const numericPnlMin = parsePnlBound(pnlMin)
  const numericPnlMax = parsePnlBound(pnlMax)
  const params = useMemo(() => {
    return {
      trader_id: traderId,
      status: status === 'all' ? undefined : status,
      side: side === 'all' ? undefined : side,
      symbol: symbol || undefined,
      from,
      to,
      q: q || undefined,
      pnl: pnl === 'all' ? undefined : pnl,
      pnl_min: numericPnlMin,
      pnl_max: numericPnlMax,
      limit: pageSize,
      offset: (page - 1) * pageSize,
    }
  }, [traderId, status, side, symbol, from, to, q, pnl, numericPnlMin, numericPnlMax, page, pageSize])

  const { data: deals } = useSWR<Deal[]>(
    traderId ? ['deals', traderId, status, side, symbol, range, q, pnl, numericPnlMin, numericPnlMax, page, pageSize] : null,
    () => api.getDeals(params),
    { refreshInterval: 30000, revalidateOnFocus: false }
  )

  const { data: total } = useSWR<number>(
    traderId ? ['deals-count', traderId, status, side, symbol, range, q, pnl, numericPnlMin, numericPnlMax] : null,
    () => api.getDealsCount({ trader_id: traderId, status: params.status, side: params.side, symbol: params.symbol, from: params.from, to: params.to, q: params.q, pnl: params.pnl as any, pnl_min: numericPnlMin, pnl_max: numericPnlMax }),
    { refreshInterval: 60000, revalidateOnFocus: false }
  )

  async function download(url: string, filename: string) {
    const headers: Record<string, string> = {}
    const token = localStorage.getItem('auth_token')
    if (token) headers['Authorization'] = `Bearer ${token}`
    const res = await fetch(url, { headers })
    if (!res.ok) throw new Error('Export failed')
    const blob = await res.blob()
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = filename
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(link.href)
  }
  const exportCSV = () => {
    const url = api.getDealsExportURL('csv', {
      trader_id: traderId,
      status: params.status,
      side: params.side,
      symbol: params.symbol,
      from: params.from,
      to: params.to,
      q: params.q,
      pnl: params.pnl as any,
      pnl_min: numericPnlMin,
      pnl_max: numericPnlMax,
      columns: selectedColumns,
    })
    download(url, 'deals.csv')
  }
  const exportJSONL = () => {
    const url = api.getDealsExportURL('jsonl', {
      trader_id: traderId,
      status: params.status,
      side: params.side,
      symbol: params.symbol,
      from: params.from,
      to: params.to,
      q: params.q,
      pnl: params.pnl as any,
      pnl_min: numericPnlMin,
      pnl_max: numericPnlMax,
      mode: jsonlMode,
      train_actions: jsonlMode === 'train' ? trainActions : undefined,
    })
    download(url, 'deals.jsonl')
  }

  const allColumns = [
    'id','user_id','trader_id','exchange','symbol','side','leverage','position_size_usd','quantity','open_price','open_time','open_order_id','stop_loss','take_profit','close_price','close_time','realized_pnl','realized_pnl_pct','status'
  ] as const
  const [selectedColumns, setSelectedColumns] = useState<string[]>(['id','symbol','side','leverage','quantity','open_price','close_price','realized_pnl','realized_pnl_pct','status'])

  const toggleCol = (c: string) => {
    setSelectedColumns((prev) => prev.includes(c) ? prev.filter(x => x!==c) : [...prev, c])
  }

  const applyPresetState = (preset: DealsPreset) => {
    setStatus(preset.filters.status)
    setSide(preset.filters.side)
    setRange(preset.filters.range)
    setSymbol(preset.filters.symbol)
    setQ(preset.filters.q)
    setPnl(preset.filters.pnl)
    setPnlMin(preset.filters.pnlMin)
    setPnlMax(preset.filters.pnlMax)
    setSelectedColumns(preset.columns.length ? preset.columns : [...allColumns])
    setJsonlMode(preset.jsonlMode)
    setTrainActions(preset.trainActions)
    setPage(1)
  }

  const handlePresetSelect = (name: string) => {
    setSelectedPresetName(name)
    const preset = presets.find((p) => p.name === name)
    if (preset) {
      applyPresetState(preset)
    }
  }

  const savePreset = () => {
    const name = prompt('Preset name', selectedPresetName || 'My preset')
    if (!name) return
    const preset: DealsPreset = {
      name,
      createdAt: Date.now(),
      filters: {
        status,
        side,
        range,
        symbol,
        q,
        pnl,
        pnlMin,
        pnlMax,
      },
      columns: selectedColumns,
      jsonlMode,
      trainActions,
    }
    setPresets((prev) => [...prev.filter((p) => p.name !== name), preset])
    setSelectedPresetName(name)
  }

  const deletePreset = () => {
    if (!selectedPresetName) return
    setPresets((prev) => prev.filter((p) => p.name !== selectedPresetName))
    setSelectedPresetName('')
  }

  const [selectedDealId, setSelectedDealId] = useState<number | null>(null)

  return (
    <div className="binance-card p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
          📜 Deals
        </h2>
        <div className="flex items-center gap-2">
          <input
            placeholder="Symbol..."
            value={symbol}
            onChange={(e) => setSymbol(e.target.value.toUpperCase())}
            className="px-2 py-1 rounded text-sm"
            style={{ background: '#0E1318', border: '1px solid #2B3139', color: '#EAECEF' }}
          />
          <input
            placeholder="Search in prompts/reasoning..."
            value={q}
            onChange={(e) => { setQ(e.target.value); setPage(1) }}
            className="px-2 py-1 rounded text-sm"
            style={{ background: '#0E1318', border: '1px solid #2B3139', color: '#EAECEF', width: '240px' }}
          />
          <select
            value={range}
            onChange={(e) => setRange(e.target.value as any)}
            className="px-2 py-1 rounded text-sm"
            style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
          >
            <option value="24h">24h</option>
            <option value="7d">7d</option>
            <option value="30d">30d</option>
            <option value="all">All</option>
          </select>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value as any)}
            className="px-2 py-1 rounded text-sm"
            style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
          >
            <option value="all">All</option>
            <option value="open">Open</option>
            <option value="closed">Closed</option>
          </select>
          <select
            value={pnl}
            onChange={(e) => setPnl(e.target.value as any)}
            className="px-2 py-1 rounded text-sm"
            style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
          >
            <option value="all">PnL: All</option>
            <option value="win">PnL: &gt; 0</option>
            <option value="loss">PnL: &lt; 0</option>
          </select>
          <input
            type="number"
            placeholder="PnL ≥"
            value={pnlMin}
            onChange={(e) => { setPnlMin(e.target.value); setPage(1) }}
            className="px-2 py-1 rounded text-sm w-28"
            style={{ background: '#0E1318', border: '1px solid #2B3139', color: '#EAECEF' }}
          />
          <input
            type="number"
            placeholder="PnL ≤"
            value={pnlMax}
            onChange={(e) => { setPnlMax(e.target.value); setPage(1) }}
            className="px-2 py-1 rounded text-sm w-28"
            style={{ background: '#0E1318', border: '1px solid #2B3139', color: '#EAECEF' }}
          />
          <select
            value={side}
            onChange={(e) => setSide(e.target.value as any)}
            className="px-2 py-1 rounded text-sm"
            style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
          >
            <option value="all">All</option>
            <option value="long">Long</option>
            <option value="short">Short</option>
          </select>
          <select
            value={jsonlMode}
            onChange={(e) => setJsonlMode(e.target.value as any)}
            className="px-2 py-1 rounded text-sm"
            style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
          >
            <option value="full">JSONL: Full</option>
            <option value="slim">JSONL: Slim</option>
            <option value="train">JSONL: Train</option>
          </select>
          {jsonlMode === 'train' && (
            <select
              value={trainActions}
              onChange={(e) => setTrainActions(e.target.value as any)}
              className="px-2 py-1 rounded text-sm"
              style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
            >
              <option value="all">Train actions: All</option>
              <option value="final">Train actions: Final only</option>
            </select>
          )}
          <button onClick={exportCSV} className="px-3 py-1 rounded text-sm" style={{ background: '#2B3139', color: '#EAECEF' }}>Export CSV</button>
          <button onClick={exportJSONL} className="px-3 py-1 rounded text-sm" style={{ background: '#2B3139', color: '#EAECEF' }}>Export JSONL</button>
        </div>
      </div>

      {/* Column selection */}
      <div className="mb-3 flex flex-wrap gap-2 text-xs" style={{ color: '#EAECEF' }}>
        {allColumns.map((c) => (
          <label key={c} className="inline-flex items-center gap-1 px-2 py-1 rounded" style={{ background: '#0E1318', border: '1px solid #2B3139' }}>
            <input type="checkbox" checked={selectedColumns.includes(c)} onChange={() => toggleCol(c)} />
            <span>{c}</span>
          </label>
        ))}
      </div>

      <div className="flex flex-wrap items-center gap-2 mb-4">
        <select
          value={selectedPresetName}
          onChange={(e) => handlePresetSelect(e.target.value)}
          className="px-2 py-1 rounded text-sm"
          style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
        >
          <option value="">Presets</option>
          {presets.map((preset) => (
            <option key={preset.name} value={preset.name}>💾 {preset.name}</option>
          ))}
        </select>
        <button onClick={savePreset} className="px-3 py-1 rounded text-sm" style={{ background: '#2B3139', color: '#EAECEF' }}>Save Preset</button>
        <button onClick={deletePreset} disabled={!selectedPresetName} className="px-3 py-1 rounded text-sm" style={{ background: '#2B3139', color: '#EAECEF', opacity: selectedPresetName ? 1 : 0.5 }}>Delete Preset</button>
      </div>

      {deals && deals.length > 0 ? (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left" style={{ color: '#848E9C' }}>
                <th className="py-2">Time</th>
                <th className="py-2">Symbol</th>
                <th className="py-2">Side</th>
                <th className="py-2">Lev</th>
                <th className="py-2">Qty</th>
                <th className="py-2">Open</th>
                <th className="py-2">Close</th>
                <th className="py-2">PnL</th>
                <th className="py-2">Status</th>
              </tr>
            </thead>
            <tbody>
              {deals.map((d) => (
                <tr key={d.id} className="border-t cursor-pointer hover:bg-white/5" style={{ borderColor: '#2B3139' }} onClick={() => setSelectedDealId(d.id)}>
                  <td className="py-2 font-mono">{new Date(d.open_time).toLocaleString()}</td>
                  <td className="py-2 font-mono">{d.symbol}</td>
                  <td className="py-2 font-mono" style={{ color: d.side === 'long' ? '#0ECB81' : '#F6465D' }}>{d.side.toUpperCase()}</td>
                  <td className="py-2 font-mono">{d.leverage}x</td>
                  <td className="py-2 font-mono">{Number.isFinite(Number(d.quantity)) ? Number(d.quantity).toFixed(4) : '-'}</td>
                  <td className="py-2 font-mono">{Number.isFinite(Number(d.open_price)) ? Number(d.open_price).toFixed(4) : '-'}</td>
                  <td className="py-2 font-mono">{d.close_price === null || d.close_price === undefined ? '-' : (Number.isFinite(Number(d.close_price)) ? Number(d.close_price).toFixed(4) : '-')}</td>
                  <td className="py-2 font-mono" style={{ color: (d.realized_pnl ?? 0) >= 0 ? '#0ECB81' : '#F6465D', fontWeight: 'bold' }}>
                    {d.realized_pnl === null || d.realized_pnl === undefined ? '-' : `${Number(d.realized_pnl) >= 0 ? '+' : ''}${Number.isFinite(Number(d.realized_pnl)) ? Number(d.realized_pnl).toFixed(2) : '0.00'} (${Number.isFinite(Number(d.realized_pnl_pct)) ? Number(d.realized_pnl_pct).toFixed(2) : '0.00'}%)`}
                  </td>
                  <td className="py-2 font-mono" style={{ color: d.status === 'open' ? '#F0B90B' : '#848E9C' }}>{d.status}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="text-center py-12" style={{ color: '#848E9C' }}>
          No deals yet.
        </div>
      )}

      {/* Pagination */}
      <div className="flex items-center justify-between mt-3">
        <div className="flex items-center gap-2">
          <button disabled={page<=1} onClick={() => setPage(p => Math.max(1, p-1))} className="px-3 py-1 rounded" style={{ background: '#2B3139', color: '#EAECEF', opacity: page<=1 ? .5 : 1 }}>Prev</button>
          <span className="text-sm" style={{ color: '#848E9C' }}>Page {page}{total !== undefined ? ` / ${Math.max(1, Math.ceil(total / pageSize))}` : ''}</span>
          <button disabled={total !== undefined ? (page * pageSize >= (total || 0)) : !(deals && deals.length === pageSize)} onClick={() => setPage(p => p+1)} className="px-3 py-1 rounded" style={{ background: '#2B3139', color: '#EAECEF', opacity: total !== undefined ? (page * pageSize < (total || 0) ? 1 : .5) : (deals && deals.length === pageSize ? 1 : .5) }}>Next</button>
        </div>
        <div className="flex items-center gap-2 text-sm" style={{ color: '#848E9C' }}>
          <span>Page size</span>
          <select value={pageSize} onChange={(e) => { setPageSize(parseInt(e.target.value)); setPage(1) }} className="px-2 py-1 rounded text-sm" style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}>
            <option value={25}>25</option>
            <option value={50}>50</option>
            <option value={100}>100</option>
          </select>
        </div>
      </div>

      {selectedDealId && (
        <DealDetailModal traderId={traderId} dealId={selectedDealId} onClose={() => setSelectedDealId(null)} />
      )}
    </div>
  )
}
