export function resampleLatest<T extends { ts: number }>(samples: T[], intervalMs: number): T[] {
  const buckets = new Map<number, T>()
  for (const sample of samples) {
    const bucket = Math.floor(sample.ts / intervalMs) * intervalMs
    const existing = buckets.get(bucket)
    if (!existing || sample.ts > existing.ts) {
      buckets.set(bucket, sample)
    }
  }
  return Array.from(buckets.entries())
    .sort((a, b) => a[0] - b[0])
    .map(([, sample]) => sample)
}
