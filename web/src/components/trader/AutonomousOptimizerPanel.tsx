import { useEffect, useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { notify } from '../../lib/notify'
import { NofxSelect } from '../ui/select'
import type {
  AIModel,
  AutonomousOptimizerBacklogItem,
  AutonomousOptimizerConfig,
  AutonomousOptimizerModelOutcome,
  AutonomousOptimizerRun,
  AutonomousOptimizerRunDetail,
  DealReviewJSONDiffEntry,
  DealReviewStrategyVersionDetail,
  RemoteModelInfo,
} from '../../types'

interface AutonomousOptimizerPanelProps {
  traderId?: string
  traderName?: string
  onOpenStrategyVersion?: (
    versionId: string,
    detail?: DealReviewStrategyVersionDetail | null
  ) => void
  onApplyRunWindowFilter?: (fromTime: number, toTime: number) => void
}

const optimizerStatusOptions = [
  { value: '', label: 'All run statuses' },
  { value: 'scheduled', label: 'Scheduled' },
  { value: 'running', label: 'Running' },
  { value: 'insufficient_evidence', label: 'Insufficient evidence' },
  { value: 'no_change', label: 'No change' },
  { value: 'backlog_only', label: 'Backlog only' },
  { value: 'blocked_by_gate', label: 'Blocked by gate' },
  { value: 'deferred_for_next_window', label: 'Deferred for next window' },
  { value: 'auto_applied', label: 'Auto applied' },
  { value: 'monitoring', label: 'Monitoring' },
  { value: 'rollback_pending', label: 'Rollback pending' },
  { value: 'rolled_back', label: 'Rolled back' },
  { value: 'kept', label: 'Kept' },
  { value: 'paused', label: 'Paused' },
  { value: 'failed', label: 'Failed' },
]

const configStatusOptions = [
  { value: 'scheduled', label: 'Scheduled' },
  { value: 'paused', label: 'Paused' },
]

const backlogStatusOptions = [
  { value: '', label: 'All backlog statuses' },
  { value: 'new', label: 'New' },
  { value: 'confirmed', label: 'Confirmed' },
  { value: 'planned', label: 'Planned' },
  { value: 'in_progress', label: 'In progress' },
  { value: 'done', label: 'Done' },
  { value: 'rejected', label: 'Rejected' },
]

const backlogCategoryOptions = [
  { value: 'missing_indicator', label: 'Missing indicator' },
  { value: 'missing_market_data', label: 'Missing market data' },
  { value: 'missing_execution_telemetry', label: 'Missing execution telemetry' },
  { value: 'missing_regime_metadata', label: 'Missing regime metadata' },
  { value: 'missing_risk_control', label: 'Missing risk control' },
  { value: 'missing_prompt_instruction', label: 'Missing prompt instruction' },
  { value: 'missing_review_metric', label: 'Missing review metric' },
  { value: 'other_capability_gap', label: 'Other capability gap' },
]

interface BacklogEditDraft {
  title: string
  category: string
  description: string
  expectedImpact: string
  confidence: string
  implementationCost: string
  urgency: string
  recurrenceCount: string
  status: string
}

function normalizeTime(value?: string): string {
  if (!value || value.startsWith('0001-01-01')) return '-'
  return new Date(value).toLocaleString()
}

function formatRelativeTime(value?: string): string {
  if (!value || value.startsWith('0001-01-01')) return '-'
  const deltaMs = new Date(value).getTime() - Date.now()
  if (!Number.isFinite(deltaMs)) return '-'
  const mins = Math.round(deltaMs / 60000)
  if (Math.abs(mins) < 60) {
    return mins >= 0 ? `in ${mins}m` : `${Math.abs(mins)}m ago`
  }
  const hours = Math.round(mins / 60)
  if (Math.abs(hours) < 48) {
    return hours >= 0 ? `in ${hours}h` : `${Math.abs(hours)}h ago`
  }
  const days = Math.round(hours / 24)
  return days >= 0 ? `in ${days}d` : `${Math.abs(days)}d ago`
}

function formatLabel(value?: string): string {
  if (!value) return '-'
  return value
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (match) => match.toUpperCase())
}

function formatScore(value?: number): string {
  const safe = typeof value === 'number' && Number.isFinite(value) ? value : 0
  return `${Math.round(safe)}/100`
}

function statusToneClasses(status?: string): string {
  switch (status) {
    case 'auto_applied':
    case 'kept':
    case 'done':
      return 'border-emerald-400/25 bg-emerald-500/15 text-emerald-300'
    case 'blocked_by_gate':
    case 'deferred_for_next_window':
    case 'insufficient_evidence':
    case 'backlog_only':
    case 'monitoring':
    case 'rollback_pending':
    case 'planned':
    case 'in_progress':
      return 'border-amber-400/25 bg-amber-500/15 text-amber-200'
    case 'failed':
    case 'rolled_back':
    case 'rejected':
      return 'border-rose-400/25 bg-rose-500/15 text-rose-300'
    default:
      return 'border-white/10 bg-white/5 text-nofx-text-muted'
  }
}

function modelLabel(model?: AIModel): string {
  if (!model) return 'Select model account'
  return model.customModelName?.trim() || model.name || model.provider
}

function formatNumber(value: unknown): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return value.toFixed(2)
}

function formatPct(value: unknown): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return `${value.toFixed(1)}%`
}

function sortBacklogItems(items: AutonomousOptimizerBacklogItem[]) {
  return [...items].sort((left, right) => {
    if (right.composite_score !== left.composite_score) {
      return right.composite_score - left.composite_score
    }
    return (
      new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime()
    )
  })
}

function buildBacklogEditDraft(item: AutonomousOptimizerBacklogItem): BacklogEditDraft {
  return {
    title: item.title || '',
    category: item.category || 'other_capability_gap',
    description: item.description || '',
    expectedImpact: item.expected_impact || '',
    confidence: String(item.confidence ?? 0),
    implementationCost: String(item.implementation_cost ?? 0),
    urgency: String(item.urgency ?? 0),
    recurrenceCount: String(item.recurrence_count ?? 1),
    status: item.status || 'new',
  }
}

function prettyJSON(value: unknown): string {
  if (!value) return '{}'
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return '{}'
  }
}

function readNestedString(
  body: Record<string, unknown> | undefined,
  ...path: string[]
): string {
  let current: unknown = body
  for (const key of path) {
    if (!current || typeof current !== 'object') return ''
    current = (current as Record<string, unknown>)[key]
  }
  return typeof current === 'string' ? current : ''
}

function readNestedNumber(
  body: Record<string, unknown> | undefined,
  ...path: string[]
): number | undefined {
  let current: unknown = body
  for (const key of path) {
    if (!current || typeof current !== 'object') return undefined
    current = (current as Record<string, unknown>)[key]
  }
  return typeof current === 'number' && Number.isFinite(current)
    ? current
    : undefined
}

function readNestedObjectArray(
  body: Record<string, unknown> | undefined,
  ...path: string[]
): Record<string, unknown>[] {
  let current: unknown = body
  for (const key of path) {
    if (!current || typeof current !== 'object') return []
    current = (current as Record<string, unknown>)[key]
  }
  if (!Array.isArray(current)) return []
  return current.filter(
    (item): item is Record<string, unknown> =>
      Boolean(item) && typeof item === 'object' && !Array.isArray(item)
  )
}

function readStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string')
}

function readNestedStringArray(
  body: Record<string, unknown> | undefined,
  ...path: string[]
): string[] {
  let current: unknown = body
  for (const key of path) {
    if (!current || typeof current !== 'object') return []
    current = (current as Record<string, unknown>)[key]
  }
  return readStringArray(current)
}

