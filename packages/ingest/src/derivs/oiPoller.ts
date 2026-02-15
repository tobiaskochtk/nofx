import pLimit from 'p-limit'
import { DerivsCachePublisher } from './cache.js'
import { resampleLatest } from './resample.js'
import { parseInterval } from './time.js'
import type { CachePayload, DerivsConfig, OISample, SourceStatus, VenueAdapter, VenueKey } from './types.js'

export class OIPoller {
  private readonly intervalMs: number
  private readonly limiter = pLimit(4)

  constructor(
    private readonly cfg: DerivsConfig,
    private readonly publisher: DerivsCachePublisher,
    private readonly adapters: Record<VenueKey, VenueAdapter>
  ) {
    this.intervalMs = parseInterval(cfg.timings.sample_interval)
  }

  async run(): Promise<void> {
    await Promise.all(this.cfg.symbols.map((symbol) => this.processSymbol(symbol)))
  }

  private async processSymbol(symbol: string): Promise<void> {
    const samples: CachePayload<OISample>['samples'] = {}
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
            const raw = await adapter.fetchOpenInterest(symbol)
            if (!raw.length) {
              status[venue] = 'miss'
              return
            }
            samples[venue] = resampleLatest(raw, this.intervalMs)
            status[venue] = 'ok'
          } catch (err) {
            console.error(`[OIPoller] ${venue} ${symbol}`, err)
            status[venue] = 'miss'
          }
        })
      )
    )

    await this.publisher.publish('oi', symbol, {
      symbol,
      samples,
      status,
      updated_at: Date.now()
    })
  }
}
