export function parseInterval(value: string): number {
  const match = value.trim().match(/^(\d+)([smhd])$/)
  if (!match) {
    throw new Error(`unsupported interval: ${value}`)
  }
  const amount = Number(match[1])
  const unit = match[2]
  const msMap: Record<string, number> = {
    s: 1000,
    m: 60 * 1000,
    h: 3600 * 1000,
    d: 24 * 3600 * 1000
  }
  return amount * msMap[unit]
}