function renderDiffList(diffs?: DealReviewJSONDiffEntry[]) {
  if (!diffs || diffs.length === 0) {
    return (
      <div className="text-sm text-nofx-text-muted">
        No before/after diff is available for this run.
      </div>
    )
  }
  return (
    <div className="space-y-2">
      {diffs.map((entry) => (
        <div
          key={`${entry.path}-${entry.left}-${entry.right}`}
          className="rounded-lg border border-white/10 bg-black/20 p-3"
        >
          <div className="text-xs uppercase tracking-[0.18em] text-nofx-text-muted">
            {entry.path}
          </div>
          <div className="grid grid-cols-1 xl:grid-cols-2 gap-3 mt-2 text-xs">
            <div>
              <div className="text-nofx-text-muted mb-1">Before</div>
              <pre className="whitespace-pre-wrap break-words text-rose-200">
                {entry.left}
              </pre>
            </div>
            <div>
              <div className="text-nofx-text-muted mb-1">After</div>
              <pre className="whitespace-pre-wrap break-words text-emerald-200">
                {entry.right}
              </pre>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

export function AutonomousOptimizerPanel({
  traderId,
  traderName,
  onOpenStrategyVersion,
  onApplyRunWindowFilter,
}: AutonomousOptimizerPanelProps) {
  const [config, setConfig] = useState<AutonomousOptimizerConfig | null>(null)
  const [runs, setRuns] = useState<AutonomousOptimizerRun[]>([])
  const [backlog, setBacklog] = useState<AutonomousOptimizerBacklogItem[]>([])
  const [modelOutcomes, setModelOutcomes] = useState<AutonomousOptimizerModelOutcome[]>([])
  const [models, setModels] = useState<AIModel[]>([])
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null)
  const [selectedRunDetail, setSelectedRunDetail] =
    useState<AutonomousOptimizerRunDetail | null>(null)
  const [runDetailLoading, setRunDetailLoading] = useState(false)
  const [loading, setLoading] = useState(false)
  const [savingConfig, setSavingConfig] = useState(false)
  const [savingBacklogId, setSavingBacklogId] = useState<string | null>(null)
  const [editingBacklogId, setEditingBacklogId] = useState<string | null>(null)
  const [backlogDraft, setBacklogDraft] = useState<BacklogEditDraft | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [runStatusFilter, setRunStatusFilter] = useState('')
  const [backlogStatusFilter, setBacklogStatusFilter] = useState('')
  const [primaryRemoteModels, setPrimaryRemoteModels] = useState<
    RemoteModelInfo[]
  >([])
  const [criticRemoteModels, setCriticRemoteModels] = useState<RemoteModelInfo[]>(
    []
  )

  const [enabled, setEnabled] = useState(false)
  const [configStatus, setConfigStatus] = useState('paused')
  const [reviewIntervalHours, setReviewIntervalHours] = useState('12')
  const [autoApplyCooldownHours, setAutoApplyCooldownHours] = useState('12')
  const [maxConsecutiveAutoApplies, setMaxConsecutiveAutoApplies] = useState('2')
  const [autoApplyConfigPatch, setAutoApplyConfigPatch] = useState(true)
  const [autoApplyPromptPatch, setAutoApplyPromptPatch] = useState(true)
  const [autoRollbackEnabled, setAutoRollbackEnabled] = useState(true)
  const [selfPauseEnabled, setSelfPauseEnabled] = useState(true)
  const [primaryModelConfigID, setPrimaryModelConfigID] = useState('')
  const [primaryModelName, setPrimaryModelName] = useState('')
  const [criticModelConfigID, setCriticModelConfigID] = useState('')
  const [criticModelName, setCriticModelName] = useState('')
  const [proposalPromptInstructions, setProposalPromptInstructions] = useState('')
  const [criticPromptInstructions, setCriticPromptInstructions] = useState('')

  const loadPanel = async () => {
    if (!traderId) return
    setLoading(true)
    setError(null)
    try {
      const [nextConfig, nextRuns, nextBacklog, nextModelOutcomes, nextModels] =
        await Promise.all([
          api.getTraderAutonomousOptimizerConfig(traderId),
          api.getTraderAutonomousOptimizerRuns(traderId, 20),
          api.getTraderAutonomousOptimizerBacklog(traderId, 30),
          api.getTraderAutonomousOptimizerModelOutcomes(traderId),
          api.getModelConfigs(),
        ])
      setConfig(nextConfig)
      setRuns(nextRuns)
      setBacklog(nextBacklog)
      setModelOutcomes(nextModelOutcomes)
      setModels(nextModels.filter((item) => item.enabled))
      const preferredRunId =
        nextRuns.find((item) => item.id === selectedRunId)?.id ||
        nextRuns[0]?.id ||
        ''
      if (preferredRunId) {
        void loadRunDetail(preferredRunId)
      }
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : 'Failed to load autonomous optimizer panel'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const loadRunDetail = async (runId: string) => {
    if (!traderId || !runId) return
    setRunDetailLoading(true)
    try {
      const detail = await api.getTraderAutonomousOptimizerRunDetail(
        traderId,
        runId
      )
      setSelectedRunId(runId)
      setSelectedRunDetail(detail)
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to load autonomous optimizer run detail'
      )
    } finally {
      setRunDetailLoading(false)
    }
  }

  useEffect(() => {
    void loadPanel()
  }, [traderId])

  useEffect(() => {
    setSelectedRunId(null)
    setSelectedRunDetail(null)
    setEditingBacklogId(null)
    setBacklogDraft(null)
  }, [traderId])

  useEffect(() => {
    if (!config) return
    setEnabled(config.enabled)
    setConfigStatus(config.status || 'paused')
    setReviewIntervalHours(String(config.review_interval_hours || 12))
    setAutoApplyCooldownHours(String(config.auto_apply_cooldown_hours || 12))
    setMaxConsecutiveAutoApplies(
      String(config.max_consecutive_auto_applies || 2)
    )
    setAutoApplyConfigPatch(config.auto_apply_config_patch)
    setAutoApplyPromptPatch(config.auto_apply_prompt_patch)
    setAutoRollbackEnabled(config.auto_rollback_enabled)
    setSelfPauseEnabled(config.self_pause_enabled)
    setPrimaryModelConfigID(config.primary_model_config_id || '')
    setPrimaryModelName(config.primary_model_name || '')
    setCriticModelConfigID(config.critic_model_config_id || '')
    setCriticModelName(config.critic_model_name || '')
    setProposalPromptInstructions(config.proposal_prompt_instructions || '')
    setCriticPromptInstructions(config.critic_prompt_instructions || '')
  }, [config])

  useEffect(() => {
    if (!primaryModelConfigID) {
      setPrimaryRemoteModels([])
      return
    }
    api
      .getAvailableRemoteModels(primaryModelConfigID)
      .then(setPrimaryRemoteModels)
      .catch(() => setPrimaryRemoteModels([]))
  }, [primaryModelConfigID])

  useEffect(() => {
    if (!criticModelConfigID) {
      setCriticRemoteModels([])
      return
    }
    api
      .getAvailableRemoteModels(criticModelConfigID)
      .then(setCriticRemoteModels)
      .catch(() => setCriticRemoteModels([]))
  }, [criticModelConfigID])

  const filteredRuns = useMemo(() => {
    if (!runStatusFilter) return runs
    return runs.filter((item) => item.status === runStatusFilter)
  }, [runStatusFilter, runs])

  const filteredBacklog = useMemo(() => {
    if (!backlogStatusFilter) return backlog
    return backlog.filter((item) => item.status === backlogStatusFilter)
  }, [backlog, backlogStatusFilter])

  const activeBacklog = useMemo(
    () =>
      backlog.filter(
        (item) => item.status !== 'done' && item.status !== 'rejected'
      ),
    [backlog]
  )

  useEffect(() => {
    if (filteredRuns.length === 0) {
      setSelectedRunId(null)
      setSelectedRunDetail(null)
      return
    }
    const selectedStillVisible = filteredRuns.some(
      (item) => item.id === selectedRunId
    )
    const nextRunId = selectedStillVisible
      ? selectedRunId
      : filteredRuns[0]?.id || null
    if (nextRunId && nextRunId !== selectedRunId) {
      void loadRunDetail(nextRunId)
    }
  }, [filteredRuns, selectedRunId])

  const lastRun = runs[0] || null
  const activeModelKey = useMemo(() => {
    if (!config) return ''
    return [
      config.primary_model_config_id || '',
      config.primary_model_name || '',
      config.critic_model_config_id || '',
      config.critic_model_name || '',
    ].join('|')
  }, [config])
  const lastAppliedRun =
    runs.find(
      (item) =>
        item.status === 'auto_applied' ||
        item.status === 'monitoring' ||
        item.status === 'rollback_pending' ||
        item.status === 'kept' ||
        item.status === 'rolled_back'
    ) || null
  const selectedValidation = selectedRunDetail?.validation
  const selectedMetadata = selectedRunDetail?.metadata
  const proposalType = readNestedString(
    selectedMetadata,
    'proposal',
    'proposal_type'
  )
  const proposalExpectedEffect = readNestedString(
    selectedMetadata,
    'proposal',
    'expected_effect'
  )
  const criticSummary = readNestedString(selectedValidation, 'critic', 'summary')
  const configValidationStatus = readNestedString(
    selectedValidation,
    'config_validation',
    'status'
  )
  const promptValidationIssues = readNestedNumber(
    selectedValidation,
    'prompt_validation',
    'changed_field_count'
  )
  const reviewWindowClosedDeals = readNestedNumber(
    selectedMetadata,
    'closed_deals'
  )
  const reviewWindowCandidates = readNestedNumber(
    selectedMetadata,
    'decision_candidate_count'
  )
  const openDecisionCount = readNestedNumber(
    selectedMetadata,
    'open_decision_count'
  )
  const reviewWindowCycles = readNestedNumber(
    selectedMetadata,
    'decision_record_count'
  )
  const reviewWindowPnL = readNestedNumber(selectedMetadata, 'net_pnl')
  const holdDecisionCount = readNestedNumber(
    selectedMetadata,
    'hold_decision_count'
  )
  const waitDecisionCount = readNestedNumber(
    selectedMetadata,
    'wait_decision_count'
  )
  const decisionConversionRate = readNestedNumber(
    selectedMetadata,
    'decision_conversion_rate'
  )
  const avgDecisionConfidence = readNestedNumber(
    selectedMetadata,
    'avg_decision_confidence'
  )
  const rejectReasons = readNestedObjectArray(selectedMetadata, 'reject_reasons')
  const confidenceBands = readNestedObjectArray(
    selectedMetadata,
    'confidence_bands'
  )
  const opportunitySessions = readNestedObjectArray(
    selectedMetadata,
    'opportunity_sessions'
  )
  const opportunitySymbols = readNestedObjectArray(
    selectedMetadata,
    'opportunity_symbols'
  )
  const recentOptimizerContext = readNestedObjectArray(
    selectedMetadata,
    'recent_optimizer_runs'
  )
  const cooldownUntilMs = readNestedNumber(selectedMetadata, 'cooldown_until_ms')
  const nextEligibleRunMs = readNestedNumber(
    selectedMetadata,
    'next_eligible_run_ms'
  )
  const monitoringRootRunId = readNestedString(
    selectedMetadata,
    'monitoring_root_run_id'
  )
  const monitoringWindowsObserved = readNestedNumber(
    selectedMetadata,
    'monitoring_chain',
    'windows_observed'
  )
  const monitoringObservedClosedDeals = readNestedNumber(
    selectedMetadata,
    'rollback_analysis',
    'observed_closed_deals'
  )
  const monitoringObservedNetPnL = readNestedNumber(
    selectedMetadata,
    'rollback_analysis',
    'observed_net_pnl'
  )
  const monitoringObservedNegativeWindows = readNestedNumber(
    selectedMetadata,
    'rollback_analysis',
    'observed_negative_windows'
  )
  const rollbackTriggerCodes = readNestedStringArray(
    selectedMetadata,
    'rollback_analysis',
    'trigger_codes'
  )
  const rollbackTriggerReasons = readNestedStringArray(
    selectedMetadata,
    'rollback_analysis',
    'trigger_reasons'
  )
  const sourceBaselineNetPnL = readNestedNumber(
    selectedMetadata,
    'monitoring_source_snapshot',
    'net_pnl'
  )
  const sourceBaselineWinRate = readNestedNumber(
    selectedMetadata,
    'monitoring_source_snapshot',
    'win_rate'
  )
  const sourceBaselineExitEfficiency = readNestedNumber(
    selectedMetadata,
    'monitoring_source_snapshot',
    'avg_exit_efficiency_score'
  )

  const saveConfig = async () => {
    if (!traderId) return
    setSavingConfig(true)
    try {
      const nextInterval = Number(reviewIntervalHours)
      const nextCooldown = Number(autoApplyCooldownHours)
      const nextMaxConsecutive = Number(maxConsecutiveAutoApplies)
      const nextConfig = await api.updateTraderAutonomousOptimizerConfig(
        traderId,
        {
          enabled,
          status: configStatus,
          review_interval_hours: Number.isFinite(nextInterval)
            ? nextInterval
            : 12,
          auto_apply_cooldown_hours: Number.isFinite(nextCooldown)
            ? nextCooldown
            : 12,
          max_consecutive_auto_applies: Number.isFinite(nextMaxConsecutive)
            ? nextMaxConsecutive
            : 2,
          auto_apply_config_patch: autoApplyConfigPatch,
          auto_apply_prompt_patch: autoApplyPromptPatch,
          auto_rollback_enabled: autoRollbackEnabled,
          self_pause_enabled: selfPauseEnabled,
          primary_model_config_id: primaryModelConfigID,
          primary_model_name: primaryModelName.trim() || 'gpt-5.4',
          critic_model_config_id: criticModelConfigID,
          critic_model_name: criticModelName.trim() || 'gpt-5.4',
          proposal_prompt_instructions: proposalPromptInstructions,
          critic_prompt_instructions: criticPromptInstructions,
        }
      )
      setConfig(nextConfig)
      notify.success('Autonomous optimizer config saved')
      await loadPanel()
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to save autonomous optimizer config'
      )
    } finally {
      setSavingConfig(false)
    }
  }

  const updateBacklogStatus = async (
    item: AutonomousOptimizerBacklogItem,
    nextStatus: string
  ) => {
    if (!traderId) return
    setSavingBacklogId(item.id)
    try {
      const saved = await api.saveTraderAutonomousOptimizerBacklog(traderId, {
        id: item.id,
        title: item.title,
        category: item.category,
        description: item.description,
        expected_impact: item.expected_impact,
        confidence: item.confidence,
        implementation_cost: item.implementation_cost,
        urgency: item.urgency,
        recurrence_count: item.recurrence_count,
        status: nextStatus,
        user_edited: true,
      })
      setBacklog((current) =>
        sortBacklogItems(
          current.map((entry) => (entry.id === saved.id ? saved : entry))
        )
      )
      notify.success(`Backlog item marked as ${formatLabel(nextStatus)}`)
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to update autonomous optimizer backlog item'
      )
    } finally {
      setSavingBacklogId(null)
    }
  }

  const beginBacklogEdit = (item: AutonomousOptimizerBacklogItem) => {
    setEditingBacklogId(item.id)
    setBacklogDraft(buildBacklogEditDraft(item))
  }

  const cancelBacklogEdit = () => {
    setEditingBacklogId(null)
    setBacklogDraft(null)
  }

  const saveBacklogEdit = async (item: AutonomousOptimizerBacklogItem) => {
    if (!traderId || !backlogDraft) return
    setSavingBacklogId(item.id)
    try {
      const parsedConfidence = Number(backlogDraft.confidence)
      const parsedCost = Number(backlogDraft.implementationCost)
      const parsedUrgency = Number(backlogDraft.urgency)
      const parsedRecurrence = Number(backlogDraft.recurrenceCount)
      const saved = await api.saveTraderAutonomousOptimizerBacklog(traderId, {
        id: item.id,
        title: backlogDraft.title.trim() || item.title,
        category: backlogDraft.category.trim() || item.category,
        description: backlogDraft.description.trim(),
        expected_impact: backlogDraft.expectedImpact.trim(),
        confidence: Number.isFinite(parsedConfidence)
          ? parsedConfidence
          : item.confidence,
        implementation_cost: Number.isFinite(parsedCost)
          ? parsedCost
          : item.implementation_cost,
        urgency: Number.isFinite(parsedUrgency) ? parsedUrgency : item.urgency,
        recurrence_count:
          Number.isFinite(parsedRecurrence) && parsedRecurrence > 0
            ? Math.round(parsedRecurrence)
            : item.recurrence_count,
        status: backlogDraft.status || item.status,
        user_edited: true,
      })
      setBacklog((current) =>
        sortBacklogItems(
          current.map((entry) => (entry.id === saved.id ? saved : entry))
        )
      )
      setEditingBacklogId(null)
      setBacklogDraft(null)
      notify.success('Backlog item updated')
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : 'Failed to save autonomous optimizer backlog item'
      )
    } finally {
      setSavingBacklogId(null)
    }
  }

  const openLinkedStrategyVersion = () => {
    const version =
      selectedRunDetail?.linked_strategy_version ||
      (selectedRunDetail?.run.applied_strategy_version_id
        ? null
        : null)
    const versionId =
      version?.version.id ||
      selectedRunDetail?.run.applied_strategy_version_id ||
      ''
    if (!versionId) {
      notify.info('No linked strategy version is stored for this run')
      return
    }
    onOpenStrategyVersion?.(versionId, version || null)
  }

  const applyRunWindowFilter = () => {
    const fromTime = selectedRunDetail?.review_window_start_ms || 0
    const toTime = selectedRunDetail?.review_window_end_ms || 0
    if (!fromTime || !toTime) {
      notify.info('No stored review window is available for this run')
      return
    }
    onApplyRunWindowFilter?.(fromTime, toTime)
  }

  const modelOptions = [
    { value: '', label: 'Select model account' },
    ...models.map((item) => ({
      value: item.id,
      label: modelLabel(item),
    })),
  ]

  return (
    <div className="nofx-glass rounded-xl p-5 space-y-5">
      <div className="flex flex-col xl:flex-row xl:items-start xl:justify-between gap-4">
        <div>
          <div className="text-xs uppercase tracking-[0.24em] text-nofx-gold/80">
            Autonomous Optimizer
          </div>
          <h2 className="text-xl font-semibold text-nofx-text-main mt-1">
            Self-improving loop for {traderName || 'selected trader'}
          </h2>
          <p className="text-sm text-nofx-text-muted mt-2">
            Visible here are the AI improvement steps, the current live
            optimizer status, and the recent automatic review outcomes.
          </p>
        </div>
        <button
          onClick={() => void loadPanel()}
          disabled={!traderId || loading}
          className="h-10 px-4 rounded-lg border border-white/10 bg-black/20 text-sm font-semibold disabled:opacity-50"
        >
          {loading ? 'Refreshing…' : 'Refresh optimizer'}
        </button>
      </div>

      {error && (
        <div className="rounded-lg border border-rose-400/20 bg-rose-500/10 p-3 text-sm text-rose-300">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3">
        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
          <div className="text-xs text-nofx-text-muted">Current state</div>
          <div className="mt-2">
            <span
              className={`inline-flex px-2.5 py-1 rounded-full border text-xs font-semibold ${statusToneClasses(config?.status)}`}
            >
              {formatLabel(config?.status || 'paused')}
            </span>
          </div>
          <div className="text-xs text-nofx-text-muted mt-3">
            Enabled {config?.enabled ? 'Yes' : 'No'}
          </div>
        </div>

        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
          <div className="text-xs text-nofx-text-muted">Next review window</div>
          <div className="text-lg font-semibold mt-2">
            {formatRelativeTime(config?.next_run_at)}
          </div>
          <div className="text-xs text-nofx-text-muted mt-2">
            {normalizeTime(config?.next_run_at)}
          </div>
        </div>

        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
          <div className="text-xs text-nofx-text-muted">Last run</div>
          <div className="text-lg font-semibold mt-2">
            {formatLabel(lastRun?.status || 'none')}
          </div>
          <div className="text-xs text-nofx-text-muted mt-2">
            {lastRun ? normalizeTime(lastRun.completed_at || lastRun.started_at) : '-'}
          </div>
        </div>

        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
          <div className="text-xs text-nofx-text-muted">Open improvement steps</div>
          <div className="text-lg font-semibold mt-2">{activeBacklog.length}</div>
          <div className="text-xs text-nofx-text-muted mt-2">
            Highest score {activeBacklog[0] ? formatScore(activeBacklog[0].composite_score) : '-'}
          </div>
        </div>

        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
          <div className="text-xs text-nofx-text-muted">Last applied change</div>
          <div className="text-sm font-semibold mt-2 line-clamp-2">
            {lastAppliedRun?.summary || 'No applied autonomous change yet'}
          </div>
          <div className="text-xs text-nofx-text-muted mt-2">
            {lastAppliedRun ? normalizeTime(lastAppliedRun.updated_at) : '-'}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[0.95fr_1.05fr] gap-5">
        <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
              Config
            </div>
            <div className="text-sm text-nofx-text-muted mt-2">
              Choose the active optimizer account and model pair here. This is
              where `GPT-5.4` is currently wired by default.
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">Enabled</div>
              <input
                type="checkbox"
                checked={enabled}
                onChange={(event) => setEnabled(event.target.checked)}
              />
            </label>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-xs text-nofx-text-muted mb-2">Status</div>
              <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center">
                <NofxSelect
                  value={configStatus}
                  onChange={setConfigStatus}
                  options={configStatusOptions}
                />
              </div>
            </div>
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                Review interval hours
              </div>
              <input
                value={reviewIntervalHours}
                onChange={(event) => setReviewIntervalHours(event.target.value)}
                className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
              />
            </label>
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                Cooldown hours
              </div>
              <input
                value={autoApplyCooldownHours}
                onChange={(event) =>
                  setAutoApplyCooldownHours(event.target.value)
                }
                className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
              />
            </label>
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                Max consecutive applies
              </div>
              <input
                value={maxConsecutiveAutoApplies}
                onChange={(event) =>
                  setMaxConsecutiveAutoApplies(event.target.value)
                }
                className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
              />
            </label>
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                Auto-apply config patches
              </div>
              <input
                type="checkbox"
                checked={autoApplyConfigPatch}
                onChange={(event) =>
                  setAutoApplyConfigPatch(event.target.checked)
                }
              />
            </label>
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                Auto-apply prompt patches
              </div>
              <input
                type="checkbox"
                checked={autoApplyPromptPatch}
                onChange={(event) =>
                  setAutoApplyPromptPatch(event.target.checked)
                }
              />
            </label>
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                Auto rollback
              </div>
              <input
                type="checkbox"
                checked={autoRollbackEnabled}
                onChange={(event) =>
                  setAutoRollbackEnabled(event.target.checked)
                }
              />
            </label>
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                Self-pause
              </div>
              <input
                type="checkbox"
                checked={selfPauseEnabled}
                onChange={(event) => setSelfPauseEnabled(event.target.checked)}
              />
            </label>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-xs text-nofx-text-muted mb-2">
                Proposer account
              </div>
              <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center">
                <NofxSelect
                  value={primaryModelConfigID}
                  onChange={setPrimaryModelConfigID}
                  options={modelOptions}
                />
              </div>
              <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center mt-3">
                <NofxSelect
                  value={primaryModelName}
                  onChange={setPrimaryModelName}
                  options={[
                    { value: primaryModelName || '', label: 'Current model name' },
                    ...primaryRemoteModels
                      .filter((item) => item.available)
                      .map((item) => ({
                        value: item.id,
                        label: item.label,
                      })),
                  ]}
                />
              </div>
              <input
                value={primaryModelName}
                onChange={(event) => setPrimaryModelName(event.target.value)}
                placeholder="e.g. gpt-5.4"
                className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm mt-3"
              />
            </div>

            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-xs text-nofx-text-muted mb-2">
                Critic account
              </div>
              <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center">
                <NofxSelect
                  value={criticModelConfigID}
                  onChange={setCriticModelConfigID}
                  options={modelOptions}
                />
              </div>
              <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center mt-3">
                <NofxSelect
                  value={criticModelName}
                  onChange={setCriticModelName}
                  options={[
                    { value: criticModelName || '', label: 'Current model name' },
                    ...criticRemoteModels
                      .filter((item) => item.available)
                      .map((item) => ({
                        value: item.id,
                        label: item.label,
                      })),
                  ]}
                />
              </div>
              <input
                value={criticModelName}
                onChange={(event) => setCriticModelName(event.target.value)}
                placeholder="e.g. gpt-5.4"
                className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm mt-3"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                Proposal instructions overlay
              </div>
              <textarea
                value={proposalPromptInstructions}
                onChange={(event) =>
                  setProposalPromptInstructions(event.target.value)
                }
                placeholder="Optional extra instructions merged into the optimizer proposer prompt."
                className="min-h-[140px] w-full rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm"
              />
            </label>
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                Critic instructions overlay
              </div>
              <textarea
                value={criticPromptInstructions}
                onChange={(event) =>
                  setCriticPromptInstructions(event.target.value)
                }
                placeholder="Optional extra instructions merged into the optimizer critic prompt."
                className="min-h-[140px] w-full rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm"
              />
            </label>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-5 gap-3 text-xs">
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted">Seed source trader</div>
              <div className="mt-1 break-all">
                {config?.seed_source_trader_id || '-'}
              </div>
            </div>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted">Current seed strategy</div>
              <div className="mt-1 break-all">
                {config?.current_seed_strategy_id || '-'}
              </div>
            </div>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted">Last applied version</div>
              <div className="mt-1 break-all">
                {lastAppliedRun?.applied_strategy_version_id || '-'}
              </div>
            </div>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted">Cooldown</div>
              <div className="mt-1 break-all">
                {config?.auto_apply_cooldown_hours || 12}h
              </div>
            </div>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted">Max apply streak</div>
              <div className="mt-1 break-all">
                {config?.max_consecutive_auto_applies || 2}
              </div>
            </div>
          </div>

          <button
            onClick={() => void saveConfig()}
            disabled={!traderId || savingConfig}
            className="h-11 px-4 rounded-lg bg-nofx-gold text-black font-semibold disabled:opacity-50"
          >
            {savingConfig ? 'Saving…' : 'Save optimizer config'}
          </button>
        </div>

        <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
          <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
            <div>
              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                Improvement Backlog
              </div>
              <div className="text-sm text-nofx-text-muted mt-2">
                These are the AI-detected next improvement steps. If the AI says
                indicators, telemetry, or prompt changes are missing, they land
                here with a score.
              </div>
            </div>
            <div className="w-full md:w-56 h-10 rounded-lg border border-white/10 px-3 flex items-center">
              <NofxSelect
                value={backlogStatusFilter}
                onChange={setBacklogStatusFilter}
                options={backlogStatusOptions}
              />
            </div>
          </div>

          <div className="space-y-3 max-h-[32rem] overflow-y-auto pr-1">
            {filteredBacklog.length === 0 ? (
              <div className="rounded-lg border border-dashed border-white/10 bg-black/10 p-4 text-sm text-nofx-text-muted">
                No visible improvement steps yet for this filter. Once the
                optimizer proposes missing capabilities or next build items,
                they will appear here.
              </div>
            ) : (
              filteredBacklog.map((item) => (
                <div
                  key={item.id}
                  className="rounded-xl border border-white/10 bg-black/20 p-4"
                >
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex flex-wrap gap-2 items-center">
                        <div className="font-semibold">{item.title}</div>
                        <span
                          className={`inline-flex px-2 py-1 rounded-full border text-[11px] ${statusToneClasses(item.status)}`}
                        >
                          {formatLabel(item.status)}
                        </span>
                        <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                          {formatLabel(item.category)}
                        </span>
                        {item.ai_generated && (
                          <span className="inline-flex px-2 py-1 rounded-full border border-sky-400/20 bg-sky-500/10 text-[11px] text-sky-300">
                            AI
                          </span>
                        )}
                        {item.user_edited && (
                          <span className="inline-flex px-2 py-1 rounded-full border border-nofx-gold/20 bg-nofx-gold/10 text-[11px] text-nofx-gold">
                            User edited
                          </span>
                        )}
                      </div>
                      {item.description && (
                        <div className="text-sm text-nofx-text-muted mt-2 whitespace-pre-wrap">
                          {item.description}
                        </div>
                      )}
                      {item.expected_impact && (
                        <div className="text-sm text-emerald-300 mt-2">
                          Expected impact: {item.expected_impact}
                        </div>
                      )}
                      <div className="flex flex-wrap gap-4 text-xs text-nofx-text-muted mt-3">
                        <span>Score {formatScore(item.composite_score)}</span>
                        <span>Confidence {formatScore(item.confidence)}</span>
                        <span>Urgency {formatScore(item.urgency)}</span>
                        <span>Cost {formatScore(item.implementation_cost)}</span>
                        <span>Recurrent {item.recurrence_count}x</span>
                        <span>Merged {item.merged_finding_count}x</span>
                        <span>Source run {item.run_id ? item.run_id.slice(0, 8) : '-'}</span>
                      </div>
                      <div className="flex flex-wrap gap-2 mt-3">
                        <button
                          onClick={() => beginBacklogEdit(item)}
                          disabled={savingBacklogId === item.id}
                          className="h-9 px-3 rounded-lg border border-white/10 bg-white/5 text-xs font-semibold disabled:opacity-50"
                        >
                          Edit fields
                        </button>
                        {editingBacklogId === item.id && (
                          <>
                            <button
                              onClick={() => void saveBacklogEdit(item)}
                              disabled={savingBacklogId === item.id || !backlogDraft}
                              className="h-9 px-3 rounded-lg bg-nofx-gold text-black text-xs font-semibold disabled:opacity-50"
                            >
                              {savingBacklogId === item.id ? 'Saving…' : 'Save edit'}
                            </button>
                            <button
                              onClick={cancelBacklogEdit}
                              disabled={savingBacklogId === item.id}
                              className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-xs font-semibold disabled:opacity-50"
                            >
                              Cancel
                            </button>
                          </>
                        )}
                      </div>
                    </div>

                    <div className="w-full lg:w-48">
                      <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center">
                        <NofxSelect
                          value={item.status}
                          onChange={(value) =>
                            void updateBacklogStatus(item, value)
                          }
                          disabled={savingBacklogId === item.id}
                          options={backlogStatusOptions.filter(
                            (option) => option.value
                          )}
                        />
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-2">
                        Updated {normalizeTime(item.updated_at)}
                      </div>
                    </div>
                  </div>

                  {editingBacklogId === item.id && backlogDraft && (
                    <div className="mt-4 grid grid-cols-1 xl:grid-cols-2 gap-3">
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
                        <div className="text-xs text-nofx-text-muted mb-2">Title</div>
                        <input
                          value={backlogDraft.title}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? { ...current, title: event.target.value }
                                : current
                            )
                          }
                          className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                        />
                      </label>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted mb-2">Category</div>
                        <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center">
                          <NofxSelect
                            value={backlogDraft.category}
                            onChange={(value) =>
                              setBacklogDraft((current) =>
                                current ? { ...current, category: value } : current
                              )
                            }
                            options={backlogCategoryOptions}
                          />
                        </div>
                      </div>
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm xl:col-span-2">
                        <div className="text-xs text-nofx-text-muted mb-2">Description</div>
                        <textarea
                          value={backlogDraft.description}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? { ...current, description: event.target.value }
                                : current
                            )
                          }
                          rows={4}
                          className="w-full rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm"
                        />
                      </label>
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm xl:col-span-2">
                        <div className="text-xs text-nofx-text-muted mb-2">Expected impact</div>
                        <textarea
                          value={backlogDraft.expectedImpact}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? { ...current, expectedImpact: event.target.value }
                                : current
                            )
                          }
                          rows={3}
                          className="w-full rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm"
                        />
                      </label>
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
                        <div className="text-xs text-nofx-text-muted mb-2">Confidence</div>
                        <input
                          value={backlogDraft.confidence}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? { ...current, confidence: event.target.value }
                                : current
                            )
                          }
                          className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                        />
                      </label>
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
                        <div className="text-xs text-nofx-text-muted mb-2">Implementation cost</div>
                        <input
                          value={backlogDraft.implementationCost}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? { ...current, implementationCost: event.target.value }
                                : current
                            )
                          }
                          className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                        />
                      </label>
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
                        <div className="text-xs text-nofx-text-muted mb-2">Urgency</div>
                        <input
                          value={backlogDraft.urgency}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? { ...current, urgency: event.target.value }
                                : current
                            )
                          }
                          className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                        />
                      </label>
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
                        <div className="text-xs text-nofx-text-muted mb-2">Recurrence count</div>
                        <input
                          value={backlogDraft.recurrenceCount}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? { ...current, recurrenceCount: event.target.value }
                                : current
                            )
                          }
                          className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                        />
                      </label>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted mb-2">Status</div>
                        <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center">
                          <NofxSelect
                            value={backlogDraft.status}
                            onChange={(value) =>
                              setBacklogDraft((current) =>
                                current ? { ...current, status: value } : current
                              )
                            }
                            options={backlogStatusOptions.filter((option) => option.value)}
                          />
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              ))
            )}
          </div>
        </div>
      </div>

      <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
        <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-3">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
              Model Outcomes
            </div>
            <div className="text-sm text-nofx-text-muted mt-2">
              This ranks proposer/critic pairs by actual optimizer outcomes:
              apply rate, rollback rate, kept-win rate, and backlog usefulness.
            </div>
          </div>
        </div>
        {modelOutcomes.length === 0 ? (
          <div className="rounded-lg border border-dashed border-white/10 bg-black/10 p-4 text-sm text-nofx-text-muted">
            No per-model optimizer outcome data is available yet.
          </div>
        ) : (
          <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
            {modelOutcomes.map((item) => {
              const isActive = activeModelKey && item.model_key === activeModelKey
              return (
                <div
                  key={item.model_key}
                  className={`rounded-xl border p-4 ${
                    isActive
                      ? 'border-nofx-gold/40 bg-nofx-gold/10'
                      : 'border-white/10 bg-black/20'
                  }`}
                >
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <div className="font-semibold">{item.model_label}</div>
                        {isActive && (
                          <span className="inline-flex px-2.5 py-1 rounded-full border border-nofx-gold/30 bg-nofx-gold/10 text-[11px] text-nofx-gold">
                            Active pair
                          </span>
                        )}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-2">
                        Last used {normalizeTime(item.last_used_at)}
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="text-xs text-nofx-text-muted">Outcome score</div>
                      <div className="text-lg font-semibold mt-1">
                        {formatScore(item.outcome_score)}
                      </div>
                    </div>
                  </div>

                  <div className="grid grid-cols-2 xl:grid-cols-4 gap-3 mt-4">
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">Apply rate</div>
                      <div className="text-base font-semibold mt-1">
                        {formatPct(item.apply_rate)}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {item.apply_count} of {item.total_runs} runs
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">Rollback rate</div>
                      <div className="text-base font-semibold mt-1">
                        {formatPct(item.rollback_rate)}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {item.rollback_count} rollback / {item.apply_count} applies
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">Kept-win rate</div>
                      <div className="text-base font-semibold mt-1">
                        {formatPct(item.kept_win_rate)}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {item.kept_win_count} positive kept / {item.kept_count} kept
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">Backlog usefulness</div>
                      <div className="text-base font-semibold mt-1">
                        {formatPct(item.backlog_usefulness)}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {item.useful_backlog_count} useful / {item.backlog_item_count} items
                      </div>
                    </div>
                  </div>

                  <div className="grid grid-cols-2 xl:grid-cols-4 gap-3 mt-3 text-xs text-nofx-text-muted">
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      Monitoring applies: {item.monitoring_count}
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      Done backlog items: {item.done_backlog_count}
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      Rejected backlog items: {item.rejected_backlog_count}
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      Blocked runs: {item.status_counts?.blocked_by_gate || 0}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
              Run History
            </div>
            <div className="text-sm text-nofx-text-muted mt-2">
              This shows what the optimizer actually did every window: applied,
              blocked, backlog-only, monitoring, rollback, or no change.
            </div>
          </div>
          <div className="w-full md:w-56 h-10 rounded-lg border border-white/10 px-3 flex items-center">
            <NofxSelect
              value={runStatusFilter}
              onChange={setRunStatusFilter}
              options={optimizerStatusOptions}
            />
          </div>
        </div>

        <div className="space-y-3">
          {filteredRuns.length === 0 ? (
            <div className="rounded-lg border border-dashed border-white/10 bg-black/10 p-4 text-sm text-nofx-text-muted">
              No optimizer runs visible for this filter yet.
            </div>
          ) : (
            filteredRuns.map((run) => (
              <div
                key={run.id}
                onClick={() => void loadRunDetail(run.id)}
                className={`rounded-xl border bg-black/20 p-4 cursor-pointer transition ${
                  selectedRunId === run.id
                    ? 'border-nofx-gold/40'
                    : 'border-white/10 hover:border-white/20'
                }`}
              >
                <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span
                        className={`inline-flex px-2.5 py-1 rounded-full border text-xs font-semibold ${statusToneClasses(run.status)}`}
                      >
                        {formatLabel(run.status)}
                      </span>
                      <span className="inline-flex px-2.5 py-1 rounded-full border border-white/10 bg-white/5 text-xs text-nofx-text-muted">
                        {formatLabel(run.trigger)}
                      </span>
                      <span className="inline-flex px-2.5 py-1 rounded-full border border-white/10 bg-white/5 text-xs text-nofx-text-muted">
                        {run.primary_model_name || '-'} /{' '}
                        {run.critic_model_name || '-'}
                      </span>
                    </div>
                    <div className="font-semibold mt-3">
                      {run.summary || 'No summary stored'}
                    </div>
                    <div className="flex flex-wrap gap-4 text-xs text-nofx-text-muted mt-3">
                      <span>Started {normalizeTime(run.started_at)}</span>
                      <span>Finished {normalizeTime(run.completed_at)}</span>
                      <span>Updated {normalizeTime(run.updated_at)}</span>
                      {run.applied_strategy_version_id && (
                        <span>
                          Strategy version {run.applied_strategy_version_id.slice(0, 8)}
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="text-xs text-nofx-text-muted">
                    Run ID {run.id.slice(0, 8)}
                  </div>
                </div>
              </div>
            ))
          )}
        </div>

        {(runDetailLoading || selectedRunDetail) && (
          <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
            <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-3">
              <div>
                <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                  Run Detail
                </div>
                <div className="text-lg font-semibold mt-2">
                  {runDetailLoading
                    ? 'Loading run detail…'
                    : selectedRunDetail?.run.summary || 'No summary stored'}
                </div>
                {!runDetailLoading && (
                  <div className="flex flex-wrap gap-2 mt-3">
                    <span
                      className={`inline-flex px-2.5 py-1 rounded-full border text-xs font-semibold ${statusToneClasses(selectedRunDetail?.run.status)}`}
                    >
                      {formatLabel(selectedRunDetail?.run.status)}
                    </span>
                    <span className="inline-flex px-2.5 py-1 rounded-full border border-white/10 bg-white/5 text-xs text-nofx-text-muted">
                      {formatLabel(selectedRunDetail?.run.trigger)}
                    </span>
                    {proposalType && (
                      <span className="inline-flex px-2.5 py-1 rounded-full border border-sky-400/20 bg-sky-500/10 text-xs text-sky-300">
                        {formatLabel(proposalType)}
                      </span>
                    )}
                  </div>
                )}
              </div>

              {!runDetailLoading && (
                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={openLinkedStrategyVersion}
                    disabled={!selectedRunDetail?.run.applied_strategy_version_id}
                    className="h-10 px-3 rounded-lg border border-nofx-gold/30 text-nofx-gold disabled:opacity-50"
                  >
                    Open strategy version
                  </button>
                  <button
                    onClick={applyRunWindowFilter}
                    disabled={
                      !selectedRunDetail?.review_window_start_ms ||
                      !selectedRunDetail?.review_window_end_ms
                    }
                    className="h-10 px-3 rounded-lg border border-sky-400/30 text-sky-300 disabled:opacity-50"
                  >
                    Open review cohort
                  </button>
                </div>
              )}
            </div>

            {!runDetailLoading && selectedRunDetail && (
              <>
                <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3 text-sm">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                    <div className="text-xs text-nofx-text-muted">Started</div>
                    <div className="mt-1">{normalizeTime(selectedRunDetail.run.started_at)}</div>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                    <div className="text-xs text-nofx-text-muted">Finished</div>
                    <div className="mt-1">{normalizeTime(selectedRunDetail.run.completed_at)}</div>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                    <div className="text-xs text-nofx-text-muted">Window stats</div>
                    <div className="mt-1">
                      {reviewWindowClosedDeals ?? '-'} deals | {reviewWindowCycles ?? '-'} cycles | {reviewWindowCandidates ?? '-'} candidates
                    </div>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                    <div className="text-xs text-nofx-text-muted">Window PnL</div>
                    <div className="mt-1">{formatNumber(reviewWindowPnL)}</div>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                    <div className="text-xs text-nofx-text-muted">Next eligible apply</div>
                    <div className="mt-1">
                      {typeof nextEligibleRunMs === 'number'
                        ? normalizeTime(new Date(nextEligibleRunMs).toISOString())
                        : '-'}
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {typeof cooldownUntilMs === 'number'
                        ? `Cooldown until ${normalizeTime(
                            new Date(cooldownUntilMs).toISOString()
                          )}`
                        : 'No cooldown hold stored'}
                    </div>
                  </div>
                </div>

                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    Monitoring / Rollback Analysis
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 text-sm">
                    <div>
                      <div className="text-xs text-nofx-text-muted">Root apply run</div>
                      <div className="mt-1 break-all">
                        {monitoringRootRunId || '-'}
                      </div>
                    </div>
                    <div>
                      <div className="text-xs text-nofx-text-muted">Observed windows</div>
                      <div className="mt-1 font-semibold">
                        {typeof monitoringWindowsObserved === 'number'
                          ? Math.round(monitoringWindowsObserved + 1)
                          : '-'}
                      </div>
                    </div>
                    <div>
                      <div className="text-xs text-nofx-text-muted">
                        Cumulative monitored deals / PnL
                      </div>
                      <div className="mt-1 font-semibold">
                        {typeof monitoringObservedClosedDeals === 'number'
                          ? Math.round(monitoringObservedClosedDeals)
                          : '-'}{' '}
                        /{' '}
                        {typeof monitoringObservedNetPnL === 'number'
                          ? formatNumber(monitoringObservedNetPnL)
                          : '-'}
                      </div>
                    </div>
                    <div>
                      <div className="text-xs text-nofx-text-muted">
                        Negative windows
                      </div>
                      <div className="mt-1 font-semibold">
                        {typeof monitoringObservedNegativeWindows === 'number'
                          ? Math.round(monitoringObservedNegativeWindows)
                          : '-'}
                      </div>
                    </div>
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-sm mt-4">
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">Baseline net PnL</div>
                      <div className="mt-1 font-semibold">
                        {typeof sourceBaselineNetPnL === 'number'
                          ? formatNumber(sourceBaselineNetPnL)
                          : '-'}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">Baseline win rate</div>
                      <div className="mt-1 font-semibold">
                        {typeof sourceBaselineWinRate === 'number'
                          ? `${formatNumber(sourceBaselineWinRate)}%`
                          : '-'}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        Baseline exit efficiency
                      </div>
                      <div className="mt-1 font-semibold">
                        {typeof sourceBaselineExitEfficiency === 'number'
                          ? formatNumber(sourceBaselineExitEfficiency)
                          : '-'}
                      </div>
                    </div>
                  </div>
                  {rollbackTriggerCodes.length > 0 && (
                    <div className="flex flex-wrap gap-2 mt-4">
                      {rollbackTriggerCodes.map((item) => (
                        <span
                          key={item}
                          className="inline-flex px-2 py-1 rounded-full border border-rose-400/20 bg-rose-500/10 text-[11px] text-rose-300"
                        >
                          {formatLabel(item)}
                        </span>
                      ))}
                    </div>
                  )}
                  {rollbackTriggerReasons.length > 0 ? (
                    <div className="space-y-2 mt-4">
                      {rollbackTriggerReasons.map((item) => (
                        <div
                          key={item}
                          className="rounded-lg border border-rose-400/15 bg-rose-500/5 p-3 text-sm text-rose-200"
                        >
                          {item}
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="text-sm text-nofx-text-muted mt-4">
                      No rollback trigger fired for this run.
                    </div>
                  )}
                </div>

                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Low-Trade Telemetry
                    </div>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                      <div>
                        <div className="text-xs text-nofx-text-muted">Open decisions</div>
                        <div className="mt-1 font-semibold">
                          {typeof openDecisionCount === 'number'
                            ? Math.round(openDecisionCount)
                            : '-'}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">Hold decisions</div>
                        <div className="mt-1 font-semibold">
                          {typeof holdDecisionCount === 'number'
                            ? Math.round(holdDecisionCount)
                            : '-'}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">Wait decisions</div>
                        <div className="mt-1 font-semibold">
                          {typeof waitDecisionCount === 'number'
                            ? Math.round(waitDecisionCount)
                            : '-'}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">Conversion</div>
                        <div className="mt-1 font-semibold">
                          {typeof decisionConversionRate === 'number'
                            ? `${formatNumber(decisionConversionRate)}%`
                            : '-'}
                        </div>
                      </div>
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-3">
                      Avg decision confidence:{' '}
                      {typeof avgDecisionConfidence === 'number'
                        ? formatNumber(avgDecisionConfidence)
                        : '-'}
                    </div>
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Recent Optimizer Context
                    </div>
                    {recentOptimizerContext.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        No previous optimizer runs were attached to this run.
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {recentOptimizerContext.slice(0, 4).map((item, index) => (
                          <div
                            key={`${String(item.id || index)}`}
                            className="rounded-lg border border-white/10 bg-black/20 p-3"
                          >
                            <div className="flex flex-wrap items-center gap-2">
                              <span
                                className={`inline-flex px-2 py-1 rounded-full border text-[11px] ${statusToneClasses(
                                  typeof item.status === 'string' ? item.status : ''
                                )}`}
                              >
                                {formatLabel(
                                  typeof item.status === 'string'
                                    ? item.status
                                    : 'unknown'
                                )}
                              </span>
                              <span className="text-xs text-nofx-text-muted">
                                {typeof item.primary_model_name === 'string'
                                  ? item.primary_model_name
                                  : '-'}{' '}
                                /{' '}
                                {typeof item.critic_model_name === 'string'
                                  ? item.critic_model_name
                                  : '-'}
                              </span>
                            </div>
                            <div className="text-sm mt-2">
                              {typeof item.summary === 'string'
                                ? item.summary
                                : 'No summary stored'}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Reject Reasons
                    </div>
                    {rejectReasons.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        No hold/wait reject reasons were stored in this window.
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {rejectReasons.map((item, index) => (
                          <div
                            key={`${String(item.reason || index)}`}
                            className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm"
                          >
                            <div className="font-semibold">
                              {formatLabel(
                                typeof item.reason === 'string'
                                  ? item.reason
                                  : 'unknown'
                              )}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {typeof item.count === 'number'
                                ? Math.round(item.count)
                                : '-'}{' '}
                              times
                              {typeof item.share_pct === 'number'
                                ? ` • ${formatNumber(item.share_pct)}%`
                                : ''}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Opportunity Sessions
                    </div>
                    {opportunitySessions.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        No session density was stored for this window.
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {opportunitySessions.map((item, index) => (
                          <div
                            key={`${String(item.session || index)}`}
                            className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm"
                          >
                            <div className="font-semibold">
                              {formatLabel(
                                typeof item.session === 'string'
                                  ? item.session
                                  : 'unknown'
                              )}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {typeof item.candidate_count === 'number'
                                ? Math.round(item.candidate_count)
                                : '-'}{' '}
                              candidates •{' '}
                              {typeof item.open_decision_count === 'number'
                                ? Math.round(item.open_decision_count)
                                : '-'}{' '}
                              opens •{' '}
                              {typeof item.hold_decision_count === 'number'
                                ? Math.round(item.hold_decision_count)
                                : '-'}{' '}
                              holds •{' '}
                              {typeof item.wait_decision_count === 'number'
                                ? Math.round(item.wait_decision_count)
                                : '-'}{' '}
                              waits
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Confidence Bands
                    </div>
                    {confidenceBands.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        No confidence-band telemetry was stored for this window.
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {confidenceBands.map((item, index) => (
                          <div
                            key={`${String(item.band || index)}`}
                            className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm"
                          >
                            <div className="font-semibold">
                              {typeof item.band === 'string' ? item.band : 'unknown'}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {typeof item.decision_count === 'number'
                                ? Math.round(item.decision_count)
                                : '-'}{' '}
                              decisions • avg{' '}
                              {typeof item.avg_confidence === 'number'
                                ? formatNumber(item.avg_confidence)
                                : '-'}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    Opportunity Symbols
                  </div>
                  {opportunitySymbols.length === 0 ? (
                    <div className="text-sm text-nofx-text-muted">
                      No symbol-level opportunity density was stored for this run.
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {opportunitySymbols.map((item, index) => (
                        <div
                          key={`${String(item.symbol || index)}`}
                          className="rounded-lg border border-white/10 bg-black/20 p-3"
                        >
                          <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-2">
                            <div>
                              <div className="font-semibold">
                                {typeof item.symbol === 'string'
                                  ? item.symbol
                                  : 'UNKNOWN'}
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                {typeof item.candidate_count === 'number'
                                  ? Math.round(item.candidate_count)
                                  : '-'}{' '}
                                candidates •{' '}
                                {typeof item.open_decision_count === 'number'
                                  ? Math.round(item.open_decision_count)
                                  : '-'}{' '}
                                opens •{' '}
                                {typeof item.hold_decision_count === 'number'
                                  ? Math.round(item.hold_decision_count)
                                  : '-'}{' '}
                                holds •{' '}
                                {typeof item.wait_decision_count === 'number'
                                  ? Math.round(item.wait_decision_count)
                                  : '-'}{' '}
                                waits
                              </div>
                            </div>
                            <div className="text-xs text-nofx-text-muted">
                              Avg confidence{' '}
                              {typeof item.avg_confidence === 'number'
                                ? formatNumber(item.avg_confidence)
                                : '-'}
                            </div>
                          </div>
                          <div className="flex flex-wrap gap-2 mt-2">
                            {readStringArray(item.selection_buckets).map((bucket) => (
                              <span
                                key={`${String(item.symbol)}-${bucket}`}
                                className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted"
                              >
                                {formatLabel(bucket)}
                              </span>
                            ))}
                            {readStringArray(item.sessions).map((session) => (
                              <span
                                key={`${String(item.symbol)}-session-${session}`}
                                className="inline-flex px-2 py-1 rounded-full border border-sky-400/20 bg-sky-500/10 text-[11px] text-sky-300"
                              >
                                {formatLabel(session)}
                              </span>
                            ))}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {(criticSummary ||
                  proposalExpectedEffect ||
                  (selectedRunDetail.gate_reasons &&
                    selectedRunDetail.gate_reasons.length > 0) ||
                  configValidationStatus) && (
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Gate And Critic
                    </div>
                    <div className="grid grid-cols-1 xl:grid-cols-2 gap-4 text-sm">
                      <div className="space-y-2">
                        <div className="text-nofx-text-muted">Critic summary</div>
                        <div>{criticSummary || '-'}</div>
                        <div className="text-nofx-text-muted pt-2">
                          Expected effect
                        </div>
                        <div>{proposalExpectedEffect || '-'}</div>
                        <div className="text-nofx-text-muted pt-2">
                          Config validation
                        </div>
                        <div>{formatLabel(configValidationStatus) || '-'}</div>
                        <div className="text-nofx-text-muted pt-2">
                          Prompt changed fields
                        </div>
                        <div>
                          {typeof promptValidationIssues === 'number'
                            ? Math.round(promptValidationIssues)
                            : '-'}
                        </div>
                      </div>
                      <div>
                        <div className="text-nofx-text-muted mb-2">
                          Gate reasons
                        </div>
                        {!selectedRunDetail.gate_reasons ||
                        selectedRunDetail.gate_reasons.length === 0 ? (
                          <div className="text-sm text-emerald-300">
                            No blocking or explanatory gate reasons stored.
                          </div>
                        ) : (
                          <div className="space-y-2">
                            {selectedRunDetail.gate_reasons.map((item) => (
                              <div
                                key={item}
                                className="rounded-lg border border-amber-400/20 bg-amber-500/10 p-2 text-amber-200"
                              >
                                {item}
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  </div>
                )}

                <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Strategy Diff
                    </div>
                    {renderDiffList(selectedRunDetail.strategy_differences)}
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Trader Prompt Diff
                    </div>
                    {renderDiffList(selectedRunDetail.trader_differences)}
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Optimizer Prompt Diff
                    </div>
                    {renderDiffList(selectedRunDetail.optimizer_differences)}
                  </div>
                </div>

                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Proposed Config Patch
                    </div>
                    <pre className="text-xs whitespace-pre-wrap break-words text-nofx-text-muted">
                      {prettyJSON(selectedRunDetail.config_patch)}
                    </pre>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Proposed Prompt Patch
                    </div>
                    <pre className="text-xs whitespace-pre-wrap break-words text-nofx-text-muted">
                      {prettyJSON(selectedRunDetail.prompt_patch)}
                    </pre>
                  </div>
                </div>

                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Validation Payload
                    </div>
                    <pre className="text-xs whitespace-pre-wrap break-words text-nofx-text-muted">
                      {prettyJSON(selectedRunDetail.validation)}
                    </pre>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      Metadata Payload
                    </div>
                    <pre className="text-xs whitespace-pre-wrap break-words text-nofx-text-muted">
                      {prettyJSON(selectedRunDetail.metadata)}
                    </pre>
                  </div>
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
