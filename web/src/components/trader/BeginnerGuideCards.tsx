import { Brain, Landmark, Rocket, Sparkles } from 'lucide-react'

interface BeginnerGuideCardsProps {
  language: string
  claw402Ready: boolean
  exchangeReady: boolean
  strategyReady: boolean
  traderReady: boolean
  canCreateTrader: boolean
  walletAddress?: string | null
  onQuickSetupClaw402: () => void
  onOpenExchange: () => void
  onOpenStrategy: () => void
  onCreateTrader: () => void
}

function truncateAddress(address: string) {
  if (address.length <= 12) return address
  return `${address.slice(0, 6)}...${address.slice(-4)}`
}

export function BeginnerGuideCards({
  language,
  claw402Ready,
  exchangeReady,
  strategyReady,
  traderReady,
  canCreateTrader,
  walletAddress,
  onQuickSetupClaw402,
  onOpenExchange,
  onOpenStrategy,
  onCreateTrader,
}: BeginnerGuideCardsProps) {
  const pickText = (copy: { zh: string; de: string; en: string }) => {
    if (language === 'zh') return copy.zh
    if (language === 'de') return copy.de
    return copy.en
  }

  const cards = [
    {
      key: 'model',
      icon: Brain,
      title: pickText({
        zh: '1. 极速模型',
        de: '1. Schnellstart-KI',
        en: '1. Fast AI',
      }),
      desc: pickText({
        zh: '默认就是 Claw402 + DeepSeek。第一次不用挑模型，先跑起来。',
        de: 'Starte direkt mit Claw402 + DeepSeek. Für den ersten Lauf musst du kein Modell auswählen.',
        en: 'Start with Claw402 + DeepSeek. No model picking needed for the first run.',
      }),
      meta: walletAddress
        ? pickText({
          zh: `钱包 ${truncateAddress(walletAddress)}`,
          de: `Wallet ${truncateAddress(walletAddress)}`,
          en: `Wallet ${truncateAddress(walletAddress)}`,
        })
        : pickText({
          zh: 'Base 链 USDC 按次付费',
          de: 'Bezahlung pro Aufruf mit USDC auf Base',
          en: 'Pay per call with Base USDC',
        }),
      ready: claw402Ready,
      actionLabel: claw402Ready
        ? pickText({
          zh: '已配置',
          de: 'Konfiguriert',
          en: 'Configured',
        })
        : pickText({
          zh: '一键配置',
          de: 'Ein-Klick-Setup',
          en: 'One-click setup',
        }),
      onAction: onQuickSetupClaw402,
      disabled: claw402Ready,
    },
    {
      key: 'exchange',
      icon: Landmark,
      title: pickText({
        zh: '2. 连接交易所',
        de: '2. Börse verbinden',
        en: '2. Add Exchange',
      }),
      desc: pickText({
        zh: '交易所接好以后，AI 才能真正下单。',
        de: 'Erst mit einer verbundenen Börse kann die KI tatsächlich Orders platzieren.',
        en: 'Connect an exchange so the AI can actually place trades.',
      }),
      meta: exchangeReady
        ? pickText({
          zh: '已准备好',
          de: 'Bereit',
          en: 'Ready',
        })
        : 'Binance / OKX / Bybit / Hyperliquid',
      ready: exchangeReady,
      actionLabel: exchangeReady
        ? pickText({
          zh: '继续管理',
          de: 'Verwalten',
          en: 'Manage',
        })
        : pickText({
          zh: '去配置',
          de: 'Konfigurieren',
          en: 'Configure',
        }),
      onAction: onOpenExchange,
      disabled: false,
    },
    {
      key: 'strategy',
      icon: Sparkles,
      title: pickText({
        zh: '3. 选择策略',
        de: '3. Strategie wählen',
        en: '3. Pick Strategy',
      }),
      desc: pickText({
        zh: '先用默认策略也可以，后面再慢慢细调。',
        de: 'Du kannst mit der Standardstrategie starten und später im Detail nachschärfen.',
        en: 'You can start with a default strategy and fine-tune later.',
      }),
      meta: strategyReady
        ? pickText({
          zh: '已有策略可用',
          de: 'Strategie bereit',
          en: 'Strategy ready',
        })
        : pickText({
          zh: '可选，但建议提前看一眼',
          de: 'Optional, aber ein kurzer Blick lohnt sich',
          en: 'Optional, but worth a quick look',
        }),
      ready: strategyReady,
      actionLabel: pickText({
        zh: '打开策略页',
        de: 'Strategie öffnen',
        en: 'Open strategy',
      }),
      onAction: onOpenStrategy,
      disabled: false,
    },
    {
      key: 'trader',
      icon: Rocket,
      title: pickText({
        zh: '4. 创建 Trader',
        de: '4. Trader erstellen',
        en: '4. Create Trader',
      }),
      desc: pickText({
        zh: '最后一步，把模型和交易所绑在一起，就能开始运行。',
        de: 'Letzter Schritt: Modell und Börse verbinden, dann kann der Trader starten.',
        en: 'Last step: bind your model and exchange, then start running.',
      }),
      meta: traderReady
        ? pickText({
          zh: '已创建 Trader，可继续添加',
          de: 'Trader erstellt, du kannst weitere hinzufügen',
          en: 'Trader created, you can add more',
        })
        : canCreateTrader
          ? pickText({
            zh: '已经可以创建',
            de: 'Bereit zum Erstellen',
            en: 'Ready to create',
          })
        : pickText({
          zh: '先完成前三步',
          de: 'Bitte zuerst die ersten drei Schritte abschließen',
          en: 'Finish the first three steps first',
        }),
      ready: traderReady,
      actionLabel: traderReady
        ? pickText({
          zh: '继续创建',
          de: 'Weiteren erstellen',
          en: 'Create another',
        })
        : pickText({
          zh: '立即创建',
          de: 'Jetzt erstellen',
          en: 'Create now',
        }),
      onAction: onCreateTrader,
      disabled: !canCreateTrader,
    },
  ]

  return (
    <section className="space-y-4 rounded-[28px] border border-white/10 bg-zinc-950/60 p-5 backdrop-blur-xl">
      <div className="flex items-center justify-between gap-4">
        <div>
          <div className="text-xs font-semibold uppercase tracking-[0.3em] text-nofx-gold/80">
            {pickText({
              zh: '新手引导',
              de: 'Schnellstart',
              en: 'Quickstart',
            })}
          </div>
          <h2 className="mt-1 text-xl font-bold text-white">
            {pickText({
              zh: '先按这 4 步走，最快上手',
              de: 'Mit diesen 4 Schritten bist du am schnellsten startklar',
              en: 'Follow these 4 steps to get started fast',
            })}
          </h2>
        </div>
        {/* <div className="rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs text-zinc-400">
          {isZh ? '老手模式不会看到这块' : 'Hidden in advanced mode'}
        </div> */}
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {cards.map((card) => {
          const Icon = card.icon
          return (
            <div
              key={card.key}
              className="rounded-[22px] border border-white/8 bg-black/25 p-4"
            >
              <div className="flex items-center justify-between gap-3">
                <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-white/6 text-nofx-gold">
                  <Icon className="h-5 w-5" />
                </div>
                <span
                  className={`rounded-full px-2.5 py-1 text-[10px] font-bold uppercase tracking-[0.22em] ${
                    card.ready
                      ? 'bg-emerald-500/15 text-emerald-300'
                      : 'bg-zinc-800 text-zinc-400'
                  }`}
                >
                  {card.ready
                    ? pickText({
                      zh: '已就绪',
                      de: 'Bereit',
                      en: 'Ready',
                    })
                    : pickText({
                      zh: '待完成',
                      de: 'Offen',
                      en: 'Pending',
                    })}
                </span>
              </div>

              <h3 className="mt-4 text-base font-semibold text-white">
                {card.title}
              </h3>
              <p className="mt-2 min-h-[72px] text-sm leading-6 text-zinc-400">
                {card.desc}
              </p>
              <div className="mt-3 text-xs text-zinc-500">{card.meta}</div>

              <button
                type="button"
                onClick={card.onAction}
                disabled={card.disabled}
                className={`mt-5 w-full rounded-2xl px-4 py-3 text-sm font-semibold transition ${
                  card.disabled
                    ? 'cursor-not-allowed bg-zinc-900 text-zinc-500'
                    : 'bg-nofx-gold text-black hover:bg-yellow-400'
                }`}
              >
                {card.actionLabel}
              </button>
            </div>
          )
        })}
      </div>
    </section>
  )
}
