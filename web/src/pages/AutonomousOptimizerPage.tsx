import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { AutonomousOptimizerPanel } from '../components/trader/AutonomousOptimizerPanel'
import { NofxSelect } from '../components/ui/select'
import { useLanguage } from '../contexts/LanguageContext'
import type { TraderInfo } from '../types'

interface AutonomousOptimizerPageProps {
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
}

export function AutonomousOptimizerPage({
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
}: AutonomousOptimizerPageProps) {
  const { language } = useLanguage()
  const pickText = (values: { en: string; de: string }) =>
    language === 'de' ? values.de : values.en
  const selectedTrader = traders?.find(
    (item) => item.trader_id === selectedTraderId
  )

  const openDealReview = (params: Record<string, string>) => {
    const url = new URL(window.location.href)
    url.pathname = '/deal-review'
    Object.entries(params).forEach(([key, value]) => {
      if (value) {
        url.searchParams.set(key, value)
      }
    })
    window.history.pushState({}, '', url.toString())
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  if (tradersError) {
    return (
      <div className="p-8 text-red-400">
        {pickText({
          en: 'Failed to load traders',
          de: 'Trader konnten nicht geladen werden',
        })}
        : {tradersError.message}
      </div>
    )
  }

  return (
    <DeepVoidBackground className="min-h-screen pb-12" disableAnimation>
      <div className="w-full px-4 md:px-8 relative z-10 pt-6 space-y-6">
        <div className="nofx-glass rounded-xl p-6">
          <div className="flex flex-col lg:flex-row lg:items-end gap-4">
            <div className="flex-1">
              <div className="text-xs uppercase tracking-[0.24em] text-nofx-gold/80 mb-2">
                {pickText({
                  en: 'Autonomous Optimizer',
                  de: 'Autonomer Optimierer',
                })}
              </div>
              <h1 className="text-3xl font-semibold text-nofx-text-main">
                {pickText({
                  en: 'Self-improving trader loop',
                  de: 'Sich selbst verbessernde Trader-Schleife',
                })}
              </h1>
              <p className="text-sm text-nofx-text-muted mt-2">
                {pickText({
                  en: 'Inspect the optimizer separately from deal review: runs, gate results, scored improvement backlog, model pair, and direct jumps into the linked review cohort or strategy version.',
                  de: 'Pruefe den Optimierer getrennt vom Deal-Review: Laeufe, Gate-Ergebnisse, bewerteten Verbesserungs-Backlog, Modellpaar und direkte Spruenge in die verknuepfte Review-Kohorte oder Strategieversion.',
                })}
              </p>
            </div>
            <div className="w-full lg:w-80">
              <label className="text-xs text-nofx-text-muted block mb-2">
                {pickText({ en: 'Trader', de: 'Trader' })}
              </label>
              <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center nofx-glass">
                <NofxSelect
                  value={selectedTraderId || ''}
                  onChange={onTraderSelect}
                  options={(traders || []).map((item) => ({
                    value: item.trader_id,
                    label: item.trader_name,
                  }))}
                />
              </div>
            </div>
          </div>
        </div>

        <AutonomousOptimizerPanel
          traderId={selectedTraderId}
          traderName={selectedTrader?.trader_name}
          onOpenStrategyVersion={(versionId) =>
            openDealReview({ optimizer_version_id: versionId })
          }
          onApplyRunWindowFilter={(fromTime, toTime) =>
            openDealReview({
              optimizer_from_time: String(fromTime),
              optimizer_to_time: String(toTime),
            })
          }
        />
      </div>
    </DeepVoidBackground>
  )
}
