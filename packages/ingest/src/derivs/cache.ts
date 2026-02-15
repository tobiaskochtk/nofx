import { createClient, type RedisClientType } from 'redis'
import type { CachePayload, RedisCacheConfig, SourceStatus } from './types.js'

const MAX_POINTS = 400
const DEFAULT_STATUS: SourceStatus = 'miss'

type CacheType = 'oi' | 'funding' | 'basis'

export class DerivsCachePublisher {
  private client: RedisClientType | null = null

  constructor(private readonly cfg: RedisCacheConfig) {}

  async connect(): Promise<void> {
    if (this.client) {
      return
    }
    const { host, port } = parseAddr(this.cfg.redis_addr)
    this.client = createClient({
      socket: { host, port },
      password: this.cfg.redis_password || undefined,
      database: this.cfg.redis_db ?? 0
    })
    this.client.on('error', (err) => {
      console.error('[DerivsCachePublisher] redis error', err)
    })
    await this.client.connect()
  }

  async disconnect(): Promise<void> {
    if (this.client) {
      await this.client.quit()
      this.client = null
    }
  }

  async publish<T>(type: CacheType, symbol: string, payload: CachePayload<T>): Promise<void> {
    try {
      const client = this.ensureClient()
      const key = this.key(type, symbol)
      const existing = await this.fetchPayload<T>(client, key)
      const mergedSamples: Record<string, T[]> = {}
      const venues = new Set([...Object.keys(existing.samples), ...Object.keys(payload.samples)])
      for (const venue of venues) {
        const previous = existing.samples[venue] ?? []
        const incoming = payload.samples[venue] ?? []
        mergedSamples[venue] = trimSeries([...previous, ...incoming])
      }
      const mergedStatus = { ...existing.status }
      for (const [venue, state] of Object.entries(payload.status ?? {})) {
        mergedStatus[venue] = state ?? DEFAULT_STATUS
      }
      const body = {
        symbol,
        updated_at: Date.now(),
        samples: mergedSamples,
        status: mergedStatus
      }
      await client.set(key, JSON.stringify(body))
    } catch (err) {
      console.error(`[DerivsCachePublisher] failed to publish ${type}:${symbol}`, err)
    }
  }

  private ensureClient(): RedisClientType {
    if (!this.client) {
      throw new Error('redis client not initialized; call connect() first')
    }
    return this.client
  }

  private key(type: CacheType, symbol: string): string {
    const keyspace = this.cfg.keyspace || 'derivs:v1'
    return `${keyspace}:${type}:${sanitizeSymbol(symbol)}`
  }

  private async fetchPayload<T>(client: RedisClientType, key: string): Promise<CachePayload<T>> {
    const raw = await client.get(key)
    if (!raw) {
      return { symbol: '', updated_at: 0, samples: {}, status: {} }
    }
    try {
      return JSON.parse(raw)
    } catch (err) {
      console.warn(`[DerivsCachePublisher] failed to parse payload for ${key}, resetting`, err)
      return { symbol: '', updated_at: 0, samples: {}, status: {} }
    }
  }
}

function parseAddr(addr: string): { host: string; port: number } {
  if (!addr) {
    return { host: '127.0.0.1', port: 6379 }
  }
  const [host, port] = addr.split(':')
  return { host, port: Number(port ?? 6379) }
}

function sanitizeSymbol(symbol: string): string {
  return symbol.toUpperCase().replace(/[\/:\-]/g, '')
}

function trimSeries<T>(series: T[]): T[] {
  if (series.length <= MAX_POINTS) {
    return series
  }
  return series.slice(series.length - MAX_POINTS)
}
