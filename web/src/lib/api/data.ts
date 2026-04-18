import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  CompetitionData,
  PositionHistoryResponse,
  TraderAI500BucketReview,
  AutonomousOptimizerBacklogItem,
  AutonomousOptimizerConfig,
  AutonomousOptimizerModelOutcome,
  AutonomousOptimizerRunDetail,
  AutonomousOptimizerRun,
  DealReviewListResponse,
  DealReviewCaseDetail,
  DealReviewAIScanDetail,
  DealReviewAIScanCompareResponse,
  DealReviewAnomalySummary,
  DealReviewClassifierAssist,
  DealReviewChallengerCompareDetail,
  DealReviewFilterPresetDetail,
  DealReviewStrategyVersionDetail,
} from '../../types'
import { API_BASE, httpClient } from './helpers'

export const dataApi = {
  async getStatus(traderId?: string, silent?: boolean): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const result = await httpClient.request<SystemStatus>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch system status')
    return result.data!
  },

  async getAccount(traderId?: string, silent?: boolean): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const result = await httpClient.request<AccountInfo>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch account info')
    return result.data!
  },

  async getPositions(traderId?: string, silent?: boolean): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const result = await httpClient.request<Position[]>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch positions')
    return result.data!
  },

  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const result = await httpClient.get<DecisionRecord[]>(url)
    if (!result.success) throw new Error('Failed to fetch decision logs')
    return result.data!
  },

  async getLatestDecisions(
    traderId?: string,
    limit: number = 5,
    silent?: boolean
  ): Promise<DecisionRecord[]> {
    const params = new URLSearchParams()
    if (traderId) {
      params.append('trader_id', traderId)
    }
    params.append('limit', limit.toString())

    const result = await httpClient.request<DecisionRecord[]>(
      `${API_BASE}/decisions/latest?${params}`,
      { silent }
    )
    if (!result.success) throw new Error('Failed to fetch latest decisions')
    return result.data!
  },

  async getStatistics(
    traderId?: string,
    silent?: boolean
  ): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const result = await httpClient.request<Statistics>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch statistics')
    return result.data!
  },

  async getEquityHistory(traderId?: string, silent?: boolean): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const result = await httpClient.request<any[]>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch equity history')
    return result.data!
  },

  async getEquityHistoryBatch(
    traderIds: string[],
    hours?: number
  ): Promise<any> {
    const result = await httpClient.post<any>(
      `${API_BASE}/equity-history-batch`,
      { trader_ids: traderIds, hours: hours || 0 }
    )
    if (!result.success) throw new Error('Failed to fetch batch equity history')
    return result.data!
  },

  async getTopTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/top-traders`)
    if (!result.success) throw new Error('Failed to fetch top traders')
    return result.data!
  },

  async getPublicTraderConfig(traderId: string): Promise<any> {
    const result = await httpClient.get<any>(
      `${API_BASE}/trader/${traderId}/config`
    )
    if (!result.success) throw new Error('Failed to fetch public trader config')
    return result.data!
  },

  async getCompetition(): Promise<CompetitionData> {
    const result = await httpClient.get<CompetitionData>(
      `${API_BASE}/competition`
    )
    if (!result.success) throw new Error('Failed to fetch competition data')
    return result.data!
  },

  async getPositionHistory(
    traderId: string,
    limit: number = 100,
    silent?: boolean
  ): Promise<PositionHistoryResponse> {
    const result = await httpClient.request<PositionHistoryResponse>(
      `${API_BASE}/positions/history?trader_id=${traderId}&limit=${limit}`,
      { silent }
    )
    if (!result.success) throw new Error('Failed to fetch position history')
    return result.data!
  },

  async getTraderAI500BucketReview(
    traderId: string,
    hours: number = 24,
    cycles: number = 20,
    silent?: boolean
  ): Promise<TraderAI500BucketReview> {
    const result = await httpClient.request<TraderAI500BucketReview>(
      `${API_BASE}/traders/${traderId}/ai500-bucket-review?hours=${hours}&cycles=${cycles}`,
      { silent }
    )
    if (!result.success) throw new Error('Failed to fetch AI500 bucket review')
    return result.data!
  },

  async getTraderAutonomousOptimizerConfig(
    traderId: string
  ): Promise<AutonomousOptimizerConfig> {
    const result = await httpClient.get<AutonomousOptimizerConfig>(
      `${API_BASE}/traders/${traderId}/autonomous-optimizer/config`
    )
    if (!result.success) {
      throw new Error('Failed to fetch autonomous optimizer config')
    }
    return result.data!
  },

  async updateTraderAutonomousOptimizerConfig(
    traderId: string,
    body: Record<string, unknown>
  ): Promise<AutonomousOptimizerConfig> {
    const result = await httpClient.put<AutonomousOptimizerConfig>(
      `${API_BASE}/traders/${traderId}/autonomous-optimizer/config`,
      body
    )
    if (!result.success) {
      throw new Error('Failed to save autonomous optimizer config')
    }
    return result.data!
  },

  async getTraderAutonomousOptimizerRuns(
    traderId: string,
    limit: number = 20
  ): Promise<AutonomousOptimizerRun[]> {
    const result = await httpClient.get<{ items: AutonomousOptimizerRun[] }>(
      `${API_BASE}/traders/${traderId}/autonomous-optimizer/runs?limit=${limit}`
    )
    if (!result.success) {
      throw new Error('Failed to fetch autonomous optimizer runs')
    }
    return result.data?.items || []
  },

  async getTraderAutonomousOptimizerModelOutcomes(
    traderId: string
  ): Promise<AutonomousOptimizerModelOutcome[]> {
    const result = await httpClient.get<{ items: AutonomousOptimizerModelOutcome[] }>(
      `${API_BASE}/traders/${traderId}/autonomous-optimizer/model-outcomes`
    )
    if (!result.success) {
      throw new Error('Failed to fetch autonomous optimizer model outcomes')
    }
    return result.data?.items || []
  },

  async getTraderAutonomousOptimizerRunDetail(
    traderId: string,
    runId: string
  ): Promise<AutonomousOptimizerRunDetail> {
    const result = await httpClient.get<AutonomousOptimizerRunDetail>(
      `${API_BASE}/traders/${traderId}/autonomous-optimizer/runs/${runId}`
    )
    if (!result.success) {
      throw new Error('Failed to fetch autonomous optimizer run detail')
    }
    return result.data!
  },

  async runTraderAutonomousOptimizerNow(
    traderId: string
  ): Promise<AutonomousOptimizerRunDetail> {
    const result = await httpClient.post<AutonomousOptimizerRunDetail>(
      `${API_BASE}/traders/${traderId}/autonomous-optimizer/run-now`,
      {}
    )
    if (!result.success) {
      throw new Error(
        result.message || 'Failed to execute autonomous optimizer run'
      )
    }
    return result.data!
  },

  async getTraderAutonomousOptimizerBacklog(
    traderId: string,
    limit: number = 50
  ): Promise<AutonomousOptimizerBacklogItem[]> {
    const result = await httpClient.get<{
      items: AutonomousOptimizerBacklogItem[]
    }>(
      `${API_BASE}/traders/${traderId}/autonomous-optimizer/backlog?limit=${limit}`
    )
    if (!result.success) {
      throw new Error('Failed to fetch autonomous optimizer backlog')
    }
    return result.data?.items || []
  },

  async saveTraderAutonomousOptimizerBacklog(
    traderId: string,
    body: Record<string, unknown>
  ): Promise<AutonomousOptimizerBacklogItem> {
    const result = await httpClient.post<AutonomousOptimizerBacklogItem>(
      `${API_BASE}/traders/${traderId}/autonomous-optimizer/backlog`,
      body
    )
    if (!result.success) {
      throw new Error('Failed to save autonomous optimizer backlog item')
    }
    return result.data!
  },

  async getDealReviewCases(
    traderId: string,
    params: Record<string, string | number | undefined>
  ): Promise<DealReviewListResponse> {
    const search = new URLSearchParams()
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        search.set(key, String(value))
      }
    })
    const result = await httpClient.get<DealReviewListResponse>(
      `${API_BASE}/traders/${traderId}/deal-review/cases?${search.toString()}`
    )
    if (!result.success) throw new Error('Failed to fetch deal review cases')
    return result.data!
  },

  async getDealReviewCaseDetail(
    traderId: string,
    caseId: string
  ): Promise<DealReviewCaseDetail> {
    const result = await httpClient.get<DealReviewCaseDetail>(
      `${API_BASE}/traders/${traderId}/deal-review/cases/${caseId}`
    )
    if (!result.success) throw new Error('Failed to fetch deal review case')
    return result.data!
  },

  async getDealReviewFilterPresets(
    traderId: string
  ): Promise<DealReviewFilterPresetDetail[]> {
    const result = await httpClient.get<{ items: DealReviewFilterPresetDetail[] }>(
      `${API_BASE}/traders/${traderId}/deal-review/filter-presets`
    )
    if (!result.success) {
      throw new Error('Failed to fetch deal review filter presets')
    }
    return result.data?.items || []
  },

  async saveDealReviewFilterPreset(
    traderId: string,
    body: Record<string, unknown>
  ): Promise<DealReviewFilterPresetDetail[]> {
    const result = await httpClient.post<{ items: DealReviewFilterPresetDetail[] }>(
      `${API_BASE}/traders/${traderId}/deal-review/filter-presets`,
      body
    )
    if (!result.success) {
      throw new Error('Failed to save deal review filter preset')
    }
    return result.data?.items || []
  },

  async deleteDealReviewFilterPreset(
    traderId: string,
    presetId: string
  ): Promise<void> {
    const result = await httpClient.delete(
      `${API_BASE}/traders/${traderId}/deal-review/filter-presets/${presetId}`
    )
    if (!result.success) {
      throw new Error('Failed to delete deal review filter preset')
    }
  },

  async updateDealReviewCaseReview(
    traderId: string,
    caseId: string,
    body: { labels: string[]; analyst_note: string }
  ): Promise<DealReviewCaseDetail> {
    const result = await httpClient.put<DealReviewCaseDetail>(
      `${API_BASE}/traders/${traderId}/deal-review/cases/${caseId}/review`,
      body
    )
    if (!result.success)
      throw new Error('Failed to update deal review annotations')
    return result.data!
  },

  async applyDealReviewClassifierFeedback(
    traderId: string,
    caseId: string,
    body: {
      classifier_id: string
      suggestion_key: string
      label: string
      issue_type?: string
      verdict: 'accepted' | 'rejected'
      rationale?: string
      apply_label?: boolean
    }
  ): Promise<DealReviewCaseDetail> {
    const result = await httpClient.post<DealReviewCaseDetail>(
      `${API_BASE}/traders/${traderId}/deal-review/cases/${caseId}/classifier-feedback`,
      body
    )
    if (!result.success)
      throw new Error('Failed to apply classifier feedback')
    return result.data!
  },

  async runDealReviewCaseAIAssist(
    traderId: string,
    caseId: string,
    body?: { model_id?: string; override_model_name?: string }
  ): Promise<DealReviewClassifierAssist> {
    const result = await httpClient.post<DealReviewClassifierAssist>(
      `${API_BASE}/traders/${traderId}/deal-review/cases/${caseId}/ai-assist`,
      body || {}
    )
    if (!result.success) throw new Error('Failed to run AI review assist')
    return result.data!
  },

  async getDealReviewAnomalies(
    traderId: string,
    params: Record<string, string | number | undefined>
  ): Promise<DealReviewAnomalySummary> {
    const search = new URLSearchParams()
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        search.set(key, String(value))
      }
    })
    const result = await httpClient.get<DealReviewAnomalySummary>(
      `${API_BASE}/traders/${traderId}/deal-review/anomalies?${search.toString()}`
    )
    if (!result.success)
      throw new Error('Failed to fetch deal review anomalies')
    return result.data!
  },

  async getDealReviewAIScans(
    traderId: string,
    limit: number = 10
  ): Promise<DealReviewAIScanDetail[]> {
    const result = await httpClient.get<{ items: DealReviewAIScanDetail[] }>(
      `${API_BASE}/traders/${traderId}/deal-review/ai-scans?limit=${limit}`
    )
    if (!result.success) throw new Error('Failed to fetch deal review AI scans')
    return result.data?.items || []
  },

  async compareDealReviewAIScans(
    traderId: string,
    leftScanId: string,
    rightScanId: string
  ): Promise<DealReviewAIScanCompareResponse> {
    const result = await httpClient.get<DealReviewAIScanCompareResponse>(
      `${API_BASE}/traders/${traderId}/deal-review/ai-scans/compare?left_scan_id=${leftScanId}&right_scan_id=${rightScanId}`
    )
    if (!result.success) throw new Error('Failed to compare AI scans')
    return result.data!
  },

  async runDealReviewAIScan(
    traderId: string,
    body: Record<string, unknown>
  ): Promise<DealReviewAIScanDetail> {
    const result = await httpClient.post<DealReviewAIScanDetail>(
      `${API_BASE}/traders/${traderId}/deal-review/ai-scans`,
      body
    )
    if (!result.success) {
      throw new Error(result.message || 'Failed to run deal review AI scan')
    }
    return result.data!
  },

  async applyDealReviewAIScan(
    traderId: string,
    scanId: string
  ): Promise<{ message: string }> {
    const result = await httpClient.post<{ message: string }>(
      `${API_BASE}/traders/${traderId}/deal-review/ai-scans/${scanId}/apply`
    )
    if (!result.success)
      throw new Error('Failed to apply AI scan recommendations')
    return result.data!
  },

  async validateDealReviewAIScan(
    traderId: string,
    scanId: string
  ): Promise<DealReviewAIScanDetail> {
    const result = await httpClient.post<DealReviewAIScanDetail>(
      `${API_BASE}/traders/${traderId}/deal-review/ai-scans/${scanId}/validate`
    )
    if (!result.success) {
      throw new Error(result.message || 'Failed to validate AI scan')
    }
    return result.data!
  },

  async launchDealReviewChallenger(
    traderId: string,
    scanId: string,
    body: { mode: string; exchange_id: string; window_hours: number }
  ): Promise<DealReviewChallengerCompareDetail> {
    const result = await httpClient.post<DealReviewChallengerCompareDetail>(
      `${API_BASE}/traders/${traderId}/deal-review/ai-scans/${scanId}/challenger`,
      body
    )
    if (!result.success) throw new Error('Failed to launch challenger compare')
    return result.data!
  },

  async getDealReviewStrategyVersions(
    traderId: string,
    limit: number = 10
  ): Promise<DealReviewStrategyVersionDetail[]> {
    const result = await httpClient.get<{
      items: DealReviewStrategyVersionDetail[]
    }>(
      `${API_BASE}/traders/${traderId}/deal-review/strategy-versions?limit=${limit}`
    )
    if (!result.success)
      throw new Error('Failed to fetch strategy version history')
    return result.data?.items || []
  },

  async rollbackDealReviewStrategyVersion(
    traderId: string,
    versionId: string
  ): Promise<{ message: string }> {
    const result = await httpClient.post<{ message: string }>(
      `${API_BASE}/traders/${traderId}/deal-review/strategy-versions/${versionId}/rollback`
    )
    if (!result.success) throw new Error('Failed to rollback strategy version')
    return result.data!
  },

  async getDealReviewChallengerCompares(
    traderId: string,
    limit: number = 10
  ): Promise<DealReviewChallengerCompareDetail[]> {
    const result = await httpClient.get<{
      items: DealReviewChallengerCompareDetail[]
    }>(
      `${API_BASE}/traders/${traderId}/deal-review/challenger-compares?limit=${limit}`
    )
    if (!result.success)
      throw new Error('Failed to fetch challenger compare history')
    return result.data?.items || []
  },

  async getDealReviewChallengerCompare(
    traderId: string,
    compareId: string
  ): Promise<DealReviewChallengerCompareDetail> {
    const result = await httpClient.get<DealReviewChallengerCompareDetail>(
      `${API_BASE}/traders/${traderId}/deal-review/challenger-compares/${compareId}`
    )
    if (!result.success) throw new Error('Failed to fetch challenger compare')
    return result.data!
  },

  async stopDealReviewChallengerCompare(
    traderId: string,
    compareId: string
  ): Promise<DealReviewChallengerCompareDetail> {
    const result = await httpClient.post<DealReviewChallengerCompareDetail>(
      `${API_BASE}/traders/${traderId}/deal-review/challenger-compares/${compareId}/stop`
    )
    if (!result.success) throw new Error('Failed to stop challenger compare')
    return result.data!
  },

  async resolveDealReviewChallengerCompare(
    traderId: string,
    compareId: string,
    winnerTraderId: string
  ): Promise<DealReviewChallengerCompareDetail> {
    const result = await httpClient.post<DealReviewChallengerCompareDetail>(
      `${API_BASE}/traders/${traderId}/deal-review/challenger-compares/${compareId}/resolve`,
      {
        winner_trader_id: winnerTraderId,
      }
    )
    if (!result.success)
      throw new Error('Failed to resolve challenger compare')
    return result.data!
  },
}
