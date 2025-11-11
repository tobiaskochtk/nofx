import pLimit from 'p-limit'
import { DerivsCachePublisher } from './cache.js'
import type { CachePayload, DerivsConfig, FundingSample, SourceStatus, VenueAdapter, VenueKey } from './types.js'

export class FundingPoller {
  private readonly limiter = pLimit(4)

  constructor(
    private readonly cfg: DerivsConfig,
    private readonly publisher: DerivsCachePublisher,
    private readonly adapters: Record<VenueKey, VenueAdapter>
  ) {}

  async run(): Promise<void> {
    await Promise.all(this.cfg.symbols.map((symbol) => this.processSymbol(symbol)))
  }

  private async processSymbol(symbol: string): Promise<void> {
    const samples: CachePayload<FundingSample>['samples'] = {}
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
            const raw = await adapter.fetchFunding(symbol)
            const trimmed = raw.slice(-this.cfg.timings.funding_window_intervals)
            samples[venue] = trimmed
            status[venue] = trimmed.length ? 'ok' : 'miss'
          } catch (err) {
            console.error(`[FundingPoller] ${venue} ${symbol}`, err)
            status[venue] = 'miss'
          }
        })
      )
    )

    await this.publisher.publish('funding', symbol, {
      symbol,
      samples,
      status,
      updated_at: Date.now()
    })
  }
}
