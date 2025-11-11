import 'dotenv/config'
import { DerivsCachePublisher } from './cache.js'
import { BasisFeed } from './basisFeed.js'
import { FundingPoller } from './fundingPoller.js'
import { OIPoller } from './oiPoller.js'
import { parseInterval } from './time.js'
import { loadDerivsConfig } from './config.js'
import { createVenueAdapters } from './venueAdapters.js'

async function main() {
  const config = loadDerivsConfig()
  const publisher = new DerivsCachePublisher(config.cache)
  await publisher.connect()
  const adapters = await createVenueAdapters(config.exchangesPerp, config.timeouts_ms.rest)

  const oiPoller = new OIPoller(config, publisher, adapters)
  const fundingPoller = new FundingPoller(config, publisher, adapters)
  const basisFeed = new BasisFeed(config, publisher, adapters)

  const intervalMs = parseInterval(config.timings.sample_interval)

  const cycle = async () => {
    console.log(`[DerivsIngest] cycle start ${new Date().toISOString()}`)
    await oiPoller.run()
    await fundingPoller.run()
    await basisFeed.run()
    console.log(`[DerivsIngest] cycle done ${new Date().toISOString()}`)
  }

  const schedule = () => {
    cycle().catch((err) => console.error('[DerivsIngest] cycle error', err))
  }

  await cycle()
  const timer = setInterval(schedule, intervalMs)

  const gracefulShutdown = async () => {
    clearInterval(timer)
    await publisher.disconnect()
    process.exit(0)
  }
  process.once('SIGINT', gracefulShutdown)
  process.once('SIGTERM', gracefulShutdown)
}

main().catch((err) => {
  console.error('[DerivsIngest] fatal', err)
  process.exitCode = 1
})
