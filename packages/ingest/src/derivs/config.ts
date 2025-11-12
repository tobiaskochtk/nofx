import fs from 'node:fs'
import path from 'node:path'
import yaml from 'js-yaml'
import { z } from 'zod'
import type { DerivsConfig } from './types.js'

const schema = z.object({
  derivs_v1: z.object({
    enabled: z.boolean().default(false),
    symbols: z.array(z.string()),
    exchangesPerp: z.array(z.string()),
    spotIndexExchanges: z.array(z.string()),
    timings: z.object({
      sample_interval: z.string(),
      oi_z_window_hours: z.number().int(),
      basis_z_window_hours: z.number().int(),
      funding_window_intervals: z.number().int(),
      min_history_hours: z.number().int()
    }),
    smoothing_alpha: z.number(),
    timeouts_ms: z.object({ rest: z.number().int(), ws: z.number().int() }),
    cache: z
      .object({
        redis_addr: z.string(),
        redis_password: z.string().default(''),
        redis_db: z.number().int().default(0),
        keyspace: z.string().default('derivs:v1')
      })
      .default({ redis_addr: '127.0.0.1:6379', redis_password: '', redis_db: 0, keyspace: 'derivs:v1' })
  })
})

export function loadDerivsConfig(configPath = path.resolve(process.cwd(), '../../config/derivs_v1.yaml')): DerivsConfig {
  const file = fs.readFileSync(configPath, 'utf-8')
  const parsed = schema.parse(yaml.load(file) ?? {})
  if (!parsed.derivs_v1.enabled) {
    throw new Error('derivs_v1.enabled=false; nothing to ingest')
  }
  const cfg = parsed.derivs_v1 as DerivsConfig
  const { DERIVS_REDIS_ADDR, DERIVS_REDIS_PASSWORD, DERIVS_REDIS_DB, DERIVS_REDIS_KEYSPACE } = process.env
  if (DERIVS_REDIS_ADDR) {
    cfg.cache.redis_addr = DERIVS_REDIS_ADDR
  }
  if (DERIVS_REDIS_PASSWORD) {
    cfg.cache.redis_password = DERIVS_REDIS_PASSWORD
  }
  if (DERIVS_REDIS_DB && !Number.isNaN(Number(DERIVS_REDIS_DB))) {
    cfg.cache.redis_db = Number(DERIVS_REDIS_DB)
  }
  if (DERIVS_REDIS_KEYSPACE) {
    cfg.cache.keyspace = DERIVS_REDIS_KEYSPACE
  }
  return cfg
}
