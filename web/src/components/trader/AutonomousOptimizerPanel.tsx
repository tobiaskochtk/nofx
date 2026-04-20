import { useEffect, useMemo, useState } from 'react'
import { useLanguage } from '../../contexts/LanguageContext'
import { toDateTimeLocale } from '../../i18n/locale'
import { api } from '../../lib/api'
import { notify } from '../../lib/notify'
import { NofxSelect } from '../ui/select'
import type { Language } from '../../i18n/translations'
import type {
  AIModel,
  AutonomousOptimizerBacklogItem,
  AutonomousOptimizerConfig,
  AutonomousOptimizerModelOutcome,
  AutonomousOptimizerRun,
  AutonomousOptimizerRunDetail,
  DealReviewJSONDiffEntry,
  SemanticMemorySearchHit,
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

const optimizerTextTranslations: Record<
  string,
  Partial<Record<Language, string>>
> = {
  'Autonomous Optimizer': { de: 'Autonomer Optimierer' },
  'Self-improving trader loop': {
    de: 'Sich selbst verbessernde Trader-Schleife',
  },
  'Visible here are the AI improvement steps, the current live optimizer status, and the recent automatic review outcomes.':
    {
      de: 'Hier siehst du die KI-Verbesserungsschritte, den aktuellen Live-Status des Optimierers und die juengsten automatischen Review-Ergebnisse.',
    },
  'Running…': { de: 'Laeuft…' },
  'Run now': { de: 'Jetzt ausfuehren' },
  'Refreshing…': { de: 'Aktualisiere…' },
  'Refresh optimizer': { de: 'Optimierer aktualisieren' },
  'Current state': { de: 'Aktueller Status' },
  Enabled: { de: 'Aktiviert' },
  Yes: { de: 'Ja' },
  No: { de: 'Nein' },
  'Next review window': { de: 'Naechstes Review-Fenster' },
  'Last run': { de: 'Letzter Lauf' },
  'Open improvement steps': { de: 'Offene Verbesserungsschritte' },
  'Highest score': { de: 'Hoechster Score' },
  'Last applied change': { de: 'Zuletzt uebernommene Aenderung' },
  'No applied autonomous change yet': {
    de: 'Noch keine autonome Aenderung uebernommen',
  },
  Config: { de: 'Konfiguration' },
  'Choose the active optimizer account and model pair here. This is where `GPT-5.4` is currently wired by default.':
    {
      de: 'Waehle hier das aktive Optimizer-Konto und Modellpaar. Standardmaessig ist hier aktuell `GPT-5.4` verdrahtet.',
    },
  Status: { de: 'Status' },
  'Review interval hours': { de: 'Review-Intervall (Stunden)' },
  'Cooldown hours': { de: 'Cooldown (Stunden)' },
  'Max consecutive applies': {
    de: 'Max. aufeinanderfolgende Uebernahmen',
  },
  'Auto-apply config patches': {
    de: 'Config-Patches automatisch uebernehmen',
  },
  'Auto-apply prompt patches': {
    de: 'Prompt-Patches automatisch uebernehmen',
  },
  'Auto rollback': { de: 'Auto-Rollback' },
  'Self-pause': { de: 'Selbstpause' },
  'Proposer account': { de: 'Proposer-Konto' },
  'Current model name': { de: 'Aktueller Modellname' },
  'e.g. gpt-5.4': { de: 'z. B. gpt-5.4' },
  'Critic account': { de: 'Critic-Konto' },
  'Proposal instructions overlay': {
    de: 'Proposal-Anweisungs-Overlay',
  },
  'Optional extra instructions merged into the optimizer proposer prompt.':
    {
      de: 'Optionale Zusatzanweisungen, die in den Proposer-Prompt des Optimierers eingefuegt werden.',
    },
  'Critic instructions overlay': {
    de: 'Critic-Anweisungs-Overlay',
  },
  'Optional extra instructions merged into the optimizer critic prompt.': {
    de: 'Optionale Zusatzanweisungen, die in den Critic-Prompt des Optimierers eingefuegt werden.',
  },
  'Seed source trader': { de: 'Seed-Quell-Trader' },
  'Current seed strategy': { de: 'Aktuelle Seed-Strategie' },
  'Last applied version': { de: 'Zuletzt uebernommene Version' },
  Cooldown: { de: 'Cooldown' },
  'Max apply streak': { de: 'Max. Uebernahme-Serie' },
  'Saving…': { de: 'Speichere…' },
  'Save optimizer config': { de: 'Optimizer-Konfiguration speichern' },
  'Improvement Backlog': { de: 'Verbesserungs-Backlog' },
  'These are the AI-detected next improvement steps. If the AI says indicators, telemetry, or prompt changes are missing, they land here with a score.':
    {
      de: 'Das sind die von der KI erkannten naechsten Verbesserungsschritte. Wenn laut KI Indikatoren, Telemetrie oder Prompt-Aenderungen fehlen, landen sie hier mit einem Score.',
    },
  'No visible improvement steps yet for this filter. Once the optimizer proposes missing capabilities or next build items, they will appear here.':
    {
      de: 'Fuer diesen Filter sind noch keine Verbesserungsschritte sichtbar. Sobald der Optimierer fehlende Faehigkeiten oder naechste Build-Items vorschlaegt, erscheinen sie hier.',
    },
  AI: { de: 'KI' },
  'User edited': { de: 'Vom Nutzer bearbeitet' },
  'Expected impact:': { de: 'Erwarteter Effekt:' },
  Score: { de: 'Score' },
  Confidence: { de: 'Konfidenz' },
  Urgency: { de: 'Dringlichkeit' },
  Cost: { de: 'Kosten' },
  Recurrent: { de: 'Wiederkehrend' },
  Merged: { de: 'Zusammengefuehrt' },
  'Source run': { de: 'Quelllauf' },
  'Edit fields': { de: 'Felder bearbeiten' },
  'Save edit': { de: 'Bearbeitung speichern' },
  Cancel: { de: 'Abbrechen' },
  Updated: { de: 'Aktualisiert' },
  Title: { de: 'Titel' },
  Category: { de: 'Kategorie' },
  Description: { de: 'Beschreibung' },
  'Expected impact': { de: 'Erwarteter Effekt' },
  'Implementation cost': { de: 'Implementierungskosten' },
  'Recurrence count': { de: 'Wiederholungsanzahl' },
  'Model Outcomes': { de: 'Modell-Ergebnisse' },
  'This ranks proposer/critic pairs by actual optimizer outcomes: apply rate, rollback rate, kept-win rate, and backlog usefulness.':
    {
      de: 'Dies bewertet Proposer-/Critic-Paare nach realen Optimizer-Ergebnissen: Uebernahmerate, Rollback-Rate, Beibehalten-Gewinnrate und Nutzen des Backlogs.',
    },
  'No per-model optimizer outcome data is available yet.': {
    de: 'Noch keine optimizerbezogenen Ergebnisdaten pro Modell verfuegbar.',
  },
  'Active pair': { de: 'Aktives Paar' },
  'Last used': { de: 'Zuletzt verwendet' },
  'Outcome score': { de: 'Ergebnis-Score' },
  'Apply rate': { de: 'Uebernahmerate' },
  'Rollback rate': { de: 'Rollback-Rate' },
  'Kept-win rate': { de: 'Behalten-Gewinnrate' },
  'Backlog usefulness': { de: 'Backlog-Nutzen' },
  'Operational health': { de: 'Betriebszustand' },
  'Monitoring applies:': { de: 'Monitoring-Uebernahmen:' },
  'Failed runs:': { de: 'Fehlgeschlagene Laeufe:' },
  'Evidence-gap overlaps:': { de: 'Belegluecken-Ueberschneidungen:' },
  'Done backlog items:': { de: 'Erledigte Backlog-Eintraege:' },
  'Rejected backlog items:': { de: 'Abgelehnte Backlog-Eintraege:' },
  'Blocked runs:': { de: 'Blockierte Laeufe:' },
  'Run History': { de: 'Laufhistorie' },
  'This shows what the optimizer actually did every window: applied, blocked, backlog-only, monitoring, rollback, or no change.':
    {
      de: 'Das zeigt, was der Optimierer pro Fenster tatsaechlich getan hat: uebernommen, blockiert, nur Backlog, Monitoring, Rollback oder keine Aenderung.',
    },
  'No optimizer runs visible for this filter yet.': {
    de: 'Fuer diesen Filter sind noch keine Optimizer-Laeufe sichtbar.',
  },
  'No summary stored': { de: 'Keine Zusammenfassung gespeichert' },
  Started: { de: 'Gestartet' },
  Finished: { de: 'Beendet' },
  'Strategy version': { de: 'Strategieversion' },
  'Run ID': { de: 'Lauf-ID' },
  'Run Detail': { de: 'Laufdetails' },
  'Loading run detail…': { de: 'Laufdetails werden geladen…' },
  'Open strategy version': { de: 'Strategieversion oeffnen' },
  'Open review cohort': { de: 'Review-Kohorte oeffnen' },
  'Select model account': { de: 'Modellkonto waehlen' },
  'Failed to load autonomous optimizer panel': {
    de: 'Panel des autonomen Optimierers konnte nicht geladen werden',
  },
  'Failed to load autonomous optimizer run detail': {
    de: 'Laufdetails des autonomen Optimierers konnten nicht geladen werden',
  },
  'Autonomous optimizer run completed': {
    de: 'Lauf des autonomen Optimierers abgeschlossen',
  },
  'Failed to execute autonomous optimizer run': {
    de: 'Lauf des autonomen Optimierers konnte nicht ausgefuehrt werden',
  },
  'Failed to fetch similar autonomous optimizer runs': {
    de: 'Aehnliche Laeufe des autonomen Optimierers konnten nicht geladen werden',
  },
  'Failed to update autonomous optimizer backlog item': {
    de: 'Backlog-Eintrag des autonomen Optimierers konnte nicht aktualisiert werden',
  },
  'Backlog item updated': { de: 'Backlog-Eintrag aktualisiert' },
  'Failed to save autonomous optimizer backlog item': {
    de: 'Backlog-Eintrag des autonomen Optimierers konnte nicht gespeichert werden',
  },
  'Autonomous optimizer config saved': {
    de: 'Konfiguration des autonomen Optimierers gespeichert',
  },
  'Failed to save autonomous optimizer config': {
    de: 'Konfiguration des autonomen Optimierers konnte nicht gespeichert werden',
  },
  'No linked strategy version is stored for this run': {
    de: 'Fuer diesen Lauf ist keine verknuepfte Strategieversion gespeichert',
  },
  'No stored review window is available for this run': {
    de: 'Fuer diesen Lauf ist kein gespeichertes Review-Fenster verfuegbar',
  },
  'All run statuses': { de: 'Alle Laufstatus' },
  'All backlog statuses': { de: 'Alle Backlog-Status' },
}

const optimizerValueTranslations: Record<
  string,
  Partial<Record<Language, string>>
> = {
  scheduled: { de: 'Geplant' },
  running: { de: 'Laeuft' },
  insufficient_evidence: { de: 'Zu wenig Belege' },
  no_change: { de: 'Keine Aenderung' },
  backlog_only: { de: 'Nur Backlog' },
  blocked_by_gate: { de: 'Durch Gate blockiert' },
  deferred_for_next_window: { de: 'Auf naechstes Fenster verschoben' },
  auto_applied: { de: 'Automatisch uebernommen' },
  monitoring: { de: 'Monitoring' },
  rollback_pending: { de: 'Rollback ausstehend' },
  rolled_back: { de: 'Zurueckgerollt' },
  kept: { de: 'Beibehalten' },
  paused: { de: 'Pausiert' },
  failed: { de: 'Fehlgeschlagen' },
  none: { de: 'Keiner' },
  new: { de: 'Neu' },
  confirmed: { de: 'Bestaetigt' },
  planned: { de: 'Geplant' },
  in_progress: { de: 'In Arbeit' },
  done: { de: 'Erledigt' },
  rejected: { de: 'Abgelehnt' },
  missing_indicator: { de: 'Fehlender Indikator' },
  missing_market_data: { de: 'Fehlende Marktdaten' },
  missing_execution_telemetry: { de: 'Fehlende Ausfuehrungs-Telemetrie' },
  missing_regime_metadata: { de: 'Fehlende Regime-Metadaten' },
  missing_risk_control: { de: 'Fehlende Risikosteuerung' },
  missing_prompt_instruction: { de: 'Fehlende Prompt-Anweisung' },
  missing_review_metric: { de: 'Fehlende Review-Metrik' },
  other_capability_gap: { de: 'Sonstige Faehigkeitsluecke' },
  submitted: { de: 'Eingereicht' },
  blocked_by_risk_control: { de: 'Durch Risikosteuerung blockiert' },
  blocked_by_position_state: { de: 'Durch Positionsstatus blockiert' },
  failed_exchange_validation: { de: 'Boersenvalidierung fehlgeschlagen' },
  rejected_or_canceled: { de: 'Abgelehnt oder abgebrochen' },
  submit_failed: { de: 'Einreichen fehlgeschlagen' },
  never_handed_to_execution: { de: 'Nie an Ausfuehrung uebergeben' },
  review: { de: 'Review' },
  unknown: { de: 'Unbekannt' },
  conversation: { de: 'Konversation' },
  review_hint: { de: 'Review-Hinweis' },
  prompt_hint: { de: 'Prompt-Hinweis' },
  config_candidate: { de: 'Konfig-Kandidat' },
  monitoring_rule: { de: 'Monitoring-Regel' },
  monitor_only: { de: 'Nur beobachten' },
  open: { de: 'Offen' },
  accepted: { de: 'Akzeptiert' },
  overridden: { de: 'Ueberschrieben' },
  manual_action: { de: 'Manuelle Aktion' },
  suggested: { de: 'Vorgeschlagen' },
}

function optimizerText(language: Language, value: string): string {
  return optimizerTextTranslations[value]?.[language] || value
}

function optimizerValueText(language: Language, value: string): string {
  return (
    optimizerValueTranslations[value]?.[language] ||
    value
      .replace(/_/g, ' ')
      .replace(/\b\w/g, (match) => match.toUpperCase())
  )
}

function createOptimizerStatusOptions(language: Language) {
  return [
    { value: '', label: optimizerText(language, 'All run statuses') },
    { value: 'scheduled', label: optimizerValueText(language, 'scheduled') },
    { value: 'running', label: optimizerValueText(language, 'running') },
    {
      value: 'insufficient_evidence',
      label: optimizerValueText(language, 'insufficient_evidence'),
    },
    { value: 'no_change', label: optimizerValueText(language, 'no_change') },
    {
      value: 'backlog_only',
      label: optimizerValueText(language, 'backlog_only'),
    },
    {
      value: 'blocked_by_gate',
      label: optimizerValueText(language, 'blocked_by_gate'),
    },
    {
      value: 'deferred_for_next_window',
      label: optimizerValueText(language, 'deferred_for_next_window'),
    },
    {
      value: 'auto_applied',
      label: optimizerValueText(language, 'auto_applied'),
    },
    { value: 'monitoring', label: optimizerValueText(language, 'monitoring') },
    {
      value: 'rollback_pending',
      label: optimizerValueText(language, 'rollback_pending'),
    },
    {
      value: 'rolled_back',
      label: optimizerValueText(language, 'rolled_back'),
    },
    { value: 'kept', label: optimizerValueText(language, 'kept') },
    { value: 'paused', label: optimizerValueText(language, 'paused') },
    { value: 'failed', label: optimizerValueText(language, 'failed') },
  ]
}

function createConfigStatusOptions(language: Language) {
  return [
    { value: 'scheduled', label: optimizerValueText(language, 'scheduled') },
    { value: 'paused', label: optimizerValueText(language, 'paused') },
  ]
}

function createBacklogStatusOptions(language: Language) {
  return [
    { value: '', label: optimizerText(language, 'All backlog statuses') },
    { value: 'new', label: optimizerValueText(language, 'new') },
    { value: 'confirmed', label: optimizerValueText(language, 'confirmed') },
    { value: 'planned', label: optimizerValueText(language, 'planned') },
    {
      value: 'in_progress',
      label: optimizerValueText(language, 'in_progress'),
    },
    { value: 'done', label: optimizerValueText(language, 'done') },
    { value: 'rejected', label: optimizerValueText(language, 'rejected') },
  ]
}

function createBacklogCategoryOptions(language: Language) {
  return [
    {
      value: 'missing_indicator',
      label: optimizerValueText(language, 'missing_indicator'),
    },
    {
      value: 'missing_market_data',
      label: optimizerValueText(language, 'missing_market_data'),
    },
    {
      value: 'missing_execution_telemetry',
      label: optimizerValueText(language, 'missing_execution_telemetry'),
    },
    {
      value: 'missing_regime_metadata',
      label: optimizerValueText(language, 'missing_regime_metadata'),
    },
    {
      value: 'missing_risk_control',
      label: optimizerValueText(language, 'missing_risk_control'),
    },
    {
      value: 'missing_prompt_instruction',
      label: optimizerValueText(language, 'missing_prompt_instruction'),
    },
    {
      value: 'missing_review_metric',
      label: optimizerValueText(language, 'missing_review_metric'),
    },
    {
      value: 'other_capability_gap',
      label: optimizerValueText(language, 'other_capability_gap'),
    },
  ]
}

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

function formatScore(value?: number): string {
  const safe = typeof value === 'number' && Number.isFinite(value) ? value : 0
  return `${Math.round(safe)}/100`
}

function statusToneClasses(status?: string): string {
  switch (status) {
    case 'auto_applied':
    case 'kept':
    case 'done':
    case 'submitted':
      return 'border-emerald-400/25 bg-emerald-500/15 text-emerald-300'
    case 'blocked_by_gate':
    case 'deferred_for_next_window':
    case 'insufficient_evidence':
    case 'backlog_only':
    case 'monitoring':
    case 'rollback_pending':
    case 'planned':
    case 'in_progress':
    case 'blocked_by_risk_control':
    case 'blocked_by_position_state':
      return 'border-amber-400/25 bg-amber-500/15 text-amber-200'
    case 'failed':
    case 'rolled_back':
    case 'rejected':
    case 'failed_exchange_validation':
    case 'rejected_or_canceled':
    case 'submit_failed':
    case 'never_handed_to_execution':
      return 'border-rose-400/25 bg-rose-500/15 text-rose-300'
    default:
      return 'border-white/10 bg-white/5 text-nofx-text-muted'
  }
}

function actionHintToneClasses(action?: string, priority?: string): string {
  if (priority === 'critical' || action === 'retire') {
    return 'border-rose-400/25 bg-rose-500/15 text-rose-300'
  }
  if (priority === 'warning' || action === 'suppress') {
    return 'border-amber-400/25 bg-amber-500/15 text-amber-200'
  }
  if (action === 'rearm' || action === 'acknowledge_live') {
    return 'border-sky-400/25 bg-sky-500/15 text-sky-200'
  }
  return 'border-white/10 bg-white/5 text-nofx-text-muted'
}

function modelLabel(
  model: AIModel | undefined,
  language: Language = 'en'
): string {
  if (!model) return optimizerText(language, 'Select model account')
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

function buildBacklogEditDraft(
  item: AutonomousOptimizerBacklogItem
): BacklogEditDraft {
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

function readNestedBoolean(
  body: Record<string, unknown> | undefined,
  ...path: string[]
): boolean | undefined {
  let current: unknown = body
  for (const key of path) {
    if (!current || typeof current !== 'object') return undefined
    current = (current as Record<string, unknown>)[key]
  }
  return typeof current === 'boolean' ? current : undefined
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

function readNestedObject(
  body: Record<string, unknown> | undefined,
  ...path: string[]
): Record<string, unknown> | undefined {
  let current: unknown = body
  for (const key of path) {
    if (!current || typeof current !== 'object') return undefined
    current = (current as Record<string, unknown>)[key]
  }
  if (!current || typeof current !== 'object' || Array.isArray(current)) {
    return undefined
  }
  return current as Record<string, unknown>
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

function renderDiffList(
  diffs: DealReviewJSONDiffEntry[] | undefined,
  language: Language = 'en'
) {
  if (!diffs || diffs.length === 0) {
    return (
      <div className="text-sm text-nofx-text-muted">
        {language === 'de'
          ? 'Fuer diesen Lauf ist kein Vorher/Nachher-Diff verfuegbar.'
          : 'No before/after diff is available for this run.'}
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
              <div className="text-nofx-text-muted mb-1">
                {language === 'de' ? 'Vorher' : 'Before'}
              </div>
              <pre className="whitespace-pre-wrap break-words text-rose-200">
                {entry.left}
              </pre>
            </div>
            <div>
              <div className="text-nofx-text-muted mb-1">
                {language === 'de' ? 'Nachher' : 'After'}
              </div>
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
  const { language } = useLanguage()
  const ot = (value: string) => optimizerText(language, value)
  const pickText = (en: string, de: string) => (language === 'de' ? de : en)
  const formatLabel = (value: string | undefined, lang: Language = language) => {
    if (!value) return '-'
    return optimizerValueText(lang, value)
  }
  const normalizeTime = (
    value: string | undefined,
    lang: Language = language
  ) => {
    if (!value || value.startsWith('0001-01-01')) return '-'
    return new Date(value).toLocaleString(toDateTimeLocale(lang))
  }
  const formatRelativeTime = (
    value: string | undefined,
    lang: Language = language
  ) => {
    if (!value || value.startsWith('0001-01-01')) return '-'
    const deltaMs = new Date(value).getTime() - Date.now()
    if (!Number.isFinite(deltaMs)) return '-'
    const mins = Math.round(deltaMs / 60000)
    if (Math.abs(mins) < 60) {
      return mins >= 0
        ? lang === 'de'
          ? `in ${mins} Min.`
          : `in ${mins}m`
        : lang === 'de'
          ? `vor ${Math.abs(mins)} Min.`
          : `${Math.abs(mins)}m ago`
    }
    const hours = Math.round(mins / 60)
    if (Math.abs(hours) < 48) {
      return hours >= 0
        ? lang === 'de'
          ? `in ${hours} Std.`
          : `in ${hours}h`
        : lang === 'de'
          ? `vor ${Math.abs(hours)} Std.`
          : `${Math.abs(hours)}h ago`
    }
    const days = Math.round(hours / 24)
    return days >= 0
      ? lang === 'de'
        ? `in ${days} Tg.`
        : `in ${days}d`
      : lang === 'de'
        ? `vor ${Math.abs(days)} Tg.`
        : `${Math.abs(days)}d ago`
  }
  const formatSemanticSimilarityScore = (
    value: number | undefined,
    lang: Language = language
  ) =>
    typeof value !== 'number' || Number.isNaN(value)
      ? '-'
      : lang === 'de'
        ? `${Math.round(Math.max(0, Math.min(1, value)) * 100)}% Treffer`
        : `${Math.round(Math.max(0, Math.min(1, value)) * 100)}% match`
  const formatCooldownOutcomeSummary = (
    summary: Record<string, unknown> | undefined,
    lang: Language = language
  ) => {
    const trades = readNestedNumber(summary, 'trade_count')
    const wins = readNestedNumber(summary, 'win_count')
    const losses = readNestedNumber(summary, 'loss_count')
    const avgPnL = readNestedNumber(summary, 'avg_pnl_pct')
    const netPnL = readNestedNumber(summary, 'net_pnl_pct')
    if (typeof trades !== 'number') {
      return lang === 'de' ? 'Keine Ergebnisse' : 'No outcomes'
    }
    return lang === 'de'
      ? `${Math.round(trades)} Trades • ${Math.round(wins || 0)}/${Math.round(losses || 0)} S/N • Ø ${formatNumber(avgPnL)}% • netto ${formatNumber(netPnL)}%`
      : `${Math.round(trades)} trades • ${Math.round(wins || 0)}/${Math.round(losses || 0)} W/L • avg ${formatNumber(avgPnL)}% • net ${formatNumber(netPnL)}%`
  }
  const optimizerStatusOptions = useMemo(
    () => createOptimizerStatusOptions(language),
    [language]
  )
  const configStatusOptions = useMemo(
    () => createConfigStatusOptions(language),
    [language]
  )
  const backlogStatusOptions = useMemo(
    () => createBacklogStatusOptions(language),
    [language]
  )
  const backlogCategoryOptions = useMemo(
    () => createBacklogCategoryOptions(language),
    [language]
  )
  const [config, setConfig] = useState<AutonomousOptimizerConfig | null>(null)
  const [runs, setRuns] = useState<AutonomousOptimizerRun[]>([])
  const [backlog, setBacklog] = useState<AutonomousOptimizerBacklogItem[]>([])
  const [modelOutcomes, setModelOutcomes] = useState<
    AutonomousOptimizerModelOutcome[]
  >([])
  const [models, setModels] = useState<AIModel[]>([])
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null)
  const [selectedRunDetail, setSelectedRunDetail] =
    useState<AutonomousOptimizerRunDetail | null>(null)
  const [similarRuns, setSimilarRuns] = useState<SemanticMemorySearchHit[]>([])
  const [similarRunsLoading, setSimilarRunsLoading] = useState(false)
  const [similarRunsError, setSimilarRunsError] = useState<string | null>(null)
  const [runDetailLoading, setRunDetailLoading] = useState(false)
  const [manualRunLoading, setManualRunLoading] = useState(false)
  const [loading, setLoading] = useState(false)
  const [savingConfig, setSavingConfig] = useState(false)
  const [savingBacklogId, setSavingBacklogId] = useState<string | null>(null)
  const [editingBacklogId, setEditingBacklogId] = useState<string | null>(null)
  const [backlogDraft, setBacklogDraft] = useState<BacklogEditDraft | null>(
    null
  )
  const [error, setError] = useState<string | null>(null)
  const [runStatusFilter, setRunStatusFilter] = useState('')
  const [backlogStatusFilter, setBacklogStatusFilter] = useState('')
  const [primaryRemoteModels, setPrimaryRemoteModels] = useState<
    RemoteModelInfo[]
  >([])
  const [criticRemoteModels, setCriticRemoteModels] = useState<
    RemoteModelInfo[]
  >([])

  const [enabled, setEnabled] = useState(false)
  const [configStatus, setConfigStatus] = useState('paused')
  const [reviewIntervalHours, setReviewIntervalHours] = useState('12')
  const [autoApplyCooldownHours, setAutoApplyCooldownHours] = useState('12')
  const [maxConsecutiveAutoApplies, setMaxConsecutiveAutoApplies] =
    useState('2')
  const [autoApplyConfigPatch, setAutoApplyConfigPatch] = useState(true)
  const [autoApplyPromptPatch, setAutoApplyPromptPatch] = useState(true)
  const [autoRollbackEnabled, setAutoRollbackEnabled] = useState(true)
  const [selfPauseEnabled, setSelfPauseEnabled] = useState(true)
  const [primaryModelConfigID, setPrimaryModelConfigID] = useState('')
  const [primaryModelName, setPrimaryModelName] = useState('')
  const [criticModelConfigID, setCriticModelConfigID] = useState('')
  const [criticModelName, setCriticModelName] = useState('')
  const [proposalPromptInstructions, setProposalPromptInstructions] =
    useState('')
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
          : ot('Failed to load autonomous optimizer panel')
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
          : ot('Failed to load autonomous optimizer run detail')
      )
    } finally {
      setRunDetailLoading(false)
    }
  }

  const runOptimizerNow = async () => {
    if (!traderId) return
    setManualRunLoading(true)
    try {
      const detail = await api.runTraderAutonomousOptimizerNow(traderId)
      setSelectedRunId(detail.run.id)
      setSelectedRunDetail(detail)
      await loadPanel()
      notify.success(ot('Autonomous optimizer run completed'))
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : ot('Failed to execute autonomous optimizer run')
      )
    } finally {
      setManualRunLoading(false)
    }
  }

  useEffect(() => {
    void loadPanel()
  }, [traderId])

  useEffect(() => {
    setSelectedRunId(null)
    setSelectedRunDetail(null)
    setSimilarRuns([])
    setSimilarRunsError(null)
    setEditingBacklogId(null)
    setBacklogDraft(null)
  }, [traderId])

  useEffect(() => {
    if (!traderId || !selectedRunDetail?.run.id) {
      setSimilarRuns([])
      setSimilarRunsError(null)
      setSimilarRunsLoading(false)
      return
    }
    let active = true
    setSimilarRunsLoading(true)
    setSimilarRunsError(null)
    void (async () => {
      try {
        const result = await api.getTraderAutonomousOptimizerRunSimilar(
          traderId,
          selectedRunDetail.run.id,
          { limit: 5 }
        )
        if (!active) return
        setSimilarRuns(result.items || [])
      } catch (err) {
        if (!active) return
        setSimilarRuns([])
        setSimilarRunsError(
          err instanceof Error
            ? err.message
            : ot('Failed to fetch similar autonomous optimizer runs')
        )
      } finally {
        if (!active) return
        setSimilarRunsLoading(false)
      }
    })()
    return () => {
      active = false
    }
  }, [traderId, selectedRunDetail?.run.id])

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
  const criticSummary = readNestedString(
    selectedValidation,
    'critic',
    'summary'
  )
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
  const promptRequestedFieldCount = readNestedNumber(
    selectedValidation,
    'prompt_validation',
    'requested_field_count'
  )
  const promptDeferredFields = readNestedStringArray(
    selectedValidation,
    'prompt_validation',
    'deferred_fields'
  )
  const promptAutoTrimmed = readNestedBoolean(
    selectedValidation,
    'prompt_validation',
    'auto_trimmed'
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
  const rejectedCandidateCount = readNestedNumber(
    selectedMetadata,
    'rejected_candidate_count'
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
  const rejectReasons = readNestedObjectArray(
    selectedMetadata,
    'reject_reasons'
  )
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
  const executionStatuses = readNestedObjectArray(
    selectedMetadata,
    'execution_statuses'
  )
  const recentOpenExecutions = readNestedObjectArray(
    selectedMetadata,
    'recent_open_executions'
  )
  const regimeSummaries = readNestedObjectArray(
    selectedMetadata,
    'regime_summaries'
  )
  const trailingStopTelemetry = readNestedObject(
    selectedMetadata,
    'trailing_stop_telemetry'
  )
  const trailingStopSamples = readNestedObjectArray(
    trailingStopTelemetry,
    'sample_updates'
  )
  const trailingStopTierBreakdown = readNestedObjectArray(
    trailingStopTelemetry,
    'tier_breakdown'
  )
  const trailingStopProfitBandBreakdown = readNestedObjectArray(
    trailingStopTelemetry,
    'profit_band_breakdown'
  )
  const trailingStopEntryProtectionBreakdown = readNestedObjectArray(
    trailingStopTelemetry,
    'entry_protection_breakdown'
  )
  const adaptiveCooldownTelemetry = readNestedObject(
    selectedMetadata,
    'adaptive_cooldown_telemetry'
  )
  const adaptiveCooldownConfig = readNestedObject(
    adaptiveCooldownTelemetry,
    'config'
  )
  const adaptiveCooldownBlockedSymbolOutcomes = readNestedObject(
    adaptiveCooldownTelemetry,
    'blocked_symbol_reentry_outcomes'
  )
  const adaptiveCooldownPostSymbolOutcomes = readNestedObject(
    adaptiveCooldownTelemetry,
    'post_symbol_cooldown_outcomes'
  )
  const adaptiveCooldownBlockedRegimeOutcomes = readNestedObject(
    adaptiveCooldownTelemetry,
    'blocked_regime_reentry_outcomes'
  )
  const adaptiveCooldownPostRegimeOutcomes = readNestedObject(
    adaptiveCooldownTelemetry,
    'post_regime_cooldown_outcomes'
  )
  const adaptiveCooldownTopSymbols = readNestedObjectArray(
    adaptiveCooldownTelemetry,
    'top_symbols'
  )
  const adaptiveCooldownTopRegimes = readNestedObjectArray(
    adaptiveCooldownTelemetry,
    'top_regimes'
  )
  const recentOptimizerContext = readNestedObjectArray(
    selectedMetadata,
    'recent_optimizer_runs'
  )
  const learnedPatternPayload = readNestedObject(
    selectedMetadata,
    'learned_patterns'
  )
  const learnedPatternItems = readNestedObjectArray(
    learnedPatternPayload,
    'items'
  )
  const learnedPatternTopPositive = readNestedObjectArray(
    learnedPatternPayload,
    'top_positive_patterns'
  )
  const learnedPatternTopNegative = readNestedObjectArray(
    learnedPatternPayload,
    'top_negative_patterns'
  )
  const learnedPatternTopOverrides = readNestedObjectArray(
    learnedPatternPayload,
    'top_symbol_overrides'
  )
  const learnedPatternNotes = readNestedStringArray(
    learnedPatternPayload,
    'notes'
  )
  const proposalConversation = readNestedObject(
    selectedMetadata,
    'proposal_conversation'
  )
  const criticConversation = readNestedObject(
    selectedMetadata,
    'critic_conversation'
  )
  const cooldownUntilMs = readNestedNumber(
    selectedMetadata,
    'cooldown_until_ms'
  )
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
  const proposalLearnedPatternRefs = readNestedStringArray(
    selectedMetadata,
    'proposal',
    'learned_pattern_references'
  )
  const criticLearnedPatternRefs = readNestedStringArray(
    selectedValidation,
    'critic',
    'learned_pattern_references'
  )
  const learnedPatternRefLookup = new Set([
    ...proposalLearnedPatternRefs,
    ...criticLearnedPatternRefs,
  ])

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
      notify.success(ot('Autonomous optimizer config saved'))
      await loadPanel()
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : ot('Failed to save autonomous optimizer config')
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
      notify.success(
        `${ot('Status')}: ${formatLabel(nextStatus, language)}`
      )
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : ot('Failed to update autonomous optimizer backlog item')
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
      notify.success(ot('Backlog item updated'))
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : ot('Failed to save autonomous optimizer backlog item')
      )
    } finally {
      setSavingBacklogId(null)
    }
  }

  const openLinkedStrategyVersion = () => {
    const version =
      selectedRunDetail?.linked_strategy_version ||
      (selectedRunDetail?.run.applied_strategy_version_id ? null : null)
    const versionId =
      version?.version.id ||
      selectedRunDetail?.run.applied_strategy_version_id ||
      ''
    if (!versionId) {
      notify.info(ot('No linked strategy version is stored for this run'))
      return
    }
    onOpenStrategyVersion?.(versionId, version || null)
  }

  const applyRunWindowFilter = () => {
    const fromTime = selectedRunDetail?.review_window_start_ms || 0
    const toTime = selectedRunDetail?.review_window_end_ms || 0
    if (!fromTime || !toTime) {
      notify.info(ot('No stored review window is available for this run'))
      return
    }
    onApplyRunWindowFilter?.(fromTime, toTime)
  }

  const modelOptions = [
    { value: '', label: ot('Select model account') },
    ...models.map((item) => ({
      value: item.id,
      label: modelLabel(item, language),
    })),
  ]

  return (
    <div className="nofx-glass rounded-xl p-5 space-y-5">
      <div className="flex flex-col xl:flex-row xl:items-start xl:justify-between gap-4">
        <div>
          <div className="text-xs uppercase tracking-[0.24em] text-nofx-gold/80">
            {ot('Autonomous Optimizer')}
          </div>
          <h2 className="text-xl font-semibold text-nofx-text-main mt-1">
            {language === 'de'
              ? `Sich selbst verbessernde Schleife fuer ${
                  traderName || 'ausgewaehlten Trader'
                }`
              : `Self-improving loop for ${traderName || 'selected trader'}`}
          </h2>
          <p className="text-sm text-nofx-text-muted mt-2">
            {ot(
              'Visible here are the AI improvement steps, the current live optimizer status, and the recent automatic review outcomes.'
            )}
          </p>
        </div>
        <div className="flex flex-col sm:flex-row gap-2">
          <button
            onClick={() => void runOptimizerNow()}
            disabled={!traderId || manualRunLoading || loading}
            className="h-10 px-4 rounded-lg bg-nofx-gold text-black text-sm font-semibold disabled:opacity-50"
          >
            {manualRunLoading ? ot('Running…') : ot('Run now')}
          </button>
          <button
            onClick={() => void loadPanel()}
            disabled={!traderId || loading || manualRunLoading}
            className="h-10 px-4 rounded-lg border border-white/10 bg-black/20 text-sm font-semibold disabled:opacity-50"
          >
            {loading ? ot('Refreshing…') : ot('Refresh optimizer')}
          </button>
        </div>
      </div>

      {error && (
        <div className="rounded-lg border border-rose-400/20 bg-rose-500/10 p-3 text-sm text-rose-300">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3">
        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
          <div className="text-xs text-nofx-text-muted">{ot('Current state')}</div>
          <div className="mt-2">
            <span
              className={`inline-flex px-2.5 py-1 rounded-full border text-xs font-semibold ${statusToneClasses(config?.status)}`}
            >
              {formatLabel(config?.status || 'paused', language)}
            </span>
          </div>
          <div className="text-xs text-nofx-text-muted mt-3">
            {ot('Enabled')} {config?.enabled ? ot('Yes') : ot('No')}
          </div>
        </div>

        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
          <div className="text-xs text-nofx-text-muted">{ot('Next review window')}</div>
          <div className="text-lg font-semibold mt-2">
            {formatRelativeTime(config?.next_run_at, language)}
          </div>
          <div className="text-xs text-nofx-text-muted mt-2">
            {normalizeTime(config?.next_run_at, language)}
          </div>
        </div>

        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
          <div className="text-xs text-nofx-text-muted">{ot('Last run')}</div>
          <div className="text-lg font-semibold mt-2">
            {formatLabel(lastRun?.status || 'none', language)}
          </div>
          <div className="text-xs text-nofx-text-muted mt-2">
            {lastRun
              ? normalizeTime(lastRun.completed_at || lastRun.started_at, language)
              : '-'}
          </div>
        </div>

        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
          <div className="text-xs text-nofx-text-muted">
            {ot('Open improvement steps')}
          </div>
          <div className="text-lg font-semibold mt-2">
            {activeBacklog.length}
          </div>
          <div className="text-xs text-nofx-text-muted mt-2">
            {ot('Highest score')}{' '}
            {activeBacklog[0]
              ? formatScore(activeBacklog[0].composite_score)
              : '-'}
          </div>
        </div>

        <div className="rounded-xl border border-white/10 bg-black/20 p-4">
          <div className="text-xs text-nofx-text-muted">
            {ot('Last applied change')}
          </div>
          <div className="text-sm font-semibold mt-2 line-clamp-2">
            {lastAppliedRun?.summary || ot('No applied autonomous change yet')}
          </div>
          <div className="text-xs text-nofx-text-muted mt-2">
            {lastAppliedRun ? normalizeTime(lastAppliedRun.updated_at, language) : '-'}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[0.95fr_1.05fr] gap-5">
        <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
              {ot('Config')}
            </div>
            <div className="text-sm text-nofx-text-muted mt-2">
              {ot(
                'Choose the active optimizer account and model pair here. This is where `GPT-5.4` is currently wired by default.'
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">{ot('Enabled')}</div>
              <input
                type="checkbox"
                checked={enabled}
                onChange={(event) => setEnabled(event.target.checked)}
              />
            </label>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-xs text-nofx-text-muted mb-2">{ot('Status')}</div>
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
                {ot('Review interval hours')}
              </div>
              <input
                value={reviewIntervalHours}
                onChange={(event) => setReviewIntervalHours(event.target.value)}
                className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
              />
            </label>
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                {ot('Cooldown hours')}
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
                {ot('Max consecutive applies')}
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
                {ot('Auto-apply config patches')}
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
                {ot('Auto-apply prompt patches')}
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
                {ot('Auto rollback')}
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
                {ot('Self-pause')}
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
                {ot('Proposer account')}
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
                    {
                      value: primaryModelName || '',
                      label: ot('Current model name'),
                    },
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
                placeholder={ot('e.g. gpt-5.4')}
                className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm mt-3"
              />
            </div>

            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-xs text-nofx-text-muted mb-2">
                {ot('Critic account')}
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
                    {
                      value: criticModelName || '',
                      label: ot('Current model name'),
                    },
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
                placeholder={ot('e.g. gpt-5.4')}
                className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm mt-3"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                {ot('Proposal instructions overlay')}
              </div>
              <textarea
                value={proposalPromptInstructions}
                onChange={(event) =>
                  setProposalPromptInstructions(event.target.value)
                }
                placeholder={ot(
                  'Optional extra instructions merged into the optimizer proposer prompt.'
                )}
                className="min-h-[140px] w-full rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm"
              />
            </label>
            <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
              <div className="text-xs text-nofx-text-muted mb-2">
                {ot('Critic instructions overlay')}
              </div>
              <textarea
                value={criticPromptInstructions}
                onChange={(event) =>
                  setCriticPromptInstructions(event.target.value)
                }
                placeholder={ot(
                  'Optional extra instructions merged into the optimizer critic prompt.'
                )}
                className="min-h-[140px] w-full rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm"
              />
            </label>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-5 gap-3 text-xs">
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted">{ot('Seed source trader')}</div>
              <div className="mt-1 break-all">
                {config?.seed_source_trader_id || '-'}
              </div>
            </div>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted">{ot('Current seed strategy')}</div>
              <div className="mt-1 break-all">
                {config?.current_seed_strategy_id || '-'}
              </div>
            </div>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted">{ot('Last applied version')}</div>
              <div className="mt-1 break-all">
                {lastAppliedRun?.applied_strategy_version_id || '-'}
              </div>
            </div>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted">{ot('Cooldown')}</div>
              <div className="mt-1 break-all">
                {config?.auto_apply_cooldown_hours || 12}h
              </div>
            </div>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted">{ot('Max apply streak')}</div>
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
            {savingConfig ? ot('Saving…') : ot('Save optimizer config')}
          </button>
        </div>

        <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
          <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
            <div>
              <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                {ot('Improvement Backlog')}
              </div>
              <div className="text-sm text-nofx-text-muted mt-2">
                {ot(
                  'These are the AI-detected next improvement steps. If the AI says indicators, telemetry, or prompt changes are missing, they land here with a score.'
                )}
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
                {ot(
                  'No visible improvement steps yet for this filter. Once the optimizer proposes missing capabilities or next build items, they will appear here.'
                )}
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
                          {formatLabel(item.status, language)}
                        </span>
                        <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                          {formatLabel(item.category, language)}
                        </span>
                        {item.ai_generated && (
                          <span className="inline-flex px-2 py-1 rounded-full border border-sky-400/20 bg-sky-500/10 text-[11px] text-sky-300">
                            {ot('AI')}
                          </span>
                        )}
                        {item.user_edited && (
                          <span className="inline-flex px-2 py-1 rounded-full border border-nofx-gold/20 bg-nofx-gold/10 text-[11px] text-nofx-gold">
                            {ot('User edited')}
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
                          {ot('Expected impact:')} {item.expected_impact}
                        </div>
                      )}
                      <div className="flex flex-wrap gap-4 text-xs text-nofx-text-muted mt-3">
                        <span>{ot('Score')} {formatScore(item.composite_score)}</span>
                        <span>{ot('Confidence')} {formatScore(item.confidence)}</span>
                        <span>{ot('Urgency')} {formatScore(item.urgency)}</span>
                        <span>
                          {ot('Cost')} {formatScore(item.implementation_cost)}
                        </span>
                        <span>{ot('Recurrent')} {item.recurrence_count}x</span>
                        <span>{ot('Merged')} {item.merged_finding_count}x</span>
                        <span>
                          {ot('Source run')}{' '}
                          {item.run_id ? item.run_id.slice(0, 8) : '-'}
                        </span>
                      </div>
                      <div className="flex flex-wrap gap-2 mt-3">
                        <button
                          onClick={() => beginBacklogEdit(item)}
                          disabled={savingBacklogId === item.id}
                          className="h-9 px-3 rounded-lg border border-white/10 bg-white/5 text-xs font-semibold disabled:opacity-50"
                        >
                          {ot('Edit fields')}
                        </button>
                        {editingBacklogId === item.id && (
                          <>
                            <button
                              onClick={() => void saveBacklogEdit(item)}
                              disabled={
                                savingBacklogId === item.id || !backlogDraft
                              }
                              className="h-9 px-3 rounded-lg bg-nofx-gold text-black text-xs font-semibold disabled:opacity-50"
                            >
                              {savingBacklogId === item.id
                                ? ot('Saving…')
                                : ot('Save edit')}
                            </button>
                            <button
                              onClick={cancelBacklogEdit}
                              disabled={savingBacklogId === item.id}
                              className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-xs font-semibold disabled:opacity-50"
                            >
                              {ot('Cancel')}
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
                        {ot('Updated')} {normalizeTime(item.updated_at, language)}
                      </div>
                    </div>
                  </div>

                  {editingBacklogId === item.id && backlogDraft && (
                    <div className="mt-4 grid grid-cols-1 xl:grid-cols-2 gap-3">
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
                        <div className="text-xs text-nofx-text-muted mb-2">
                          {ot('Title')}
                        </div>
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
                        <div className="text-xs text-nofx-text-muted mb-2">
                          {ot('Category')}
                        </div>
                        <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center">
                          <NofxSelect
                            value={backlogDraft.category}
                            onChange={(value) =>
                              setBacklogDraft((current) =>
                                current
                                  ? { ...current, category: value }
                                  : current
                              )
                            }
                            options={backlogCategoryOptions}
                          />
                        </div>
                      </div>
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm xl:col-span-2">
                        <div className="text-xs text-nofx-text-muted mb-2">
                          {ot('Description')}
                        </div>
                        <textarea
                          value={backlogDraft.description}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? {
                                    ...current,
                                    description: event.target.value,
                                  }
                                : current
                            )
                          }
                          rows={4}
                          className="w-full rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm"
                        />
                      </label>
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm xl:col-span-2">
                        <div className="text-xs text-nofx-text-muted mb-2">
                          {ot('Expected impact')}
                        </div>
                        <textarea
                          value={backlogDraft.expectedImpact}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? {
                                    ...current,
                                    expectedImpact: event.target.value,
                                  }
                                : current
                            )
                          }
                          rows={3}
                          className="w-full rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm"
                        />
                      </label>
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
                        <div className="text-xs text-nofx-text-muted mb-2">
                          {ot('Confidence')}
                        </div>
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
                        <div className="text-xs text-nofx-text-muted mb-2">
                          {ot('Implementation cost')}
                        </div>
                        <input
                          value={backlogDraft.implementationCost}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? {
                                    ...current,
                                    implementationCost: event.target.value,
                                  }
                                : current
                            )
                          }
                          className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                        />
                      </label>
                      <label className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm">
                        <div className="text-xs text-nofx-text-muted mb-2">
                          {ot('Urgency')}
                        </div>
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
                        <div className="text-xs text-nofx-text-muted mb-2">
                          {ot('Recurrence count')}
                        </div>
                        <input
                          value={backlogDraft.recurrenceCount}
                          onChange={(event) =>
                            setBacklogDraft((current) =>
                              current
                                ? {
                                    ...current,
                                    recurrenceCount: event.target.value,
                                  }
                                : current
                            )
                          }
                          className="h-10 w-full rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
                        />
                      </label>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                        <div className="text-xs text-nofx-text-muted mb-2">
                          {ot('Status')}
                        </div>
                        <div className="h-10 rounded-lg border border-white/10 px-3 flex items-center">
                          <NofxSelect
                            value={backlogDraft.status}
                            onChange={(value) =>
                              setBacklogDraft((current) =>
                                current
                                  ? { ...current, status: value }
                                  : current
                              )
                            }
                            options={backlogStatusOptions.filter(
                              (option) => option.value
                            )}
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
              {ot('Model Outcomes')}
            </div>
            <div className="text-sm text-nofx-text-muted mt-2">
              {ot(
                'This ranks proposer/critic pairs by actual optimizer outcomes: apply rate, rollback rate, kept-win rate, and backlog usefulness.'
              )}
            </div>
          </div>
        </div>
        {modelOutcomes.length === 0 ? (
          <div className="rounded-lg border border-dashed border-white/10 bg-black/10 p-4 text-sm text-nofx-text-muted">
            {ot('No per-model optimizer outcome data is available yet.')}
          </div>
        ) : (
          <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
            {modelOutcomes.map((item) => {
              const isActive =
                activeModelKey && item.model_key === activeModelKey
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
                            {ot('Active pair')}
                          </span>
                        )}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-2">
                        {ot('Last used')} {normalizeTime(item.last_used_at, language)}
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="text-xs text-nofx-text-muted">
                        {ot('Outcome score')}
                      </div>
                      <div className="text-lg font-semibold mt-1">
                        {formatScore(item.outcome_score)}
                      </div>
                    </div>
                  </div>

                  <div className="grid grid-cols-2 xl:grid-cols-5 gap-3 mt-4">
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {ot('Apply rate')}
                      </div>
                      <div className="text-base font-semibold mt-1">
                        {formatPct(item.apply_rate)}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {item.apply_count} of {item.total_runs} runs
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {ot('Rollback rate')}
                      </div>
                      <div className="text-base font-semibold mt-1">
                        {formatPct(item.rollback_rate)}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {item.rollback_count} rollback / {item.apply_count}{' '}
                        applies
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {ot('Kept-win rate')}
                      </div>
                      <div className="text-base font-semibold mt-1">
                        {formatPct(item.kept_win_rate)}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {item.kept_win_count} positive kept / {item.kept_count}{' '}
                        kept
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {ot('Backlog usefulness')}
                      </div>
                      <div className="text-base font-semibold mt-1">
                        {formatPct(item.backlog_usefulness)}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {item.useful_backlog_count} useful /{' '}
                        {item.backlog_item_count} items
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {ot('Operational health')}
                      </div>
                      <div className="text-base font-semibold mt-1">
                        {formatPct(item.operational_health_score)}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {item.failed_count} failed • {item.stale_recovery_count}{' '}
                        stale
                      </div>
                    </div>
                  </div>

                  <div className="grid grid-cols-2 xl:grid-cols-3 gap-3 mt-3 text-xs text-nofx-text-muted">
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      {ot('Monitoring applies:')} {item.monitoring_count}
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      {ot('Failed runs:')} {item.failed_count}
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      {ot('Evidence-gap overlaps:')}{' '}
                      {item.failure_overlap_insufficient_evidence_count}
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      {ot('Done backlog items:')} {item.done_backlog_count}
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      {ot('Rejected backlog items:')} {item.rejected_backlog_count}
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      {ot('Blocked runs:')} {item.status_counts?.blocked_by_gate || 0}
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
              {ot('Run History')}
            </div>
            <div className="text-sm text-nofx-text-muted mt-2">
              {ot(
                'This shows what the optimizer actually did every window: applied, blocked, backlog-only, monitoring, rollback, or no change.'
              )}
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
              {ot('No optimizer runs visible for this filter yet.')}
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
                        {formatLabel(run.status, language)}
                      </span>
                      <span className="inline-flex px-2.5 py-1 rounded-full border border-white/10 bg-white/5 text-xs text-nofx-text-muted">
                        {formatLabel(run.trigger, language)}
                      </span>
                      <span className="inline-flex px-2.5 py-1 rounded-full border border-white/10 bg-white/5 text-xs text-nofx-text-muted">
                        {run.primary_model_name || '-'} /{' '}
                        {run.critic_model_name || '-'}
                      </span>
                    </div>
                    <div className="font-semibold mt-3">
                      {run.summary || ot('No summary stored')}
                    </div>
                    <div className="flex flex-wrap gap-4 text-xs text-nofx-text-muted mt-3">
                      <span>{ot('Started')} {normalizeTime(run.started_at, language)}</span>
                      <span>{ot('Finished')} {normalizeTime(run.completed_at, language)}</span>
                      <span>{ot('Updated')} {normalizeTime(run.updated_at, language)}</span>
                      {run.applied_strategy_version_id && (
                        <span>
                          {ot('Strategy version')}{' '}
                          {run.applied_strategy_version_id.slice(0, 8)}
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="text-xs text-nofx-text-muted">
                    {ot('Run ID')} {run.id.slice(0, 8)}
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
                  {ot('Run Detail')}
                </div>
                <div className="text-lg font-semibold mt-2">
                  {runDetailLoading
                    ? ot('Loading run detail…')
                    : selectedRunDetail?.run.summary || ot('No summary stored')}
                </div>
                {!runDetailLoading && (
                  <div className="flex flex-wrap gap-2 mt-3">
                    <span
                      className={`inline-flex px-2.5 py-1 rounded-full border text-xs font-semibold ${statusToneClasses(selectedRunDetail?.run.status)}`}
                    >
                      {formatLabel(selectedRunDetail?.run.status, language)}
                    </span>
                    <span className="inline-flex px-2.5 py-1 rounded-full border border-white/10 bg-white/5 text-xs text-nofx-text-muted">
                      {formatLabel(selectedRunDetail?.run.trigger, language)}
                    </span>
                    {proposalType && (
                      <span className="inline-flex px-2.5 py-1 rounded-full border border-sky-400/20 bg-sky-500/10 text-xs text-sky-300">
                        {formatLabel(proposalType, language)}
                      </span>
                    )}
                  </div>
                )}
              </div>

              {!runDetailLoading && (
                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={openLinkedStrategyVersion}
                    disabled={
                      !selectedRunDetail?.run.applied_strategy_version_id
                    }
                    className="h-10 px-3 rounded-lg border border-nofx-gold/30 text-nofx-gold disabled:opacity-50"
                  >
                    {ot('Open strategy version')}
                  </button>
                  <button
                    onClick={applyRunWindowFilter}
                    disabled={
                      !selectedRunDetail?.review_window_start_ms ||
                      !selectedRunDetail?.review_window_end_ms
                    }
                    className="h-10 px-3 rounded-lg border border-sky-400/30 text-sky-300 disabled:opacity-50"
                  >
                    {ot('Open review cohort')}
                  </button>
                </div>
              )}
            </div>

            {!runDetailLoading && selectedRunDetail && (
              <>
                <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3 text-sm">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                    <div className="text-xs text-nofx-text-muted">
                      {ot('Started')}
                    </div>
                    <div className="mt-1">
                      {normalizeTime(selectedRunDetail.run.started_at)}
                    </div>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                    <div className="text-xs text-nofx-text-muted">
                      {ot('Finished')}
                    </div>
                    <div className="mt-1">
                      {normalizeTime(selectedRunDetail.run.completed_at)}
                    </div>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                    <div className="text-xs text-nofx-text-muted">
                      {pickText('Window stats', 'Fenster-Statistik')}
                    </div>
                    <div className="mt-1">
                      {reviewWindowClosedDeals ?? '-'}{' '}
                      {pickText('deals', 'Deals')} | {reviewWindowCycles ?? '-'}{' '}
                      {pickText('cycles', 'Zyklen')} |{' '}
                      {reviewWindowCandidates ?? '-'}{' '}
                      {pickText('candidates', 'Kandidaten')}
                    </div>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                    <div className="text-xs text-nofx-text-muted">
                      {pickText('Window PnL', 'Fenster-PnL')}
                    </div>
                    <div className="mt-1">{formatNumber(reviewWindowPnL)}</div>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                    <div className="text-xs text-nofx-text-muted">
                      {pickText('Next eligible apply', 'Naechste moegliche Uebernahme')}
                    </div>
                    <div className="mt-1">
                      {typeof nextEligibleRunMs === 'number'
                        ? normalizeTime(
                            new Date(nextEligibleRunMs).toISOString()
                          )
                        : '-'}
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {typeof cooldownUntilMs === 'number'
                        ? `${pickText('Cooldown until', 'Cooldown bis')} ${normalizeTime(
                            new Date(cooldownUntilMs).toISOString()
                          )}`
                        : pickText(
                            'No cooldown hold stored',
                            'Kein gespeicherter Cooldown-Hold'
                          )}
                    </div>
                  </div>
                </div>

                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    {pickText(
                      'Monitoring / Rollback Analysis',
                      'Monitoring- / Rollback-Analyse'
                    )}
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 text-sm">
                    <div>
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Root apply run', 'Ausgangs-Uebernahmelauf')}
                      </div>
                      <div className="mt-1 break-all">
                        {monitoringRootRunId || '-'}
                      </div>
                    </div>
                    <div>
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Observed windows', 'Beobachtete Fenster')}
                      </div>
                      <div className="mt-1 font-semibold">
                        {typeof monitoringWindowsObserved === 'number'
                          ? Math.round(monitoringWindowsObserved + 1)
                          : '-'}
                      </div>
                    </div>
                    <div>
                      <div className="text-xs text-nofx-text-muted">
                        {pickText(
                          'Cumulative monitored deals / PnL',
                          'Kumulierte ueberwachte Deals / PnL'
                        )}
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
                        {pickText('Negative windows', 'Negative Fenster')}
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
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Baseline net PnL', 'Baseline Netto-PnL')}
                      </div>
                      <div className="mt-1 font-semibold">
                        {typeof sourceBaselineNetPnL === 'number'
                          ? formatNumber(sourceBaselineNetPnL)
                          : '-'}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText('Baseline win rate', 'Baseline Gewinnrate')}
                      </div>
                      <div className="mt-1 font-semibold">
                        {typeof sourceBaselineWinRate === 'number'
                          ? `${formatNumber(sourceBaselineWinRate)}%`
                          : '-'}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="text-xs text-nofx-text-muted">
                        {pickText(
                          'Baseline exit efficiency',
                          'Baseline Exit-Effizienz'
                        )}
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
                      {pickText(
                        'No rollback trigger fired for this run.',
                        'Fuer diesen Lauf wurde kein Rollback-Trigger ausgeloest.'
                      )}
                    </div>
                  )}
                </div>

                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Low-Trade Telemetry', 'Low-Trade-Telemetrie')}
                    </div>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                      <div>
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Open decisions', 'Open-Entscheidungen')}
                        </div>
                        <div className="mt-1 font-semibold">
                          {typeof openDecisionCount === 'number'
                            ? Math.round(openDecisionCount)
                            : '-'}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Hold decisions', 'Hold-Entscheidungen')}
                        </div>
                        <div className="mt-1 font-semibold">
                          {typeof holdDecisionCount === 'number'
                            ? Math.round(holdDecisionCount)
                            : '-'}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Wait decisions', 'Wait-Entscheidungen')}
                        </div>
                        <div className="mt-1 font-semibold">
                          {typeof waitDecisionCount === 'number'
                            ? Math.round(waitDecisionCount)
                            : '-'}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-nofx-text-muted">
                          {pickText('Conversion', 'Konversion')}
                        </div>
                        <div className="mt-1 font-semibold">
                          {typeof decisionConversionRate === 'number'
                            ? `${formatNumber(decisionConversionRate)}%`
                            : '-'}
                        </div>
                      </div>
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-3">
                      {pickText('Avg decision confidence:', 'Ø Entscheidungs-Konfidenz:')}{' '}
                      {typeof avgDecisionConfidence === 'number'
                        ? formatNumber(avgDecisionConfidence)
                        : '-'}
                    </div>
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Recent Optimizer Context', 'Juengster Optimizer-Kontext')}
                    </div>
                    {recentOptimizerContext.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No previous optimizer runs were attached to this run.',
                          'Diesem Lauf wurden keine frueheren Optimizer-Laeufe zugeordnet.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {recentOptimizerContext
                          .slice(0, 4)
                          .map((item, index) => (
                            <div
                              key={`${String(item.id || index)}`}
                              className="rounded-lg border border-white/10 bg-black/20 p-3"
                            >
                              <div className="flex flex-wrap items-center gap-2">
                                <span
                                  className={`inline-flex px-2 py-1 rounded-full border text-[11px] ${statusToneClasses(
                                    typeof item.status === 'string'
                                      ? item.status
                                      : ''
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
                                  : ot('No summary stored')}
                              </div>
                            </div>
                          ))}
                      </div>
                    )}
                  </div>
                </div>

                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    {pickText('Similar Prior Runs', 'Aehnliche fruehere Laeufe')}
                  </div>
                  {similarRunsLoading ? (
                    <div className="text-sm text-nofx-text-muted">
                      {pickText(
                        'Loading similar prior runs…',
                        'Aehnliche fruehere Laeufe werden geladen…'
                      )}
                    </div>
                  ) : similarRunsError ? (
                    <div className="text-sm text-amber-200">
                      {similarRunsError}
                    </div>
                  ) : similarRuns.length === 0 ? (
                    <div className="text-sm text-nofx-text-muted">
                      {pickText(
                        'No similar prior optimizer runs were found yet.',
                        'Es wurden noch keine aehnlichen frueheren Optimizer-Laeufe gefunden.'
                      )}
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {similarRuns.map((hit) => (
                        <div
                          key={`${hit.document.id}-${hit.document.source_id}`}
                          className="rounded-lg border border-white/10 bg-black/20 p-3"
                        >
                          <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-3">
                            <div>
                              <div className="font-semibold">
                                {hit.document.title || hit.document.source_id}
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                {formatSemanticSimilarityScore(hit.similarity_score)} ·{' '}
                                {pickText('updated', 'aktualisiert')}{' '}
                                {hit.document.source_updated_at
                                  ? normalizeTime(hit.document.source_updated_at)
                                  : '-'}
                              </div>
                            </div>
                            <button
                              onClick={() => void loadRunDetail(hit.document.source_id)}
                              className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-sm"
                            >
                              {pickText('Focus run', 'Lauf fokussieren')}
                            </button>
                          </div>
                          <div className="text-sm text-nofx-text-muted mt-3">
                            {hit.document.summary || ot('No summary stored')}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                  <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                    {pickText('Learned Pattern Evidence', 'Belege gelernter Muster')}
                  </div>
                  {!learnedPatternPayload ? (
                    <div className="text-sm text-nofx-text-muted">
                      {pickText(
                        'No learned-pattern evidence was attached to this optimizer run.',
                        'Diesem Optimizer-Lauf wurden keine Belege gelernter Muster zugeordnet.'
                      )}
                    </div>
                  ) : (
                    <div className="space-y-4">
                      <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-8 gap-3 text-sm">
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {pickText('Available', 'Verfuegbar')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'available_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'available_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {pickText('Relevant', 'Relevant')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'relevant_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'relevant_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {pickText('Positive / Negative', 'Positiv / Negativ')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'positive_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'positive_count'
                                  ) || 0
                                )
                              : '-'}{' '}
                            /{' '}
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'negative_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'negative_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {formatLabel('confirmed')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'confirmed_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'confirmed_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {pickText('Config candidates', 'Konfig-Kandidaten')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'config_candidate_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'config_candidate_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {pickText('Prompt only', 'Nur Prompt')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'prompt_only_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'prompt_only_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {pickText('Monitoring rules', 'Monitoring-Regeln')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'monitoring_rule_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'monitoring_rule_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {pickText('False pos / reverse', 'Falsch pos. / Umkehr')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'false_positive_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'false_positive_count'
                                  ) || 0
                                )
                              : '-'}{' '}
                            /{' '}
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'reverse_risk_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'reverse_risk_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {pickText('Degrading rules', 'Schlechter werdende Regeln')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'lifecycle_degrading_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'lifecycle_degrading_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {pickText('Rollback watch', 'Rollback-Watch')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'lifecycle_rollback_watch_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'lifecycle_rollback_watch_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                          <div className="text-xs text-nofx-text-muted">
                            {formatLabel('drifting')}
                          </div>
                          <div className="mt-1 font-semibold">
                            {typeof readNestedNumber(
                              learnedPatternPayload,
                              'drifting_count'
                            ) === 'number'
                              ? Math.round(
                                  readNestedNumber(
                                    learnedPatternPayload,
                                    'drifting_count'
                                  ) || 0
                                )
                              : '-'}
                          </div>
                        </div>
                      </div>

                      {(proposalLearnedPatternRefs.length > 0 ||
                        criticLearnedPatternRefs.length > 0) && (
                        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted mb-2">
                              {pickText('Proposal references', 'Proposal-Referenzen')}
                            </div>
                            {proposalLearnedPatternRefs.length === 0 ? (
                              <div className="text-sm text-nofx-text-muted">
                                {pickText(
                                  'No learned pattern was explicitly cited by the proposer.',
                                  'Es wurde kein gelerntes Muster explizit vom Proposer referenziert.'
                                )}
                              </div>
                            ) : (
                              <div className="flex flex-wrap gap-2">
                                {proposalLearnedPatternRefs.map((item) => (
                                  <span
                                    key={item}
                                    className="inline-flex px-2 py-1 rounded-full border border-sky-400/20 bg-sky-500/10 text-[11px] text-sky-300"
                                  >
                                    {item.slice(0, 12)}
                                  </span>
                                ))}
                              </div>
                            )}
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted mb-2">
                              {pickText('Critic references', 'Critic-Referenzen')}
                            </div>
                            {criticLearnedPatternRefs.length === 0 ? (
                              <div className="text-sm text-nofx-text-muted">
                                {pickText(
                                  'No learned pattern was explicitly cited by the critic.',
                                  'Es wurde kein gelerntes Muster explizit vom Critic referenziert.'
                                )}
                              </div>
                            ) : (
                              <div className="flex flex-wrap gap-2">
                                {criticLearnedPatternRefs.map((item) => (
                                  <span
                                    key={item}
                                    className="inline-flex px-2 py-1 rounded-full border border-emerald-400/20 bg-emerald-500/10 text-[11px] text-emerald-300"
                                  >
                                    {item.slice(0, 12)}
                                  </span>
                                ))}
                              </div>
                            )}
                          </div>
                        </div>
                      )}

                      {learnedPatternNotes.length > 0 && (
                        <div className="space-y-2">
                          {learnedPatternNotes.map((item) => (
                            <div
                              key={item}
                              className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm text-nofx-text-muted"
                            >
                              {item}
                            </div>
                          ))}
                        </div>
                      )}

                      <div className="grid grid-cols-1 xl:grid-cols-3 gap-3">
                        {[
                          {
                            title: pickText('Top positive', 'Top positiv'),
                            items: learnedPatternTopPositive,
                          },
                          {
                            title: pickText('Top anti-patterns', 'Top Anti-Muster'),
                            items: learnedPatternTopNegative,
                          },
                          {
                            title: pickText('Symbol overrides', 'Symbol-Overrides'),
                            items: learnedPatternTopOverrides,
                          },
                        ].map((group) => (
                          <div
                            key={group.title}
                            className="rounded-lg border border-white/10 bg-black/20 p-3"
                          >
                            <div className="text-xs text-nofx-text-muted mb-2">
                              {group.title}
                            </div>
                            {group.items.length === 0 ? (
                              <div className="text-sm text-nofx-text-muted">
                                {pickText('No items stored.', 'Keine Eintraege gespeichert.')}
                              </div>
                            ) : (
                              <div className="space-y-2">
                                {group.items.slice(0, 3).map((item, index) => (
                                  <div
                                    key={`${group.title}-${String(
                                      item.pattern_id || index
                                    )}`}
                                    className="rounded-lg border border-white/10 bg-white/[0.03] p-3"
                                  >
                                    <div className="flex flex-wrap items-center gap-2 text-sm">
                                      <span className="font-semibold">
                                        {typeof item.symbol === 'string' &&
                                        item.symbol
                                          ? item.symbol
                                          : pickText('Trader-wide', 'Trader-weit')}
                                      </span>
                                      <span className="text-nofx-text-muted">
                                        {formatLabel(
                                          typeof item.side === 'string'
                                            ? item.side
                                            : ''
                                        )}
                                      </span>
                                      <span
                                        className={`inline-flex px-2 py-1 rounded-full border text-[11px] ${statusToneClasses(
                                          typeof item.validation_label ===
                                            'string'
                                            ? item.validation_label
                                            : ''
                                        )}`}
                                      >
                                        {formatLabel(
                                          typeof item.validation_label ===
                                            'string'
                                            ? item.validation_label
                                            : 'unknown'
                                        )}
                                      </span>
                                    </div>
                                    <div className="text-xs text-nofx-text-muted mt-2">
                                        {typeof item.summary === 'string' &&
                                      item.summary
                                        ? item.summary
                                        : typeof item.pattern_signature ===
                                            'string' && item.pattern_signature
                                          ? item.pattern_signature
                                          : ot('No summary stored')}
                                    </div>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        ))}
                      </div>

                      {learnedPatternItems.length === 0 ? (
                        <div className="text-sm text-nofx-text-muted">
                          {pickText(
                            'No current-window learned-pattern matches were stored for this run.',
                            'Fuer diesen Lauf wurden keine Learned-Pattern-Treffer des aktuellen Fensters gespeichert.'
                          )}
                        </div>
                      ) : (
                        <div className="space-y-3">
                          {learnedPatternItems.map((item, index) => {
                            const patternId =
                              typeof item.pattern_id === 'string'
                                ? item.pattern_id
                                : ''
                            const actionHintAction =
                              typeof item.action_hint_recommended_action ===
                              'string'
                                ? item.action_hint_recommended_action
                                : ''
                            const actionHintPriority =
                              typeof item.action_hint_priority_label === 'string'
                                ? item.action_hint_priority_label
                                : ''
                            const actionHintSummary =
                              typeof item.action_hint_summary === 'string'
                                ? item.action_hint_summary
                                : ''
                            const actionHintAutoNote =
                              typeof item.action_hint_auto_note === 'string'
                                ? item.action_hint_auto_note
                                : ''
                            const interventionOpenCount =
                              typeof item.intervention_open_suggestion_count ===
                              'number'
                                ? item.intervention_open_suggestion_count
                                : 0
                            const interventionAcceptedCount =
                              typeof item.intervention_accepted_count === 'number'
                                ? item.intervention_accepted_count
                                : 0
                            const interventionOverriddenCount =
                              typeof item.intervention_overridden_count ===
                              'number'
                                ? item.intervention_overridden_count
                                : 0
                            const interventionManualCount =
                              typeof item.intervention_manual_action_count ===
                              'number'
                                ? item.intervention_manual_action_count
                                : 0
                            const interventionLatestStatus =
                              typeof item.intervention_latest_status === 'string'
                                ? item.intervention_latest_status
                                : ''
                            const interventionLatestEventType =
                              typeof item.intervention_latest_event_type ===
                              'string'
                                ? item.intervention_latest_event_type
                                : ''
                            const interventionLatestSummary =
                              typeof item.intervention_latest_summary ===
                              'string'
                                ? item.intervention_latest_summary
                                : ''
                            const interventionLatestNote =
                              typeof item.intervention_latest_note === 'string'
                                ? item.intervention_latest_note
                                : ''
                            return (
                              <div
                                key={`${patternId || index}`}
                                className="rounded-lg border border-white/10 bg-black/20 p-4"
                              >
                                <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-3">
                                  <div className="min-w-0">
                                    <div className="flex flex-wrap items-center gap-2">
                                      <span className="font-semibold">
                                        {typeof item.symbol === 'string' &&
                                        item.symbol
                                          ? item.symbol
                                          : pickText('Trader-wide', 'Trader-weit')}
                                      </span>
                                      <span className="text-nofx-text-muted">
                                        {formatLabel(
                                          typeof item.side === 'string'
                                            ? item.side
                                            : ''
                                        )}
                                      </span>
                                      <span
                                        className={`inline-flex px-2 py-1 rounded-full border text-[11px] ${statusToneClasses(
                                          typeof item.validation_label ===
                                            'string'
                                            ? item.validation_label
                                            : ''
                                        )}`}
                                      >
                                        {formatLabel(
                                          typeof item.validation_label ===
                                            'string'
                                            ? item.validation_label
                                            : 'unknown'
                                        )}
                                      </span>
                                      <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                                        {formatLabel(
                                          typeof item.implication_type ===
                                            'string'
                                            ? item.implication_type
                                            : 'review_hint'
                                        )}
                                      </span>
                                      {typeof item.recommended_use === 'string' &&
                                        item.recommended_use && (
                                          <span className="inline-flex px-2 py-1 rounded-full border border-amber-400/20 bg-amber-500/10 text-[11px] text-amber-200">
                                            {formatLabel(item.recommended_use)}
                                          </span>
                                        )}
                                      {actionHintAction && (
                                        <span
                                          className={`inline-flex px-2 py-1 rounded-full border text-[11px] ${actionHintToneClasses(
                                            actionHintAction,
                                            actionHintPriority
                                          )}`}
                                        >
                                          {formatLabel(actionHintAction)}
                                        </span>
                                      )}
                                      {patternId &&
                                        learnedPatternRefLookup.has(
                                          patternId
                                        ) && (
                                          <span className="inline-flex px-2 py-1 rounded-full border border-nofx-gold/30 bg-nofx-gold/10 text-[11px] text-nofx-gold">
                                            {pickText('Cited', 'Zitiert')}
                                          </span>
                                        )}
                                    </div>
                                    <div className="text-sm mt-2">
                                      {typeof item.summary === 'string' &&
                                      item.summary
                                        ? item.summary
                                        : typeof item.pattern_signature ===
                                            'string' && item.pattern_signature
                                          ? item.pattern_signature
                                          : ot('No summary stored')}
                                    </div>
                                    {typeof item.implication_summary ===
                                      'string' &&
                                      item.implication_summary && (
                                        <div className="text-sm text-sky-300 mt-2">
                                          {item.implication_summary}
                                        </div>
                                      )}
                                    {typeof item.validation_alert ===
                                      'string' &&
                                      item.validation_alert && (
                                        <div className="text-sm text-amber-200 mt-2">
                                          {item.validation_alert}
                                        </div>
                                      )}
                                    {typeof item.lifecycle_summary ===
                                      'string' &&
                                      item.lifecycle_summary && (
                                        <div className="text-sm text-rose-200 mt-2">
                                          {item.lifecycle_summary}
                                        </div>
                                      )}
                                    {actionHintSummary && (
                                      <div className="mt-3 rounded-lg border border-white/10 bg-white/[0.03] p-3">
                                        <div className="flex flex-wrap items-center gap-2">
                                          <span className="text-xs uppercase tracking-[0.18em] text-nofx-text-muted">
                                            {pickText('Suggested action', 'Vorgeschlagene Aktion')}
                                          </span>
                                          <span
                                            className={`inline-flex px-2 py-1 rounded-full border text-[11px] ${actionHintToneClasses(
                                              actionHintAction,
                                              actionHintPriority
                                            )}`}
                                          >
                                            {formatLabel(actionHintAction || 'review')}
                                          </span>
                                          {actionHintPriority && (
                                            <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                                              {formatLabel(actionHintPriority)}
                                            </span>
                                          )}
                                        </div>
                                        <div className="text-sm mt-2 text-nofx-text-main">
                                          {actionHintSummary}
                                        </div>
                                        {actionHintAutoNote && (
                                          <div className="text-xs text-nofx-text-muted mt-2">
                                            {actionHintAutoNote}
                                          </div>
                                        )}
                                      </div>
                                    )}
                                    {(interventionOpenCount > 0 ||
                                      interventionAcceptedCount > 0 ||
                                      interventionOverriddenCount > 0 ||
                                      interventionManualCount > 0 ||
                                      interventionLatestSummary) && (
                                      <div className="mt-3 rounded-lg border border-white/10 bg-black/20 p-3">
                                        <div className="flex flex-wrap items-center gap-2">
                                          <span className="text-xs uppercase tracking-[0.18em] text-nofx-text-muted">
                                            {pickText('Intervention history', 'Interventionsverlauf')}
                                          </span>
                                          {interventionLatestEventType && (
                                            <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                                              {formatLabel(interventionLatestEventType)}
                                            </span>
                                          )}
                                          {interventionLatestStatus && (
                                            <span
                                              className={`inline-flex px-2 py-1 rounded-full border text-[11px] ${statusToneClasses(
                                                interventionLatestStatus
                                              )}`}
                                            >
                                              {formatLabel(interventionLatestStatus)}
                                            </span>
                                          )}
                                        </div>
                                        <div className="text-xs text-nofx-text-muted mt-2">
                                          {pickText('Open', 'Offen')} {interventionOpenCount} • {pickText('Accepted', 'Akzeptiert')}{' '}
                                          {interventionAcceptedCount} • {pickText('Overridden', 'Ueberschrieben')}{' '}
                                          {interventionOverriddenCount} • {pickText('Manual', 'Manuell')}{' '}
                                          {interventionManualCount}
                                        </div>
                                        {interventionLatestSummary && (
                                          <div className="text-sm text-nofx-text-main mt-2">
                                            {interventionLatestSummary}
                                          </div>
                                        )}
                                        {interventionLatestNote && (
                                          <div className="text-xs text-nofx-text-muted mt-2">
                                            {interventionLatestNote}
                                          </div>
                                        )}
                                      </div>
                                    )}
                                  </div>
                                  <div className="text-xs text-nofx-text-muted">
                                    {patternId ? `ID ${patternId.slice(0, 12)}` : '-'}
                                  </div>
                                </div>

                                <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-6 gap-3 mt-4 text-xs">
                                  <div className="rounded-lg border border-white/10 bg-white/[0.03] p-3">
                                    {pickText('Match type', 'Treffertyp')}
                                    <div className="mt-1 font-semibold text-nofx-text-main">
                                      {formatLabel(
                                        typeof item.current_window_match_type ===
                                          'string'
                                          ? item.current_window_match_type
                                          : 'unknown'
                                      )}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-white/10 bg-white/[0.03] p-3">
                                    {pickText('Closed / recent', 'Geschlossen / aktuell')}
                                    <div className="mt-1 font-semibold text-nofx-text-main">
                                      {typeof item.closed_case_match_count ===
                                      'number'
                                        ? Math.round(
                                            item.closed_case_match_count
                                          )
                                        : '-'}{' '}
                                      /{' '}
                                      {typeof item.recent_execution_match_count ===
                                      'number'
                                        ? Math.round(
                                            item.recent_execution_match_count
                                          )
                                        : '-'}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-white/10 bg-white/[0.03] p-3">
                                    {pickText('Samples', 'Samples')}
                                    <div className="mt-1 font-semibold text-nofx-text-main">
                                      {typeof item.sample_count === 'number'
                                        ? Math.round(item.sample_count)
                                        : '-'}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-white/10 bg-white/[0.03] p-3">
                                    {pickText('Avg PnL / lift', 'Ø PnL / Lift')}
                                    <div className="mt-1 font-semibold text-nofx-text-main">
                                      {formatNumber(item.avg_pnl_pct)}% /{' '}
                                      {formatNumber(item.lift_avg_pnl_pct)}%
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-white/10 bg-white/[0.03] p-3">
                                    {pickText('Composite / confidence', 'Composite / Konfidenz')}
                                    <div className="mt-1 font-semibold text-nofx-text-main">
                                      {formatNumber(item.composite_score)} /{' '}
                                      {formatNumber(item.confidence_score)}
                                    </div>
                                  </div>
                                  <div className="rounded-lg border border-white/10 bg-white/[0.03] p-3">
                                    {pickText('Window net PnL', 'Fenster Netto-PnL')}
                                    <div className="mt-1 font-semibold text-nofx-text-main">
                                      {formatNumber(item.current_window_net_pnl)}
                                    </div>
                                  </div>
                                </div>

                                <div className="flex flex-wrap gap-2 mt-3">
                                  {readStringArray(item.feature_set).map(
                                    (feature) => (
                                      <span
                                        key={`${patternId}-${feature}`}
                                        className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted"
                                      >
                                        {feature}
                                      </span>
                                    )
                                  )}
                                </div>
                              </div>
                            )
                          })}
                        </div>
                      )}
                    </div>
                  )}
                </div>

                <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Reject Reasons', 'Ablehnungsgruende')}
                    </div>
                    {rejectReasons.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No structured skipped-candidate reject reasons were stored in this window.',
                          'In diesem Fenster wurden keine strukturierten Ablehnungsgruende fuer uebersprungene Kandidaten gespeichert.'
                        )}
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
                              {pickText('times', 'mal')}
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
                      {pickText('Opportunity Sessions', 'Chancen-Sessions')}
                    </div>
                    {opportunitySessions.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No session density was stored for this window.',
                          'Fuer dieses Fenster wurde keine Session-Dichte gespeichert.'
                        )}
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
                              {pickText('candidates', 'Kandidaten')} •{' '}
                              {typeof item.open_decision_count === 'number'
                                ? Math.round(item.open_decision_count)
                                : '-'}{' '}
                              {pickText('opens', 'Opens')} •{' '}
                              {typeof item.hold_decision_count === 'number'
                                ? Math.round(item.hold_decision_count)
                                : '-'}{' '}
                              {pickText('holds', 'Holds')} •{' '}
                              {typeof item.wait_decision_count === 'number'
                                ? Math.round(item.wait_decision_count)
                                : '-'}{' '}
                              {pickText('waits', 'Waits')}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Confidence Bands', 'Konfidenz-Baender')}
                    </div>
                    {confidenceBands.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No confidence-band telemetry was stored for this window.',
                          'Fuer dieses Fenster wurde keine Konfidenzband-Telemetrie gespeichert.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {confidenceBands.map((item, index) => (
                          <div
                            key={`${String(item.band || index)}`}
                            className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm"
                          >
                            <div className="font-semibold">
                              {typeof item.band === 'string'
                                ? item.band
                                : 'unknown'}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {typeof item.decision_count === 'number'
                                ? Math.round(item.decision_count)
                                : '-'}{' '}
                              {pickText('decisions', 'Entscheidungen')} • {pickText('avg', 'Ø')}{' '}
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
                      {pickText('Opportunity Symbols', 'Chancen-Symbole')}
                  </div>
                  {opportunitySymbols.length === 0 ? (
                    <div className="text-sm text-nofx-text-muted">
                      {pickText(
                        'No symbol-level opportunity density was stored for this run.',
                        'Fuer diesen Lauf wurde keine symbolbasierte Opportunity-Dichte gespeichert.'
                      )}
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
                                  : pickText('UNKNOWN', 'UNBEKANNT')}
                              </div>
                              <div className="text-xs text-nofx-text-muted mt-1">
                                {typeof item.candidate_count === 'number'
                                  ? Math.round(item.candidate_count)
                                  : '-'}{' '}
                                {pickText('candidates', 'Kandidaten')} •{' '}
                                {typeof item.open_decision_count === 'number'
                                  ? Math.round(item.open_decision_count)
                                  : '-'}{' '}
                                {pickText('opens', 'Opens')} •{' '}
                                {typeof item.hold_decision_count === 'number'
                                  ? Math.round(item.hold_decision_count)
                                  : '-'}{' '}
                                {pickText('holds', 'Holds')} •{' '}
                                {typeof item.wait_decision_count === 'number'
                                  ? Math.round(item.wait_decision_count)
                                  : '-'}{' '}
                                {pickText('waits', 'Waits')}
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
                            {readStringArray(item.selection_buckets).map(
                              (bucket) => (
                                <span
                                  key={`${String(item.symbol)}-${bucket}`}
                                  className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted"
                                >
                                  {formatLabel(bucket)}
                                </span>
                              )
                            )}
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

                <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Execution Statuses', 'Ausfuehrungsstatus')}
                    </div>
                    <div className="text-xs text-nofx-text-muted mb-3">
                      {typeof openDecisionCount === 'number'
                        ? Math.round(openDecisionCount)
                        : '-'}{' '}
                      {pickText('open decisions', 'Open-Entscheidungen')}
                      {typeof rejectedCandidateCount === 'number'
                        ? ` • ${Math.round(rejectedCandidateCount)} ${pickText(
                            'rejected candidates',
                            'abgelehnte Kandidaten'
                          )}`
                        : ''}
                    </div>
                    {executionStatuses.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No open-decision execution telemetry was stored in this window.',
                          'In diesem Fenster wurde keine Open-Decision-Ausfuehrungstelemetrie gespeichert.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {executionStatuses.map((item, index) => (
                          <div
                            key={`${String(item.status || index)}`}
                            className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm"
                          >
                            <div className="font-semibold">
                              {formatLabel(
                                typeof item.status === 'string'
                                  ? item.status
                                  : 'unknown'
                              )}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {typeof item.count === 'number'
                                ? Math.round(item.count)
                                : '-'}{' '}
                              {pickText('times', 'mal')}
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
                      {pickText('Recent Open Executions', 'Juengste Open-Ausfuehrungen')}
                    </div>
                    {recentOpenExecutions.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No recent open-decision execution samples were stored.',
                          'Es wurden keine aktuellen Open-Decision-Ausfuehrungsbeispiele gespeichert.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {recentOpenExecutions.map((item, index) => (
                          <div
                            key={`${String(item.symbol || index)}-${String(item.timestamp || index)}`}
                            className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm"
                          >
                            <div className="flex flex-wrap items-center gap-2">
                              <div className="font-semibold">
                                {typeof item.symbol === 'string'
                                  ? item.symbol
                                  : pickText('UNKNOWN', 'UNBEKANNT')}
                              </div>
                              <span
                                className={`inline-flex px-2 py-1 rounded-full border text-[11px] ${statusToneClasses(
                                  typeof item.terminal_status === 'string'
                                    ? item.terminal_status
                                    : ''
                                )}`}
                              >
                                {formatLabel(
                                  typeof item.terminal_status === 'string'
                                    ? item.terminal_status
                                    : 'unknown'
                                )}
                              </span>
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {typeof item.side === 'string'
                                ? formatLabel(item.side)
                                : '-'}{' '}
                              • {pickText('conf', 'Konf.')}{' '}
                              {typeof item.confidence === 'number'
                                ? Math.round(item.confidence)
                                : '-'}{' '}
                              •{' '}
                              {normalizeTime(
                                typeof item.timestamp === 'string'
                                  ? item.timestamp
                                  : ''
                              )}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {typeof item.failure_category === 'string' &&
                              item.failure_category
                                ? `${formatLabel(item.failure_category)} • `
                                : ''}
                              {typeof item.liquidity_tier === 'string' &&
                              item.liquidity_tier
                                ? `${formatLabel(item.liquidity_tier)} ${pickText(
                                    'liquidity',
                                    'Liquiditaet'
                                  )}`
                                : pickText(
                                    'No liquidity tier stored',
                                    'Keine Liquiditaetsstufe gespeichert'
                                  )}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Regime Snapshots', 'Regime-Snapshots')}
                    </div>
                    {regimeSummaries.length === 0 ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No persisted regime summary was stored for this run window.',
                          'Fuer dieses Lauf-Fenster wurde keine persistierte Regime-Zusammenfassung gespeichert.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {regimeSummaries.map((item, index) => (
                          <div
                            key={`${String(item.key || index)}`}
                            className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm"
                          >
                            <div className="font-semibold">
                              {formatLabel(
                                typeof item.trend_regime === 'string' &&
                                  item.trend_regime
                                  ? item.trend_regime
                                  : 'unknown'
                              )}{' '}
                              /{' '}
                              {formatLabel(
                                typeof item.volatility_regime === 'string' &&
                                  item.volatility_regime
                                  ? item.volatility_regime
                                  : 'unknown'
                              )}
                            </div>
                            <div className="text-xs text-nofx-text-muted mt-1">
                              {typeof item.candidate_count === 'number'
                                ? Math.round(item.candidate_count)
                                : '-'}{' '}
                              {pickText('candidates', 'Kandidaten')} •{' '}
                              {typeof item.open_decision_count === 'number'
                                ? Math.round(item.open_decision_count)
                                : '-'}{' '}
                              {pickText('opens', 'Opens')}
                            </div>
                            <div className="flex flex-wrap gap-2 mt-2">
                              {[
                                typeof item.oi_regime === 'string'
                                  ? item.oi_regime
                                  : '',
                                typeof item.funding_regime === 'string'
                                  ? item.funding_regime
                                  : '',
                                typeof item.session_bucket === 'string'
                                  ? item.session_bucket
                                  : '',
                                typeof item.liquidity_tier === 'string'
                                  ? item.liquidity_tier
                                  : '',
                              ]
                                .filter(Boolean)
                                .map((token) => (
                                  <span
                                    key={`${String(item.key)}-${token}`}
                                    className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted"
                                  >
                                    {formatLabel(token)}
                                  </span>
                                ))}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Trailing Stop Telemetry', 'Trailing-Stop-Telemetrie')}
                    </div>
                    {!trailingStopTelemetry ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No trailing-stop versus initial stop-loss telemetry was stored for this run window.',
                          'Fuer dieses Lauf-Fenster wurde keine Telemetrie fuer Trailing-Stop versus initialen Stop-Loss gespeichert.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-3">
                        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 text-sm">
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('Trailing exits', 'Trailing-Exits')}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof trailingStopTelemetry.trailing_exit_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.trailing_exit_count
                                  )
                                : '-'}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('Profit / loss', 'Gewinn / Verlust')}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof trailingStopTelemetry.trailing_profit_exit_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.trailing_profit_exit_count
                                  )
                                : '-'}{' '}
                              /{' '}
                              {typeof trailingStopTelemetry.trailing_loss_exit_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.trailing_loss_exit_count
                                  )
                                : '-'}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('Initial stop losses', 'Initiale Stop-Losses')}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof trailingStopTelemetry.initial_stop_loss_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.initial_stop_loss_count
                                  )
                                : '-'}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('Early tightening', 'Fruehes Nachziehen')}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof trailingStopTelemetry.early_tightening_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.early_tightening_count
                                  )
                                : '-'}{' '}
                              /{' '}
                              {typeof trailingStopTelemetry.early_tightening_loss_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.early_tightening_loss_count
                                  )
                                : '-'}{' '}
                              {pickText('red', 'negativ')}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('One-cycle exits', 'Ein-Zyklus-Exits')}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof trailingStopTelemetry.one_cycle_exit_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.one_cycle_exit_count
                                  )
                                : '-'}{' '}
                              /{' '}
                              {typeof trailingStopTelemetry.early_tightening_one_cycle_exit_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.early_tightening_one_cycle_exit_count
                                  )
                                : '-'}{' '}
                              {pickText('early', 'frueh')}
                            </div>
                            <div className="mt-1 text-[11px] text-nofx-text-muted">
                              {pickText('Threshold', 'Schwelle')}{' '}
                              {typeof trailingStopTelemetry.one_cycle_exit_threshold_sec ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.one_cycle_exit_threshold_sec
                                  )
                                : '-'}
                              s
                            </div>
                          </div>
                        </div>
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-sm">
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText(
                                'Avg trailing exit PnL / initial SL PnL',
                                'Ø Trailing-Exit-PnL / initiales SL-PnL'
                              )}
                            </div>
                            <div className="mt-1 font-semibold">
                              {formatNumber(
                                trailingStopTelemetry.trailing_exit_avg_pnl_pct
                              )}
                              % /{' '}
                              {formatNumber(
                                trailingStopTelemetry.initial_stop_loss_avg_pnl_pct
                              )}
                              %
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText(
                                'Avg first update / update-to-exit',
                                'Ø erstes Update / Update-bis-Exit'
                              )}
                            </div>
                            <div className="mt-1 font-semibold">
                              {formatNumber(
                                trailingStopTelemetry.avg_minutes_to_first_update
                              )}
                              m /{' '}
                              {formatNumber(
                                trailingStopTelemetry.avg_minutes_from_first_update_to_exit
                              )}
                              m
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('Audited first updates', 'Gepruefte erste Updates')}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof trailingStopTelemetry.first_update_audit_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.first_update_audit_count
                                  )
                                : '-'}{' '}
                              /{' '}
                              {typeof trailingStopTelemetry.breakeven_protected_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.breakeven_protected_count
                                  )
                                : '-'}{' '}
                              breakeven
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText(
                                'Early below entry / protected',
                                'Frueh unter Einstieg / geschuetzt'
                              )}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof trailingStopTelemetry.early_tightening_below_entry_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.early_tightening_below_entry_count
                                  )
                                : '-'}{' '}
                              /{' '}
                              {typeof trailingStopTelemetry.early_tightening_protected_count ===
                              'number'
                                ? Math.round(
                                    trailingStopTelemetry.early_tightening_protected_count
                                  )
                                : '-'}
                            </div>
                          </div>
                        </div>
                        {(trailingStopTierBreakdown.length > 0 ||
                          trailingStopProfitBandBreakdown.length > 0 ||
                          trailingStopEntryProtectionBreakdown.length > 0) && (
                          <div className="grid grid-cols-1 xl:grid-cols-3 gap-3 text-sm">
                            {trailingStopTierBreakdown.length > 0 && (
                              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                                <div className="text-xs text-nofx-text-muted mb-2">
                                  {pickText('By Tier Trigger', 'Nach Tier-Trigger')}
                                </div>
                                <div className="space-y-2">
                                  {trailingStopTierBreakdown.map((item, index) => (
                                    <div
                                      key={`${String(readNestedNumber(item, 'tier_trigger_profit_pct') ?? index)}-${String(readNestedString(item, 'trailing_mode') || index)}`}
                                      className="rounded-lg border border-white/10 bg-white/[0.03] p-3"
                                    >
                                      <div className="flex flex-wrap items-center justify-between gap-2">
                                        <div className="font-semibold">
                                          {formatNumber(
                                            readNestedNumber(
                                              item,
                                              'tier_trigger_profit_pct'
                                            )
                                          )}
                                          % {pickText('trigger', 'Trigger')}
                                        </div>
                                        <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                                          {formatLabel(
                                            String(
                                              readNestedString(
                                                item,
                                                'trailing_mode'
                                              ) || 'unknown'
                                            )
                                          )}
                                        </span>
                                      </div>
                                      <div className="mt-2 text-xs text-nofx-text-muted">
                                        {Math.round(
                                          readNestedNumber(item, 'audit_count') || 0
                                        )}{' '}
                                        {pickText('audits', 'Audits')} •{' '}
                                        {Math.round(
                                          readNestedNumber(
                                            item,
                                            'early_tightening_count'
                                          ) || 0
                                        )}{' '}
                                        {pickText('early', 'frueh')} •{' '}
                                        {Math.round(
                                          readNestedNumber(
                                            item,
                                            'early_tightening_loss_count'
                                          ) || 0
                                        )}{' '}
                                        {pickText('harmful', 'schaedlich')}
                                      </div>
                                      <div className="mt-2 text-xs text-nofx-text-muted">
                                        {pickText('Below entry', 'Unter Einstieg')}{' '}
                                        {Math.round(
                                          readNestedNumber(
                                            item,
                                            'below_entry_count'
                                          ) || 0
                                        )}{' '}
                                        • {pickText('protected', 'geschuetzt')}{' '}
                                        {Math.round(
                                          readNestedNumber(
                                            item,
                                            'breakeven_or_better_count'
                                          ) || 0
                                        )}{' '}
                                        • {pickText('one-cycle', 'ein Zyklus')}{' '}
                                        {Math.round(
                                          readNestedNumber(
                                            item,
                                            'one_cycle_exit_count'
                                          ) || 0
                                        )}
                                      </div>
                                      <div className="mt-2 text-xs text-nofx-text-muted">
                                        {pickText('Avg pre-update uPnL', 'Ø uPnL vor Update')}{' '}
                                        {formatNumber(
                                          readNestedNumber(
                                            item,
                                            'avg_pre_update_unrealized_pnl_pct'
                                          )
                                        )}
                                        %
                                      </div>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            )}
                            {trailingStopProfitBandBreakdown.length > 0 && (
                              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                                <div className="text-xs text-nofx-text-muted mb-2">
                                  {pickText('By Profit Band', 'Nach Profit-Band')}
                                </div>
                                <div className="space-y-2">
                                  {trailingStopProfitBandBreakdown.map(
                                    (item, index) => (
                                      <div
                                        key={`${String(readNestedString(item, 'profit_band') || index)}`}
                                        className="rounded-lg border border-white/10 bg-white/[0.03] p-3"
                                      >
                                        <div className="font-semibold">
                                          {String(
                                            readNestedString(
                                              item,
                                              'profit_band'
                                            ) || '-'
                                          )}
                                        </div>
                                        <div className="mt-2 text-xs text-nofx-text-muted">
                                          {Math.round(
                                            readNestedNumber(item, 'audit_count') || 0
                                          )}{' '}
                                          {pickText('audits', 'Audits')} •{' '}
                                          {Math.round(
                                            readNestedNumber(
                                              item,
                                              'early_tightening_count'
                                            ) || 0
                                          )}{' '}
                                          {pickText('early', 'frueh')} •{' '}
                                          {Math.round(
                                            readNestedNumber(
                                              item,
                                              'early_tightening_loss_count'
                                            ) || 0
                                          )}{' '}
                                          {pickText('harmful', 'schaedlich')}
                                        </div>
                                        <div className="mt-2 text-xs text-nofx-text-muted">
                                          {pickText('Loss / profit', 'Verlust / Gewinn')}{' '}
                                          {Math.round(
                                            readNestedNumber(
                                              item,
                                              'loss_exit_count'
                                            ) || 0
                                          )}{' '}
                                          /{' '}
                                          {Math.round(
                                            readNestedNumber(
                                              item,
                                              'profit_exit_count'
                                            ) || 0
                                          )}{' '}
                                          • {pickText('one-cycle', 'ein Zyklus')}{' '}
                                          {Math.round(
                                            readNestedNumber(
                                              item,
                                              'one_cycle_exit_count'
                                            ) || 0
                                          )}
                                        </div>
                                        <div className="mt-2 text-xs text-nofx-text-muted">
                                          {pickText('Avg first update', 'Ø erstes Update')}{' '}
                                          {formatNumber(
                                            readNestedNumber(
                                              item,
                                              'avg_minutes_to_first_update'
                                            )
                                          )}
                                          m
                                        </div>
                                      </div>
                                    )
                                  )}
                                </div>
                              </div>
                            )}
                            {trailingStopEntryProtectionBreakdown.length > 0 && (
                              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                                <div className="text-xs text-nofx-text-muted mb-2">
                                  {pickText('By Entry Protection', 'Nach Einstiegsschutz')}
                                </div>
                                <div className="space-y-2">
                                  {trailingStopEntryProtectionBreakdown.map(
                                    (item, index) => (
                                      <div
                                        key={`${String(readNestedString(item, 'entry_protection_state') || index)}`}
                                        className="rounded-lg border border-white/10 bg-white/[0.03] p-3"
                                      >
                                        <div className="font-semibold">
                                          {formatLabel(
                                            String(
                                              readNestedString(
                                                item,
                                                'entry_protection_state'
                                              ) || 'unknown'
                                            )
                                          )}
                                        </div>
                                        <div className="mt-2 text-xs text-nofx-text-muted">
                                          {Math.round(
                                            readNestedNumber(item, 'audit_count') || 0
                                          )}{' '}
                                          {pickText('audits', 'Audits')} •{' '}
                                          {Math.round(
                                            readNestedNumber(
                                              item,
                                              'early_tightening_count'
                                            ) || 0
                                          )}{' '}
                                          {pickText('early', 'frueh')} •{' '}
                                          {Math.round(
                                            readNestedNumber(
                                              item,
                                              'early_tightening_loss_count'
                                            ) || 0
                                          )}{' '}
                                          {pickText('harmful', 'schaedlich')}
                                        </div>
                                        <div className="mt-2 text-xs text-nofx-text-muted">
                                          {pickText('Loss / profit', 'Verlust / Gewinn')}{' '}
                                          {Math.round(
                                            readNestedNumber(
                                              item,
                                              'loss_exit_count'
                                            ) || 0
                                          )}{' '}
                                          /{' '}
                                          {Math.round(
                                            readNestedNumber(
                                              item,
                                              'profit_exit_count'
                                            ) || 0
                                          )}{' '}
                                          • {pickText('one-cycle', 'ein Zyklus')}{' '}
                                          {Math.round(
                                            readNestedNumber(
                                              item,
                                              'one_cycle_exit_count'
                                            ) || 0
                                          )}
                                        </div>
                                      </div>
                                    )
                                  )}
                                </div>
                              </div>
                            )}
                          </div>
                        )}
                        {trailingStopSamples.length > 0 && (
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted mb-2">
                              {pickText('First Update Audit', 'Audit des ersten Updates')}
                            </div>
                            <div className="space-y-2">
                              {trailingStopSamples.map((item, index) => (
                                <div
                                  key={`${String(readNestedNumber(item, 'position_id') ?? index)}-${index}`}
                                  className="rounded-lg border border-white/10 bg-white/[0.03] p-3"
                                >
                                  <div className="flex flex-wrap items-center gap-2 text-sm">
                                    <span className="font-semibold">
                                      {String(readNestedString(item, 'symbol') || '-')}
                                    </span>
                                    <span className="text-nofx-text-muted">
                                      {formatLabel(
                                        String(readNestedString(item, 'side') || '')
                                      ) || '-'}
                                    </span>
                                    <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                                      {formatLabel(
                                        String(
                                          readNestedString(item, 'close_reason') ||
                                            'unknown'
                                        )
                                      )}
                                    </span>
                                    <span className="inline-flex px-2 py-1 rounded-full border border-sky-400/20 bg-sky-500/10 text-[11px] text-sky-300">
                                      {formatNumber(
                                        readNestedNumber(
                                          item,
                                          'tier_trigger_profit_pct'
                                        )
                                      )}
                                      % {pickText('tier', 'Tier')}
                                    </span>
                                    <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                                      {String(
                                        readNestedString(
                                          item,
                                          'pre_update_profit_band'
                                        ) || '-'
                                      )}
                                    </span>
                                    <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                                      {formatLabel(
                                        String(
                                          readNestedString(
                                            item,
                                            'update_source'
                                          ) || 'unknown'
                                        )
                                      )}
                                    </span>
                                    {readNestedBoolean(item, 'protects_breakeven') && (
                                      <span className="inline-flex px-2 py-1 rounded-full border border-emerald-400/25 bg-emerald-500/15 text-[11px] text-emerald-300">
                                        {pickText('Breakeven protected', 'Breakeven geschuetzt')}
                                      </span>
                                    )}
                                    {readNestedBoolean(
                                      item,
                                      'exit_within_one_cycle'
                                    ) && (
                                      <span className="inline-flex px-2 py-1 rounded-full border border-amber-400/25 bg-amber-500/15 text-[11px] text-amber-200">
                                        {pickText('One-cycle exit', 'Ein-Zyklus-Exit')}
                                      </span>
                                    )}
                                  </div>
                                  <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-3 mt-3 text-xs text-nofx-text-muted">
                                    <div>
                                      {pickText('First update:', 'Erstes Update:')}{' '}
                                      {formatNumber(
                                        readNestedNumber(
                                          item,
                                          'minutes_to_first_update'
                                        )
                                      )}
                                      m
                                    </div>
                                    <div>
                                      {pickText('Update to exit:', 'Update bis Exit:')}{' '}
                                      {formatNumber(
                                        readNestedNumber(
                                          item,
                                          'minutes_from_update_to_exit'
                                        )
                                      )}
                                      m
                                    </div>
                                    <div>
                                      {pickText('Pre-update uPnL:', 'uPnL vor Update:')}{' '}
                                      {formatNumber(
                                        readNestedNumber(
                                          item,
                                          'pre_update_unrealized_pnl_pct'
                                        )
                                      )}
                                      % /{' '}
                                      {formatNumber(
                                        readNestedNumber(
                                          item,
                                          'pre_update_unrealized_pnl'
                                        )
                                      )}
                                    </div>
                                    <div>
                                      {pickText('Stop profit:', 'Stop-Gewinn:')}{' '}
                                      {formatNumber(
                                        readNestedNumber(item, 'stop_profit_pct')
                                      )}
                                      %
                                    </div>
                                    <div>
                                      {pickText('Entry state:', 'Einstiegsstatus:')}{' '}
                                      {formatLabel(
                                        String(
                                          readNestedString(
                                            item,
                                            'entry_protection_state'
                                          ) || 'unknown'
                                        )
                                      )}
                                    </div>
                                    <div>
                                      {pickText('Mode:', 'Modus:')}{' '}
                                      {formatLabel(
                                        String(
                                          readNestedString(
                                            item,
                                            'trailing_mode'
                                          ) || 'unknown'
                                        )
                                      )}
                                    </div>
                                  </div>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    )}
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Adaptive Re-entry Guard', 'Adaptiver Wiedereinstiegs-Guard')}
                    </div>
                    {!adaptiveCooldownTelemetry ? (
                      <div className="text-sm text-nofx-text-muted">
                        {pickText(
                          'No cooldown telemetry was stored for this run window.',
                          'Fuer dieses Lauf-Fenster wurde keine Cooldown-Telemetrie gespeichert.'
                        )}
                      </div>
                    ) : (
                      <div className="space-y-3">
                        <div className="flex flex-wrap gap-2">
                          <span
                            className={`inline-flex px-2 py-1 rounded-full border text-[11px] ${
                              readNestedBoolean(
                                adaptiveCooldownConfig,
                                'enabled'
                              )
                                ? 'border-emerald-400/25 bg-emerald-500/15 text-emerald-300'
                                : 'border-white/10 bg-white/5 text-nofx-text-muted'
                            }`}
                          >
                            {pickText('Guard', 'Guard')}{' '}
                            {readNestedBoolean(adaptiveCooldownConfig, 'enabled')
                              ? pickText('enabled', 'aktiv')
                              : pickText('disabled', 'deaktiviert')}
                          </span>
                          {readNestedBoolean(
                            adaptiveCooldownConfig,
                            'require_weak_execution_regime'
                          ) && (
                            <span className="inline-flex px-2 py-1 rounded-full border border-amber-400/25 bg-amber-500/15 text-[11px] text-amber-200">
                              {pickText('Weak regime only', 'Nur schwaches Regime')}
                            </span>
                          )}
                          {typeof readNestedNumber(
                            adaptiveCooldownConfig,
                            'same_symbol_loss_cooldown_minutes'
                          ) === 'number' && (
                            <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                              {Math.round(
                                readNestedNumber(
                                  adaptiveCooldownConfig,
                                  'same_symbol_loss_cooldown_minutes'
                                ) || 0
                              )}
                              m {pickText('cooldown', 'Cooldown')}
                            </span>
                          )}
                          {typeof readNestedNumber(
                            adaptiveCooldownConfig,
                            'pair_loss_lookback_hours'
                          ) === 'number' && (
                            <span className="inline-flex px-2 py-1 rounded-full border border-white/10 bg-white/5 text-[11px] text-nofx-text-muted">
                              {Math.round(
                                readNestedNumber(
                                  adaptiveCooldownConfig,
                                  'pair_loss_lookback_hours'
                                ) || 0
                              )}
                              h {pickText('lookback', 'Lookback')}
                            </span>
                          )}
                        </div>

                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('Cooldown candidates', 'Cooldown-Kandidaten')}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof adaptiveCooldownTelemetry.cooldown_candidate_count ===
                              'number'
                                ? Math.round(
                                    adaptiveCooldownTelemetry.cooldown_candidate_count
                                  )
                                : '-'}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('Repeat after loss', 'Wiederholung nach Verlust')}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof adaptiveCooldownTelemetry.repeat_after_loss_count ===
                              'number'
                                ? Math.round(
                                    adaptiveCooldownTelemetry.repeat_after_loss_count
                                  )
                                : '-'}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('Same-session reentries', 'Re-Entries in derselben Session')}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof adaptiveCooldownTelemetry.same_session_reentry_count ===
                              'number'
                                ? Math.round(
                                    adaptiveCooldownTelemetry.same_session_reentry_count
                                  )
                                : '-'}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText(
                                'Regime repeats after loss',
                                'Regime-Wiederholungen nach Verlust'
                              )}
                            </div>
                            <div className="mt-1 font-semibold">
                              {typeof adaptiveCooldownTelemetry.regime_repeat_loss_count ===
                              'number'
                                ? Math.round(
                                    adaptiveCooldownTelemetry.regime_repeat_loss_count
                                  )
                                : '-'}
                            </div>
                          </div>
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 text-sm">
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('Blocked symbol reentries', 'Blockierte Symbol-Re-Entries')}
                            </div>
                            <div className="mt-1">
                              {formatCooldownOutcomeSummary(
                                adaptiveCooldownBlockedSymbolOutcomes
                              )}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('After symbol cooldown', 'Nach Symbol-Cooldown')}
                            </div>
                            <div className="mt-1">
                              {formatCooldownOutcomeSummary(
                                adaptiveCooldownPostSymbolOutcomes
                              )}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('Blocked regime repeats', 'Blockierte Regime-Wiederholungen')}
                            </div>
                            <div className="mt-1">
                              {formatCooldownOutcomeSummary(
                                adaptiveCooldownBlockedRegimeOutcomes
                              )}
                            </div>
                          </div>
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted">
                              {pickText('After regime cooldown', 'Nach Regime-Cooldown')}
                            </div>
                            <div className="mt-1">
                              {formatCooldownOutcomeSummary(
                                adaptiveCooldownPostRegimeOutcomes
                              )}
                            </div>
                          </div>
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted mb-2">
                              {pickText('Top Symbols', 'Top Symbole')}
                            </div>
                            {adaptiveCooldownTopSymbols.length === 0 ? (
                              <div className="text-sm text-nofx-text-muted">
                                {pickText(
                                  'No repeated symbol re-entry pattern was stored.',
                                  'Es wurde kein wiederholtes Symbol-Re-Entry-Muster gespeichert.'
                                )}
                              </div>
                            ) : (
                              <div className="space-y-2">
                                {adaptiveCooldownTopSymbols.map((item, index) => (
                                  <div
                                    key={`${String(item.symbol || index)}`}
                                    className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm"
                                  >
                                    <div className="font-semibold">
                                      {typeof item.symbol === 'string'
                                        ? item.symbol
                                        : pickText('UNKNOWN', 'UNBEKANNT')}
                                    </div>
                                    <div className="text-xs text-nofx-text-muted mt-1">
                                      {typeof item.reentry_count === 'number'
                                        ? Math.round(item.reentry_count)
                                        : '-'}{' '}
                                      {pickText('reentries', 'Re-Entries')} •{' '}
                                      {typeof item.repeat_after_loss_count ===
                                      'number'
                                        ? Math.round(
                                            item.repeat_after_loss_count
                                          )
                                        : '-'}{' '}
                                      {pickText('after loss', 'nach Verlust')} • {pickText('avg', 'Ø')}{' '}
                                      {formatNumber(item.avg_pnl_pct)}%
                                    </div>
                                    <div className="text-xs text-nofx-text-muted mt-2">
                                      {pickText('Blocked:', 'Blockiert:')} {formatCooldownOutcomeSummary(
                                        readNestedObject(item, 'blocked_outcomes')
                                      )}
                                    </div>
                                    <div className="text-xs text-nofx-text-muted mt-1">
                                      {pickText('After cooldown:', 'Nach Cooldown:')}{' '}
                                      {formatCooldownOutcomeSummary(
                                        readNestedObject(
                                          item,
                                          'post_cooldown_outcomes'
                                        )
                                      )}
                                    </div>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>

                          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                            <div className="text-xs text-nofx-text-muted mb-2">
                              {pickText('Top Regimes', 'Top Regime')}
                            </div>
                            {adaptiveCooldownTopRegimes.length === 0 ? (
                              <div className="text-sm text-nofx-text-muted">
                                {pickText(
                                  'No repeated regime-loss pattern was stored.',
                                  'Es wurde kein wiederholtes Regime-Verlust-Muster gespeichert.'
                                )}
                              </div>
                            ) : (
                              <div className="space-y-2">
                                {adaptiveCooldownTopRegimes.map((item, index) => (
                                  <div
                                    key={`${String(item.trend_regime || index)}-${String(item.volatility_regime || index)}-${String(item.oi_regime || index)}`}
                                    className="rounded-lg border border-white/10 bg-black/20 p-3 text-sm"
                                  >
                                    <div className="font-semibold">
                                      {formatLabel(
                                        typeof item.trend_regime === 'string'
                                          ? item.trend_regime
                                          : 'unknown'
                                      )}{' '}
                                      /{' '}
                                      {formatLabel(
                                        typeof item.volatility_regime ===
                                          'string'
                                          ? item.volatility_regime
                                          : 'unknown'
                                      )}{' '}
                                      /{' '}
                                      {formatLabel(
                                        typeof item.oi_regime === 'string'
                                          ? item.oi_regime
                                          : 'unknown'
                                      )}
                                    </div>
                                    <div className="text-xs text-nofx-text-muted mt-1">
                                      {typeof item.reentry_count === 'number'
                                        ? Math.round(item.reentry_count)
                                        : '-'}{' '}
                                      {pickText('repeats', 'Wiederholungen')} •{' '}
                                      {typeof item.repeat_after_loss_count ===
                                      'number'
                                        ? Math.round(
                                            item.repeat_after_loss_count
                                          )
                                        : '-'}{' '}
                                      {pickText('after loss', 'nach Verlust')} • {pickText('avg', 'Ø')}{' '}
                                      {formatNumber(item.avg_pnl_pct)}%
                                    </div>
                                    <div className="text-xs text-nofx-text-muted mt-2">
                                      {pickText('Blocked:', 'Blockiert:')} {formatCooldownOutcomeSummary(
                                        readNestedObject(item, 'blocked_outcomes')
                                      )}
                                    </div>
                                    <div className="text-xs text-nofx-text-muted mt-1">
                                      {pickText('After cooldown:', 'Nach Cooldown:')}{' '}
                                      {formatCooldownOutcomeSummary(
                                        readNestedObject(
                                          item,
                                          'post_cooldown_outcomes'
                                        )
                                      )}
                                    </div>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                </div>

                {(criticSummary ||
                  proposalExpectedEffect ||
                  (selectedRunDetail.gate_reasons &&
                    selectedRunDetail.gate_reasons.length > 0) ||
                  configValidationStatus) && (
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Gate And Critic', 'Gate und Kritiker')}
                    </div>
                    <div className="grid grid-cols-1 xl:grid-cols-2 gap-4 text-sm">
                      <div className="space-y-2">
                        <div className="text-nofx-text-muted">
                          {pickText('Critic summary', 'Critic-Zusammenfassung')}
                        </div>
                        <div>{criticSummary || '-'}</div>
                        <div className="text-nofx-text-muted pt-2">
                          {pickText('Expected effect', 'Erwarteter Effekt')}
                        </div>
                        <div>{proposalExpectedEffect || '-'}</div>
                        <div className="text-nofx-text-muted pt-2">
                          {pickText('Config validation', 'Config-Validierung')}
                        </div>
                        <div>{formatLabel(configValidationStatus) || '-'}</div>
                        <div className="text-nofx-text-muted pt-2">
                          {pickText('Prompt changed fields', 'Geaenderte Prompt-Felder')}
                        </div>
                        <div>
                          {typeof promptValidationIssues === 'number'
                            ? Math.round(promptValidationIssues)
                            : '-'}
                        </div>
                        <div className="text-nofx-text-muted pt-2">
                          {pickText('Prompt requested fields', 'Angeforderte Prompt-Felder')}
                        </div>
                        <div>
                          {typeof promptRequestedFieldCount === 'number'
                            ? Math.round(promptRequestedFieldCount)
                            : '-'}
                        </div>
                        <div className="text-nofx-text-muted pt-2">
                          {pickText('Prompt auto-trim', 'Prompt Auto-Trim')}
                        </div>
                        <div>{promptAutoTrimmed ? ot('Yes') : ot('No')}</div>
                      </div>
                      <div>
                        <div className="text-nofx-text-muted mb-2">
                          {pickText('Gate reasons', 'Gate-Gruende')}
                        </div>
                        {!selectedRunDetail.gate_reasons ||
                        selectedRunDetail.gate_reasons.length === 0 ? (
                          <div className="text-sm text-emerald-300">
                            {pickText(
                              'No blocking or explanatory gate reasons stored.',
                              'Keine blockierenden oder erklaerenden Gate-Gruende gespeichert.'
                            )}
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
                        <div className="text-nofx-text-muted mt-4 mb-2">
                          {pickText('Deferred prompt fields', 'Zurueckgestellte Prompt-Felder')}
                        </div>
                        {promptDeferredFields.length === 0 ? (
                          <div className="text-sm text-nofx-text-muted">
                            {pickText('No deferred prompt fields.', 'Keine zurueckgestellten Prompt-Felder.')}
                          </div>
                        ) : (
                          <div className="flex flex-wrap gap-2">
                            {promptDeferredFields.map((item) => (
                              <span
                                key={item}
                                className="inline-flex px-2 py-1 rounded-full border border-amber-400/20 bg-amber-500/10 text-[11px] text-amber-200"
                              >
                                {item}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  </div>
                )}

                {(proposalConversation || criticConversation) && (
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Conversation Memory', 'Konversationsspeicher')}
                    </div>
                    <div className="grid grid-cols-1 xl:grid-cols-2 gap-4 text-sm">
                      {[proposalConversation, criticConversation]
                        .filter(
                          (
                            item
                          ): item is Record<string, unknown> => Boolean(item)
                        )
                        .map((item) => {
                          const purpose =
                            typeof item.purpose === 'string'
                              ? item.purpose
                              : 'conversation'
                          return (
                            <div
                              key={purpose}
                              className="rounded-lg border border-white/10 bg-black/20 p-3"
                            >
                              <div className="font-semibold">
                                {formatLabel(purpose)}
                              </div>
                              <div className="grid grid-cols-2 gap-3 mt-3 text-xs">
                                <div>
                                  <div className="text-nofx-text-muted">
                                    {pickText('Mode', 'Modus')}
                                  </div>
                                  <div className="mt-1">
                                    {typeof item.mode === 'string'
                                      ? formatLabel(item.mode)
                                      : '-'}
                                  </div>
                                </div>
                                <div>
                                  <div className="text-nofx-text-muted">
                                    {pickText('Replay messages', 'Replay-Nachrichten')}
                                  </div>
                                  <div className="mt-1">
                                    {typeof item.history_messages_replayed ===
                                    'number'
                                      ? Math.round(
                                          item.history_messages_replayed
                                        )
                                      : '-'}
                                  </div>
                                </div>
                                <div>
                                  <div className="text-nofx-text-muted">
                                    {pickText('Replay limit', 'Replay-Limit')}
                                  </div>
                                  <div className="mt-1">
                                    {typeof item.replay_message_limit ===
                                    'number'
                                      ? Math.round(item.replay_message_limit)
                                      : '-'}
                                  </div>
                                </div>
                                <div>
                                  <div className="text-nofx-text-muted">
                                    {pickText('Conversation ID', 'Conversation-ID')}
                                  </div>
                                  <div className="mt-1 break-all">
                                    {typeof item.conversation_id === 'string'
                                      ? item.conversation_id
                                      : '-'}
                                  </div>
                                </div>
                              </div>
                            </div>
                          )
                        })}
                    </div>
                  </div>
                )}

                <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Strategy Diff', 'Strategie-Diff')}
                    </div>
                    {renderDiffList(selectedRunDetail.strategy_differences, language)}
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Trader Prompt Diff', 'Trader-Prompt-Diff')}
                    </div>
                    {renderDiffList(selectedRunDetail.trader_differences, language)}
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Optimizer Prompt Diff', 'Optimizer-Prompt-Diff')}
                    </div>
                    {renderDiffList(selectedRunDetail.optimizer_differences, language)}
                  </div>
                </div>

                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Proposed Config Patch', 'Vorgeschlagener Config-Patch')}
                    </div>
                    <pre className="text-xs whitespace-pre-wrap break-words text-nofx-text-muted">
                      {prettyJSON(selectedRunDetail.config_patch)}
                    </pre>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Proposed Prompt Patch', 'Vorgeschlagener Prompt-Patch')}
                    </div>
                    <pre className="text-xs whitespace-pre-wrap break-words text-nofx-text-muted">
                      {prettyJSON(selectedRunDetail.prompt_patch)}
                    </pre>
                  </div>
                </div>

                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Validation Payload', 'Validierungs-Payload')}
                    </div>
                    <pre className="text-xs whitespace-pre-wrap break-words text-nofx-text-muted">
                      {prettyJSON(selectedRunDetail.validation)}
                    </pre>
                  </div>
                  <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted mb-3">
                      {pickText('Metadata Payload', 'Metadaten-Payload')}
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
