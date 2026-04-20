import { motion } from 'framer-motion'
import { Brain, Swords, BarChart3, Shield, Blocks, LineChart } from 'lucide-react'
import { t, Language } from '../../i18n/translations'

interface FeaturesSectionProps {
  language: Language
}

export default function FeaturesSection({ language }: FeaturesSectionProps) {
  const pickText = (values: Record<Language, string>) => values[language]

  const features = [
    {
      icon: Brain,
      title: pickText({
        zh: 'AI 策略编排引擎',
        en: 'AI Strategy Orchestration',
        de: 'AI-Strategie-Orchestrierung',
        id: 'AI Strategy Orchestration',
      }),
      desc: pickText({
        zh: '支持 DeepSeek、GPT、Claude、Qwen 等多种大模型，自定义 Prompt 策略，AI 自主分析市场并做出交易决策',
        en: 'Support DeepSeek, GPT, Claude, Qwen and more. Custom prompts, AI autonomously analyzes markets and makes trading decisions',
        de: 'Unterstuetzt DeepSeek, GPT, Claude, Qwen und weitere Modelle. Eigene Prompts, autonome Marktanalyse und KI-gestuetzte Handelsentscheidungen.',
        id: 'Support DeepSeek, GPT, Claude, Qwen and more. Custom prompts, AI autonomously analyzes markets and makes trading decisions',
      }),
      highlight: true,
      badge: pickText({
        zh: '核心能力',
        en: 'Core',
        de: 'Kern',
        id: 'Core',
      }),
    },
    {
      icon: Swords,
      title: pickText({
        zh: '多 AI 竞技场',
        en: 'Multi-AI Arena',
        de: 'Multi-AI-Arena',
        id: 'Multi-AI Arena',
      }),
      desc: pickText({
        zh: '多个 AI 交易员同台竞技，实时 PnL 排行榜，自动优胜劣汰，让最强策略脱颖而出',
        en: 'Multiple AI traders compete in real-time, live PnL leaderboard, automatic survival of the fittest',
        de: 'Mehrere AI-Trader treten in Echtzeit gegeneinander an, mit Live-PnL-Rangliste und automatischer Selektion der staerksten Strategien.',
        id: 'Multiple AI traders compete in real-time, live PnL leaderboard, automatic survival of the fittest',
      }),
      highlight: true,
      badge: pickText({
        zh: '独创',
        en: 'Unique',
        de: 'Einzigartig',
        id: 'Unique',
      }),
    },
    {
      icon: LineChart,
      title: pickText({
        zh: '专业量化数据',
        en: 'Pro Quant Data',
        de: 'Professionelle Quant-Daten',
        id: 'Pro Quant Data',
      }),
      desc: pickText({
        zh: '集成 K线、技术指标、市场深度、资金费率、持仓量等专业量化数据，为 AI 决策提供全面信息',
        en: 'Integrated candlesticks, indicators, order book, funding rates, open interest - comprehensive data for AI decisions',
        de: 'Integrierte Candles, Indikatoren, Orderbuch, Funding-Rates und Open Interest liefern umfassende Daten fuer KI-Entscheidungen.',
        id: 'Integrated candlesticks, indicators, order book, funding rates, open interest - comprehensive data for AI decisions',
      }),
      highlight: true,
      badge: pickText({
        zh: '专业',
        en: 'Pro',
        de: 'Pro',
        id: 'Pro',
      }),
    },
    {
      icon: Blocks,
      title: pickText({
        zh: '多交易所支持',
        en: 'Multi-Exchange Support',
        de: 'Multi-Exchange-Support',
        id: 'Multi-Exchange Support',
      }),
      desc: pickText({
        zh: 'Binance、OKX、Bybit、Hyperliquid、Aster DEX，一套系统管理多个交易所',
        en: 'Binance, OKX, Bybit, Hyperliquid, Aster DEX - one system, multiple exchanges',
        de: 'Binance, OKX, Bybit, Hyperliquid und Aster DEX in einem System verwalten.',
        id: 'Binance, OKX, Bybit, Hyperliquid, Aster DEX - one system, multiple exchanges',
      }),
    },
    {
      icon: BarChart3,
      title: pickText({
        zh: '实时可视化看板',
        en: 'Real-time Dashboard',
        de: 'Echtzeit-Dashboard',
        id: 'Real-time Dashboard',
      }),
      desc: pickText({
        zh: '交易监控、收益曲线、持仓分析、AI 决策日志，一目了然',
        en: 'Trade monitoring, PnL curves, position analysis, AI decision logs at a glance',
        de: 'Trading-Monitoring, PnL-Verlaeufe, Positionsanalyse und AI-Entscheidungslogs auf einen Blick.',
        id: 'Trade monitoring, PnL curves, position analysis, AI decision logs at a glance',
      }),
    },
    {
      icon: Shield,
      title: pickText({
        zh: '开源自托管',
        en: 'Open Source & Self-Hosted',
        de: 'Open Source und Self-Hosted',
        id: 'Open Source & Self-Hosted',
      }),
      desc: pickText({
        zh: '代码完全开源可审计，数据存储在本地，API 密钥不经过第三方',
        en: 'Fully open source, data stored locally, API keys never leave your server',
        de: 'Vollstaendig Open Source, lokal gespeicherte Daten und API-Schluessel, die den eigenen Server nicht verlassen.',
        id: 'Fully open source, data stored locally, API keys never leave your server',
      }),
    },
  ]

  return (
    <section className="py-24 relative" style={{ background: '#0B0E11' }}>
      {/* Background */}
      <div
        className="absolute inset-0 opacity-[0.02]"
        style={{
          backgroundImage: `linear-gradient(#F0B90B 1px, transparent 1px), linear-gradient(90deg, #F0B90B 1px, transparent 1px)`,
          backgroundSize: '40px 40px',
        }}
      />

      <div className="max-w-6xl mx-auto px-4 relative z-10">
        {/* Header */}
        <motion.div
          className="text-center mb-16"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          <h2 className="text-4xl lg:text-5xl font-bold mb-4" style={{ color: '#EAECEF' }}>
            {t('whyChooseNofx', language)}
          </h2>
          <p className="text-lg max-w-2xl mx-auto" style={{ color: '#848E9C' }}>
            {pickText({
              zh: '不只是交易机器人，而是完整的 AI 交易操作系统',
              en: 'Not just a trading bot, but a complete AI trading operating system',
              de: 'Nicht nur ein Trading-Bot, sondern ein vollstaendiges KI-Handelsbetriebssystem',
              id: 'Not just a trading bot, but a complete AI trading operating system',
            })}
          </p>
        </motion.div>

        {/* Features Grid */}
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-5">
          {features.map((feature, index) => (
            <motion.div
              key={feature.title}
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: index * 0.1 }}
              className={`
                relative group rounded-2xl p-6 transition-all duration-300
                ${feature.highlight ? 'md:col-span-1 lg:col-span-1' : ''}
              `}
              style={{
                background: feature.highlight
                  ? 'linear-gradient(135deg, rgba(240, 185, 11, 0.08) 0%, rgba(240, 185, 11, 0.02) 100%)'
                  : '#12161C',
                border: feature.highlight
                  ? '1px solid rgba(240, 185, 11, 0.2)'
                  : '1px solid rgba(255, 255, 255, 0.06)',
              }}
            >
              {/* Badge */}
              {feature.badge && (
                <div
                  className="absolute top-4 right-4 px-2 py-1 rounded text-xs font-medium"
                  style={{
                    background: 'rgba(240, 185, 11, 0.15)',
                    color: '#F0B90B',
                  }}
                >
                  {feature.badge}
                </div>
              )}

              {/* Icon */}
              <motion.div
                className="w-12 h-12 rounded-xl flex items-center justify-center mb-4"
                style={{
                  background: feature.highlight
                    ? 'rgba(240, 185, 11, 0.15)'
                    : 'rgba(240, 185, 11, 0.1)',
                  border: '1px solid rgba(240, 185, 11, 0.2)',
                }}
                whileHover={{ scale: 1.1, rotate: 5 }}
              >
                <feature.icon
                  className="w-6 h-6"
                  style={{ color: '#F0B90B' }}
                />
              </motion.div>

              {/* Text */}
              <h3
                className="text-xl font-bold mb-3"
                style={{ color: '#EAECEF' }}
              >
                {feature.title}
              </h3>
              <p
                className="text-sm leading-relaxed"
                style={{ color: '#848E9C' }}
              >
                {feature.desc}
              </p>

              {/* Hover Glow */}
              <div
                className="absolute -bottom-10 -right-10 w-32 h-32 rounded-full blur-3xl opacity-0 group-hover:opacity-30 transition-opacity duration-500"
                style={{ background: '#F0B90B' }}
              />
            </motion.div>
          ))}
        </div>

        {/* Bottom Stats */}
        <motion.div
          className="mt-16 grid grid-cols-2 md:grid-cols-4 gap-6"
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          {[
            { value: '10+', label: pickText({ zh: 'AI 模型支持', en: 'AI Models', de: 'AI-Modelle', id: 'AI Models' }) },
            { value: '5+', label: pickText({ zh: '交易所集成', en: 'Exchanges', de: 'Boersen', id: 'Exchanges' }) },
            { value: '24/7', label: pickText({ zh: '自动交易', en: 'Auto Trading', de: 'Auto-Trading', id: 'Auto Trading' }) },
            { value: '100%', label: pickText({ zh: '开源免费', en: 'Open Source', de: 'Open Source', id: 'Open Source' }) },
          ].map((stat) => (
            <div
              key={stat.label}
              className="text-center p-4 rounded-xl"
              style={{
                background: 'rgba(255, 255, 255, 0.02)',
                border: '1px solid rgba(255, 255, 255, 0.05)',
              }}
            >
              <div
                className="text-2xl font-bold mb-1"
                style={{
                  background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
                  WebkitBackgroundClip: 'text',
                  WebkitTextFillColor: 'transparent',
                }}
              >
                {stat.value}
              </div>
              <div className="text-xs" style={{ color: '#5E6673' }}>
                {stat.label}
              </div>
            </div>
          ))}
        </motion.div>
      </div>
    </section>
  )
}
