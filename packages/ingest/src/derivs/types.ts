import type { Exchange } from 'ccxt'

export type VenueKey = 'binanceusdm' | 'bybit' | 'okx' | 'bitget'
export type SourceStatus = 'ok' | 'miss' | 'stale'

export interface OISample { ts: number; venue: VenueKey; value: number }
export interface FundingSample { ts: number; venue: VenueKey; rate: number }
export interface BasisSample { ts: number; venue: VenueKey; mark: number; index: number }

export interface DerivsConfig {
  symbols: string[]
  exchangesPerp: VenueKey[]
  spotIndexExchanges: string[]
  timings: {
    sample_interval: string
    oi_z_window_hours: number
    basis_z_window_hours: number
    funding_window_intervals: number
    min_history_hours: number
  }
  smoothing_alpha: number
  timeouts_ms: { rest: number; ws: number }
  cache: RedisCacheConfig
}

export interface CachePayload<TSample> {
  symbol: string
  updated_at: number
  samples: Partial<Record<string, TSample[]>>
  status: Partial<Record<string, SourceStatus>>
}

export interface RedisCacheConfig {
  redis_addr: string
  redis_password?: string
  redis_db: number
  keyspace: string
}
export interface VenueAdapter {
  venue: VenueKey
  client: Exchange
  hasMarket(symbol: string): boolean
  fetchOpenInterest(symbol: string): Promise<OISample[]>
  fetchFunding(symbol: string): Promise<FundingSample[]>
  fetchBasis(symbol: string, spotIndexFn: SpotIndexResolver): Promise<BasisSample | null>
}

export type SpotIndexResolver = (symbol: string) => Promise<number | null>
