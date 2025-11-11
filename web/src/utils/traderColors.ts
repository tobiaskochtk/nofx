// Trader颜色配置 - 統一的颜色分配逻辑 | Trader color configuration - Unified color allocation logic
// 用于 ComparisonChart 和 Leaderboard，確保颜色一致性 | Used for ComparisonChart and Leaderboard, ensuring color consistency

export const TRADER_COLORS = [
  '#60a5fa', // blue-400
  '#c084fc', // purple-400
  '#34d399', // emerald-400
  '#fb923c', // orange-400
  '#f472b6', // pink-400
  '#fbbf24', // amber-400
  '#38bdf8', // sky-400
  '#a78bfa', // violet-400
  '#4ade80', // green-400
  '#fb7185', // rose-400
]

/**
 * 根據trader的索引位置獲取颜色 | Get color based on trader's index position
 * @param traders - trader列表 | trader list
 * @param traderId - 當前trader的ID | current trader's ID
 * @returns 対応的颜色值 | corresponding color value
 */
export function getTraderColor(
  traders: Array<{ trader_id: string }>,
  traderId: string
): string {
  const traderIndex = traders.findIndex((t) => t.trader_id === traderId)
  if (traderIndex === -1) return TRADER_COLORS[0] // 默認返回第一個颜色 | Return first color by default
  // 如果超出颜色池大小，循環使用 | If exceeds color pool size, use cyclically
  return TRADER_COLORS[traderIndex % TRADER_COLORS.length]
}
