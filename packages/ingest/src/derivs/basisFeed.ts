import ccxt from 'ccxt'
import type { Exchange } from 'ccxt'
import pLimit from 'p-limit'
import { DerivsCachePublisher } from './cache.js'
import type { BasisSample, DerivsConfig, SourceStatus, SpotIndexResolver, VenueAdapter, VenueKey } from './types.js'

export class BasisFeed {
  private readonly limiter = pLimit(4)
  private readonly spotResolver: SpotIndexResolver

  constructor(
    private readonly cfg: DerivsConfig,
    private readonly publisher: DerivsCachePublisher,
    private readonly adapters: Record<VenueKey, VenueAdapter>
  ) {
    this.spotResolver = createSpotResolver(cfg.spotIndexExchanges, cfg.timeouts_ms.rest)
  }

  async run(): Promise<void> {
    await Promise.all(this.cfg.symbols.map((symbol) => this.processSymbol(symbol)))
  }

  private async processSymbol(symbol: string): Promise<void> {
    const samples: Record<string, BasisSample[]> = {}
    const status: Record<string, SourceStatus> = {}
    await Promise.all(
      this.cfg.exchangesPerp.map((venue) =>
        this.limiter(async () => {
          const adapter = this.adapters[venue]
          if (!adapter || !adapter.hasMarket(symbol)) {
            status[venue] = 'miss'
            return
          }
          try {
            const sample = await adapter.fetchBasis(symbol, this.spotResolver)
            if (sample) {
              samples[venue] = [sample]
              status[venue] = 'ok'
            } else {
              status[venue] = 'miss'
            }
          } catch (err) {
            console.error(`[BasisFeed] ${venue} ${symbol}`, err)
            status[venue] = 'miss'
          }
        })
      )
    )

    await this.publisher.publish('basis', symbol, {
      symbol,
      samples,
      status,
      updated_at: Date.now()
    })
  }
}

function createSpotResolver(exchanges: string[], timeoutMs: number): SpotIndexResolver {
  const clients: Exchange[] = exchanges
    .map((id) => {
      const Ctor = (ccxt as any)[id]
      if (!Ctor) {
        return null
      }
      return new Ctor({ enableRateLimit: true, timeout: timeoutMs }) as Exchange
    })
    .filter((client): client is Exchange => Boolean(client))

  return async (symbol: string) => {
    const spotSymbol = deriveSpotSymbol(symbol)
    for (const client of clients) {
      try {
        const ticker = await client.fetchTicker(spotSymbol)
        const price = Number(ticker.last ?? ticker.info?.price ?? ticker.info?.lastPrice)
        if (price && Number.isFinite(price)) {
          return price
        }
      } catch (err) {
        console.warn(`[SpotResolver] ${client.id} ${spotSymbol}`, err)
      }
    }
    return null
  }
}

function deriveSpotSymbol(perpSymbol: string): string {
  if (perpSymbol.includes(':')) {
    return perpSymbol.split(':')[0]
  }
  const normalized = perpSymbol.toUpperCase().replace(/[\\/:\\-]/g, '')
  const parts = [normalized.slice(0, -4), normalized.slice(-4)]
  return `${parts[0]}/${parts[1]}`
}
