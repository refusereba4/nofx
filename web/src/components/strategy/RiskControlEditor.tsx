import { Shield, AlertTriangle } from 'lucide-react'
import type {
  RiskControlConfig,
  TakeProfitLevelConfig,
  StagedStopLossConfig,
} from '../../types'

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
  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      positionLimits: { zh: '仓位限制', en: 'Position Limits' },
      maxPositions: { zh: '最大持仓数量', en: 'Max Positions' },
      maxPositionsDesc: { zh: '同时持有的最大币种数量', en: 'Maximum coins held simultaneously' },
      // Trading leverage (exchange leverage)
      tradingLeverage: { zh: '交易杠杆（交易所杠杆）', en: 'Trading Leverage (Exchange)' },
      btcEthLeverage: { zh: 'BTC/ETH 交易杠杆', en: 'BTC/ETH Trading Leverage' },
      btcEthLeverageDesc: { zh: '交易所开仓使用的杠杆倍数', en: 'Exchange leverage for opening positions' },
      altcoinLeverage: { zh: '山寨币交易杠杆', en: 'Altcoin Trading Leverage' },
      altcoinLeverageDesc: { zh: '交易所开仓使用的杠杆倍数', en: 'Exchange leverage for opening positions' },
      // Position value ratio (risk control) - CODE ENFORCED
      positionValueRatio: { zh: '仓位价值比例（代码强制）', en: 'Position Value Ratio (CODE ENFORCED)' },
      positionValueRatioDesc: { zh: '单仓位名义价值 / 账户净值，由代码强制执行', en: 'Position notional value / equity, enforced by code' },
      btcEthPositionValueRatio: { zh: 'BTC/ETH 仓位价值比例', en: 'BTC/ETH Position Value Ratio' },
      btcEthPositionValueRatioDesc: { zh: '单仓最大名义价值 = 净值 × 此值（代码强制）', en: 'Max position value = equity × this ratio (CODE ENFORCED)' },
      altcoinPositionValueRatio: { zh: '山寨币仓位价值比例', en: 'Altcoin Position Value Ratio' },
      altcoinPositionValueRatioDesc: { zh: '单仓最大名义价值 = 净值 × 此值（代码强制）', en: 'Max position value = equity × this ratio (CODE ENFORCED)' },
      riskParameters: { zh: '风险参数', en: 'Risk Parameters' },
      minRiskReward: { zh: '最小风险回报比', en: 'Min Risk/Reward Ratio' },
      minRiskRewardDesc: { zh: '开仓要求的最低盈亏比', en: 'Minimum profit ratio for opening' },
      maxMarginUsage: { zh: '最大保证金使用率（代码强制）', en: 'Max Margin Usage (CODE ENFORCED)' },
      maxMarginUsageDesc: { zh: '保证金使用率上限，由代码强制执行', en: 'Maximum margin utilization, enforced by code' },
      entryRequirements: { zh: '开仓要求', en: 'Entry Requirements' },
      minPositionSize: { zh: '最小开仓金额', en: 'Min Position Size' },
      minPositionSizeDesc: { zh: 'USDT 最小名义价值', en: 'Minimum notional value in USDT' },
      minConfidence: { zh: '最小信心度', en: 'Min Confidence' },
      minConfidenceDesc: { zh: 'AI 开仓信心度阈值', en: 'AI confidence threshold for entry' },
      stagedExit: { zh: '分批止盈与动态止损', en: 'Staged TP + Dynamic SL' },
      stagedExitDesc: { zh: '下单后由代码托管：4档止盈 + 全仓止损动态调整', en: 'Code-enforced exits: 4 TP levels + dynamic full-position stop-loss' },
      tpLevels: { zh: '分批止盈档位', en: 'Take-Profit Levels' },
      tpTarget: { zh: '止盈目标 (ROE %)', en: 'TP Target (ROE %)' },
      tpClosePct: { zh: '平仓比例 (%)', en: 'Close Position (%)' },
      tpTotal: { zh: '总平仓比例', en: 'Total Close Ratio' },
      tpTotalHint: { zh: '允许 <= 100%；大于 100% 会被自动归一化', en: 'Allows <= 100%; values > 100% will be normalized in backend' },
      stagedSl: { zh: '全仓止损阶段 (ROE %)', en: 'Full Stop-Loss Stages (ROE %)' },
      slInitial: { zh: '初始止损', en: 'Initial Stop' },
      slAfterTp1: { zh: '第一档止盈后', en: 'After TP1' },
      slAfterTp2: { zh: '第二档止盈后', en: 'After TP2' },
    }
    return translations[key]?.[language] || key
  }

  const updateField = <K extends keyof RiskControlConfig>(
    key: K,
    value: RiskControlConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...config, [key]: value })
    }
  }

  const defaultTakeProfitLevels: TakeProfitLevelConfig[] = [
    { target_roe_pct: 0.5, close_pct: 20 },
    { target_roe_pct: 1.0, close_pct: 20 },
    { target_roe_pct: 2.0, close_pct: 30 },
    { target_roe_pct: 3.0, close_pct: 20 },
  ]

  const defaultStagedStopLoss: StagedStopLossConfig = {
    initial_roe_pct: -1.6,
    after_tp1_roe_pct: -0.55,
    after_tp2_roe_pct: 0.1,
  }

  const takeProfitLevels: TakeProfitLevelConfig[] = (() => {
    const levels = Array.isArray(config.take_profit_levels) ? [...config.take_profit_levels] : []
    for (let i = levels.length; i < 4; i++) {
      levels.push(defaultTakeProfitLevels[i])
    }
    return levels.slice(0, 4)
  })()

  const stagedStopLoss = config.staged_stop_loss ?? defaultStagedStopLoss

  const updateTakeProfitLevel = (
    index: number,
    key: keyof TakeProfitLevelConfig,
    value: number,
  ) => {
    const next = [...takeProfitLevels]
    next[index] = { ...next[index], [key]: value }
    updateField('take_profit_levels', next)
  }

  const updateStagedStopLoss = (
    key: keyof StagedStopLossConfig,
    value: number,
  ) => {
    updateField('staged_stop_loss', {
      ...stagedStopLoss,
      [key]: value,
    })
  }

  const totalClosePct = takeProfitLevels.reduce((sum, level) => {
    const pct = Number.isFinite(level.close_pct) ? level.close_pct : 0
    return sum + pct
  }, 0)

  return (
    <div className="space-y-6">
      {/* Position Limits */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('positionLimits')}
          </h3>
        </div>

        <div className="grid grid-cols-1 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('maxPositions')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('maxPositionsDesc')}
            </p>
            <input
              type="number"
              value={config.max_positions ?? 3}
              onChange={(e) =>
                updateField('max_positions', parseInt(e.target.value) || 3)
              }
              disabled={disabled}
              min={1}
              max={10}
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
            {t('tradingLeverage')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('btcEthLeverage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('btcEthLeverageDesc')}
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
              {t('altcoinLeverage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('altcoinLeverageDesc')}
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
            {t('positionValueRatio')}
          </p>
          <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
            {t('positionValueRatioDesc')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('btcEthPositionValueRatio')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('btcEthPositionValueRatioDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_position_value_ratio ?? 5}
                onChange={(e) =>
                  updateField('btc_eth_max_position_value_ratio', parseFloat(e.target.value))
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
              {t('altcoinPositionValueRatio')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('altcoinPositionValueRatioDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_position_value_ratio ?? 1}
                onChange={(e) =>
                  updateField('altcoin_max_position_value_ratio', parseFloat(e.target.value))
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
            {t('riskParameters')}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minRiskReward')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minRiskRewardDesc')}
            </p>
            <div className="flex items-center">
              <span style={{ color: '#848E9C' }}>1:</span>
              <input
                type="number"
                value={config.min_risk_reward_ratio ?? 3}
                onChange={(e) =>
                  updateField('min_risk_reward_ratio', parseFloat(e.target.value) || 3)
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
              {t('maxMarginUsage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('maxMarginUsageDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={(config.max_margin_usage ?? 0.9) * 100}
                onChange={(e) =>
                  updateField('max_margin_usage', parseInt(e.target.value) / 100)
                }
                disabled={disabled}
                min={10}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span className="w-12 text-center font-mono" style={{ color: '#0ECB81' }}>
                {Math.round((config.max_margin_usage ?? 0.9) * 100)}%
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Staged TP + Dynamic SL */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <AlertTriangle className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('stagedExit')}
          </h3>
        </div>
        <p className="text-xs mb-3" style={{ color: '#848E9C' }}>
          {t('stagedExitDesc')}
        </p>

        <div
          className="p-4 rounded-lg mb-4"
          style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
        >
          <div className="flex items-center justify-between mb-3">
            <p className="text-sm font-medium" style={{ color: '#EAECEF' }}>
              {t('tpLevels')}
            </p>
            <div className="text-xs" style={{ color: totalClosePct > 100 ? '#F6465D' : '#0ECB81' }}>
              {t('tpTotal')}: {totalClosePct.toFixed(1)}%
            </div>
          </div>
          <p className="text-xs mb-3" style={{ color: '#848E9C' }}>
            {t('tpTotalHint')}
          </p>

          <div className="space-y-2">
            {takeProfitLevels.map((level, idx) => (
              <div key={idx} className="grid grid-cols-[80px_1fr_1fr] gap-2 items-center">
                <span className="text-xs font-mono" style={{ color: '#848E9C' }}>
                  TP{idx + 1}
                </span>
                <div className="flex items-center gap-2">
                  <label className="text-xs w-28" style={{ color: '#848E9C' }}>
                    {t('tpTarget')}
                  </label>
                  <input
                    type="number"
                    value={level.target_roe_pct}
                    onChange={(e) => updateTakeProfitLevel(idx, 'target_roe_pct', parseFloat(e.target.value) || 0)}
                    disabled={disabled}
                    step={0.1}
                    min={0}
                    className="w-24 px-2 py-1.5 rounded text-sm"
                    style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                  />
                </div>
                <div className="flex items-center gap-2">
                  <label className="text-xs w-28" style={{ color: '#848E9C' }}>
                    {t('tpClosePct')}
                  </label>
                  <input
                    type="number"
                    value={level.close_pct}
                    onChange={(e) => updateTakeProfitLevel(idx, 'close_pct', parseFloat(e.target.value) || 0)}
                    disabled={disabled}
                    step={1}
                    min={0}
                    max={100}
                    className="w-24 px-2 py-1.5 rounded text-sm"
                    style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                  />
                </div>
              </div>
            ))}
          </div>
        </div>

        <div
          className="p-4 rounded-lg"
          style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
        >
          <p className="text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
            {t('stagedSl')}
          </p>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div className="flex items-center gap-2">
              <label className="text-xs w-28" style={{ color: '#848E9C' }}>
                {t('slInitial')}
              </label>
              <input
                type="number"
                value={stagedStopLoss.initial_roe_pct}
                onChange={(e) => updateStagedStopLoss('initial_roe_pct', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                step={0.05}
                className="w-24 px-2 py-1.5 rounded text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
            </div>
            <div className="flex items-center gap-2">
              <label className="text-xs w-28" style={{ color: '#848E9C' }}>
                {t('slAfterTp1')}
              </label>
              <input
                type="number"
                value={stagedStopLoss.after_tp1_roe_pct}
                onChange={(e) => updateStagedStopLoss('after_tp1_roe_pct', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                step={0.05}
                className="w-24 px-2 py-1.5 rounded text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
            </div>
            <div className="flex items-center gap-2">
              <label className="text-xs w-28" style={{ color: '#848E9C' }}>
                {t('slAfterTp2')}
              </label>
              <input
                type="number"
                value={stagedStopLoss.after_tp2_roe_pct}
                onChange={(e) => updateStagedStopLoss('after_tp2_roe_pct', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                step={0.05}
                className="w-24 px-2 py-1.5 rounded text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
            </div>
          </div>
        </div>
      </div>

      {/* Entry Requirements */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#0ECB81' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('entryRequirements')}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minPositionSize')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minPositionSizeDesc')}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={config.min_position_size ?? 12}
                onChange={(e) =>
                  updateField('min_position_size', parseFloat(e.target.value) || 12)
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
              {t('minConfidence')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minConfidenceDesc')}
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
              <span className="w-12 text-center font-mono" style={{ color: '#0ECB81' }}>
                {config.min_confidence ?? 75}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
