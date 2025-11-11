import ccxt from 'ccxt'
import type { Exchange } from 'ccxt'
import type { BasisSample, FundingSample, OISample, SpotIndexResolver, VenueAdapter, VenueKey } from './types.js'

const venueToExchange: Record<VenueKey, keyof typeof ccxt> = {
  binanceusdm: 'binanceusdm',
  bybit: 'bybit',
  okx: 'okx',
  bitget: 'bitget'
}

type ExchangeCtor = new (params?: Record<string, unknown>) => Exchange

export async function createVenueAdapters(venues: VenueKey[], timeoutMs: number): Promise<Record<VenueKey, VenueAdapter>> {
  const adapters: Partial<Record<VenueKey, VenueAdapter>> = {}
  for (const venue of venues) {
    const id = venueToExchange[venue]
    const ExchangeCtor = (ccxt as unknown as Record<string, ExchangeCtor | undefined>)[id]
    if (!ExchangeCtor) {
      continue
    }
    const client = new ExchangeCtor({ enableRateLimit: true, timeout: timeoutMs }) as Exchange
    if (client.options) {
      client.options.defaultType = 'swap'
    }
    try {
      await client.loadMarkets()
    } catch (err) {
      console.error(`[VenueAdapter] failed to load markets for ${venue}`, err)
      continue
    }
    adapters[venue] = new DefaultVenueAdapter(venue, client)
  }
  return adapters as Record<VenueKey, VenueAdapter>
}

class DefaultVenueAdapter implements VenueAdapter {
  constructor(public readonly venue: VenueKey, public readonly client: Exchange) {}

  hasMarket(symbol: string): boolean {
    return !!this.client.markets?.[symbol]
  }

  async fetchOpenInterest(symbol: string): Promise<OISample[]> {
    if (!this.client.has?.fetchOpenInterestHistory) {
      return []
    }
    const since = Date.now() - 3600 * 1000 * 336
    const entries = await this.client.fetchOpenInterestHistory(symbol, '1h', since, 200)
    if (!Array.isArray(entries)) {
      return []
    }
    return entries
      .map((row: any) => ({ ts: Number(row.timestamp ?? row.time ?? row[0]), venue: this.venue, value: Number(row.openInterest ?? row[1] ?? 0) }))
      .filter((sample: OISample) => sample.ts > 0 && Number.isFinite(sample.value))
  }

  async fetchFunding(symbol: string): Promise<FundingSample[]> {
    if (this.client.has?.fetchFundingRateHistory) {
      const entries = await this.client.fetchFundingRateHistory(symbol, undefined, undefined, 200)
      return (entries ?? [])
        .map((row: any) => ({ ts: Number(row.timestamp ?? row[0]), venue: this.venue, rate: Number(row.fundingRate ?? row[1] ?? 0) }))
        .filter((sample: FundingSample) => sample.ts > 0 && Number.isFinite(sample.rate))
    }
    if (this.client.has?.fetchFundingRate) {
      const ticker = await this.client.fetchFundingRate(symbol)
      return [
        {
          ts: Number(ticker.timestamp ?? Date.now()),
          venue: this.venue,
          rate: Number((ticker as any).fundingRate ?? 0)
        }
      ]
    }
    return []
  }

  async fetchBasis(symbol: string, resolveSpotIndex: SpotIndexResolver): Promise<BasisSample | null> {
    if (!this.client.has?.fetchTicker) {
      return null
    }
    const ticker = await this.client.fetchTicker(symbol)
    const mark = extractNumber((ticker.info && (ticker.info.markPrice ?? ticker.info.lastPrice)) ?? ticker.markPrice ?? ticker.last)
    let index = extractNumber((ticker.info && (ticker.info.indexPrice ?? ticker.info.index)) ?? (ticker as any).index)
    if (!index || index <= 0) {
      index = await resolveSpotIndex(symbol)
    }
    if (!mark || !index || index <= 0) {
      return null
    }
    return { ts: Number(ticker.timestamp ?? Date.now()), venue: this.venue, mark, index }
  }
}

function extractNumber(value: unknown): number | null {
  const num = typeof value === 'string' ? Number(value) : typeof value === 'number' ? value : null
  if (!num || !Number.isFinite(num)) {
    return null
  }
  return num
}
