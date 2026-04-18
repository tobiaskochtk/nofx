import { Shield, AlertTriangle, Plus, Trash2 } from 'lucide-react'
import type {
  AdaptiveReentryGuardConfig,
  RiskControlConfig,
  TrailingStopConfig,
  TrailingStopMode,
  TrailingStopTier,
} from '../../types'
import { riskControl, ts } from '../../i18n/strategy-translations'

interface RiskControlEditorProps {
  config: RiskControlConfig
  onChange: (config: RiskControlConfig) => void
  disabled?: boolean
  language: string
}

export function RiskControlEditor({
  config,
  onChange,
  disabled,
  language,
}: RiskControlEditorProps) {
  const trailingStop: TrailingStopConfig = config.trailing_stop ?? {
    enabled: false,
    check_interval_sec: 30,
    update_threshold_pct: 0.3,
    first_tighten_delay_sec: 0,
    min_first_update_profit_pct: 0,
    tiers: [
      { trigger_profit_pct: 0.5, mode: 'lock_profit', lock_profit_pct: 0.2 },
      { trigger_profit_pct: 1.0, mode: 'trail_offset', trail_offset_pct: 0.5 },
      { trigger_profit_pct: 3.0, mode: 'trail_offset', trail_offset_pct: 1.0 },
      { trigger_profit_pct: 10.0, mode: 'trail_offset', trail_offset_pct: 3.0 },
    ],
  }
  const adaptiveReentryGuard: AdaptiveReentryGuardConfig =
    config.adaptive_reentry_guard ?? {
      enabled: true,
      require_weak_execution_regime: true,
      recent_trade_window: 8,
      min_recent_trades: 4,
      same_symbol_loss_cooldown_minutes: 180,
      pair_loss_lookback_hours: 12,
    }

  const updateField = <K extends keyof RiskControlConfig>(
    key: K,
    value: RiskControlConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...config, [key]: value })
    }
  }

  const updateTrailingStop = (value: TrailingStopConfig) => {
    updateField('trailing_stop', value)
  }

  const updateAdaptiveReentryGuard = (value: AdaptiveReentryGuardConfig) => {
    updateField('adaptive_reentry_guard', value)
  }

  const updateTrailingStopField = <K extends keyof TrailingStopConfig>(
    key: K,
    value: TrailingStopConfig[K]
  ) => {
    updateTrailingStop({ ...trailingStop, [key]: value })
  }

  const updateAdaptiveReentryField = <
    K extends keyof AdaptiveReentryGuardConfig,
  >(
    key: K,
    value: AdaptiveReentryGuardConfig[K]
  ) => {
    updateAdaptiveReentryGuard({ ...adaptiveReentryGuard, [key]: value })
  }

  const updateTrailingTier = (
    index: number,
    patch: Partial<TrailingStopTier>
  ) => {
    const tiers = trailingStop.tiers.map((tier, tierIndex) =>
      tierIndex === index ? { ...tier, ...patch } : tier
    )
    updateTrailingStop({
      ...trailingStop,
      tiers: tiers.sort((a, b) => a.trigger_profit_pct - b.trigger_profit_pct),
    })
  }

  const addTrailingTier = () => {
    const lastTier = trailingStop.tiers[trailingStop.tiers.length - 1]
    const nextTrigger = lastTier
      ? Math.max(
          lastTier.trigger_profit_pct + 1,
          lastTier.trigger_profit_pct * 1.5
        )
      : 1

    updateTrailingStop({
      ...trailingStop,
      tiers: [
        ...trailingStop.tiers,
        {
          trigger_profit_pct: Number(nextTrigger.toFixed(2)),
          mode: 'trail_offset',
          trail_offset_pct: 0.5,
        },
      ],
    })
  }

  const removeTrailingTier = (index: number) => {
    if (trailingStop.tiers.length <= 1) {
      return
    }
    updateTrailingStop({
      ...trailingStop,
      tiers: trailingStop.tiers.filter((_, tierIndex) => tierIndex !== index),
    })
  }

  return (
    <div className="space-y-6">
      {/* Position Limits */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(riskControl.positionLimits, language)}
          </h3>
        </div>

        <div className="grid grid-cols-1 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(riskControl.maxPositions, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(riskControl.maxPositionsDesc, language)}
            </p>
            <input
              type="number"
              value={config.max_positions ?? 3}
              onChange={(e) =>
                updateField('max_positions', parseInt(e.target.value) || 3)
              }
              disabled={disabled}
              min={1}
              max={3}
              className="w-32 px-3 py-2 rounded"
              style={{
                background: '#1E2329',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
          </div>
        </div>

        {/* Trading Leverage (Exchange) */}
        <div className="mb-2">
          <p className="text-xs font-medium mb-2" style={{ color: '#F0B90B' }}>
            {ts(riskControl.tradingLeverage, language)}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(riskControl.btcEthLeverage, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(riskControl.btcEthLeverageDesc, language)}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_leverage ?? 5}
                onChange={(e) =>
                  updateField('btc_eth_max_leverage', parseInt(e.target.value))
                }
                disabled={disabled}
                min={1}
                max={20}
                className="flex-1 accent-yellow-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#F0B90B' }}
              >
                {config.btc_eth_max_leverage ?? 5}x
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(riskControl.altcoinLeverage, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(riskControl.altcoinLeverageDesc, language)}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_leverage ?? 5}
                onChange={(e) =>
                  updateField('altcoin_max_leverage', parseInt(e.target.value))
                }
                disabled={disabled}
                min={1}
                max={20}
                className="flex-1 accent-yellow-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#F0B90B' }}
              >
                {config.altcoin_max_leverage ?? 5}x
              </span>
            </div>
          </div>
        </div>

        {/* Position Value Ratio (Risk Control - CODE ENFORCED) */}
        <div className="mb-2">
          <p className="text-xs font-medium" style={{ color: '#0ECB81' }}>
            {ts(riskControl.positionValueRatio, language)}
          </p>
          <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
            {ts(riskControl.positionValueRatioDesc, language)}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(riskControl.btcEthPositionValueRatio, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(riskControl.btcEthPositionValueRatioDesc, language)}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_position_value_ratio ?? 5}
                onChange={(e) =>
                  updateField(
                    'btc_eth_max_position_value_ratio',
                    parseFloat(e.target.value)
                  )
                }
                disabled={disabled}
                min={0.5}
                max={10}
                step={0.5}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.btc_eth_max_position_value_ratio ?? 5}x
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(riskControl.altcoinPositionValueRatio, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(riskControl.altcoinPositionValueRatioDesc, language)}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_position_value_ratio ?? 1}
                onChange={(e) =>
                  updateField(
                    'altcoin_max_position_value_ratio',
                    parseFloat(e.target.value)
                  )
                }
                disabled={disabled}
                min={0.5}
                max={10}
                step={0.5}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.altcoin_max_position_value_ratio ?? 1}x
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Risk Parameters */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <AlertTriangle className="w-5 h-5" style={{ color: '#F6465D' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(riskControl.riskParameters, language)}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(riskControl.minRiskReward, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(riskControl.minRiskRewardDesc, language)}
            </p>
            <div className="flex items-center">
              <span style={{ color: '#848E9C' }}>1:</span>
              <input
                type="number"
                value={config.min_risk_reward_ratio ?? 3}
                onChange={(e) =>
                  updateField(
                    'min_risk_reward_ratio',
                    parseFloat(e.target.value) || 3
                  )
                }
                disabled={disabled}
                min={1}
                max={10}
                step={0.5}
                className="w-20 px-3 py-2 rounded ml-2"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(riskControl.maxMarginUsage, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(riskControl.maxMarginUsageDesc, language)}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={(config.max_margin_usage ?? 0.9) * 100}
                onChange={(e) =>
                  updateField(
                    'max_margin_usage',
                    parseInt(e.target.value) / 100
                  )
                }
                disabled={disabled}
                min={10}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {Math.round((config.max_margin_usage ?? 0.9) * 100)}%
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Trailing Stop */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <AlertTriangle className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(riskControl.trailingStop, language)}
          </h3>
        </div>

        <div
          className="p-4 rounded-lg space-y-4"
          style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
        >
          <div className="flex items-start justify-between gap-4">
            <div>
              <label
                className="block text-sm mb-1"
                style={{ color: '#EAECEF' }}
              >
                {ts(riskControl.trailingStopEnable, language)}
              </label>
              <p className="text-xs" style={{ color: '#848E9C' }}>
                {ts(riskControl.trailingStopDesc, language)}
              </p>
            </div>
            <label className="inline-flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={trailingStop.enabled}
                onChange={(e) =>
                  updateTrailingStopField('enabled', e.target.checked)
                }
                disabled={disabled}
                className="accent-yellow-500"
              />
              <span className="text-sm" style={{ color: '#EAECEF' }}>
                {trailingStop.enabled
                  ? ts(riskControl.trailingStopOn, language)
                  : ts(riskControl.trailingStopOff, language)}
              </span>
            </label>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label
                className="block text-sm mb-1"
                style={{ color: '#EAECEF' }}
              >
                {ts(riskControl.trailingStopCheckInterval, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {ts(riskControl.trailingStopCheckIntervalDesc, language)}
              </p>
              <div className="flex items-center">
                <input
                  type="number"
                  value={trailingStop.check_interval_sec}
                  onChange={(e) =>
                    updateTrailingStopField(
                      'check_interval_sec',
                      Math.max(1, parseInt(e.target.value) || 30)
                    )
                  }
                  disabled={disabled}
                  min={1}
                  max={3600}
                  className="w-24 px-3 py-2 rounded"
                  style={{
                    background: '#1E2329',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                />
                <span className="ml-2" style={{ color: '#848E9C' }}>
                  s
                </span>
              </div>
            </div>

            <div>
              <label
                className="block text-sm mb-1"
                style={{ color: '#EAECEF' }}
              >
                {ts(riskControl.trailingStopUpdateThreshold, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {ts(riskControl.trailingStopUpdateThresholdDesc, language)}
              </p>
              <div className="flex items-center">
                <input
                  type="number"
                  value={trailingStop.update_threshold_pct}
                  onChange={(e) =>
                    updateTrailingStopField(
                      'update_threshold_pct',
                      Math.max(0.01, parseFloat(e.target.value) || 0.3)
                    )
                  }
                  disabled={disabled}
                  min={0.01}
                  max={100}
                  step={0.01}
                  className="w-24 px-3 py-2 rounded"
                  style={{
                    background: '#1E2329',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                />
                <span className="ml-2" style={{ color: '#848E9C' }}>
                  %
                </span>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label
                className="block text-sm mb-1"
                style={{ color: '#EAECEF' }}
              >
                {ts(riskControl.trailingStopFirstTightenDelay, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {ts(riskControl.trailingStopFirstTightenDelayDesc, language)}
              </p>
              <div className="flex items-center">
                <input
                  type="number"
                  value={trailingStop.first_tighten_delay_sec}
                  onChange={(e) =>
                    updateTrailingStopField(
                      'first_tighten_delay_sec',
                      Math.max(0, parseInt(e.target.value) || 0)
                    )
                  }
                  disabled={disabled}
                  min={0}
                  max={86400}
                  className="w-24 px-3 py-2 rounded"
                  style={{
                    background: '#1E2329',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                />
                <span className="ml-2" style={{ color: '#848E9C' }}>
                  s
                </span>
              </div>
            </div>

            <div>
              <label
                className="block text-sm mb-1"
                style={{ color: '#EAECEF' }}
              >
                {ts(riskControl.trailingStopMinFirstUpdateProfit, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {ts(riskControl.trailingStopMinFirstUpdateProfitDesc, language)}
              </p>
              <div className="flex items-center">
                <input
                  type="number"
                  value={trailingStop.min_first_update_profit_pct}
                  onChange={(e) =>
                    updateTrailingStopField(
                      'min_first_update_profit_pct',
                      Math.max(0, parseFloat(e.target.value) || 0)
                    )
                  }
                  disabled={disabled}
                  min={0}
                  max={500}
                  step={0.01}
                  className="w-24 px-3 py-2 rounded"
                  style={{
                    background: '#1E2329',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                />
                <span className="ml-2" style={{ color: '#848E9C' }}>
                  %
                </span>
              </div>
            </div>
          </div>

          <div className="space-y-3">
            <div className="flex items-center justify-between gap-3">
              <div>
                <label className="block text-sm" style={{ color: '#EAECEF' }}>
                  {ts(riskControl.trailingStopLevels, language)}
                </label>
                <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
                  {ts(riskControl.trailingStopLevelsDesc, language)}
                </p>
              </div>
              <button
                type="button"
                onClick={addTrailingTier}
                disabled={disabled}
                className="inline-flex items-center gap-2 px-3 py-2 rounded text-sm transition-colors"
                style={{
                  background: disabled ? '#1E2329' : '#1E2329',
                  border: '1px solid #2B3139',
                  color: disabled ? '#5E6673' : '#EAECEF',
                }}
              >
                <Plus className="w-4 h-4" />
                {ts(riskControl.trailingStopAddLevel, language)}
              </button>
            </div>

            {trailingStop.tiers.map((tier, index) => {
              const mode: TrailingStopMode =
                tier.mode === 'trail_offset' ? 'trail_offset' : 'lock_profit'

              return (
                <div
                  key={`${tier.trigger_profit_pct}-${index}`}
                  className="grid grid-cols-[1fr_1fr_1fr_auto] gap-3 items-end p-3 rounded-lg"
                  style={{ background: '#11151B', border: '1px solid #2B3139' }}
                >
                  <div>
                    <label
                      className="block text-xs mb-1"
                      style={{ color: '#848E9C' }}
                    >
                      {ts(riskControl.trailingStopTrigger, language)}
                    </label>
                    <div className="flex items-center">
                      <input
                        type="number"
                        value={tier.trigger_profit_pct}
                        onChange={(e) =>
                          updateTrailingTier(index, {
                            trigger_profit_pct: Math.max(
                              0.01,
                              parseFloat(e.target.value) || 0.5
                            ),
                          })
                        }
                        disabled={disabled}
                        min={0.01}
                        step={0.01}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#1E2329',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                      />
                      <span className="ml-2" style={{ color: '#848E9C' }}>
                        %
                      </span>
                    </div>
                  </div>

                  <div>
                    <label
                      className="block text-xs mb-1"
                      style={{ color: '#848E9C' }}
                    >
                      {ts(riskControl.trailingStopMode, language)}
                    </label>
                    <select
                      value={mode}
                      onChange={(e) =>
                        updateTrailingTier(index, {
                          mode: e.target.value as TrailingStopMode,
                          lock_profit_pct:
                            e.target.value === 'lock_profit'
                              ? tier.lock_profit_pct || 0
                              : 0,
                          trail_offset_pct:
                            e.target.value === 'trail_offset'
                              ? tier.trail_offset_pct || 0.5
                              : 0,
                        })
                      }
                      disabled={disabled}
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: '#1E2329',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                    >
                      <option value="lock_profit">
                        {ts(riskControl.trailingStopModeLock, language)}
                      </option>
                      <option value="trail_offset">
                        {ts(riskControl.trailingStopModeOffset, language)}
                      </option>
                    </select>
                  </div>

                  <div>
                    <label
                      className="block text-xs mb-1"
                      style={{ color: '#848E9C' }}
                    >
                      {mode === 'lock_profit'
                        ? ts(riskControl.trailingStopLockProfit, language)
                        : ts(riskControl.trailingStopTrailOffset, language)}
                    </label>
                    <div className="flex items-center">
                      <input
                        type="number"
                        value={
                          mode === 'lock_profit'
                            ? (tier.lock_profit_pct ?? 0)
                            : (tier.trail_offset_pct ?? 0.5)
                        }
                        onChange={(e) =>
                          updateTrailingTier(
                            index,
                            mode === 'lock_profit'
                              ? {
                                  lock_profit_pct: Math.max(
                                    0,
                                    parseFloat(e.target.value) || 0
                                  ),
                                }
                              : {
                                  trail_offset_pct: Math.max(
                                    0.01,
                                    parseFloat(e.target.value) || 0.5
                                  ),
                                }
                          )
                        }
                        disabled={disabled}
                        min={mode === 'lock_profit' ? 0 : 0.01}
                        step={0.01}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#1E2329',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                      />
                      <span className="ml-2" style={{ color: '#848E9C' }}>
                        %
                      </span>
                    </div>
                  </div>

                  <button
                    type="button"
                    onClick={() => removeTrailingTier(index)}
                    disabled={disabled || trailingStop.tiers.length <= 1}
                    className="inline-flex items-center justify-center p-2 rounded transition-colors"
                    style={{
                      background: '#1E2329',
                      border: '1px solid #2B3139',
                      color:
                        disabled || trailingStop.tiers.length <= 1
                          ? '#5E6673'
                          : '#F6465D',
                    }}
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              )
            })}
          </div>
        </div>
      </div>

      {/* Adaptive Re-entry Guard */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(riskControl.adaptiveReentryGuard, language)}
          </h3>
        </div>

        <div
          className="p-4 rounded-lg space-y-4"
          style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
        >
          <div className="flex items-start justify-between gap-4">
            <div>
              <label
                className="block text-sm mb-1"
                style={{ color: '#EAECEF' }}
              >
                {ts(riskControl.adaptiveReentryGuardEnable, language)}
              </label>
              <p className="text-xs" style={{ color: '#848E9C' }}>
                {ts(riskControl.adaptiveReentryGuardDesc, language)}
              </p>
            </div>
            <label className="inline-flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={adaptiveReentryGuard.enabled}
                onChange={(e) =>
                  updateAdaptiveReentryField('enabled', e.target.checked)
                }
                disabled={disabled}
                className="accent-yellow-500"
              />
              <span className="text-sm" style={{ color: '#EAECEF' }}>
                {adaptiveReentryGuard.enabled
                  ? ts(riskControl.adaptiveReentryGuardOn, language)
                  : ts(riskControl.adaptiveReentryGuardOff, language)}
              </span>
            </label>
          </div>

          <div
            className="p-3 rounded-lg"
            style={{ background: '#11151B', border: '1px solid #2B3139' }}
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <label
                  className="block text-sm mb-1"
                  style={{ color: '#EAECEF' }}
                >
                  {ts(riskControl.adaptiveReentryRequireWeak, language)}
                </label>
                <p className="text-xs" style={{ color: '#848E9C' }}>
                  {ts(riskControl.adaptiveReentryRequireWeakDesc, language)}
                </p>
              </div>
              <input
                type="checkbox"
                checked={adaptiveReentryGuard.require_weak_execution_regime}
                onChange={(e) =>
                  updateAdaptiveReentryField(
                    'require_weak_execution_regime',
                    e.target.checked
                  )
                }
                disabled={disabled}
                className="accent-yellow-500 mt-1"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label
                className="block text-sm mb-1"
                style={{ color: '#EAECEF' }}
              >
                {ts(riskControl.adaptiveReentryRecentWindow, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {ts(riskControl.adaptiveReentryRecentWindowDesc, language)}
              </p>
              <input
                type="number"
                value={adaptiveReentryGuard.recent_trade_window}
                onChange={(e) =>
                  updateAdaptiveReentryField(
                    'recent_trade_window',
                    Math.max(1, parseInt(e.target.value) || 8)
                  )
                }
                disabled={disabled}
                min={1}
                max={50}
                className="w-24 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
            </div>

            <div>
              <label
                className="block text-sm mb-1"
                style={{ color: '#EAECEF' }}
              >
                {ts(riskControl.adaptiveReentryMinTrades, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {ts(riskControl.adaptiveReentryMinTradesDesc, language)}
              </p>
              <input
                type="number"
                value={adaptiveReentryGuard.min_recent_trades}
                onChange={(e) =>
                  updateAdaptiveReentryField(
                    'min_recent_trades',
                    Math.max(
                      1,
                      Math.min(
                        adaptiveReentryGuard.recent_trade_window,
                        parseInt(e.target.value) || 4
                      )
                    )
                  )
                }
                disabled={disabled}
                min={1}
                max={adaptiveReentryGuard.recent_trade_window}
                className="w-24 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
            </div>

            <div>
              <label
                className="block text-sm mb-1"
                style={{ color: '#EAECEF' }}
              >
                {ts(riskControl.adaptiveReentryLossCooldown, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {ts(riskControl.adaptiveReentryLossCooldownDesc, language)}
              </p>
              <div className="flex items-center">
                <input
                  type="number"
                  value={adaptiveReentryGuard.same_symbol_loss_cooldown_minutes}
                  onChange={(e) =>
                    updateAdaptiveReentryField(
                      'same_symbol_loss_cooldown_minutes',
                      Math.max(1, parseInt(e.target.value) || 180)
                    )
                  }
                  disabled={disabled}
                  min={1}
                  max={10080}
                  className="w-24 px-3 py-2 rounded"
                  style={{
                    background: '#1E2329',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                />
                <span className="ml-2" style={{ color: '#848E9C' }}>
                  min
                </span>
              </div>
            </div>

            <div>
              <label
                className="block text-sm mb-1"
                style={{ color: '#EAECEF' }}
              >
                {ts(riskControl.adaptiveReentryPairLookback, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {ts(riskControl.adaptiveReentryPairLookbackDesc, language)}
              </p>
              <div className="flex items-center">
                <input
                  type="number"
                  value={adaptiveReentryGuard.pair_loss_lookback_hours}
                  onChange={(e) =>
                    updateAdaptiveReentryField(
                      'pair_loss_lookback_hours',
                      Math.max(1, parseInt(e.target.value) || 12)
                    )
                  }
                  disabled={disabled}
                  min={1}
                  max={336}
                  className="w-24 px-3 py-2 rounded"
                  style={{
                    background: '#1E2329',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                />
                <span className="ml-2" style={{ color: '#848E9C' }}>
                  h
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Entry Requirements */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#0ECB81' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(riskControl.entryRequirements, language)}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(riskControl.minPositionSize, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(riskControl.minPositionSizeDesc, language)}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={config.min_position_size ?? 12}
                onChange={(e) =>
                  updateField(
                    'min_position_size',
                    parseFloat(e.target.value) || 12
                  )
                }
                disabled={disabled}
                min={10}
                max={1000}
                className="w-24 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span className="ml-2" style={{ color: '#848E9C' }}>
                USDT
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(riskControl.minConfidence, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(riskControl.minConfidenceDesc, language)}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.min_confidence ?? 75}
                onChange={(e) =>
                  updateField('min_confidence', parseInt(e.target.value))
                }
                disabled={disabled}
                min={50}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.min_confidence ?? 75}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
