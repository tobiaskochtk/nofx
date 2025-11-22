import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  TraderConfigData,
  AIModel,
  Exchange,
  CreateTraderRequest,
  UpdateModelConfigRequest,
  UpdateExchangeConfigRequest,
  CompetitionData,
} from '../types'
import { CryptoService } from './crypto'
import { httpClient } from './httpClient'

const API_BASE = '/api'

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json'
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }
  return headers
}

export const api = {
  // AI交易员管理接口
  async getTraders(): Promise<TraderInfo[]> {
    const result = await httpClient.get<TraderInfo[]>(`${API_BASE}/my-traders`)
    if (!result.success) throw new Error('获取trader列表失败')
    return result.data!
  },

  // 获取公开的交易员列表（无需认证）
  async getPublicTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/traders`)
    if (!result.success) throw new Error('获取公开trader列表失败')
    return result.data!
  },

  async createTrader(request: CreateTraderRequest): Promise<TraderInfo> {
    const result = await httpClient.post<TraderInfo>(
      `${API_BASE}/traders`,
      request
    )
    if (!result.success) throw new Error('创建交易员失败')
    return result.data!
  },

  async deleteTrader(traderId: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/traders/${traderId}`)
    if (!result.success) throw new Error('删除交易员失败')
  },

  async startTrader(traderId: string): Promise<void> {
    const result = await httpClient.post(
      `${API_BASE}/traders/${traderId}/start`
    )
    if (!result.success) throw new Error('启动交易员失败')
  },

  async stopTrader(traderId: string): Promise<void> {
    const result = await httpClient.post(`${API_BASE}/traders/${traderId}/stop`)
    if (!result.success) throw new Error('停止交易员失败')
  },

  async checkTraderBalance(traderId: string): Promise<{
    has_balance: boolean
    available_balance: number
    total_wallet_balance: number
    total_unrealized_profit?: number
    spot_balance?: number
    warning?: string
    error?: string
  }> {
    const res = await fetch(`${API_BASE}/traders/${traderId}/balance`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('获取余额失败')
    return res.json()
  },

  async updateTraderPrompt(
    traderId: string,
    customPrompt: string
  ): Promise<void> {
    const result = await httpClient.put(
      `${API_BASE}/traders/${traderId}/prompt`,
      { custom_prompt: customPrompt }
    )
    if (!result.success) throw new Error('更新自定义策略失败')
  },

  async getTraderConfig(traderId: string): Promise<TraderConfigData> {
    const result = await httpClient.get<TraderConfigData>(
      `${API_BASE}/traders/${traderId}/config`
    )
    if (!result.success) throw new Error('获取交易员配置失败')
    return result.data!
  },

  async updateTrader(
    traderId: string,
    request: CreateTraderRequest
  ): Promise<TraderInfo> {
    const result = await httpClient.put<TraderInfo>(
      `${API_BASE}/traders/${traderId}`,
      request
    )
    if (!result.success) throw new Error('更新交易员失败')
    return result.data!
  },

  // AI模型配置接口
  async getModelConfigs(): Promise<AIModel[]> {
    const result = await httpClient.get<AIModel[]>(`${API_BASE}/models`)
    if (!result.success) throw new Error('获取模型配置失败')
    return result.data!
  },

  // 获取系统支持的AI模型列表（无需认证）
  async getSupportedModels(): Promise<AIModel[]> {
    const result = await httpClient.get<AIModel[]>(
      `${API_BASE}/supported-models`
    )
    if (!result.success) throw new Error('获取支持的模型失败')
    return result.data!
  },

  async updateModelConfigs(request: UpdateModelConfigRequest): Promise<void> {
    // 获取RSA公钥
    const publicKey = await CryptoService.fetchPublicKey()

    // 初始化加密服务
    await CryptoService.initialize(publicKey)

    // 获取用户信息（从localStorage或其他地方）
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // 加密敏感数据
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // 发送加密数据
    const result = await httpClient.put(`${API_BASE}/models`, encryptedPayload)
    if (!result.success) throw new Error('更新模型配置失败')
  },

  // 交易所配置接口
  async getExchangeConfigs(): Promise<Exchange[]> {
    const result = await httpClient.get<Exchange[]>(`${API_BASE}/exchanges`)
    if (!result.success) throw new Error('获取交易所配置失败')
    return result.data!
  },

  // 获取系统支持的交易所列表（无需认证）
  async getSupportedExchanges(): Promise<Exchange[]> {
    const result = await httpClient.get<Exchange[]>(
      `${API_BASE}/supported-exchanges`
    )
    if (!result.success) throw new Error('获取支持的交易所失败')
    return result.data!
  },

  async updateExchangeConfigs(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/exchanges`, request)
    if (!result.success) throw new Error('更新交易所配置失败')
  },

  // 使用加密传输更新交易所配置
  async updateExchangeConfigsEncrypted(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    // 获取RSA公钥
    const publicKey = await CryptoService.fetchPublicKey()

    // 初始化加密服务
    await CryptoService.initialize(publicKey)

    // 获取用户信息（从localStorage或其他地方）
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // 加密敏感数据
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // 发送加密数据
    const result = await httpClient.put(
      `${API_BASE}/exchanges`,
      encryptedPayload
    )
    if (!result.success) throw new Error('更新交易所配置失败')
  },

  // 获取系统状态（支持trader_id）
  async getStatus(traderId?: string): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const result = await httpClient.get<SystemStatus>(url)
    if (!result.success) throw new Error('获取系统状态失败')
    return result.data!
  },

  // 获取账户信息（支持trader_id）
  async getAccount(traderId?: string): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const result = await httpClient.get<AccountInfo>(url)
    if (!result.success) throw new Error('获取账户信息失败')
    console.log('Account data fetched:', result.data)
    return result.data!
  },

  // 获取持仓列表（支持trader_id）
  async getPositions(traderId?: string): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const result = await httpClient.get<Position[]>(url)
    if (!result.success) throw new Error('获取持仓列表失败')
    return result.data!
  },

  // 获取决策日志（支持trader_id）
  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const result = await httpClient.get<DecisionRecord[]>(url)
    if (!result.success) throw new Error('获取决策日志失败')
    return result.data!
  },

  // 获取最新决策（支持trader_id）
  async getLatestDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions/latest?trader_id=${traderId}`
      : `${API_BASE}/decisions/latest`
    const res = await fetch(url, {
      // Ensure the browser does not serve a cached response
      cache: 'no-store',
      headers: {
        ...getAuthHeaders(),
        'Cache-Control': 'no-cache',
      },
    })
    if (!res.ok) throw new Error('获取最新决策失败')
    return res.json()
  },

  // 获取统计信息（支持trader_id）
  async getStatistics(traderId?: string): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const result = await httpClient.get<Statistics>(url)
    if (!result.success) throw new Error('获取统计信息失败')
    return result.data!
  },

  // 获取收益率历史数据（支持trader_id）
  async getEquityHistory(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const result = await httpClient.get<any[]>(url)
    if (!result.success) throw new Error('获取历史数据失败')
    return result.data!
  },

  // 批量获取多个交易员的历史数据（无需认证）
  async getEquityHistoryBatch(traderIds: string[]): Promise<any> {
    const result = await httpClient.post<any>(
      `${API_BASE}/equity-history-batch`,
      { trader_ids: traderIds }
    )
    if (!result.success) throw new Error('获取批量历史数据失败')
    return result.data!
  },

  // 获取前5名交易员数据（无需认证）
  async getTopTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/top-traders`)
    if (!result.success) throw new Error('获取前5名交易员失败')
    return result.data!
  },

  // 获取公开交易员配置（无需认证）
  async getPublicTraderConfig(traderId: string): Promise<any> {
    const result = await httpClient.get<any>(
      `${API_BASE}/trader/${traderId}/config`
    )
    if (!result.success) throw new Error('获取公开交易员配置失败')
    return result.data!
  },

  // 获取AI学习表现分析（支持trader_id）
  async getPerformance(traderId?: string): Promise<any> {
    const url = traderId
      ? `${API_BASE}/performance?trader_id=${traderId}`
      : `${API_BASE}/performance`
    const result = await httpClient.get<any>(url)
    if (!result.success) throw new Error('获取AI学习数据失败')
    return result.data!
  },

  // Deals APIs
  async getDeals(params?: {
    trader_id?: string
    status?: string
    symbol?: string
    side?: string
    from?: string
    to?: string
    q?: string
    pnl?: 'win' | 'loss'
    pnl_min?: number
    pnl_max?: number
    limit?: number
    offset?: number
  }): Promise<import('../types').Deal[]> {
    const qs = new URLSearchParams()
    if (params?.trader_id) qs.set('trader_id', params.trader_id)
    if (params?.status) qs.set('status', params.status)
    if (params?.symbol) qs.set('symbol', params.symbol)
    if (params?.side) qs.set('side', params.side)
    if (params?.from) qs.set('from', params.from)
    if (params?.to) qs.set('to', params.to)
    if (params?.q) qs.set('q', params.q)
    if (params?.pnl) qs.set('pnl', params.pnl)
    if (params?.pnl_min !== undefined) qs.set('pnl_min', String(params.pnl_min))
    if (params?.pnl_max !== undefined) qs.set('pnl_max', String(params.pnl_max))
    if (params?.limit) qs.set('limit', String(params.limit))
    if (params?.offset) qs.set('offset', String(params.offset))
    const url = `${API_BASE}/deals${qs.toString() ? `?${qs.toString()}` : ''}`
    const res = await fetch(url, { headers: getAuthHeaders() })
    if (!res.ok) throw new Error('获取交易记录失败')
    return res.json()
  },

  async getDealsCount(params?: {
    trader_id?: string
    status?: string
    symbol?: string
    side?: string
    from?: string
    to?: string
    q?: string
    pnl?: 'win' | 'loss'
    pnl_min?: number
    pnl_max?: number
  }): Promise<number> {
    const qs = new URLSearchParams()
    if (params?.trader_id) qs.set('trader_id', params.trader_id)
    if (params?.status) qs.set('status', params.status)
    if (params?.symbol) qs.set('symbol', params.symbol)
    if (params?.side) qs.set('side', params.side)
    if (params?.from) qs.set('from', params.from)
    if (params?.to) qs.set('to', params.to)
    if (params?.q) qs.set('q', params.q)
    if (params?.pnl) qs.set('pnl', params.pnl)
    if (params?.pnl_min !== undefined) qs.set('pnl_min', String(params.pnl_min))
    if (params?.pnl_max !== undefined) qs.set('pnl_max', String(params.pnl_max))
    const url = `${API_BASE}/deals/count${qs.toString() ? `?${qs.toString()}` : ''}`
    const res = await fetch(url, { headers: getAuthHeaders() })
    if (!res.ok) throw new Error('获取交易总数失败')
    const { total } = await res.json()
    return total ?? 0
  },

  async getDealById(traderId: string, id: number): Promise<{ deal: import('../types').Deal; events: import('../types').DealEvent[] }> {
    const url = `${API_BASE}/deals/${id}?trader_id=${encodeURIComponent(traderId)}`
    const res = await fetch(url, { headers: getAuthHeaders() })
    if (!res.ok) throw new Error('获取交易详情失败')
    return res.json()
  },

  getDealsExportURL(format: 'csv' | 'jsonl', params?: {
    trader_id?: string
    status?: string
    symbol?: string
    side?: string
    from?: string
    to?: string
    q?: string
    pnl?: 'win' | 'loss'
    pnl_min?: number
    pnl_max?: number
    columns?: string[]
    mode?: 'full' | 'slim' | 'train'
    train_actions?: 'all' | 'final'
  }): string {
    const qs = new URLSearchParams()
    if (params?.trader_id) qs.set('trader_id', params.trader_id)
    if (params?.status) qs.set('status', params.status)
    if (params?.symbol) qs.set('symbol', params.symbol)
    if (params?.side) qs.set('side', params.side)
    if (params?.from) qs.set('from', params.from)
    if (params?.to) qs.set('to', params.to)
    if (params?.q) qs.set('q', params.q)
    if (params?.columns && params.columns.length) qs.set('columns', params.columns.join(','))
    if (params?.pnl) qs.set('pnl', params.pnl)
    if (params?.pnl_min !== undefined) qs.set('pnl_min', String(params.pnl_min))
    if (params?.pnl_max !== undefined) qs.set('pnl_max', String(params.pnl_max))
    if (format === 'jsonl') {
      if (params?.mode) qs.set('mode', params.mode)
      if (params?.train_actions) qs.set('train_actions', params.train_actions)
    }
    return `${API_BASE}/deals/export.${format}${qs.toString() ? `?${qs.toString()}` : ''}`
  },

  // 获取竞赛数据（无需认证）
  async getCompetition(): Promise<CompetitionData> {
    const result = await httpClient.get<CompetitionData>(
      `${API_BASE}/competition`
    )
    if (!result.success) throw new Error('获取竞赛数据失败')
    return result.data!
  },

  // 用户信号源配置接口
  async getUserSignalSource(): Promise<{
    coin_pool_url?: string
    oi_top_url?: string
  }> {
    const result = await httpClient.get<{
      coin_pool_url?: string
      oi_top_url?: string
    }>(`${API_BASE}/user/signal-sources`)
    if (!result.success) throw new Error('获取用户信号源配置失败')
    return result.data ?? {}
  },

  async saveUserSignalSource(
    coinPoolUrl: string,
    oiTopUrl: string
  ): Promise<void> {
    const res = await fetch(`${API_BASE}/user/signal-sources`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({
        coin_pool_url: coinPoolUrl,
        oi_top_url: oiTopUrl,
      }),
    })
    if (!res.ok) throw new Error('保存用户信号源配置失败')
  },

  // Trailing Stop配置接口
  async getTrailingStopConfig(traderId: string): Promise<any> {
    const res = await fetch(
      `${API_BASE}/trailing-stop/config?trader_id=${encodeURIComponent(traderId)}`,
      { headers: getAuthHeaders() }
    )
    if (!res.ok) throw new Error('获取Trailing Stop配置失败')
    return res.json()
  },

  async updateTrailingStopConfig(
    traderId: string,
    config: {
      enabled: boolean
      tiers: string
      update_threshold_pct: number
      check_interval_sec: number
      allow_ai_override: boolean
    }
  ): Promise<{ message: string; restart_required: boolean }> {
    const res = await fetch(
      `${API_BASE}/trailing-stop/config?trader_id=${encodeURIComponent(traderId)}`,
      {
        method: 'PUT',
        headers: getAuthHeaders(),
        body: JSON.stringify(config),
      }
    )
    if (!res.ok) throw new Error('更新Trailing Stop配置失败')
    return res.json()
  },

  // 获取服务器IP（需要认证，用于白名单配置）
  async getServerIP(): Promise<{
    public_ip: string
    message: string
  }> {
    const result = await httpClient.get<{
      public_ip: string
      message: string
    }>(`${API_BASE}/server-ip`)
    if (!result.success) throw new Error('获取服务器IP失败')
    return result.data!
  },
}
