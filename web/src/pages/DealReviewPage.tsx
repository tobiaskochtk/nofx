import { useEffect, useRef, useState } from 'react'
import { mutate } from 'swr'
import { api } from '../lib/api'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { NofxSelect } from '../components/ui/select'
import { DealReviewTimelineChart } from '../components/trader/DealReviewTimelineChart'
import { confirmToast, notify } from '../lib/notify'
import type {
  AIModel,
  DealReviewAIScanCompareResponse,
  DealReviewAIScanDetail,
  DealReviewChallengerCompareDetail,
  DealReviewChallengerProtocolEvent,
  DealReviewAnomalySummary,
  DealReviewCaseDetail,
  DealReviewEventDetail,
  DealReviewEventSnapshot,
  DealReviewCaseListItem,
  DealReviewDatasetSummary,
  DealReviewStrategyVersionDetail,
  DealReviewValidationCheck,
  Exchange,
  RemoteModelInfo,
  TraderInfo,
} from '../types'

interface DealReviewPageProps {
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
}

function formatMoney(value: number): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}`
}

function formatPct(value: number): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
}

function formatValidationScope(value: string): string {
  if (!value) return '-'
  if (value === 'recent') return 'Recent live-like'
  return value.charAt(0).toUpperCase() + value.slice(1)
}

function formatValidationMetricValue(
  metric: string,
  value: number | undefined
): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  switch (metric) {
    case 'closed_deal_count':
      return `${Math.round(value)}`
    case 'profit_factor':
      return value.toFixed(2)
    case 'max_drawdown_pct':
    case 'win_rate_delta':
    case 'avg_pnl_pct_delta':
      return formatPct(value)
    case 'net_pnl_delta':
      return formatMoney(value)
    default:
      return value.toFixed(2)
  }
}

function formatValidationCheckSummary(check: DealReviewValidationCheck): string {
  if (check.comparator === 'delta_gte') {
    return `delta ${formatValidationMetricValue(check.metric, check.delta)} | floor ${formatValidationMetricValue(check.metric, check.threshold)}`
  }
  if (check.comparator === 'lte') {
    return `actual ${formatValidationMetricValue(check.metric, check.actual)} | ceiling ${formatValidationMetricValue(check.metric, check.threshold)}`
  }
  return `actual ${formatValidationMetricValue(check.metric, check.actual)} | floor ${formatValidationMetricValue(check.metric, check.threshold)}`
}

function formatDate(ms: number): string {
  if (!ms) return '-'
  return new Date(ms).toLocaleString()
}

function formatHold(ms: number): string {
  if (!ms) return '-'
  const mins = Math.round(ms / 60000)
  if (mins < 60) return `${mins}m`
  const hours = Math.floor(mins / 60)
  const rem = mins % 60
  if (hours < 24) return rem === 0 ? `${hours}h` : `${hours}h ${rem}m`
  const days = Math.floor(hours / 24)
  const remHours = hours % 24
  return remHours === 0 ? `${days}d` : `${days}d ${remHours}h`
}

function formatCompareMode(value: string): string {
  switch (value) {
    case 'shared_live':
      return 'Shared live wallet'
    case 'isolated_live':
      return 'Isolated live wallet'
    case 'paper':
      return 'Paper / simulation'
    default:
      return value || '-'
  }
}

function formatCompareStatus(value: string): string {
  if (!value) return '-'
  return value.replace(/_/g, ' ')
}

function isActiveCompareStatus(value: string): boolean {
  return value === 'starting' || value === 'running'
}

function formatCompareProtocolType(value: string): string {
  if (!value) return 'Event'
  return value.replace(/_/g, ' ')
}

function formatStrategyVersionSourceType(value: string): string {
  switch (value) {
    case 'ai_apply':
      return 'AI apply'
    case 'ai_challenger_candidate':
      return 'Challenger candidate'
    case 'rollback':
      return 'Rollback'
    default:
      return value.replace(/_/g, ' ') || 'Strategy change'
  }
}

function buildCompareProtocolMetricSummary(
  event?: DealReviewChallengerProtocolEvent
): string {
  const metrics = event?.metrics
  if (!metrics) return ''
  return `Incumbent ${formatMoney(metrics.incumbent_pnl)} (${metrics.incumbent_trade_count} trades) | Challenger ${formatMoney(metrics.challenger_pnl)} (${metrics.challenger_trade_count} trades)`
}

function formatRequestDuration(ms?: number): string {
  if (!ms) return '-'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

function firstNonEmpty(...values: Array<string | undefined>): string {
  for (const value of values) {
    if (value && value.trim()) return value
  }
  return ''
}

function formatJsonBlock(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value ?? '')
  }
}

function getEventSystemPrompt(detail?: DealReviewEventDetail | null): string {
  return firstNonEmpty(
    detail?.snapshot?.system_prompt,
    detail?.decision_record?.system_prompt
  )
}

function getEventInputPrompt(detail?: DealReviewEventDetail | null): string {
  return firstNonEmpty(
    detail?.snapshot?.user_prompt,
    detail?.decision_record?.input_prompt
  )
}

function getEventDecisionJson(detail?: DealReviewEventDetail | null): string {
  return firstNonEmpty(
    detail?.snapshot?.decision_json,
    detail?.decision_record?.decision_json
  )
}

function getEventRawResponse(detail?: DealReviewEventDetail | null): string {
  return firstNonEmpty(
    detail?.snapshot?.raw_response,
    detail?.decision_record?.raw_response
  )
}

function getEventRequestMs(detail?: DealReviewEventDetail | null): number {
  return (
    detail?.snapshot?.ai_request_duration_ms ||
    detail?.decision_record?.ai_request_duration_ms ||
    0
  )
}

function hasStructuredSnapshot(snapshot?: DealReviewEventSnapshot): boolean {
  if (!snapshot) return false
  return Boolean(
    snapshot.account_state ||
      (snapshot.positions && snapshot.positions.length > 0) ||
      (snapshot.candidate_coins && snapshot.candidate_coins.length > 0) ||
      (snapshot.candidate_details && snapshot.candidate_details.length > 0)
  )
}

const sideOptions = [
  { value: '', label: 'All Sides' },
  { value: 'LONG', label: 'LONG' },
  { value: 'SHORT', label: 'SHORT' },
]

const statusOptions = [
  { value: '', label: 'All Statuses' },
  { value: 'OPEN', label: 'OPEN' },
  { value: 'CLOSED', label: 'CLOSED' },
]

const outcomeOptions = [
  { value: '', label: 'All Outcomes' },
  { value: 'profit', label: 'Profit' },
  { value: 'loss', label: 'Loss' },
  { value: 'flat', label: 'Flat' },
  { value: 'open', label: 'Open' },
]

const rangeOptions = [
  { value: '1h', label: 'Last 1h' },
  { value: '4h', label: 'Last 4h' },
  { value: '12h', label: 'Last 12h' },
  { value: '24h', label: 'Last 24h' },
  { value: '7d', label: 'Last 7d' },
  { value: '30d', label: 'Last 30d' },
  { value: '90d', label: 'Last 90d' },
  { value: 'all', label: 'All time' },
]

const challengerModeOptions = [
  { value: 'shared_live', label: 'Shared live wallet' },
  { value: 'isolated_live', label: 'Isolated live wallet' },
  { value: 'paper', label: 'Paper / testnet' },
]

const challengerWindowOptions = [
  { value: '12', label: '12h' },
  { value: '24', label: '24h' },
  { value: '36', label: '36h' },
  { value: '48', label: '48h' },
  { value: '96', label: '96h' },
]

function getDateRangeStart(range: string): number | undefined {
  const normalized = range.trim().toLowerCase()
  if (!normalized || normalized === 'all') return undefined

  const match = normalized.match(/^(\d+)([hd])$/)
  if (!match) return undefined

  const amount = Number(match[1])
  if (!Number.isFinite(amount) || amount <= 0) return undefined

  const unit = match[2]
  const ms =
    unit === 'h' ? amount * 60 * 60 * 1000 : amount * 24 * 60 * 60 * 1000
  return Date.now() - ms
}

export function DealReviewPage({
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
}: DealReviewPageProps) {
  const [symbol, setSymbol] = useState('')
  const [side, setSide] = useState('')
  const [status, setStatus] = useState('CLOSED')
  const [outcome, setOutcome] = useState('')
  const [dateRange, setDateRange] = useState('30d')
  const [minPnl, setMinPnl] = useState('')
  const [maxPnl, setMaxPnl] = useState('')

  const [items, setItems] = useState<DealReviewCaseListItem[]>([])
  const [summary, setSummary] = useState<DealReviewDatasetSummary | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [selectedCaseId, setSelectedCaseId] = useState<string | null>(null)
  const [detail, setDetail] = useState<DealReviewCaseDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  const [models, setModels] = useState<AIModel[]>([])
  const [scans, setScans] = useState<DealReviewAIScanDetail[]>([])
  const [selectedModelId, setSelectedModelId] = useState('')
  const [availableRemoteModels, setAvailableRemoteModels] = useState<
    RemoteModelInfo[]
  >([])
  const [selectedRemoteModel, setSelectedRemoteModel] = useState('')
  const [runningScan, setRunningScan] = useState(false)
  const [applyingScanId, setApplyingScanId] = useState<string | null>(null)
  const [reviewLabelsText, setReviewLabelsText] = useState('')
  const [reviewNote, setReviewNote] = useState('')
  const [savingReview, setSavingReview] = useState(false)
  const [anomalies, setAnomalies] = useState<DealReviewAnomalySummary | null>(
    null
  )
  const [versions, setVersions] = useState<DealReviewStrategyVersionDetail[]>(
    []
  )
  const [exchanges, setExchanges] = useState<Exchange[]>([])
  const [challengerCompares, setChallengerCompares] = useState<
    DealReviewChallengerCompareDetail[]
  >([])
  const [selectedCompareId, setSelectedCompareId] = useState<string | null>(
    null
  )
  const [selectedCompareDetail, setSelectedCompareDetail] =
    useState<DealReviewChallengerCompareDetail | null>(null)
  const [compareDetailLoading, setCompareDetailLoading] = useState(false)
  const [compareActionKey, setCompareActionKey] = useState<string | null>(null)
  const [openingCompareId, setOpeningCompareId] = useState<string | null>(null)
  const [rollingBackVersionId, setRollingBackVersionId] = useState<
    string | null
  >(null)
  const [validatingScanId, setValidatingScanId] = useState<string | null>(null)
  const [launchingScanId, setLaunchingScanId] = useState<string | null>(null)
  const [launchMode, setLaunchMode] = useState('shared_live')
  const [launchExchangeId, setLaunchExchangeId] = useState('')
  const [launchWindowHours, setLaunchWindowHours] = useState('24')
  const [compareLeftScanId, setCompareLeftScanId] = useState('')
  const [compareRightScanId, setCompareRightScanId] = useState('')
  const [compareResult, setCompareResult] =
    useState<DealReviewAIScanCompareResponse | null>(null)
  const [compareLoading, setCompareLoading] = useState(false)
  const compareDetailRef = useRef<HTMLDivElement | null>(null)

  const selectedTrader = traders?.find(
    (item) => item.trader_id === selectedTraderId
  )

  const buildFilterPayload = () => {
    const filter: Record<string, string | number | undefined> = {
      symbol: symbol.trim().toUpperCase() || undefined,
      side: side || undefined,
      status: status || undefined,
      outcome: outcome || undefined,
      limit: 150,
    }
    const rangeStart = getDateRangeStart(dateRange)
    if (rangeStart !== undefined) {
      filter.from_time = rangeStart
    }
    if (minPnl.trim()) {
      const parsed = Number(minPnl.trim())
      filter.min_pnl = Number.isFinite(parsed) ? parsed : undefined
    }
    if (maxPnl.trim()) {
      const parsed = Number(maxPnl.trim())
      filter.max_pnl = Number.isFinite(parsed) ? parsed : undefined
    }
    return filter
  }

  const loadCases = async () => {
    if (!selectedTraderId) return
    setLoading(true)
    setError(null)
    try {
      const result = await api.getDealReviewCases(
        selectedTraderId,
        buildFilterPayload()
      )
      setItems(result.items)
      setSummary(result.summary)
      if (result.items.length > 0) {
        const nextCaseId =
          selectedCaseId &&
          result.items.some((item) => item.case.id === selectedCaseId)
            ? selectedCaseId
            : result.items[0].case.id
        setSelectedCaseId(nextCaseId)
      } else {
        setSelectedCaseId(null)
        setDetail(null)
      }
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to fetch deal review cases'
      )
    } finally {
      setLoading(false)
    }
  }

  const loadDetail = async () => {
    if (!selectedTraderId || !selectedCaseId) {
      setDetail(null)
      return
    }
    setDetailLoading(true)
    try {
      const result = await api.getDealReviewCaseDetail(
        selectedTraderId,
        selectedCaseId
      )
      setDetail(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch deal detail'
      )
    } finally {
      setDetailLoading(false)
    }
  }

  const loadScanHistory = async () => {
    if (!selectedTraderId) return
    try {
      const result = await api.getDealReviewAIScans(selectedTraderId, 8)
      setScans(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch AI scans'
      )
    }
  }

  const loadAnomalies = async () => {
    if (!selectedTraderId) return
    try {
      const result = await api.getDealReviewAnomalies(
        selectedTraderId,
        buildFilterPayload()
      )
      setAnomalies(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch anomalies'
      )
    }
  }

  const loadVersions = async () => {
    if (!selectedTraderId) return
    try {
      const result = await api.getDealReviewStrategyVersions(
        selectedTraderId,
        8
      )
      setVersions(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch strategy history'
      )
    }
  }

  const loadExchanges = async () => {
    try {
      const result = await api.getExchangeConfigs()
      setExchanges(result.filter((item) => item.enabled))
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch exchange wallets'
      )
    }
  }

  const loadChallengerCompares = async () => {
    if (!selectedTraderId) return
    try {
      const result = await api.getDealReviewChallengerCompares(
        selectedTraderId,
        8
      )
      setChallengerCompares(result)
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to fetch challenger history'
      )
    }
  }

  const loadChallengerCompareDetail = async () => {
    if (!selectedTraderId || !selectedCompareId) {
      setSelectedCompareDetail(null)
      return
    }
    setCompareDetailLoading(true)
    try {
      const result = await api.getDealReviewChallengerCompare(
        selectedTraderId,
        selectedCompareId
      )
      setSelectedCompareDetail(result)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to fetch challenger detail'
      )
    } finally {
      setCompareDetailLoading(false)
    }
  }

  const scrollToCompareDetail = () => {
    window.setTimeout(() => {
      compareDetailRef.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
      })
    }, 80)
  }

  useEffect(() => {
    api
      .getModelConfigs()
      .then((result) => {
        setModels(result)
        const preferred =
          result.find((item) => item.enabled && item.provider === 'openai') ||
          result.find((item) => item.enabled)
        if (preferred) setSelectedModelId(preferred.id)
      })
      .catch(() => undefined)
    void loadExchanges()
  }, [])

  useEffect(() => {
    void loadCases()
  }, [
    selectedTraderId,
    symbol,
    side,
    status,
    outcome,
    dateRange,
    minPnl,
    maxPnl,
  ])

  useEffect(() => {
    void loadDetail()
  }, [selectedTraderId, selectedCaseId])

  useEffect(() => {
    void loadScanHistory()
  }, [selectedTraderId])

  useEffect(() => {
    void loadAnomalies()
  }, [
    selectedTraderId,
    symbol,
    side,
    status,
    outcome,
    dateRange,
    minPnl,
    maxPnl,
  ])

  useEffect(() => {
    void loadVersions()
  }, [selectedTraderId])

  useEffect(() => {
    void loadChallengerCompares()
  }, [selectedTraderId])

  useEffect(() => {
    if (challengerCompares.length === 0) {
      setSelectedCompareId(null)
      setSelectedCompareDetail(null)
      return
    }
    if (
      !selectedCompareId ||
      !challengerCompares.some((item) => item.compare.id === selectedCompareId)
    ) {
      setSelectedCompareId(challengerCompares[0].compare.id)
    }
  }, [challengerCompares, selectedCompareId])

  useEffect(() => {
    void loadChallengerCompareDetail()
  }, [selectedTraderId, selectedCompareId])

  useEffect(() => {
    if (selectedTrader?.exchange_id) {
      setLaunchExchangeId(selectedTrader.exchange_id)
      return
    }
    if (!launchExchangeId && exchanges[0]?.id) {
      setLaunchExchangeId(exchanges[0].id)
    }
  }, [selectedTrader?.exchange_id, exchanges, launchExchangeId])

  useEffect(() => {
    setReviewLabelsText(detail?.labels?.join(', ') || '')
    setReviewNote(detail?.case.analyst_note || '')
  }, [detail])

  useEffect(() => {
    if (!selectedModelId) {
      setAvailableRemoteModels([])
      setSelectedRemoteModel('')
      return
    }
    api
      .getAvailableRemoteModels(selectedModelId)
      .then((result) => {
        setAvailableRemoteModels(result)
        setSelectedRemoteModel(result[0]?.id || '')
      })
      .catch(() => {
        setAvailableRemoteModels([])
        setSelectedRemoteModel('')
      })
  }, [selectedModelId])

  useEffect(() => {
    if (scans.length === 0) {
      setCompareLeftScanId('')
      setCompareRightScanId('')
      setCompareResult(null)
      return
    }
    if (!compareLeftScanId) {
      setCompareLeftScanId(scans[0].scan.id)
    }
    if (!compareRightScanId && scans[1]) {
      setCompareRightScanId(scans[1].scan.id)
    }
  }, [scans, compareLeftScanId, compareRightScanId])

  useEffect(() => {
    if (
      !selectedTraderId ||
      !compareLeftScanId ||
      !compareRightScanId ||
      compareLeftScanId === compareRightScanId
    ) {
      setCompareResult(null)
      return
    }

    setCompareLoading(true)
    api
      .compareDealReviewAIScans(
        selectedTraderId,
        compareLeftScanId,
        compareRightScanId
      )
      .then(setCompareResult)
      .catch((err) => {
        notify.error(
          err instanceof Error ? err.message : 'Failed to compare AI scans'
        )
        setCompareResult(null)
      })
      .finally(() => setCompareLoading(false))
  }, [selectedTraderId, compareLeftScanId, compareRightScanId])

  const runAIScan = async () => {
    if (!selectedTraderId) return
    setRunningScan(true)
    try {
      const result = await api.runDealReviewAIScan(selectedTraderId, {
        model_id: selectedModelId || undefined,
        override_model_name: selectedRemoteModel || undefined,
        ...buildFilterPayload(),
      })
      setScans((current) =>
        [
          result,
          ...current.filter((item) => item.scan.id !== result.scan.id),
        ].slice(0, 8)
      )
      notify.success('AI scan completed')
      await mutate(`status-${selectedTraderId}`)
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'AI scan failed')
    } finally {
      setRunningScan(false)
    }
  }

  const validateScan = async (scan: DealReviewAIScanDetail) => {
    if (!selectedTraderId) return
    setValidatingScanId(scan.scan.id)
    try {
      const result = await api.validateDealReviewAIScan(
        selectedTraderId,
        scan.scan.id
      )
      setScans((current) =>
        current.map((item) =>
          item.scan.id === result.scan.id ? result : item
        )
      )
      notify.success(
        result.validation?.status === 'passed'
          ? 'AI scan validation passed'
          : 'AI scan validation updated'
      )
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Validation failed')
    } finally {
      setValidatingScanId(null)
    }
  }

  const applyScan = async (scan: DealReviewAIScanDetail) => {
    if (!selectedTraderId) return
    const confirmed = await confirmToast(
      'Apply the strategy patch from this AI scan and reload the trader configuration?',
      {
        title: 'Apply AI Patch',
        okText: 'Apply',
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    setApplyingScanId(scan.scan.id)
    try {
      await api.applyDealReviewAIScan(selectedTraderId, scan.scan.id)
      notify.success('Strategy patch applied')
      await loadScanHistory()
      await loadVersions()
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to apply AI patch'
      )
    } finally {
      setApplyingScanId(null)
    }
  }

  const launchChallenger = async (scan: DealReviewAIScanDetail) => {
    if (!selectedTraderId || !launchExchangeId) return
    const confirmed = await confirmToast(
      `Launch a challenger trader in ${launchMode} mode for ${launchWindowHours}h using the selected wallet?`,
      {
        title: 'Launch Challenger',
        okText: 'Launch',
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    setLaunchingScanId(scan.scan.id)
    try {
      const detail = await api.launchDealReviewChallenger(
        selectedTraderId,
        scan.scan.id,
        {
          mode: launchMode,
          exchange_id: launchExchangeId,
          window_hours: Number(launchWindowHours),
        }
      )
      setSelectedCompareId(detail.compare.id)
      setSelectedCompareDetail(detail)
      notify.success('Challenger compare started')
      await loadScanHistory()
      await loadVersions()
      await loadChallengerCompares()
      await mutate(`status-${selectedTraderId}`)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to launch challenger'
      )
    } finally {
      setLaunchingScanId(null)
    }
  }

  const stopCompare = async (detail: DealReviewChallengerCompareDetail) => {
    if (!selectedTraderId) return
    const confirmed = await confirmToast(
      'Stop this running comparison and keep the incumbent trader active?',
      {
        title: 'Stop Compare',
        okText: 'Stop compare',
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    const actionKey = `stop:${detail.compare.id}`
    setCompareActionKey(actionKey)
    try {
      const result = await api.stopDealReviewChallengerCompare(
        selectedTraderId,
        detail.compare.id
      )
      setSelectedCompareDetail(result)
      await loadChallengerCompares()
      notify.success('Challenger compare stopped')
      await mutate(`status-${selectedTraderId}`)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to stop challenger compare'
      )
    } finally {
      setCompareActionKey(null)
    }
  }

  const openVersionCompare = async (version: DealReviewStrategyVersionDetail) => {
    if (!selectedTraderId) return
    const compareId = version.version.source_compare_id?.trim()
    if (!compareId) return

    const cached =
      challengerCompares.find((item) => item.compare.id === compareId) ||
      (selectedCompareDetail?.compare.id === compareId
        ? selectedCompareDetail
        : null)

    if (cached) {
      setSelectedCompareId(compareId)
      setSelectedCompareDetail(cached)
      scrollToCompareDetail()
      return
    }

    setOpeningCompareId(compareId)
    try {
      const detail = await api.getDealReviewChallengerCompare(
        selectedTraderId,
        compareId
      )
      setSelectedCompareId(compareId)
      setSelectedCompareDetail(detail)
      setChallengerCompares((current) => {
        const withoutCurrent = current.filter(
          (item) => item.compare.id !== detail.compare.id
        )
        return [detail, ...withoutCurrent]
      })
      scrollToCompareDetail()
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to open linked compare'
      )
    } finally {
      setOpeningCompareId(null)
    }
  }

  const resolveCompare = async (
    detail: DealReviewChallengerCompareDetail,
    winnerTraderId: string,
    winnerLabel: string
  ) => {
    if (!selectedTraderId) return
    const confirmed = await confirmToast(
      `Resolve this comparison manually in favor of the ${winnerLabel}? The other trader will be disabled automatically.`,
      {
        title: 'Resolve Compare',
        okText: `Keep ${winnerLabel}`,
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    const actionKey = `resolve:${detail.compare.id}:${winnerTraderId}`
    setCompareActionKey(actionKey)
    try {
      const result = await api.resolveDealReviewChallengerCompare(
        selectedTraderId,
        detail.compare.id,
        winnerTraderId
      )
      setSelectedCompareDetail(result)
      await loadChallengerCompares()
      notify.success('Challenger compare resolved')
      await mutate(`status-${selectedTraderId}`)
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to resolve challenger compare'
      )
    } finally {
      setCompareActionKey(null)
    }
  }

  const saveCaseReview = async () => {
    if (!selectedTraderId || !selectedCaseId) return
    setSavingReview(true)
    try {
      const labels = reviewLabelsText
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean)
      const result = await api.updateDealReviewCaseReview(
        selectedTraderId,
        selectedCaseId,
        {
          labels,
          analyst_note: reviewNote,
        }
      )
      setDetail(result)
      setItems((current) =>
        current.map((item) =>
          item.case.id === selectedCaseId ? { ...item, labels } : item
        )
      )
      notify.success('Deal review notes saved')
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to save deal review notes'
      )
    } finally {
      setSavingReview(false)
    }
  }

  const rollbackVersion = async (version: DealReviewStrategyVersionDetail) => {
    if (!selectedTraderId) return
    const confirmed = await confirmToast(
      'Rollback the active strategy to the previous config captured in this version?',
      {
        title: 'Rollback Strategy',
        okText: 'Rollback',
        cancelText: 'Cancel',
      }
    )
    if (!confirmed) return

    setRollingBackVersionId(version.version.id)
    try {
      await api.rollbackDealReviewStrategyVersion(
        selectedTraderId,
        version.version.id
      )
      notify.success('Strategy rollback completed')
      await loadVersions()
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to rollback strategy'
      )
    } finally {
      setRollingBackVersionId(null)
    }
  }

  if (tradersError) {
    return (
      <div className="p-8 text-red-400">
        Failed to load traders: {tradersError.message}
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
                Deal Review
              </div>
              <h1 className="text-3xl font-semibold text-nofx-text-main">
                Open/Close rationale per deal
              </h1>
              <p className="text-sm text-nofx-text-muted mt-2">
                Review one compact open entry and one close entry per deal, then
                run AI scans against the filtered dataset.
              </p>
            </div>
            <div className="w-full lg:w-80">
              <label className="text-xs text-nofx-text-muted block mb-2">
                Trader
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

        <div className="grid grid-cols-1 xl:grid-cols-[1.2fr_0.8fr] gap-6">
          <div className="space-y-6">
            <div className="nofx-glass rounded-xl p-5">
              <div className="grid grid-cols-1 md:grid-cols-3 xl:grid-cols-6 gap-3">
                <input
                  value={symbol}
                  onChange={(event) => setSymbol(event.target.value)}
                  placeholder="Symbol"
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={side}
                    onChange={setSide}
                    options={sideOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={status}
                    onChange={setStatus}
                    options={statusOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={outcome}
                    onChange={setOutcome}
                    options={outcomeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={dateRange}
                    onChange={setDateRange}
                    options={rangeOptions}
                  />
                </div>
                <button
                  onClick={loadCases}
                  className="h-11 rounded-lg bg-nofx-gold text-black font-semibold hover:opacity-90 transition"
                >
                  Refresh
                </button>
              </div>
              <div className="grid grid-cols-2 gap-3 mt-3">
                <input
                  value={minPnl}
                  onChange={(event) => setMinPnl(event.target.value)}
                  placeholder="Min PnL"
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
                <input
                  value={maxPnl}
                  onChange={(event) => setMaxPnl(event.target.value)}
                  placeholder="Max PnL"
                  className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Deals</div>
                <div className="text-2xl font-semibold">
                  {summary?.total_deals || 0}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Win Rate</div>
                <div className="text-2xl font-semibold">
                  {summary ? `${summary.win_rate.toFixed(1)}%` : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Net PnL</div>
                <div
                  className={`text-2xl font-semibold ${summary && summary.net_pnl >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}
                >
                  {summary ? formatMoney(summary.net_pnl) : '-'}
                </div>
              </div>
              <div className="nofx-glass rounded-xl p-4">
                <div className="text-xs text-nofx-text-muted">Avg Hold</div>
                <div className="text-2xl font-semibold">
                  {summary ? formatHold(summary.avg_hold_ms) : '-'}
                </div>
              </div>
            </div>

            <div className="nofx-glass rounded-xl overflow-hidden">
              <div className="px-5 py-4 border-b border-white/10 flex items-center justify-between">
                <div>
                  <h2 className="font-semibold text-lg">Filtered deals</h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    {selectedTrader
                      ? `Trader: ${selectedTrader.trader_name}`
                      : 'Select a trader'}
                  </p>
                </div>
                {loading && (
                  <div className="text-xs text-nofx-text-muted">Loading…</div>
                )}
              </div>
              {error ? (
                <div className="p-5 text-rose-400">{error}</div>
              ) : items.length === 0 ? (
                <div className="p-5 text-nofx-text-muted">
                  No deals matched the current filters.
                </div>
              ) : (
                <div className="divide-y divide-white/5">
                  {items.map((item) => (
                    <button
                      key={item.case.id}
                      onClick={() => setSelectedCaseId(item.case.id)}
                      className={`w-full text-left px-5 py-4 transition hover:bg-white/5 ${selectedCaseId === item.case.id ? 'bg-white/5' : ''}`}
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div>
                          <div className="flex items-center gap-2">
                            <span className="font-semibold">
                              {item.case.symbol}
                            </span>
                            <span
                              className={`text-xs px-2 py-0.5 rounded-full ${item.case.side === 'LONG' ? 'bg-emerald-500/15 text-emerald-300' : 'bg-rose-500/15 text-rose-300'}`}
                            >
                              {item.case.side}
                            </span>
                            <span className="text-xs text-nofx-text-muted">
                              {item.case.outcome}
                            </span>
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-2">
                            Open: {formatDate(item.case.entry_time_ms)} | Close:{' '}
                            {formatDate(item.case.exit_time_ms)}
                          </div>
                          <div className="text-sm text-nofx-text-muted mt-2 line-clamp-2">
                            {item.open_reasoning ||
                              'No open reasoning snapshot linked yet.'}
                          </div>
                          {item.labels && item.labels.length > 0 && (
                            <div className="flex flex-wrap gap-2 mt-3">
                              {item.labels.slice(0, 3).map((label) => (
                                <span
                                  key={`${item.case.id}-${label}`}
                                  className="px-2 py-1 rounded-full text-[11px] bg-nofx-gold/10 border border-nofx-gold/20 text-nofx-gold"
                                >
                                  {label}
                                </span>
                              ))}
                            </div>
                          )}
                        </div>
                        <div className="text-right">
                          <div
                            className={
                              item.case.realized_pnl >= 0
                                ? 'text-emerald-400 font-semibold'
                                : 'text-rose-400 font-semibold'
                            }
                          >
                            {formatMoney(item.case.realized_pnl)}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-1">
                            {formatPct(item.case.realized_pnl_pct)}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-2">
                            {formatHold(item.case.hold_duration_ms)}
                          </div>
                        </div>
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>

          <div className="space-y-6">
            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4 mb-4">
                <h2 className="font-semibold text-lg">Deal detail</h2>
                {detail?.case && (
                  <div className="text-xs text-nofx-text-muted">
                    {detail.trader_name}{' '}
                    {detail.strategy_name ? `| ${detail.strategy_name}` : ''}
                  </div>
                )}
              </div>
              {detailLoading ? (
                <div className="text-nofx-text-muted">Loading detail…</div>
              ) : !detail ? (
                <div className="text-nofx-text-muted">
                  Select a deal to inspect its open and close rationale.
                </div>
              ) : (
                <div className="space-y-5">
                  <div className="grid grid-cols-2 gap-3 text-sm">
                    <div>
                      <span className="text-nofx-text-muted">Symbol</span>
                      <div className="font-semibold mt-1">
                        {detail.case.symbol}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">Side</span>
                      <div className="font-semibold mt-1">
                        {detail.case.side}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">Outcome</span>
                      <div className="font-semibold mt-1">
                        {detail.case.outcome}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">Hold</span>
                      <div className="font-semibold mt-1">
                        {formatHold(detail.case.hold_duration_ms)}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">Entry</span>
                      <div className="font-semibold mt-1">
                        {formatMoney(detail.case.entry_price)} @{' '}
                        {formatDate(detail.case.entry_time_ms)}
                      </div>
                    </div>
                    <div>
                      <span className="text-nofx-text-muted">Exit</span>
                      <div className="font-semibold mt-1">
                        {detail.case.exit_time_ms
                          ? `${formatMoney(detail.case.exit_price)} @ ${formatDate(detail.case.exit_time_ms)}`
                          : '-'}
                      </div>
                    </div>
                    <div className="col-span-2">
                      <span className="text-nofx-text-muted">PnL</span>
                      <div
                        className={
                          detail.case.realized_pnl >= 0
                            ? 'text-emerald-400 font-semibold mt-1'
                            : 'text-rose-400 font-semibold mt-1'
                        }
                      >
                        {formatMoney(detail.case.realized_pnl)} /{' '}
                        {formatPct(detail.case.realized_pnl_pct)}
                      </div>
                    </div>
                  </div>

                  <DealReviewTimelineChart
                    timeline={detail.price_timeline}
                    entryPrice={detail.case.entry_price}
                    side={detail.case.side}
                    stopLoss={detail.case.open_stop_loss}
                    takeProfit={detail.case.open_take_profit}
                  />

                  {detail.open_candidate_sources &&
                    detail.open_candidate_sources.length > 0 && (
                      <div>
                        <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                          Open sources
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {detail.open_candidate_sources.map((source) => (
                            <span
                              key={source}
                              className="px-2 py-1 rounded-full text-xs bg-white/5 border border-white/10"
                            >
                              {source}
                            </span>
                          ))}
                        </div>
                      </div>
                    )}

                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3">
                    <div className="flex items-center justify-between gap-4">
                      <div>
                        <div className="font-semibold">Analyst review</div>
                        <div className="text-xs text-nofx-text-muted mt-1">
                          Add reusable labels and a manual note for this deal.
                        </div>
                      </div>
                      <button
                        onClick={saveCaseReview}
                        disabled={savingReview}
                        className="h-9 px-3 rounded-lg border border-nofx-gold/30 text-nofx-gold disabled:opacity-50"
                      >
                        {savingReview ? 'Saving…' : 'Save review'}
                      </button>
                    </div>
                    {detail.labels && detail.labels.length > 0 && (
                      <div className="flex flex-wrap gap-2">
                        {detail.labels.map((label) => (
                          <span
                            key={label}
                            className="px-2 py-1 rounded-full text-xs bg-nofx-gold/10 border border-nofx-gold/20 text-nofx-gold"
                          >
                            {label}
                          </span>
                        ))}
                      </div>
                    )}
                    <input
                      value={reviewLabelsText}
                      onChange={(event) =>
                        setReviewLabelsText(event.target.value)
                      }
                      placeholder="Comma-separated labels, e.g. good entry, avoidable loss"
                      className="w-full h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                    />
                    <textarea
                      value={reviewNote}
                      onChange={(event) => setReviewNote(event.target.value)}
                      placeholder="Manual note for this deal"
                      className="w-full min-h-[96px] rounded-lg border border-white/10 bg-black/20 px-3 py-3 text-sm resize-y"
                    />
                  </div>

                  {[
                    {
                      label: 'Open',
                      data: detail.open,
                      border: 'border-emerald-400/20',
                    },
                    {
                      label: 'Close',
                      data: detail.close,
                      border: 'border-rose-400/20',
                    },
                  ].map((section) => {
                    const snapshot = section.data?.snapshot
                    const systemPrompt = getEventSystemPrompt(section.data)
                    const inputPrompt = getEventInputPrompt(section.data)
                    const decisionJson = getEventDecisionJson(section.data)
                    const rawResponse = getEventRawResponse(section.data)
                    const aiRequestMs = getEventRequestMs(section.data)
                    const showSnapshot = hasStructuredSnapshot(snapshot)
                    const showPromptBundle = Boolean(
                      systemPrompt || inputPrompt || decisionJson || rawResponse
                    )

                    return (
                      <div
                        key={section.label}
                        className={`rounded-xl border ${section.border} bg-black/20 p-4 space-y-4`}
                      >
                      <div className="flex items-center justify-between">
                        <h3 className="font-semibold">{section.label} entry</h3>
                        <div className="text-xs text-nofx-text-muted">
                          Cycle{' '}
                          {section.data?.event?.decision_cycle_number || '-'}
                        </div>
                      </div>

                      <div className="text-sm text-nofx-text-muted whitespace-pre-wrap">
                        {section.data?.event?.reasoning ||
                          'No linked rationale snapshot.'}
                      </div>

                      <div className="grid grid-cols-2 gap-3 text-xs">
                        <div>
                          Price:{' '}
                          <span className="text-white">
                            {section.data?.event?.price || '-'}
                          </span>
                        </div>
                        <div>
                          Qty:{' '}
                          <span className="text-white">
                            {section.data?.event?.quantity || '-'}
                          </span>
                        </div>
                        <div>
                          Confidence:{' '}
                          <span className="text-white">
                            {section.data?.event?.confidence || '-'}
                          </span>
                        </div>
                        <div>
                          Action:{' '}
                          <span className="text-white">
                            {section.data?.event?.action || '-'}
                          </span>
                        </div>
                        <div>
                          SL:{' '}
                          <span className="text-white">
                            {section.data?.event?.stop_loss || '-'}
                          </span>
                        </div>
                        <div>
                          TP:{' '}
                          <span className="text-white">
                            {section.data?.event?.take_profit || '-'}
                          </span>
                        </div>
                        {section.label === 'Close' && (
                          <>
                            <div>
                              Outcome:{' '}
                              <span
                                className={
                                  detail.case.realized_pnl >= 0
                                    ? 'text-emerald-400'
                                    : 'text-rose-400'
                                }
                              >
                                {formatMoney(
                                  section.data?.event?.outcome_pnl ||
                                    detail.case.realized_pnl
                                )}
                              </span>
                            </div>
                            <div>
                              Close reason:{' '}
                              <span className="text-white">
                                {section.data?.event?.close_reason ||
                                  detail.case.close_reason ||
                                  '-'}
                              </span>
                            </div>
                          </>
                        )}
                      </div>

                      {section.data?.candidate_sources &&
                        section.data.candidate_sources.length > 0 && (
                          <div>
                            <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                              Candidate sources
                            </div>
                            <div className="flex flex-wrap gap-2">
                              {section.data.candidate_sources.map((source) => (
                                <span
                                  key={source}
                                  className="px-2 py-1 rounded-full text-xs bg-white/5 border border-white/10"
                                >
                                  {source}
                                </span>
                              ))}
                            </div>
                          </div>
                        )}

                      {section.data?.snapshot?.execution_log &&
                        section.data.snapshot.execution_log.length > 0 && (
                          <div>
                            <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                              Execution log
                            </div>
                            <div className="space-y-2 text-xs text-nofx-text-muted">
                              {section.data.snapshot.execution_log
                                .slice(0, 6)
                                .map((line, index) => (
                                  <div
                                    key={`${section.label}-${index}`}
                                    className="rounded-lg border border-white/10 bg-black/30 px-3 py-2"
                                  >
                                    {line}
                                  </div>
                                ))}
                            </div>
                          </div>
                        )}

                      {showSnapshot && (
                        <details className="rounded-xl border border-white/10 bg-black/30 p-4">
                          <summary className="cursor-pointer text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                            Structured snapshot
                          </summary>
                          <div className="mt-4 space-y-4">
                            {snapshot?.account_state && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  Account snapshot
                                </div>
                                <div className="grid grid-cols-2 gap-3 text-xs">
                                  <div>
                                    Equity:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.total_balance || '-'}
                                    </span>
                                  </div>
                                  <div>
                                    Available:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.available_balance || '-'}
                                    </span>
                                  </div>
                                  <div>
                                    Unrealized:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.total_unrealized_profit || '-'}
                                    </span>
                                  </div>
                                  <div>
                                    Margin used:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.margin_used_pct || '-'}%
                                    </span>
                                  </div>
                                  <div>
                                    Positions:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.position_count || 0}
                                    </span>
                                  </div>
                                  <div>
                                    Initial balance:{' '}
                                    <span className="text-white">
                                      {snapshot.account_state.initial_balance || '-'}
                                    </span>
                                  </div>
                                </div>
                              </div>
                            )}

                            {snapshot?.positions && snapshot.positions.length > 0 && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  Position snapshot
                                </div>
                                <div className="space-y-2">
                                  {snapshot.positions.map((position) => (
                                    <div
                                      key={`${section.label}-${position.symbol}-${position.side}`}
                                      className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-xs"
                                    >
                                      <div className="font-medium text-white">
                                        {position.symbol} {position.side}
                                      </div>
                                      <div className="text-nofx-text-muted mt-1">
                                        Qty {position.position_amt} | Entry {position.entry_price} | Mark {position.mark_price} | PnL {position.unrealized_profit}
                                      </div>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            )}

                            {snapshot?.candidate_coins &&
                              snapshot.candidate_coins.length > 0 && (
                                <div>
                                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                    Candidate coins
                                  </div>
                                  <div className="flex flex-wrap gap-2">
                                    {snapshot.candidate_coins.map((coin) => (
                                      <span
                                        key={`${section.label}-${coin}`}
                                        className="px-2 py-1 rounded-full text-xs bg-white/5 border border-white/10"
                                      >
                                        {coin}
                                      </span>
                                    ))}
                                  </div>
                                </div>
                              )}

                            {snapshot?.candidate_details &&
                              snapshot.candidate_details.length > 0 && (
                                <div>
                                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                    Candidate details
                                  </div>
                                  <pre className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-nofx-text-muted overflow-x-auto whitespace-pre-wrap">
                                    {formatJsonBlock(snapshot.candidate_details)}
                                  </pre>
                                </div>
                              )}
                          </div>
                        </details>
                      )}

                      {showPromptBundle && (
                        <details className="rounded-xl border border-white/10 bg-black/30 p-4">
                          <summary className="cursor-pointer text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                            AI prompt bundle
                          </summary>
                          <div className="mt-4 space-y-4">
                            <div className="text-xs text-nofx-text-muted">
                              AI request duration:{' '}
                              <span className="text-white">
                                {formatRequestDuration(aiRequestMs)}
                              </span>
                            </div>

                            {inputPrompt && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  Input / market context prompt
                                </div>
                                <pre className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-nofx-text-muted overflow-x-auto whitespace-pre-wrap max-h-[26rem]">
                                  {inputPrompt}
                                </pre>
                              </div>
                            )}

                            {systemPrompt && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  System prompt
                                </div>
                                <pre className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-nofx-text-muted overflow-x-auto whitespace-pre-wrap max-h-[20rem]">
                                  {systemPrompt}
                                </pre>
                              </div>
                            )}

                            {decisionJson && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  Decision JSON
                                </div>
                                <pre className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-nofx-text-muted overflow-x-auto whitespace-pre-wrap max-h-[18rem]">
                                  {decisionJson}
                                </pre>
                              </div>
                            )}

                            {rawResponse && (
                              <div>
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                  Raw AI response
                                </div>
                                <pre className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-nofx-text-muted overflow-x-auto whitespace-pre-wrap max-h-[20rem]">
                                  {rawResponse}
                                </pre>
                              </div>
                            )}
                          </div>
                        </details>
                      )}
                    </div>
                    )
                  })}
                </div>
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h2 className="font-semibold text-lg">Anomaly scan</h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    Heuristic hotspots from the currently filtered closed deals.
                  </p>
                </div>
                <div className="text-xs text-nofx-text-muted">
                  {anomalies?.closed_deals || 0} closed deals
                </div>
              </div>

              {!anomalies ? (
                <div className="text-sm text-nofx-text-muted mt-4">
                  No anomaly data available.
                </div>
              ) : (
                <div className="space-y-4 mt-4">
                  {anomalies.notes && anomalies.notes.length > 0 && (
                    <div className="space-y-2">
                      {anomalies.notes.map((note) => (
                        <div
                          key={note}
                          className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm text-nofx-text-muted"
                        >
                          {note}
                        </div>
                      ))}
                    </div>
                  )}

                  {[
                    {
                      title: 'Worst symbols',
                      items: anomalies.worst_symbols?.map((item) => ({
                        key: item.symbol,
                        primary: item.symbol,
                        secondary: `${item.deals} deals | ${item.win_rate.toFixed(1)}% win rate`,
                        value: formatMoney(item.net_pnl),
                      })),
                    },
                    {
                      title: 'Overtraded symbols',
                      items: anomalies.overtraded_symbols?.map((item) => ({
                        key: item.symbol,
                        primary: item.symbol,
                        secondary: `${item.deals} deals | avg hold ${formatHold(item.avg_hold_ms)}`,
                        value: formatMoney(item.net_pnl),
                      })),
                    },
                    {
                      title: 'Weak buckets',
                      items: anomalies.weak_buckets?.map((item) => ({
                        key: item.bucket,
                        primary: item.bucket,
                        secondary: `${item.deals} deals | ${item.win_rate.toFixed(1)}% win rate`,
                        value: formatMoney(item.net_pnl),
                      })),
                    },
                    {
                      title: 'Weak close reasons',
                      items: anomalies.weak_close_reasons?.map((item) => ({
                        key: item.reason,
                        primary: item.reason,
                        secondary: `${item.deals} closes | avg ${formatMoney(item.avg_pnl)}`,
                        value: formatMoney(item.net_pnl),
                      })),
                    },
                  ].map((section) => (
                    <div key={section.title}>
                      <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                        {section.title}
                      </div>
                      <div className="space-y-2">
                        {(section.items || []).length === 0 ? (
                          <div className="text-sm text-nofx-text-muted">
                            Nothing notable in this slice yet.
                          </div>
                        ) : (
                          section.items?.slice(0, 4).map((item) => (
                            <div
                              key={item.key}
                              className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 flex items-start justify-between gap-4"
                            >
                              <div>
                                <div className="font-medium">
                                  {item.primary}
                                </div>
                                <div className="text-xs text-nofx-text-muted mt-1">
                                  {item.secondary}
                                </div>
                              </div>
                              <div className="text-sm font-semibold text-rose-400">
                                {item.value}
                              </div>
                            </div>
                          ))
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h2 className="font-semibold text-lg">AI scan</h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    Run a model against the current filtered dataset, validate
                    the evidence gate, then either apply directly or launch a
                    timed challenger compare.
                  </p>
                </div>
                <button
                  onClick={runAIScan}
                  disabled={!selectedTraderId || runningScan}
                  className="h-10 px-4 rounded-lg bg-nofx-gold text-black font-semibold disabled:opacity-50"
                >
                  {runningScan ? 'Running…' : 'Run scan'}
                </button>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-4">
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={selectedModelId}
                    onChange={setSelectedModelId}
                    options={models
                      .filter((item) => item.enabled)
                      .map((item) => ({
                        value: item.id,
                        label: `${item.name} (${item.provider})`,
                      }))}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={selectedRemoteModel}
                    onChange={setSelectedRemoteModel}
                    options={(availableRemoteModels.length > 0
                      ? availableRemoteModels
                      : [
                          {
                            id: '',
                            label: 'Configured default',
                            provider: '',
                            available: true,
                          },
                        ]
                    ).map((item) => ({
                      value: item.id,
                      label: item.label,
                    }))}
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mt-3">
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={launchMode}
                    onChange={setLaunchMode}
                    options={challengerModeOptions}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={launchExchangeId}
                    onChange={setLaunchExchangeId}
                    options={exchanges.map((item) => ({
                      value: item.id,
                      label: `${item.name} / ${item.account_name}${item.testnet ? ' · testnet' : ''}`,
                    }))}
                  />
                </div>
                <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                  <NofxSelect
                    value={launchWindowHours}
                    onChange={setLaunchWindowHours}
                    options={challengerWindowOptions}
                  />
                </div>
              </div>

              <div className="text-xs text-nofx-text-muted mt-2">
                `paper / simulation` currently uses a selected testnet wallet.
                `isolated_live` requires a different live wallet than the
                incumbent. `shared_live` can use the same live wallet.
              </div>

              <div className="space-y-4 mt-5">
                {scans.length === 0 ? (
                  <div className="text-sm text-nofx-text-muted">
                    No AI scans saved yet.
                  </div>
                ) : (
                  scans.map((scan) => {
                    const validation = scan.validation
                    const validationStatus =
                      validation?.status || scan.scan.validation_status || 'pending'
                    const validationPassed = validationStatus === 'passed'
                    const hasPatch =
                      !!scan.strategy_patch &&
                      Object.keys(scan.strategy_patch).length > 0
                    return (
                      <div
                        key={scan.scan.id}
                        className="rounded-xl border border-white/10 bg-black/20 p-4"
                      >
                        <div className="flex items-start justify-between gap-4">
                          <div>
                            <div className="text-xs text-nofx-gold">
                              {scan.scan.provider} / {scan.scan.model_name}
                            </div>
                            <div className="font-semibold mt-1">
                              {scan.result.executive_summary || scan.scan.summary}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-2">
                              {new Date(scan.scan.created_at).toLocaleString()} |{' '}
                              {scan.scan.dataset_count} deals
                            </div>
                            <div className="flex flex-wrap items-center gap-2 mt-3">
                              <span
                                className={`px-2 py-1 rounded-full text-[11px] uppercase tracking-[0.18em] ${
                                  validationPassed
                                    ? 'bg-emerald-500/15 text-emerald-300'
                                    : validationStatus === 'failed'
                                      ? 'bg-rose-500/15 text-rose-300'
                                      : 'bg-amber-500/15 text-amber-200'
                                }`}
                              >
                                {validationStatus}
                              </span>
                              <span className="text-xs text-nofx-text-muted">
                                {scan.scan.validation_summary ||
                                  'Validation required before apply or challenger launch.'}
                              </span>
                            </div>
                          </div>
                          <div className="flex flex-wrap justify-end gap-2">
                            <button
                              onClick={() => validateScan(scan)}
                              disabled={validatingScanId === scan.scan.id}
                              className="h-9 px-3 rounded-lg border border-white/15 text-white disabled:opacity-50"
                            >
                              {validatingScanId === scan.scan.id
                                ? 'Validating…'
                                : validationStatus === 'passed'
                                  ? 'Revalidate'
                                  : 'Validate'}
                            </button>
                            <button
                              onClick={() => launchChallenger(scan)}
                              disabled={
                                launchingScanId === scan.scan.id ||
                                !validationPassed ||
                                !hasPatch ||
                                !launchExchangeId
                              }
                              className="h-9 px-3 rounded-lg border border-sky-400/30 text-sky-300 disabled:opacity-50"
                            >
                              {launchingScanId === scan.scan.id
                                ? 'Launching…'
                                : 'Launch challenger'}
                            </button>
                            <button
                              onClick={() => applyScan(scan)}
                              disabled={
                                applyingScanId === scan.scan.id ||
                                !validationPassed ||
                                !hasPatch
                              }
                              className="h-9 px-3 rounded-lg border border-nofx-gold/30 text-nofx-gold disabled:opacity-50"
                            >
                              {applyingScanId === scan.scan.id
                                ? 'Applying…'
                                : 'Apply patch'}
                            </button>
                          </div>
                        </div>

                        {validation && (
                          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 mt-4 text-sm">
                            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                Evidence gate
                              </div>
                              <div className="space-y-1 text-nofx-text-muted">
                                <div>
                                  Closed deals: {validation.closed_deal_count} /{' '}
                                  {validation.min_closed_deal_count}
                                </div>
                                <div>
                                  Train / holdout / recent: {validation.training_closed_deal_count} /{' '}
                                  {validation.holdout_closed_deal_count} /{' '}
                                  {validation.recent_closed_deal_count || 0}
                                </div>
                                <div>
                                  Config valid:{' '}
                                  {validation.config_valid ? 'yes' : 'no'}
                                </div>
                                {validation.recent_slice_label && (
                                  <div>
                                    Recent slice:{' '}
                                    {validation.recent_slice_label}
                                  </div>
                                )}
                              </div>
                            </div>
                            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                Gate output
                              </div>
                              {(validation.blocking_issues || []).length === 0 ? (
                                <div className="text-sm text-emerald-300">
                                  Ready for direct apply or challenger launch.
                                </div>
                              ) : (
                                <div className="space-y-1 text-sm text-rose-300">
                                  {validation.blocking_issues?.map((item) => (
                                    <div key={item}>• {item}</div>
                                  ))}
                                </div>
                              )}
                            </div>
                            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                Replay gate
                              </div>
                              {!validation.replay?.supported ? (
                                <div className="text-sm text-nofx-text-muted">
                                  No supported historical replay for this patch
                                  yet.
                                </div>
                              ) : (
                                <div className="space-y-1 text-nofx-text-muted">
                                  <div>
                                    Holdout delta:{' '}
                                    <span
                                      className={
                                        validation.replay.holdout_net_pnl_delta >= 0
                                          ? 'text-emerald-300'
                                          : 'text-rose-300'
                                      }
                                    >
                                      {formatMoney(
                                        validation.replay.holdout_net_pnl_delta
                                      )}
                                    </span>
                                  </div>
                                  <div>
                                    Training delta:{' '}
                                    <span
                                      className={
                                        validation.replay.training_net_pnl_delta >= 0
                                          ? 'text-emerald-300'
                                          : 'text-rose-300'
                                      }
                                    >
                                      {formatMoney(
                                        validation.replay.training_net_pnl_delta
                                      )}
                                    </span>
                                  </div>
                                  <div>
                                    Recent delta:{' '}
                                    <span
                                      className={
                                        validation.replay.recent_net_pnl_delta >= 0
                                          ? 'text-emerald-300'
                                          : 'text-rose-300'
                                      }
                                    >
                                      {formatMoney(
                                        validation.replay.recent_net_pnl_delta
                                      )}
                                    </span>
                                  </div>
                                  <div>
                                    Covered fields:{' '}
                                    {(validation.replay.supported_paths || []).join(
                                      ', '
                                    ) || '-'}
                                  </div>
                                </div>
                              )}
                            </div>
                            <div className="rounded-lg border border-white/10 bg-black/20 p-3 xl:col-span-2">
                              <div className="flex items-center justify-between gap-3 mb-2">
                                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                                  Metric gates
                                </div>
                                <div className="text-xs text-nofx-text-muted">
                                  {(validation.checks || []).filter((item) => item.passed).length} /{' '}
                                  {(validation.checks || []).length} passed
                                </div>
                              </div>
                              {!(validation.checks || []).length ? (
                                <div className="text-sm text-nofx-text-muted">
                                  No metric checks recorded yet.
                                </div>
                              ) : (
                                <div className="space-y-2 max-h-72 overflow-auto pr-1">
                                  {[...(validation.checks || [])]
                                    .sort((left, right) => Number(left.passed) - Number(right.passed))
                                    .map((item) => (
                                      <div
                                        key={item.key}
                                        className="rounded-lg border border-white/8 bg-black/20 p-3"
                                      >
                                        <div className="flex items-start justify-between gap-3">
                                          <div>
                                            <div className="font-medium text-white">
                                              {item.label}
                                            </div>
                                            <div className="text-xs text-nofx-text-muted mt-1">
                                              {formatValidationScope(item.scope)} |{' '}
                                              {item.source || 'baseline'}
                                            </div>
                                          </div>
                                          <span
                                            className={`px-2 py-1 rounded-full text-[11px] uppercase tracking-[0.18em] ${
                                              item.passed
                                                ? 'bg-emerald-500/15 text-emerald-300'
                                                : 'bg-rose-500/15 text-rose-300'
                                            }`}
                                          >
                                            {item.passed ? 'pass' : 'block'}
                                          </span>
                                        </div>
                                        <div className="text-xs text-nofx-text-muted mt-2">
                                          {formatValidationCheckSummary(item)}
                                        </div>
                                        {typeof item.baseline === 'number' &&
                                          item.comparator === 'delta_gte' && (
                                            <div className="text-xs text-nofx-text-muted mt-1">
                                              baseline{' '}
                                              {formatValidationMetricValue(
                                                item.metric,
                                                item.baseline
                                              )}
                                              {' | '}projected{' '}
                                              {formatValidationMetricValue(
                                                item.metric,
                                                item.actual
                                              )}
                                            </div>
                                          )}
                                        {!item.passed && (
                                          <div className="text-xs text-rose-300 mt-2">
                                            {item.message}
                                          </div>
                                        )}
                                      </div>
                                    ))}
                                </div>
                              )}
                            </div>
                          </div>
                        )}

                        {scan.result.weaknesses &&
                          scan.result.weaknesses.length > 0 && (
                            <div className="mt-4">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                Weaknesses
                              </div>
                              <div className="space-y-2 text-sm text-nofx-text-muted">
                                {scan.result.weaknesses
                                  .slice(0, 3)
                                  .map((item) => (
                                    <div key={item}>• {item}</div>
                                  ))}
                              </div>
                            </div>
                          )}

                        {scan.result.immediate_actions &&
                          scan.result.immediate_actions.length > 0 && (
                            <div className="mt-4">
                              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                                Immediate actions
                              </div>
                              <div className="space-y-3">
                                {scan.result.immediate_actions
                                  .slice(0, 2)
                                  .map((item) => (
                                    <div
                                      key={item.title}
                                      className="rounded-lg border border-white/10 p-3"
                                    >
                                      <div className="font-medium">
                                        {item.title}
                                      </div>
                                      <div className="text-sm text-nofx-text-muted mt-1">
                                        {item.rationale}
                                      </div>
                                    </div>
                                  ))}
                              </div>
                            </div>
                          )}
                      </div>
                    )
                  })
                )}
              </div>

              <div className="mt-6 rounded-xl border border-white/10 bg-black/20 p-4">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <div className="font-semibold">Scan compare</div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      Compare two saved AI scans on filters, overlaps and patch
                      deltas.
                    </div>
                  </div>
                  {compareLoading && (
                    <div className="text-xs text-nofx-text-muted">Loading…</div>
                  )}
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-4">
                  <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                    <NofxSelect
                      value={compareLeftScanId}
                      onChange={setCompareLeftScanId}
                      options={scans.map((item) => ({
                        value: item.scan.id,
                        label: `${item.scan.model_name} · ${new Date(item.scan.created_at).toLocaleDateString()}`,
                      }))}
                    />
                  </div>
                  <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
                    <NofxSelect
                      value={compareRightScanId}
                      onChange={setCompareRightScanId}
                      options={scans.map((item) => ({
                        value: item.scan.id,
                        label: `${item.scan.model_name} · ${new Date(item.scan.created_at).toLocaleDateString()}`,
                      }))}
                    />
                  </div>
                </div>

                {!compareResult ? (
                  <div className="text-sm text-nofx-text-muted mt-4">
                    Select two different scans to compare.
                  </div>
                ) : (
                  <div className="space-y-4 mt-4">
                    <div className="grid grid-cols-2 gap-3 text-sm">
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">Left</div>
                        <div className="font-medium mt-1">
                          {compareResult.left.scan.provider} /{' '}
                          {compareResult.left.scan.model_name}
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted">
                          Right
                        </div>
                        <div className="font-medium mt-1">
                          {compareResult.right.scan.provider} /{' '}
                          {compareResult.right.scan.model_name}
                        </div>
                      </div>
                    </div>

                    <div className="text-sm">
                      <span className="text-nofx-text-muted">Filters:</span>{' '}
                      <span
                        className={
                          compareResult.same_filters
                            ? 'text-emerald-400'
                            : 'text-amber-300'
                        }
                      >
                        {compareResult.same_filters
                          ? 'Same dataset'
                          : 'Different dataset filters'}
                      </span>
                    </div>

                    {compareResult.strength_overlap &&
                      compareResult.strength_overlap.length > 0 && (
                        <div>
                          <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                            Shared strengths
                          </div>
                          <div className="space-y-2 text-sm text-nofx-text-muted">
                            {compareResult.strength_overlap.map((item) => (
                              <div key={item}>• {item}</div>
                            ))}
                          </div>
                        </div>
                      )}

                    {compareResult.patch_differences &&
                      compareResult.patch_differences.length > 0 && (
                        <div>
                          <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                            Patch differences
                          </div>
                          <div className="space-y-2">
                            {compareResult.patch_differences
                              .slice(0, 6)
                              .map((item) => (
                                <div
                                  key={item.path}
                                  className="rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-xs"
                                >
                                  <div className="text-nofx-gold">
                                    {item.path}
                                  </div>
                                  <div className="text-nofx-text-muted mt-1">
                                    A: {item.left}
                                  </div>
                                  <div className="text-nofx-text-muted">
                                    B: {item.right}
                                  </div>
                                </div>
                              ))}
                          </div>
                        </div>
                      )}
                  </div>
                )}
              </div>
            </div>

            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h2 className="font-semibold text-lg">Challenger history</h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    Timed old-vs-new comparisons launched from validated AI
                    scans.
                  </p>
                </div>
              </div>

              <div className="space-y-3 mt-4">
                {challengerCompares.length === 0 ? (
                  <div className="text-sm text-nofx-text-muted">
                    No challenger compares started yet.
                  </div>
                ) : (
                  challengerCompares.map((item) => (
                    <div
                      key={item.compare.id}
                      className={`rounded-xl border p-4 cursor-pointer transition-colors ${
                        selectedCompareId === item.compare.id
                          ? 'border-sky-400/40 bg-sky-500/10'
                          : 'border-white/10 bg-black/20 hover:border-white/20'
                      }`}
                      onClick={() => setSelectedCompareId(item.compare.id)}
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div>
                          <div className="text-xs text-sky-300 uppercase tracking-[0.18em]">
                            {formatCompareMode(item.compare.mode)} ·{' '}
                            {formatCompareStatus(item.compare.status)}
                          </div>
                          <div className="font-medium mt-1">
                            {item.incumbent_trader_name || 'Incumbent'} vs{' '}
                            {item.challenger_trader_name || 'Challenger'}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-2">
                            {item.challenger_exchange_name || 'Wallet unknown'} ·
                            window {item.compare.window_hours}h
                            {item.compare.extension_count > 0
                              ? ` · +${item.compare.extension_count} extensions`
                              : ''}
                          </div>
                        </div>
                        <div className="text-right text-sm">
                          <div className="text-nofx-text-muted">
                            Ends {new Date(item.compare.ends_at).toLocaleString()}
                          </div>
                          {item.compare.resolved_at && (
                            <div className="text-xs text-nofx-text-muted mt-1">
                              Resolved{' '}
                              {new Date(
                                item.compare.resolved_at
                              ).toLocaleString()}
                            </div>
                          )}
                          {item.compare.winner_trader_id && (
                            <div className="text-xs text-emerald-300 mt-1">
                              Winner:{' '}
                              {item.compare.winner_trader_id ===
                              item.compare.incumbent_trader_id
                                ? item.incumbent_trader_name || 'Incumbent'
                                : item.challenger_trader_name || 'Challenger'}
                            </div>
                          )}
                        </div>
                      </div>

                      {item.metrics && (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-4 text-sm">
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              Incumbent
                            </div>
                            <div className="font-semibold mt-1">
                              {formatMoney(item.metrics.incumbent_pnl)}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {item.metrics.incumbent_trade_count} closed trades
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              Challenger
                            </div>
                            <div className="font-semibold mt-1">
                              {formatMoney(item.metrics.challenger_pnl)}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {item.metrics.challenger_trade_count} closed trades
                            </div>
                          </div>
                        </div>
                      )}

                      <div className="text-sm text-nofx-text-muted mt-4">
                        {item.compare.summary}
                      </div>

                      <div className="mt-4">
                        <button
                          onClick={(event) => {
                            event.stopPropagation()
                            setSelectedCompareId(item.compare.id)
                          }}
                          className="h-9 px-3 rounded-lg border border-white/15 text-white"
                        >
                          Open detail
                        </button>
                      </div>
                    </div>
                  ))
                )}
              </div>

              {(selectedCompareDetail || compareDetailLoading) && (
                <div
                  ref={compareDetailRef}
                  className="rounded-xl border border-white/10 bg-black/20 p-5 mt-5"
                >
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                    <div>
                      <div className="text-xs uppercase tracking-[0.2em] text-sky-300">
                        Compare detail
                      </div>
                      <h3 className="font-semibold text-lg mt-1">
                        {selectedCompareDetail?.incumbent_trader_name ||
                          'Incumbent'}{' '}
                        vs{' '}
                        {selectedCompareDetail?.challenger_trader_name ||
                          'Challenger'}
                      </h3>
                      <div className="text-sm text-nofx-text-muted mt-2">
                        {selectedCompareDetail
                          ? `${formatCompareMode(
                              selectedCompareDetail.compare.mode
                            )} · ${selectedCompareDetail.challenger_exchange_name || 'Wallet unknown'} · window ${selectedCompareDetail.compare.window_hours}h`
                          : 'Loading compare detail…'}
                      </div>
                      {selectedCompareDetail?.source_scan_summary && (
                        <div className="text-sm text-nofx-text-muted mt-2">
                          Source scan: {selectedCompareDetail.source_scan_summary}
                        </div>
                      )}
                    </div>

                    {selectedCompareDetail && (
                      <div className="flex flex-wrap gap-2">
                        {isActiveCompareStatus(
                          selectedCompareDetail.compare.status
                        ) && (
                          <>
                            <button
                              onClick={() => stopCompare(selectedCompareDetail)}
                              disabled={
                                compareActionKey ===
                                `stop:${selectedCompareDetail.compare.id}`
                              }
                              className="h-9 px-3 rounded-lg border border-rose-400/30 text-rose-300 disabled:opacity-50"
                            >
                              {compareActionKey ===
                              `stop:${selectedCompareDetail.compare.id}`
                                ? 'Stopping…'
                                : 'Stop compare'}
                            </button>
                            <button
                              onClick={() =>
                                resolveCompare(
                                  selectedCompareDetail,
                                  selectedCompareDetail.compare.incumbent_trader_id,
                                  'incumbent'
                                )
                              }
                              disabled={
                                compareActionKey ===
                                `resolve:${selectedCompareDetail.compare.id}:${selectedCompareDetail.compare.incumbent_trader_id}`
                              }
                              className="h-9 px-3 rounded-lg border border-white/15 text-white disabled:opacity-50"
                            >
                              {compareActionKey ===
                              `resolve:${selectedCompareDetail.compare.id}:${selectedCompareDetail.compare.incumbent_trader_id}`
                                ? 'Resolving…'
                                : 'Keep incumbent'}
                            </button>
                            <button
                              onClick={() =>
                                resolveCompare(
                                  selectedCompareDetail,
                                  selectedCompareDetail.compare.challenger_trader_id,
                                  'challenger'
                                )
                              }
                              disabled={
                                compareActionKey ===
                                `resolve:${selectedCompareDetail.compare.id}:${selectedCompareDetail.compare.challenger_trader_id}`
                              }
                              className="h-9 px-3 rounded-lg border border-sky-400/30 text-sky-300 disabled:opacity-50"
                            >
                              {compareActionKey ===
                              `resolve:${selectedCompareDetail.compare.id}:${selectedCompareDetail.compare.challenger_trader_id}`
                                ? 'Resolving…'
                                : 'Keep challenger'}
                            </button>
                          </>
                        )}
                      </div>
                    )}
                  </div>

                  {compareDetailLoading ? (
                    <div className="text-sm text-nofx-text-muted mt-4">
                      Loading compare detail…
                    </div>
                  ) : selectedCompareDetail ? (
                    <>
                      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 mt-5 text-sm">
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            Status
                          </div>
                          <div className="font-semibold mt-1 capitalize">
                            {formatCompareStatus(
                              selectedCompareDetail.compare.status
                            )}
                          </div>
                          {selectedCompareDetail.compare.resolved_at && (
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {new Date(
                                selectedCompareDetail.compare.resolved_at
                              ).toLocaleString()}
                            </div>
                          )}
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            Incumbent PnL
                          </div>
                          <div className="font-semibold mt-1">
                            {formatMoney(
                              selectedCompareDetail.metrics?.incumbent_pnl || 0
                            )}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-1">
                            {selectedCompareDetail.metrics?.incumbent_trade_count ||
                              0}{' '}
                            trades · fees{' '}
                            {formatMoney(
                              selectedCompareDetail.metrics?.incumbent_fees || 0
                            )}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            Challenger PnL
                          </div>
                          <div className="font-semibold mt-1">
                            {formatMoney(
                              selectedCompareDetail.metrics?.challenger_pnl || 0
                            )}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-1">
                            {selectedCompareDetail.metrics
                              ?.challenger_trade_count || 0}{' '}
                            trades · fees{' '}
                            {formatMoney(
                              selectedCompareDetail.metrics?.challenger_fees || 0
                            )}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            Winner
                          </div>
                          <div className="font-semibold mt-1">
                            {selectedCompareDetail.compare.winner_trader_id
                              ? selectedCompareDetail.compare.winner_trader_id ===
                                selectedCompareDetail.compare.incumbent_trader_id
                                ? selectedCompareDetail.incumbent_trader_name ||
                                  'Incumbent'
                                : selectedCompareDetail.challenger_trader_name ||
                                  'Challenger'
                              : 'Not resolved'}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-1">
                            {selectedCompareDetail.compare.extension_count > 0
                              ? `Extensions: ${selectedCompareDetail.compare.extension_count}`
                              : 'No extensions'}
                          </div>
                        </div>
                      </div>

                      <div className="rounded-lg border border-white/10 bg-black/20 p-4 mt-4">
                        <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-2">
                          Summary
                        </div>
                        <div className="text-sm text-nofx-text-muted">
                          {selectedCompareDetail.compare.summary}
                        </div>
                        {selectedCompareDetail.compare.error_message && (
                          <div className="text-sm text-rose-300 mt-3">
                            {selectedCompareDetail.compare.error_message}
                          </div>
                        )}
                      </div>

                      <div className="rounded-lg border border-white/10 bg-black/20 p-4 mt-4">
                        <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                          Compare protocol
                        </div>
                        {!selectedCompareDetail.protocol ||
                        selectedCompareDetail.protocol.length === 0 ? (
                          <div className="text-sm text-nofx-text-muted">
                            No protocol entries recorded yet.
                          </div>
                        ) : (
                          <div className="space-y-3">
                            {selectedCompareDetail.protocol.map((event, index) => (
                              <div
                                key={`${event.timestamp}-${event.type}-${index}`}
                                className="rounded-lg border border-white/10 bg-black/30 px-3 py-3"
                              >
                                <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-2">
                                  <div>
                                    <div className="text-xs uppercase tracking-[0.18em] text-sky-300">
                                      {formatCompareProtocolType(event.type)}
                                      {event.actor
                                        ? ` · ${event.actor}`
                                        : ''}
                                    </div>
                                    <div className="text-sm mt-1">
                                      {event.message}
                                    </div>
                                  </div>
                                  <div className="text-xs text-nofx-text-muted">
                                    {new Date(event.timestamp).toLocaleString()}
                                  </div>
                                </div>
                                {buildCompareProtocolMetricSummary(event) && (
                                  <div className="text-xs text-nofx-text-muted mt-2">
                                    {buildCompareProtocolMetricSummary(event)}
                                  </div>
                                )}
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    </>
                  ) : null}
                </div>
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h2 className="font-semibold text-lg">Strategy history</h2>
                  <p className="text-xs text-nofx-text-muted mt-1">
                    Applied AI patches and rollback points for the current
                    trader strategy.
                  </p>
                </div>
              </div>

              <div className="space-y-3 mt-4">
                {versions.length === 0 ? (
                  <div className="text-sm text-nofx-text-muted">
                    No strategy versions captured yet.
                  </div>
                ) : (
                  versions.map((version) => {
                    const relatedCompare = challengerCompares.find(
                      (item) =>
                        item.compare.id === version.version.source_compare_id
                    )
                    const hasCompareLink = Boolean(
                      version.version.source_compare_id?.trim()
                    )
                    const compareNote = relatedCompare
                      ? relatedCompare.compare.summary
                      : hasCompareLink
                        ? 'Linked challenger compare available.'
                        : ''
                    return (
                      <div
                        key={version.version.id}
                        className="rounded-xl border border-white/10 bg-black/20 p-4"
                      >
                        <div className="flex items-start justify-between gap-4">
                          <div>
                            <div className="text-xs text-nofx-gold uppercase">
                              {formatStrategyVersionSourceType(
                                version.version.source_type
                              )}
                            </div>
                            <div className="font-medium mt-1">
                              {version.version.summary || 'Strategy change'}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-2">
                              {new Date(
                                version.version.created_at
                              ).toLocaleString()}
                            </div>
                            {hasCompareLink && (
                              <div className="text-xs text-sky-300 mt-2">
                                Compare note: {compareNote}
                              </div>
                            )}
                          </div>
                          <div className="flex flex-wrap justify-end gap-2">
                            {hasCompareLink && (
                              <button
                                onClick={() => openVersionCompare(version)}
                                disabled={
                                  openingCompareId ===
                                  version.version.source_compare_id
                                }
                                className="h-9 px-3 rounded-lg border border-sky-400/30 text-sky-300 disabled:opacity-50"
                              >
                                {openingCompareId ===
                                version.version.source_compare_id
                                  ? 'Opening…'
                                  : 'Open compare'}
                              </button>
                            )}
                            <button
                              onClick={() => rollbackVersion(version)}
                              disabled={rollingBackVersionId === version.version.id}
                              className="h-9 px-3 rounded-lg border border-white/15 text-white disabled:opacity-50"
                            >
                              {rollingBackVersionId === version.version.id
                                ? 'Rolling back…'
                                : 'Rollback'}
                            </button>
                          </div>
                        </div>
                      </div>
                    )
                  })
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </DeepVoidBackground>
  )
}
