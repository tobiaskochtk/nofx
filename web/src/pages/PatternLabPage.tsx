import { useEffect, useState } from 'react'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { useLanguage } from '../contexts/LanguageContext'
import { toDateTimeLocale } from '../i18n/locale'
import { NofxSelect } from '../components/ui/select'
import { api } from '../lib/api'
import { notify } from '../lib/notify'
import type { Language } from '../i18n/translations'
import type {
  DealReviewLearnedPattern,
  DealReviewLearnedPatternSummary,
  TraderInfo,
} from '../types'

interface PatternLabPageProps {
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
}

interface PatternLabFilters {
  patternId: string
  stableKey: string
  symbol: string
  side: string
  scopeType: string
  patternClass: string
  validationLabel: string
  feature: string
  regime: string
  directLiveActionCandidate: string
  liveActionKind: string
  interventionState: string
  minConfidencePct: string
  minDriftPct: string
  minSampleCount: string
  limit: string
}

const patternLabTranslations: Record<string, Partial<Record<Language, string>>> =
  {
    'All sides': { de: 'Alle Richtungen' },
    'All scopes': { de: 'Alle Bereiche' },
    Global: { de: 'Global' },
    'Trader local': { de: 'Trader-lokal' },
    'Regime local': { de: 'Regime-lokal' },
    'Symbol override': { de: 'Symbol-Override' },
    'All pattern classes': { de: 'Alle Musterklassen' },
    'Positive edge': { de: 'Positiver Vorteil' },
    'Anti-edge': { de: 'Negativer Vorteil' },
    'All labels': { de: 'Alle Labels' },
    'All live-action kinds': { de: 'Alle Live-Aktionsarten' },
    'All intervention states': { de: 'Alle Interventionszustaende' },
    'All candidate states': { de: 'Alle Kandidatenzustaende' },
    Confirmed: { de: 'Bestaetigt' },
    Candidate: { de: 'Kandidat' },
    'Need evidence': { de: 'Mehr Belege noetig' },
    'False positive': { de: 'Falsch positiv' },
    'Reverse risk': { de: 'Umkehrrisiko' },
    Drifting: { de: 'Abdriftend' },
    Expired: { de: 'Abgelaufen' },
    '24 items': { de: '24 Eintraege' },
    '50 items': { de: '50 Eintraege' },
    '100 items': { de: '100 Eintraege' },
    Protective: { de: 'Schuetzend' },
    Overblocking: { de: 'Ueberblockierend' },
    Mixed: { de: 'Gemischt' },
    Pending: { de: 'Ausstehend' },
    Improving: { de: 'Verbessernd' },
    Stable: { de: 'Stabil' },
    Degrading: { de: 'Verschlechternd' },
    Regime: { de: 'Regime' },
    'Newly overblocking': { de: 'Neu ueberblockierend' },
    'Review hint': { de: 'Review-Hinweis' },
    'Prompt hint': { de: 'Prompt-Hinweis' },
    'Config candidate': { de: 'Konfig-Kandidat' },
    'Monitoring rule': { de: 'Monitoring-Regel' },
    'Monitor only': { de: 'Nur beobachten' },
    Active: { de: 'Aktiv' },
    'Rollback watch': { de: 'Rollback-Watch' },
    guard: { de: 'Guard' },
    snapshot: { de: 'Snapshot' },
    Live: { de: 'Live' },
    Suppressed: { de: 'Unterdrueckt' },
    Retired: { de: 'Stillgelegt' },
    'Keep live': { de: 'Live lassen' },
    Suppress: { de: 'Unterdruecken' },
    Retire: { de: 'Stilllegen' },
    'Re-arm': { de: 'Reaktivieren' },
    'Suggest suppress': { de: 'Unterdruecken empfehlen' },
    'Suggest retire': { de: 'Stilllegung empfehlen' },
    'Suggest re-arm': { de: 'Reaktivierung empfehlen' },
    'Suggest keep live': { de: 'Live lassen empfehlen' },
    'Direct suppression candidate': { de: 'Direkter Suppress-Kandidat' },
    'Rollback-grade candidate': { de: 'Rollback-Kandidat' },
    'Only direct candidates': { de: 'Nur direkte Kandidaten' },
    'Open interventions': { de: 'Offene Interventionen' },
    'Resolved interventions': { de: 'Abgeschlossene Interventionen' },
    'No interventions': { de: 'Keine Interventionen' },
    Critical: { de: 'Kritisch' },
    Warning: { de: 'Warnung' },
    Info: { de: 'Info' },
    Suggested: { de: 'Vorgeschlagen' },
    'Manual action': { de: 'Manuelle Aktion' },
    Open: { de: 'Offen' },
    Accepted: { de: 'Akzeptiert' },
    Overridden: { de: 'Ueberschrieben' },
    Superseded: { de: 'Ersetzt' },
    Cleared: { de: 'Bereinigt' },
    Standalone: { de: 'Eigenstaendig' },
    'Failed to fetch learned patterns': {
      de: 'Gelernte Muster konnten nicht geladen werden',
    },
    'Failed to load traders': { de: 'Trader konnten nicht geladen werden' },
    'Pattern Lab': { de: 'Musterlabor' },
    'Learned pattern mining and drift watch': {
      de: 'Gelernte Musteranalyse und Drift-Watch',
    },
    'Inspect learned feature combinations separately from deal review: positive edges, anti-patterns, symbol overrides, reverse-risk, and drift under the currently selected trader.':
      {
        de: 'Pruefe gelernte Merkmalskombinationen getrennt vom Deal-Review: positive Vorteile, Anti-Muster, Symbol-Overrides, Umkehrrisiko und Drift fuer den aktuell ausgewaehlten Trader.',
      },
    Trader: { de: 'Trader' },
    'Open Deal Review': { de: 'Deal-Review oeffnen' },
    Filters: { de: 'Filter' },
    'Filter the learned pattern corpus by symbol, side, scope, class, validation label, feature token, regime token, confidence, drift, and minimum sample size.':
      {
        de: 'Filtere den gelernten Musterkorpus nach Symbol, Richtung, Bereich, Klasse, Validierungslabel, Feature-Token, Regime-Token, Konfidenz, Drift und Mindestanzahl an Samples.',
      },
    'No trader selected': { de: 'Kein Trader ausgewaehlt' },
    generated: { de: 'erstellt' },
    Symbol: { de: 'Symbol' },
    'Feature token, e.g. bucket_breakout': {
      de: 'Feature-Token, z. B. bucket_breakout',
    },
    'Regime token, e.g. trend_uptrend': {
      de: 'Regime-Token, z. B. trend_uptrend',
    },
    'Min confidence %': { de: 'Min. Konfidenz %' },
    'Min drift %': { de: 'Min. Drift %' },
    'Min samples': { de: 'Min. Samples' },
    Apply: { de: 'Anwenden' },
    Reset: { de: 'Zuruecksetzen' },
    'All patterns': { de: 'Alle Muster' },
    'Positive edges': { de: 'Positive Vorteile' },
    'Anti-patterns': { de: 'Anti-Muster' },
    'Confidence 70%+': { de: 'Konfidenz 70%+' },
    'Drift 45%+': { de: 'Drift 45%+' },
    'Symbol overrides': { de: 'Symbol-Overrides' },
    'Open strong candidates': { de: 'Offene starke Kandidaten' },
    'Rollback-grade candidates': { de: 'Rollback-Kandidaten' },
    'Open interventions only': { de: 'Nur offene Interventionen' },
    'Direct pattern focus active:': {
      de: 'Direkter Musterfokus aktiv:',
    },
    'Tracked patterns': { de: 'Verfolgte Muster' },
    'Reverse / drift': { de: 'Umkehr / Drift' },
    'Monitoring rules': { de: 'Monitoring-Regeln' },
    'Degrading live rules': { de: 'Schlechter werdende Live-Regeln' },
    'Direct live actions': { de: 'Direkte Live-Aktionen' },
    'Open direct candidates': { de: 'Offene Direktkandidaten' },
    'Suppression candidates': { de: 'Unterdrueckungs-Kandidaten' },
    'Fragile live rules': { de: 'Fragile Live-Regeln' },
    'Lagging guard trails': { de: 'Nachlaufende Guard-Spuren' },
    'Protective live rules': { de: 'Schuetzende Live-Regeln' },
    'Overblocking live rules': { de: 'Ueberblockierende Live-Regeln' },
    'Improving live rules': { de: 'Verbessernde Live-Regeln' },
    'Direct live-action candidates': { de: 'Direkte Live-Aktionskandidaten' },
    'No direct live-action candidates surfaced for this slice yet.': {
      de: 'Fuer diesen Ausschnitt wurden noch keine direkten Live-Aktionskandidaten gefunden.',
    },
    'Loaded items': { de: 'Geladene Eintraege' },
    'Top Positive Patterns': { de: 'Top positive Muster' },
    'No positive patterns surfaced for this slice yet.': {
      de: 'Fuer diesen Ausschnitt wurden noch keine positiven Muster gefunden.',
    },
    'Top Anti-Patterns': { de: 'Top Anti-Muster' },
    'No anti-patterns surfaced for this slice yet.': {
      de: 'Fuer diesen Ausschnitt wurden noch keine Anti-Muster gefunden.',
    },
    'Highest Risk Watchlist': { de: 'Watchlist mit hoechstem Risiko' },
    'No false-positive, reverse-risk, or drifting patterns in this slice yet.':
      {
        de: 'In diesem Ausschnitt gibt es noch keine falsch-positiven, umkehrgefaehrdeten oder driftenden Muster.',
      },
    'Symbol Overrides': { de: 'Symbol-Overrides' },
    'No symbol-specific overrides surfaced for this slice yet.': {
      de: 'Fuer diesen Ausschnitt wurden noch keine symbolspezifischen Overrides gefunden.',
    },
    'Biggest Recent Drifts': { de: 'Groesste aktuelle Drifts' },
    'Expiring Monitoring Rules': { de: 'Auslaufende Monitoring-Regeln' },
    'No monitoring rules are currently expiring in this slice.': {
      de: 'In diesem Ausschnitt laufen derzeit keine Monitoring-Regeln aus.',
    },
    'Improving Rules': { de: 'Sich verbessernde Regeln' },
    'No monitoring rules are currently improving versus the prior window.': {
      de: 'Gegenueber dem vorigen Fenster verbessern sich derzeit keine Monitoring-Regeln.',
    },
    'Degrading Rules': { de: 'Sich verschlechternde Regeln' },
    'No monitoring rules are currently degrading versus the prior window.': {
      de: 'Gegenueber dem vorigen Fenster verschlechtern sich derzeit keine Monitoring-Regeln.',
    },
    'Overblocking Rules': { de: 'Ueberblockierende Regeln' },
    'No monitoring rules currently look overblocking in this slice.': {
      de: 'In diesem Ausschnitt wirken derzeit keine Monitoring-Regeln ueberblockierend.',
    },
    'No live monitoring rules are on rollback watch in this slice.': {
      de: 'In diesem Ausschnitt stehen derzeit keine Live-Monitoring-Regeln auf Rollback-Watch.',
    },
    'Fragile Monitoring Rules': { de: 'Fragile Monitoring-Regeln' },
    'No monitoring rules look structurally fragile in this slice.': {
      de: 'In diesem Ausschnitt wirken derzeit keine Monitoring-Regeln strukturell fragil.',
    },
    'Pattern corpus': { de: 'Musterkorpus' },
    'Full list for the current slice. Use these cards to inspect feature combinations and jump directly into deal review for the matching cohort.':
      {
        de: 'Vollstaendige Liste fuer den aktuellen Ausschnitt. Mit diesen Karten kannst du Merkmalskombinationen pruefen und direkt in das passende Deal-Review-Kohortenfenster springen.',
      },
    'Refreshing…': { de: 'Aktualisiere…' },
    'Saving…': { de: 'Speichere…' },
    'item(s) loaded': { de: 'Eintrag/Eintraege geladen' },
    'No learned patterns matched the current filter slice yet.': {
      de: 'Noch keine gelernten Muster passen auf den aktuellen Filterausschnitt.',
    },
    Analyst: { de: 'Analyst' },
    'No regime signature stored': {
      de: 'Keine Regime-Signatur gespeichert',
    },
    built: { de: 'erstellt' },
    observed: { de: 'beobachtet' },
    'Lifecycle Trend': { de: 'Lifecycle-Trend' },
    Fragile: { de: 'Fragil' },
    snapshots: { de: 'Snapshots' },
    span: { de: 'Dauer' },
    active: { de: 'aktiv' },
    degrading: { de: 'verschlechternd' },
    rollback: { de: 'Rollback' },
    'stale lag': { de: 'veralteter Lag' },
    'Live-guard attribution': { de: 'Live-Guard-Zuordnung' },
    events: { de: 'Ereignisse' },
    resolved: { de: 'geloest' },
    protective: { de: 'schuetzend' },
    overblocking: { de: 'ueberblockierend' },
    pending: { de: 'ausstehend' },
    'follow-up open': { de: 'Follow-up offen' },
    'Live-guard trend': { de: 'Live-Guard-Trend' },
    recent: { de: 'aktuell' },
    'recent support': { de: 'aktueller Support' },
    prior: { de: 'zuvor' },
    delta: { de: 'Delta' },
    confidence: { de: 'Konfidenz' },
    'Suggested analyst action': { de: 'Empfohlene Analystenaktion' },
    'Use suggested note': { de: 'Empfohlene Notiz uebernehmen' },
    'Lifecycle Trail': { de: 'Lifecycle-Verlauf' },
    'Analyst Control': { de: 'Analystensteuerung' },
    Base: { de: 'Basis' },
    effective: { de: 'effektiv' },
    Inherit: { de: 'Vererben' },
    'Last action': { de: 'Letzte Aktion' },
    'Intervention History': { de: 'Interventionsverlauf' },
    Applied: { de: 'Angewendet' },
    seen: { de: 'gesehen' },
    'Why should this rule stay live, be suppressed, retired, or re-armed?':
      {
        de: 'Warum soll diese Regel live bleiben, unterdrueckt, stillgelegt oder reaktiviert werden?',
      },
    Samples: { de: 'Samples' },
    Support: { de: 'Support' },
    'Avg PnL': { de: 'Ø PnL' },
    Lift: { de: 'Lift' },
    Confidence: { de: 'Konfidenz' },
    Stability: { de: 'Stabilitaet' },
    Holdout: { de: 'Holdout' },
    Recent: { de: 'Aktuell' },
    Drift: { de: 'Drift' },
    Reverse: { de: 'Umkehr' },
    'Avg hold': { de: 'Ø Haltedauer' },
    'Open cohort in review': { de: 'Kohorte im Review oeffnen' },
    Focus: { de: 'Fokus' },
    'Pattern signature copied': { de: 'Mustersignatur kopiert' },
    'Failed to copy pattern signature': {
      de: 'Mustersignatur konnte nicht kopiert werden',
    },
    'Copy signature': { de: 'Signatur kopieren' },
    Evidence: { de: 'Belege' },
    'Open deal': { de: 'Deal oeffnen' },
    avg: { de: 'Ø' },
    lift: { de: 'Lift' },
    drift: { de: 'Drift' },
    reverse: { de: 'Umkehr' },
    FP: { de: 'FP' },
    Deals: { de: 'Deals' },
    expiry: { de: 'Ablauf' },
    validation: { de: 'Validierung' },
    'protective delta': { de: 'Schutz-Delta' },
    'overblocking delta': { de: 'Ueberblockierungs-Delta' },
    'guard hits': { de: 'Guard-Treffer' },
    'hard block': { de: 'Harter Block' },
    degrade: { de: 'Abbau' },
    changes: { de: 'Aenderungen' },
    'A short analyst note is required.': {
      de: 'Eine kurze Analystennotiz ist erforderlich.',
    },
    'Failed to apply learned-pattern control': {
      de: 'Learned-Pattern-Steuerung konnte nicht angewendet werden',
    },
    'No suggested action is available for this pattern.': {
      de: 'Fuer dieses Muster ist keine empfohlene Aktion verfuegbar.',
    },
    'Saved {action} control.': {
      de: '{action}-Steuerung gespeichert.',
    },
  }

function patternLabText(language: Language, value: string): string {
  return patternLabTranslations[value]?.[language] || value
}

function createSideOptions(language: Language) {
  return [
    { value: '', label: patternLabText(language, 'All sides') },
    { value: 'LONG', label: 'LONG' },
    { value: 'SHORT', label: 'SHORT' },
  ]
}

function createScopeOptions(language: Language) {
  return [
    { value: '', label: patternLabText(language, 'All scopes') },
    { value: 'global', label: patternLabText(language, 'Global') },
    { value: 'trader_local', label: patternLabText(language, 'Trader local') },
    { value: 'regime_local', label: patternLabText(language, 'Regime local') },
    { value: 'symbol', label: patternLabText(language, 'Symbol override') },
  ]
}

function createPatternClassOptions(language: Language) {
  return [
    {
      value: '',
      label: patternLabText(language, 'All pattern classes'),
    },
    { value: 'positive_edge', label: patternLabText(language, 'Positive edge') },
    { value: 'negative_edge', label: patternLabText(language, 'Anti-edge') },
  ]
}

function createValidationLabelOptions(language: Language) {
  return [
    { value: '', label: patternLabText(language, 'All labels') },
    { value: 'confirmed', label: patternLabText(language, 'Confirmed') },
    { value: 'candidate', label: patternLabText(language, 'Candidate') },
    {
      value: 'insufficient_evidence',
      label: patternLabText(language, 'Need evidence'),
    },
    {
      value: 'false_positive',
      label: patternLabText(language, 'False positive'),
    },
    {
      value: 'reverse_risk',
      label: patternLabText(language, 'Reverse risk'),
    },
    { value: 'drifting', label: patternLabText(language, 'Drifting') },
    { value: 'expired', label: patternLabText(language, 'Expired') },
  ]
}

function createLiveActionKindOptions(language: Language) {
  return [
    { value: '', label: patternLabText(language, 'All live-action kinds') },
    {
      value: 'suppression',
      label: patternLabText(language, 'Direct suppression candidate'),
    },
    {
      value: 'rollback',
      label: patternLabText(language, 'Rollback-grade candidate'),
    },
  ]
}

function createInterventionStateOptions(language: Language) {
  return [
    { value: '', label: patternLabText(language, 'All intervention states') },
    { value: 'open', label: patternLabText(language, 'Open interventions') },
    {
      value: 'resolved',
      label: patternLabText(language, 'Resolved interventions'),
    },
    { value: 'none', label: patternLabText(language, 'No interventions') },
  ]
}

function createDirectCandidateOptions(language: Language) {
  return [
    { value: '', label: patternLabText(language, 'All candidate states') },
    {
      value: 'only',
      label: patternLabText(language, 'Only direct candidates'),
    },
  ]
}

function createLimitOptions(language: Language) {
  return [
    { value: '24', label: patternLabText(language, '24 items') },
    { value: '50', label: patternLabText(language, '50 items') },
    { value: '100', label: patternLabText(language, '100 items') },
  ]
}

function createDefaultFilters(): PatternLabFilters {
  return {
    patternId: '',
    stableKey: '',
    symbol: '',
    side: '',
    scopeType: '',
    patternClass: '',
    validationLabel: '',
    feature: '',
    regime: '',
    directLiveActionCandidate: '',
    liveActionKind: '',
    interventionState: '',
    minConfidencePct: '',
    minDriftPct: '',
    minSampleCount: '',
    limit: '50',
  }
}

function formatMoney(value: number): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}`
}

function formatPct(value: number): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
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

function formatTimestamp(value: string | undefined, language: Language): string {
  if (!value || value.startsWith('0001-01-01')) return '-'
  return new Date(value).toLocaleString(toDateTimeLocale(language))
}

function formatCompactTimestamp(
  value: string | undefined,
  language: Language
): string {
  if (!value || value.startsWith('0001-01-01')) return '-'
  return new Date(value).toLocaleString(toDateTimeLocale(language), {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatHoursCompact(value?: number): string {
  if (typeof value !== 'number' || Number.isNaN(value) || value <= 0) return '-'
  if (value < 24) return `${value.toFixed(value < 10 ? 1 : 0)}h`
  const days = value / 24
  return `${days.toFixed(days < 10 ? 1 : 0)}d`
}

function formatLiveGuardAttributionLabel(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'protective':
      return patternLabText(language, 'Protective')
    case 'overblocking':
      return patternLabText(language, 'Overblocking')
    case 'mixed':
      return patternLabText(language, 'Mixed')
    case 'pending':
      return patternLabText(language, 'Pending')
    default:
      return value || '-'
  }
}

function liveGuardAttributionClasses(value?: string): string {
  switch (value) {
    case 'protective':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'overblocking':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'mixed':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'pending':
      return 'border-white/10 bg-white/5 text-nofx-text-muted'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatLiveGuardTrendLabel(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'improving':
      return patternLabText(language, 'Improving')
    case 'stable':
      return patternLabText(language, 'Stable')
    case 'degrading':
      return patternLabText(language, 'Degrading')
    case 'newly_overblocking':
      return patternLabText(language, 'Newly overblocking')
    case 'insufficient_evidence':
      return patternLabText(language, 'Need evidence')
    default:
      return value || '-'
  }
}

function liveGuardTrendClasses(value?: string): string {
  switch (value) {
    case 'improving':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'stable':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    case 'degrading':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'newly_overblocking':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'insufficient_evidence':
      return 'border-white/10 bg-white/5 text-nofx-text-muted'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatPatternClass(value: string | undefined, language: Language): string {
  switch (value) {
    case 'positive_edge':
      return patternLabText(language, 'Positive edge')
    case 'negative_edge':
      return patternLabText(language, 'Anti-edge')
    default:
      return value || '-'
  }
}

function patternClassClasses(value?: string): string {
  switch (value) {
    case 'positive_edge':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'negative_edge':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatValidationLabel(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'confirmed':
      return patternLabText(language, 'Confirmed')
    case 'candidate':
      return patternLabText(language, 'Candidate')
    case 'insufficient_evidence':
      return patternLabText(language, 'Need evidence')
    case 'false_positive':
      return patternLabText(language, 'False positive')
    case 'reverse_risk':
      return patternLabText(language, 'Reverse risk')
    case 'drifting':
      return patternLabText(language, 'Drifting')
    case 'expired':
      return patternLabText(language, 'Expired')
    default:
      return value || '-'
  }
}

function validationLabelClasses(value?: string): string {
  switch (value) {
    case 'confirmed':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'false_positive':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'reverse_risk':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'drifting':
      return 'border-orange-400/20 bg-orange-500/10 text-orange-200'
    case 'expired':
      return 'border-white/10 bg-white/5 text-nofx-text-muted'
    default:
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
  }
}

function formatPatternScope(
  pattern: DealReviewLearnedPattern | null | undefined,
  language: Language
): string {
  if (!pattern) return '-'
  if (pattern.scope_type === 'global') {
    return patternLabText(language, 'Global')
  }
  if (pattern.scope_type === 'regime_local') {
    return pattern.regime_signature
      ? `${patternLabText(language, 'Regime')} ${pattern.regime_signature}`
      : patternLabText(language, 'Regime local')
  }
  if (pattern.scope_type === 'symbol') {
    return pattern.symbol
      ? `${patternLabText(language, 'Symbol')} ${pattern.symbol}`
      : patternLabText(language, 'Symbol')
  }
  return patternLabText(language, 'Trader local')
}

function formatRecommendedUse(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'review_hint':
      return patternLabText(language, 'Review hint')
    case 'prompt_hint':
      return patternLabText(language, 'Prompt hint')
    case 'config_candidate':
      return patternLabText(language, 'Config candidate')
    case 'monitoring_rule':
      return patternLabText(language, 'Monitoring rule')
    case 'monitor_only':
      return patternLabText(language, 'Monitor only')
    case 'expired_do_not_use':
      return patternLabText(language, 'Expired')
    default:
      return value || '-'
  }
}

function formatLifecycleStatus(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'active':
      return patternLabText(language, 'Active')
    case 'degrading':
      return patternLabText(language, 'Degrading')
    case 'rollback_watch':
      return patternLabText(language, 'Rollback watch')
    case 'expired':
      return patternLabText(language, 'Expired')
    default:
      return value || '-'
  }
}

function lifecycleStatusClasses(value?: string): string {
  switch (value) {
    case 'active':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'degrading':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'rollback_watch':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'expired':
      return 'border-white/10 bg-white/5 text-nofx-text-muted'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function normalizeFilters(raw: PatternLabFilters): PatternLabFilters {
  return {
    patternId: raw.patternId.trim(),
    stableKey: raw.stableKey.trim(),
    symbol: raw.symbol.trim().toUpperCase(),
    side: raw.side.trim().toUpperCase(),
    scopeType: raw.scopeType.trim(),
    patternClass: raw.patternClass.trim(),
    validationLabel: raw.validationLabel.trim(),
    feature: raw.feature.trim().toLowerCase(),
    regime: raw.regime.trim().toLowerCase(),
    directLiveActionCandidate: raw.directLiveActionCandidate.trim(),
    liveActionKind: raw.liveActionKind.trim(),
    interventionState: raw.interventionState.trim(),
    minConfidencePct: raw.minConfidencePct.trim(),
    minDriftPct: raw.minDriftPct.trim(),
    minSampleCount: raw.minSampleCount.trim(),
    limit: raw.limit.trim() || '50',
  }
}

function readFiltersFromURL(): PatternLabFilters {
  const params = new URLSearchParams(window.location.search)
  return normalizeFilters({
    patternId: params.get('pattern_id') || '',
    stableKey: params.get('stable_key') || '',
    symbol: params.get('symbol') || '',
    side: params.get('side') || '',
    scopeType: params.get('scope_type') || '',
    patternClass: params.get('pattern_class') || '',
    validationLabel: params.get('validation_label') || '',
    feature: params.get('feature') || '',
    regime: params.get('regime') || '',
    directLiveActionCandidate:
      params.get('direct_live_action_candidate') === 'true' ? 'only' : '',
    liveActionKind: params.get('live_action_kind') || '',
    interventionState: params.get('intervention_state') || '',
    minConfidencePct: params.get('min_confidence_pct') || '',
    minDriftPct: params.get('min_drift_pct') || '',
    minSampleCount: params.get('min_sample_count') || '',
    limit: params.get('limit') || '50',
  })
}

function writeFiltersToURL(filters: PatternLabFilters): void {
  const next = normalizeFilters(filters)
  const url = new URL(window.location.href)
  url.pathname = '/pattern-lab'
  url.search = ''
  if (next.patternId) url.searchParams.set('pattern_id', next.patternId)
  if (next.stableKey) url.searchParams.set('stable_key', next.stableKey)
  if (next.symbol) url.searchParams.set('symbol', next.symbol)
  if (next.side) url.searchParams.set('side', next.side)
  if (next.scopeType) url.searchParams.set('scope_type', next.scopeType)
  if (next.patternClass) url.searchParams.set('pattern_class', next.patternClass)
  if (next.validationLabel) {
    url.searchParams.set('validation_label', next.validationLabel)
  }
  if (next.feature) url.searchParams.set('feature', next.feature)
  if (next.regime) url.searchParams.set('regime', next.regime)
  if (next.directLiveActionCandidate === 'only') {
    url.searchParams.set('direct_live_action_candidate', 'true')
  }
  if (next.liveActionKind) {
    url.searchParams.set('live_action_kind', next.liveActionKind)
  }
  if (next.interventionState) {
    url.searchParams.set('intervention_state', next.interventionState)
  }
  if (next.minConfidencePct) {
    url.searchParams.set('min_confidence_pct', next.minConfidencePct)
  }
  if (next.minDriftPct) url.searchParams.set('min_drift_pct', next.minDriftPct)
  if (next.minSampleCount) {
    url.searchParams.set('min_sample_count', next.minSampleCount)
  }
  if (next.limit && next.limit !== '50') url.searchParams.set('limit', next.limit)
  window.history.replaceState({}, '', url.toString())
}

function formatLifecycleSnapshotSource(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'live_guard_event':
      return patternLabText(language, 'guard')
    case 'rebuild':
      return 'rebuild'
    default:
  return value || patternLabText(language, 'snapshot')
  }
}

function formatPatternLiveActionKind(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'suppression':
      return patternLabText(language, 'Direct suppression candidate')
    case 'rollback':
      return patternLabText(language, 'Rollback-grade candidate')
    default:
      return value || '-'
  }
}

function patternLiveActionKindClasses(value?: string): string {
  switch (value) {
    case 'suppression':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'rollback':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function patternHasOpenIntervention(pattern?: DealReviewLearnedPattern | null): boolean {
  return Boolean(
    pattern?.intervention_history?.some((event) => event.event_status === 'open')
  )
}

function patternHasResolvedIntervention(
  pattern?: DealReviewLearnedPattern | null
): boolean {
  return Boolean(
    pattern?.intervention_history?.some((event) =>
      ['accepted', 'overridden', 'superseded', 'cleared', 'standalone'].includes(
        event.event_status || ''
      )
    )
  )
}

function patternDirectLiveActionKind(
  pattern?: DealReviewLearnedPattern | null
): string {
  const directKinds = (pattern?.intervention_history || [])
    .filter((event) => event.direct_live_action_candidate)
    .map((event) => event.direct_live_action_kind || '')

  if (directKinds.includes('rollback')) return 'rollback'
  if (directKinds.includes('suppression')) return 'suppression'
  if (pattern?.live_action_hint?.candidate_kind === 'rollback') return 'rollback'
  if (pattern?.live_action_hint?.candidate_kind === 'suppression') {
    return 'suppression'
  }
  return ''
}

function patternDirectLiveActionSummary(
  pattern?: DealReviewLearnedPattern | null
): string {
  const directSummary = (pattern?.intervention_history || []).find(
    (event) => event.direct_live_action_candidate && event.direct_live_action_summary
  )?.direct_live_action_summary
  return directSummary || pattern?.live_action_hint?.summary || ''
}

function patternDirectLiveActionConfidence(
  pattern?: DealReviewLearnedPattern | null
): number {
  const directConfidence = (pattern?.intervention_history || []).reduce(
    (best, event) => {
      if (!event.direct_live_action_candidate) return best
      return Math.max(best, event.direct_live_action_confidence || 0)
    },
    0
  )
  return directConfidence || pattern?.live_action_hint?.confidence_score || 0
}

function formatManualControlState(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'live':
      return patternLabText(language, 'Live')
    case 'suppressed':
      return patternLabText(language, 'Suppressed')
    case 'retired':
      return patternLabText(language, 'Retired')
    default:
      return value || '-'
  }
}

function manualControlStateClasses(value?: string): string {
  switch (value) {
    case 'live':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'suppressed':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'retired':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatManualControlAction(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'acknowledge_live':
      return patternLabText(language, 'Keep live')
    case 'suppress':
      return patternLabText(language, 'Suppress')
    case 'retire':
      return patternLabText(language, 'Retire')
    case 'rearm':
      return patternLabText(language, 'Re-arm')
    default:
      return value || '-'
  }
}

function formatActionHintAction(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'suppress':
      return patternLabText(language, 'Suggest suppress')
    case 'retire':
      return patternLabText(language, 'Suggest retire')
    case 'rearm':
      return patternLabText(language, 'Suggest re-arm')
    case 'acknowledge_live':
      return patternLabText(language, 'Suggest keep live')
    default:
      return value || '-'
  }
}

function actionHintActionClasses(value?: string): string {
  switch (value) {
    case 'suppress':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'retire':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'rearm':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    case 'acknowledge_live':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatActionHintPriority(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'critical':
      return patternLabText(language, 'Critical')
    case 'warning':
      return patternLabText(language, 'Warning')
    case 'info':
      return patternLabText(language, 'Info')
    default:
      return value || '-'
  }
}

function actionHintPriorityClasses(value?: string): string {
  switch (value) {
    case 'critical':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'warning':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'info':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatInterventionEventType(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'suggested':
      return patternLabText(language, 'Suggested')
    case 'manual_action':
      return patternLabText(language, 'Manual action')
    default:
      return value || '-'
  }
}

function interventionEventTypeClasses(value?: string): string {
  switch (value) {
    case 'suggested':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    case 'manual_action':
      return 'border-nofx-gold/30 bg-nofx-gold/10 text-nofx-gold'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function formatInterventionStatus(
  value: string | undefined,
  language: Language
): string {
  switch (value) {
    case 'open':
      return patternLabText(language, 'Open')
    case 'accepted':
      return patternLabText(language, 'Accepted')
    case 'overridden':
      return patternLabText(language, 'Overridden')
    case 'superseded':
      return patternLabText(language, 'Superseded')
    case 'cleared':
      return patternLabText(language, 'Cleared')
    case 'standalone':
      return patternLabText(language, 'Standalone')
    default:
      return value || '-'
  }
}

function interventionStatusClasses(value?: string): string {
  switch (value) {
    case 'open':
      return 'border-amber-400/20 bg-amber-500/10 text-amber-200'
    case 'accepted':
      return 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200'
    case 'overridden':
      return 'border-rose-400/20 bg-rose-500/10 text-rose-200'
    case 'superseded':
      return 'border-sky-400/20 bg-sky-500/10 text-sky-200'
    case 'cleared':
      return 'border-white/10 bg-white/5 text-nofx-text-muted'
    case 'standalone':
      return 'border-white/10 bg-black/20 text-white/80'
    default:
      return 'border-white/10 bg-white/5 text-white'
  }
}

function interventionTimestampLabel(event: {
  resolved_at?: string
  last_seen_at?: string
  first_seen_at?: string
  created_at?: string
}): string {
  return (
    event.resolved_at ||
    event.last_seen_at ||
    event.first_seen_at ||
    event.created_at ||
    ''
  )
}

function buildReviewQueryFromPattern(
  pattern: DealReviewLearnedPattern,
  evidenceCaseId?: string
): URLSearchParams {
  const params = new URLSearchParams()
  params.set('status', 'CLOSED')
  if (pattern.symbol) params.set('symbol', pattern.symbol)
  if (pattern.side) params.set('side', pattern.side)
  if (evidenceCaseId) params.set('case_id', evidenceCaseId)

  ;(pattern.feature_set || []).forEach((feature) => {
    const [prefix, ...rest] = feature.split(':')
    const value = rest.join(':')
    if (!prefix || !value) return
    switch (prefix) {
      case 'bucket':
        params.set('open_selection_bucket', value)
        break
      case 'trend':
        params.set('open_trend_regime', value)
        break
      case 'vol':
        params.set('open_volatility_regime', value)
        break
      case 'btc':
        params.set('open_btc_strength_regime', value)
        break
      case 'funding':
        params.set('open_funding_regime', value)
        break
      case 'oi':
        params.set('open_oi_regime', value)
        break
      case 'session':
        params.set('open_session_bucket', value)
        break
      case 'weekday':
        params.set('open_weekday_bucket', value)
        break
      case 'venue':
        params.set('open_venue_tier', value)
        break
      case 'liq':
        params.set('open_liquidity_tier', value)
        break
      case 'spread':
        params.set('open_spread_bucket', value)
        break
      case 'slip':
        params.set('open_slippage_bucket', value)
        break
      default:
        break
    }
  })
  return params
}

function openDealReview(pattern: DealReviewLearnedPattern, evidenceCaseId?: string): void {
  const url = new URL(window.location.href)
  url.pathname = '/deal-review'
  url.search = buildReviewQueryFromPattern(pattern, evidenceCaseId).toString()
  window.history.pushState({}, '', url.toString())
  window.dispatchEvent(new PopStateEvent('popstate'))
}

function strongestRiskScore(pattern: DealReviewLearnedPattern): number {
  return Math.max(
    pattern.drift_score || 0,
    pattern.reverse_risk_score || 0,
    pattern.false_positive_score || 0
  )
}

export function PatternLabPage({
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
}: PatternLabPageProps) {
  const { language } = useLanguage()
  const tp = (value: string) => patternLabText(language, value)
  const sideOptions = createSideOptions(language)
  const scopeOptions = createScopeOptions(language)
  const patternClassOptions = createPatternClassOptions(language)
  const validationLabelOptions = createValidationLabelOptions(language)
  const liveActionKindOptions = createLiveActionKindOptions(language)
  const interventionStateOptions = createInterventionStateOptions(language)
  const directCandidateOptions = createDirectCandidateOptions(language)
  const limitOptions = createLimitOptions(language)
  const [filters, setFilters] = useState<PatternLabFilters>(createDefaultFilters())
  const [appliedFilters, setAppliedFilters] = useState<PatternLabFilters>(
    createDefaultFilters()
  )
  const [items, setItems] = useState<DealReviewLearnedPattern[]>([])
  const [summary, setSummary] = useState<DealReviewLearnedPatternSummary | null>(
    null
  )
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [generatedAt, setGeneratedAt] = useState('')
  const [controlNotes, setControlNotes] = useState<Record<string, string>>({})
  const [controlBusyKey, setControlBusyKey] = useState('')
  const selectedTrader = traders?.find((item) => item.trader_id === selectedTraderId)

  useEffect(() => {
    const next = readFiltersFromURL()
    setFilters(next)
    setAppliedFilters(next)

    const handlePopState = () => {
      const updated = readFiltersFromURL()
      setFilters(updated)
      setAppliedFilters(updated)
    }

    window.addEventListener('popstate', handlePopState)
    return () => {
      window.removeEventListener('popstate', handlePopState)
    }
  }, [])

  useEffect(() => {
    if (!selectedTraderId) {
      setItems([])
      setSummary(null)
      setGeneratedAt('')
      return
    }

    setLoading(true)
    setError(null)
    void api
      .getDealReviewLearnedPatterns(selectedTraderId, {
        pattern_id: appliedFilters.patternId || undefined,
        stable_key: appliedFilters.stableKey || undefined,
        symbol: appliedFilters.symbol || undefined,
        side: appliedFilters.side || undefined,
        scope_type: appliedFilters.scopeType || undefined,
        pattern_class: appliedFilters.patternClass || undefined,
        validation_label: appliedFilters.validationLabel || undefined,
        feature: appliedFilters.feature || undefined,
        regime: appliedFilters.regime || undefined,
        direct_live_action_candidate:
          appliedFilters.directLiveActionCandidate === 'only' ? 'true' : undefined,
        live_action_kind: appliedFilters.liveActionKind || undefined,
        intervention_state: appliedFilters.interventionState || undefined,
        min_confidence_pct: appliedFilters.minConfidencePct || undefined,
        min_drift_pct: appliedFilters.minDriftPct || undefined,
        min_sample_count: appliedFilters.minSampleCount || undefined,
        limit: Number(appliedFilters.limit) || 50,
      })
      .then((result) => {
        setItems(result.items || [])
        setSummary(result.summary || null)
        setGeneratedAt(result.generated_at || '')
      })
      .catch((err) => {
        const message =
          err instanceof Error
            ? err.message
            : tp('Failed to fetch learned patterns')
        setError(message)
        setItems([])
        setSummary(null)
        setGeneratedAt('')
      })
      .finally(() => setLoading(false))
  }, [selectedTraderId, appliedFilters])

  const applyFilters = () => {
    const next = normalizeFilters(filters)
    setFilters(next)
    setAppliedFilters(next)
    writeFiltersToURL(next)
  }

  const resetFilters = () => {
    const next = createDefaultFilters()
    setFilters(next)
    setAppliedFilters(next)
    writeFiltersToURL(next)
  }

  const applyQuickFilter = (patch: Partial<PatternLabFilters>) => {
    const next = normalizeFilters({ ...createDefaultFilters(), ...filters, ...patch })
    setFilters(next)
    setAppliedFilters(next)
    writeFiltersToURL(next)
  }

  const refreshPatternSlice = () => {
    setAppliedFilters((current) => ({ ...current }))
  }

  const patternControlKey = (pattern: DealReviewLearnedPattern): string =>
    pattern.stable_key || pattern.id

  const canManagePattern = (pattern: DealReviewLearnedPattern): boolean =>
    pattern.base_recommended_use === 'monitoring_rule' ||
    pattern.recommended_use === 'monitoring_rule' ||
    Boolean(pattern.manual_control)

  const applyPatternControl = async (
    pattern: DealReviewLearnedPattern,
    action: 'acknowledge_live' | 'suppress' | 'retire' | 'rearm',
    forcedNote?: string
  ) => {
    if (!selectedTraderId) return
    const key = patternControlKey(pattern)
    const note = (forcedNote || controlNotes[key] || '').trim()
    if (!note) {
      notify.error(tp('A short analyst note is required.'))
      return
    }
    setControlBusyKey(`${key}:${action}`)
    try {
      await api.applyDealReviewLearnedPatternControl(selectedTraderId, {
        pattern_id: pattern.id || undefined,
        stable_key: pattern.stable_key || undefined,
        action,
        note,
      })
      notify.success(
        tp('Saved {action} control.').replace(
          '{action}',
          formatManualControlAction(action, language)
        )
      )
      setControlNotes((current) => ({ ...current, [key]: '' }))
      refreshPatternSlice()
    } catch (err) {
      notify.error(
        err instanceof Error
          ? err.message
          : tp('Failed to apply learned-pattern control')
      )
    } finally {
      setControlBusyKey('')
    }
  }

  const applySuggestedPatternAction = async (pattern: DealReviewLearnedPattern) => {
    const action = pattern.action_hint?.recommended_action
    const autoNote = (pattern.action_hint?.auto_note || '').trim()
    if (
      action !== 'acknowledge_live' &&
      action !== 'suppress' &&
      action !== 'retire' &&
      action !== 'rearm'
    ) {
      notify.error(tp('No suggested action is available for this pattern.'))
      return
    }
    await applyPatternControl(pattern, action, autoNote)
  }

  const topRiskPatterns = [...items]
    .filter((item) =>
      ['false_positive', 'reverse_risk', 'drifting'].includes(
        item.validation_label || ''
      )
    )
    .sort((left, right) => strongestRiskScore(right) - strongestRiskScore(left))
    .slice(0, 4)

  const topDriftingPatterns = [...items]
    .filter((item) => item.validation_label === 'drifting')
    .sort((left, right) => (right.drift_score || 0) - (left.drift_score || 0))
    .slice(0, 4)

  const topDirectLiveActionCandidates = (
    summary?.top_direct_live_action_candidates ||
    []
  ).slice(0, 3)

  const visibleNotes = summary?.notes || []

  if (tradersError) {
    return (
      <div className="p-8 text-red-400">
        {tp('Failed to load traders')}: {tradersError.message}
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
                {tp('Pattern Lab')}
              </div>
              <h1 className="text-3xl font-semibold text-nofx-text-main">
                {tp('Learned pattern mining and drift watch')}
              </h1>
              <p className="text-sm text-nofx-text-muted mt-2">
                {tp(
                  'Inspect learned feature combinations separately from deal review: positive edges, anti-patterns, symbol overrides, reverse-risk, and drift under the currently selected trader.'
                )}
              </p>
            </div>
            <div className="w-full lg:w-80 space-y-2">
              <label className="text-xs text-nofx-text-muted block">
                {tp('Trader')}
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
              <button
                onClick={() => {
                  const url = new URL(window.location.href)
                  url.pathname = '/deal-review'
                  if (filters.symbol) url.searchParams.set('symbol', filters.symbol)
                  if (filters.side) url.searchParams.set('side', filters.side)
                  window.history.pushState({}, '', url.toString())
                  window.dispatchEvent(new PopStateEvent('popstate'))
                }}
                className="w-full h-10 rounded-lg border border-white/10 bg-black/20 text-sm font-medium"
              >
                {tp('Open Deal Review')}
              </button>
            </div>
          </div>
        </div>

        <div className="nofx-glass rounded-xl p-5 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold">{tp('Filters')}</div>
              <div className="text-xs text-nofx-text-muted mt-1">
                {tp(
                  'Filter the learned pattern corpus by symbol, side, scope, class, validation label, feature token, regime token, live-action candidate state, intervention state, confidence, drift, and minimum sample size.'
                )}
              </div>
            </div>
            <div className="text-xs text-nofx-text-muted">
              {selectedTrader ? selectedTrader.trader_name : tp('No trader selected')}{' '}
              {generatedAt
                ? `· ${tp('generated')} ${formatTimestamp(generatedAt, language)}`
                : ''}
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-6 gap-3">
            <input
              value={filters.symbol}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  symbol: event.target.value.toUpperCase(),
                }))
              }
              placeholder={tp('Symbol')}
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
            />
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.side}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, side: value }))
                }
                options={sideOptions}
              />
            </div>
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.scopeType}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, scopeType: value }))
                }
                options={scopeOptions}
              />
            </div>
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.patternClass}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, patternClass: value }))
                }
                options={patternClassOptions}
              />
            </div>
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.validationLabel}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, validationLabel: value }))
                }
                options={validationLabelOptions}
              />
            </div>
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.directLiveActionCandidate}
                onChange={(value) =>
                  setFilters((current) => ({
                    ...current,
                    directLiveActionCandidate: value,
                  }))
                }
                options={directCandidateOptions}
              />
            </div>
            <input
              value={filters.feature}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  feature: event.target.value.toLowerCase(),
                }))
              }
              placeholder={tp('Feature token, e.g. bucket_breakout')}
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
            />
            <input
              value={filters.regime}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  regime: event.target.value.toLowerCase(),
                }))
              }
              placeholder={tp('Regime token, e.g. trend_uptrend')}
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
            />
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.liveActionKind}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, liveActionKind: value }))
                }
                options={liveActionKindOptions}
              />
            </div>
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.interventionState}
                onChange={(value) =>
                  setFilters((current) => ({
                    ...current,
                    interventionState: value,
                  }))
                }
                options={interventionStateOptions}
              />
            </div>
            <input
              value={filters.minConfidencePct}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  minConfidencePct: event.target.value,
                }))
              }
              placeholder={tp('Min confidence %')}
              inputMode="decimal"
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
            />
            <input
              value={filters.minDriftPct}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  minDriftPct: event.target.value,
                }))
              }
              placeholder={tp('Min drift %')}
              inputMode="decimal"
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
            />
            <input
              value={filters.minSampleCount}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  minSampleCount: event.target.value,
                }))
              }
              placeholder={tp('Min samples')}
              inputMode="numeric"
              className="h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm"
            />
            <div className="h-11 rounded-lg border border-white/10 px-3 flex items-center bg-black/20">
              <NofxSelect
                value={filters.limit}
                onChange={(value) =>
                  setFilters((current) => ({ ...current, limit: value }))
                }
                options={limitOptions}
              />
            </div>
            <div className="flex items-center gap-3">
              <button
                onClick={applyFilters}
                className="flex-1 h-11 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 text-nofx-gold font-semibold"
              >
                {tp('Apply')}
              </button>
              <button
                onClick={resetFilters}
                className="flex-1 h-11 rounded-lg border border-white/10 bg-black/20 font-semibold"
              >
                {tp('Reset')}
              </button>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => applyQuickFilter(createDefaultFilters())}
              className="h-8 px-3 rounded-full border border-white/10 bg-black/20 text-xs"
            >
              {tp('All patterns')}
            </button>
            <button
              onClick={() => applyQuickFilter({ patternClass: 'positive_edge' })}
              className="h-8 px-3 rounded-full border border-emerald-400/20 bg-emerald-500/10 text-xs text-emerald-200"
            >
              {tp('Positive edges')}
            </button>
            <button
              onClick={() => applyQuickFilter({ patternClass: 'negative_edge' })}
              className="h-8 px-3 rounded-full border border-rose-400/20 bg-rose-500/10 text-xs text-rose-200"
            >
              {tp('Anti-patterns')}
            </button>
            <button
              onClick={() => applyQuickFilter({ validationLabel: 'confirmed' })}
              className="h-8 px-3 rounded-full border border-sky-400/20 bg-sky-500/10 text-xs text-sky-200"
            >
              {tp('Confirmed')}
            </button>
            <button
              onClick={() => applyQuickFilter({ validationLabel: 'reverse_risk' })}
              className="h-8 px-3 rounded-full border border-amber-400/20 bg-amber-500/10 text-xs text-amber-200"
            >
              {tp('Reverse risk')}
            </button>
            <button
              onClick={() => applyQuickFilter({ validationLabel: 'drifting' })}
              className="h-8 px-3 rounded-full border border-orange-400/20 bg-orange-500/10 text-xs text-orange-200"
            >
              {tp('Drifting')}
            </button>
            <button
              onClick={() => applyQuickFilter({ minConfidencePct: '70' })}
              className="h-8 px-3 rounded-full border border-cyan-400/20 bg-cyan-500/10 text-xs text-cyan-200"
            >
              {tp('Confidence 70%+')}
            </button>
            <button
              onClick={() => applyQuickFilter({ minDriftPct: '45' })}
              className="h-8 px-3 rounded-full border border-orange-400/20 bg-orange-500/10 text-xs text-orange-100"
            >
              {tp('Drift 45%+')}
            </button>
            <button
              onClick={() => applyQuickFilter({ scopeType: 'symbol' })}
              className="h-8 px-3 rounded-full border border-white/10 bg-white/5 text-xs"
            >
              {tp('Symbol overrides')}
            </button>
            <button
              onClick={() =>
                applyQuickFilter({
                  directLiveActionCandidate: 'only',
                  interventionState: 'open',
                })
              }
              className="h-8 px-3 rounded-full border border-rose-400/20 bg-rose-500/10 text-xs text-rose-200"
            >
              {tp('Open strong candidates')}
            </button>
            <button
              onClick={() =>
                applyQuickFilter({
                  directLiveActionCandidate: 'only',
                  liveActionKind: 'rollback',
                })
              }
              className="h-8 px-3 rounded-full border border-rose-400/20 bg-rose-500/10 text-xs text-rose-200"
            >
              {tp('Rollback-grade candidates')}
            </button>
            <button
              onClick={() => applyQuickFilter({ interventionState: 'open' })}
              className="h-8 px-3 rounded-full border border-amber-400/20 bg-amber-500/10 text-xs text-amber-100"
            >
              {tp('Open interventions only')}
            </button>
          </div>

          {appliedFilters.patternId ? (
            <div className="rounded-lg border border-sky-400/20 bg-sky-500/10 px-3 py-2 text-xs text-sky-100">
              {tp('Direct pattern focus active:')}{' '}
              <span className="font-mono">{appliedFilters.patternId}</span>
            </div>
          ) : null}
        </div>

        <div className="grid grid-cols-2 xl:grid-cols-8 gap-3 text-xs">
          <div className="nofx-glass rounded-xl px-4 py-4">
            <div className="text-nofx-text-muted">{tp('Tracked patterns')}</div>
            <div className="text-2xl font-semibold mt-1">
              {summary?.total_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-emerald-400/15 bg-emerald-500/5">
            <div className="text-nofx-text-muted">{tp('Positive edges')}</div>
            <div className="text-2xl font-semibold mt-1 text-emerald-300">
              {summary?.positive_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-rose-400/15 bg-rose-500/5">
            <div className="text-nofx-text-muted">{tp('Anti-patterns')}</div>
            <div className="text-2xl font-semibold mt-1 text-rose-300">
              {summary?.negative_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-sky-400/15 bg-sky-500/5">
            <div className="text-nofx-text-muted">{tp('Confirmed')}</div>
            <div className="text-2xl font-semibold mt-1 text-sky-200">
              {summary?.confirmed_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-amber-400/15 bg-amber-500/5">
            <div className="text-nofx-text-muted">{tp('Reverse / drift')}</div>
            <div className="text-2xl font-semibold mt-1 text-amber-200">
              {(summary?.reverse_risk_count || 0) + (summary?.drifting_count || 0)}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-sky-400/15 bg-sky-500/5">
            <div className="text-nofx-text-muted">{tp('Monitoring rules')}</div>
            <div className="text-2xl font-semibold mt-1 text-sky-200">
              {summary?.monitoring_rule_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-orange-400/15 bg-orange-500/5">
            <div className="text-nofx-text-muted">{tp('Degrading live rules')}</div>
            <div className="text-2xl font-semibold mt-1 text-orange-200">
              {summary?.lifecycle_degrading_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-rose-400/15 bg-rose-500/5">
            <div className="text-nofx-text-muted">{tp('Rollback watch')}</div>
            <div className="text-2xl font-semibold mt-1 text-rose-300">
              {summary?.lifecycle_rollback_watch_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-amber-400/15 bg-amber-500/5">
            <div className="text-nofx-text-muted">{tp('Fragile live rules')}</div>
            <div className="text-2xl font-semibold mt-1 text-amber-200">
              {summary?.lifecycle_fragile_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-white/10 bg-white/5">
            <div className="text-nofx-text-muted">{tp('Lagging guard trails')}</div>
            <div className="text-2xl font-semibold mt-1">
              {summary?.lifecycle_lagging_guard_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-emerald-400/15 bg-emerald-500/5">
            <div className="text-nofx-text-muted">{tp('Protective live rules')}</div>
            <div className="text-2xl font-semibold mt-1 text-emerald-300">
              {summary?.live_guard_protective_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-rose-400/15 bg-rose-500/5">
            <div className="text-nofx-text-muted">{tp('Overblocking live rules')}</div>
            <div className="text-2xl font-semibold mt-1 text-rose-300">
              {summary?.live_guard_overblocking_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-emerald-400/15 bg-emerald-500/5">
            <div className="text-nofx-text-muted">{tp('Improving live rules')}</div>
            <div className="text-2xl font-semibold mt-1 text-emerald-300">
              {summary?.live_guard_improving_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-amber-400/15 bg-amber-500/5">
            <div className="text-nofx-text-muted">{tp('Degrading live rules')}</div>
            <div className="text-2xl font-semibold mt-1 text-amber-200">
              {summary?.live_guard_degrading_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-rose-400/15 bg-rose-500/5">
            <div className="text-nofx-text-muted">{tp('Newly overblocking')}</div>
            <div className="text-2xl font-semibold mt-1 text-rose-300">
              {summary?.live_guard_newly_overblocking_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-rose-400/15 bg-rose-500/5">
            <div className="text-nofx-text-muted">{tp('Direct live actions')}</div>
            <div className="text-2xl font-semibold mt-1 text-rose-300">
              {summary?.direct_live_action_candidate_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-amber-400/15 bg-amber-500/5">
            <div className="text-nofx-text-muted">{tp('Open direct candidates')}</div>
            <div className="text-2xl font-semibold mt-1 text-amber-200">
              {summary?.open_direct_live_action_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-rose-400/15 bg-rose-500/5">
            <div className="text-nofx-text-muted">{tp('Rollback-grade candidates')}</div>
            <div className="text-2xl font-semibold mt-1 text-rose-300">
              {summary?.rollback_candidate_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-amber-400/15 bg-amber-500/5">
            <div className="text-nofx-text-muted">{tp('Suppression candidates')}</div>
            <div className="text-2xl font-semibold mt-1 text-amber-200">
              {summary?.suppression_candidate_count || 0}
            </div>
          </div>
          <div className="nofx-glass rounded-xl px-4 py-4 border border-white/10 bg-white/5">
            <div className="text-nofx-text-muted">{tp('Loaded items')}</div>
            <div className="text-2xl font-semibold mt-1">{items.length}</div>
          </div>
        </div>

        {visibleNotes.length > 0 && (
          <div className="space-y-2">
            {visibleNotes.map((note) => (
              <div
                key={note}
                className="nofx-glass rounded-xl px-4 py-3 text-sm text-nofx-text-muted"
              >
                {note}
              </div>
            ))}
          </div>
        )}

        <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-emerald-200">
              {tp('Top Positive Patterns')}
            </div>
            {!summary?.top_positive_patterns ||
            summary.top_positive_patterns.length === 0 ? (
              <div className="text-sm text-nofx-text-muted">
                {tp('No positive patterns surfaced for this slice yet.')}
              </div>
            ) : (
              summary.top_positive_patterns.map((pattern) => (
                <div
                  key={`positive-${pattern.id}`}
                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{pattern.pattern_signature}</span>
                    <span
                      className={`px-2 py-1 rounded-full text-[11px] border ${validationLabelClasses(
                        pattern.validation_label
                      )}`}
                    >
                      {formatValidationLabel(pattern.validation_label, language)}
                    </span>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {formatPatternScope(pattern, language)} · {tp('avg')}{' '}
                    {formatPct(pattern.avg_pnl_pct)} · {tp('lift')}{' '}
                    {formatPct(pattern.lift_avg_pnl_pct)}
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-rose-200">
              {tp('Top Anti-Patterns')}
            </div>
            {!summary?.top_negative_patterns ||
            summary.top_negative_patterns.length === 0 ? (
              <div className="text-sm text-nofx-text-muted">
                {tp('No anti-patterns surfaced for this slice yet.')}
              </div>
            ) : (
              summary.top_negative_patterns.map((pattern) => (
                <div
                  key={`negative-${pattern.id}`}
                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{pattern.pattern_signature}</span>
                    <span
                      className={`px-2 py-1 rounded-full text-[11px] border ${validationLabelClasses(
                        pattern.validation_label
                      )}`}
                    >
                      {formatValidationLabel(pattern.validation_label, language)}
                    </span>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {formatPatternScope(pattern, language)} · {tp('avg')}{' '}
                    {formatPct(pattern.avg_pnl_pct)} · {tp('lift')}{' '}
                    {formatPct(pattern.lift_avg_pnl_pct)}
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-rose-200">
              {tp('Direct live-action candidates')}
            </div>
            {topDirectLiveActionCandidates.length === 0 ? (
              <div className="text-sm text-nofx-text-muted">
                {tp('No direct live-action candidates surfaced for this slice yet.')}
              </div>
            ) : (
              topDirectLiveActionCandidates.map((pattern) => {
                const liveActionKind = patternDirectLiveActionKind(pattern)
                const liveActionSummary = patternDirectLiveActionSummary(pattern)
                const liveActionConfidence = patternDirectLiveActionConfidence(pattern)
                const hasOpenIntervention = patternHasOpenIntervention(pattern)
                const hasResolvedIntervention = patternHasResolvedIntervention(pattern)
                return (
                  <div
                    key={`direct-live-action-${pattern.id}`}
                    className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{pattern.pattern_signature}</span>
                      {liveActionKind && (
                        <span
                          className={`px-2 py-1 rounded-full text-[11px] border ${patternLiveActionKindClasses(
                            liveActionKind
                          )}`}
                        >
                          {formatPatternLiveActionKind(liveActionKind, language)}
                        </span>
                      )}
                      {hasOpenIntervention && (
                        <span className="px-2 py-1 rounded-full text-[11px] border border-amber-400/20 bg-amber-500/10 text-amber-200">
                          {tp('Open interventions')}
                        </span>
                      )}
                      {!hasOpenIntervention && hasResolvedIntervention && (
                        <span className="px-2 py-1 rounded-full text-[11px] border border-emerald-400/20 bg-emerald-500/10 text-emerald-200">
                          {tp('Resolved interventions')}
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {formatPatternScope(pattern, language)} · {tp('confidence')}{' '}
                      {formatPct(liveActionConfidence * 100)}
                    </div>
                    {liveActionSummary && (
                      <div className="text-xs text-nofx-text-muted mt-2">
                        {liveActionSummary}
                      </div>
                    )}
                  </div>
                )
              })
            )}
          </div>

          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-amber-200">
              {tp('Highest Risk Watchlist')}
            </div>
            {topRiskPatterns.length === 0 ? (
              <div className="text-sm text-nofx-text-muted">
                {tp(
                  'No false-positive, reverse-risk, or drifting patterns in this slice yet.'
                )}
              </div>
            ) : (
              topRiskPatterns.map((pattern) => (
                <div
                  key={`risk-${pattern.id}`}
                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{pattern.pattern_signature}</span>
                    <span
                      className={`px-2 py-1 rounded-full text-[11px] border ${validationLabelClasses(
                        pattern.validation_label
                      )}`}
                    >
                      {formatValidationLabel(pattern.validation_label, language)}
                    </span>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {tp('drift')} {formatPct((pattern.drift_score || 0) * 100)} · {tp('reverse')}{' '}
                    {formatPct((pattern.reverse_risk_score || 0) * 100)} · FP{' '}
                    {formatPct((pattern.false_positive_score || 0) * 100)}
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-sky-200">
              {tp('Symbol Overrides')}
            </div>
            {!summary?.top_symbol_overrides ||
            summary.top_symbol_overrides.length === 0 ? (
              <div className="text-sm text-nofx-text-muted">
                {tp('No symbol-specific overrides surfaced for this slice yet.')}
              </div>
            ) : (
              summary.top_symbol_overrides.map((pattern) => (
                <div
                  key={`override-${pattern.id}`}
                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">
                      {pattern.symbol} · {pattern.pattern_signature}
                    </span>
                    <span
                      className={`px-2 py-1 rounded-full text-[11px] border ${patternClassClasses(
                        pattern.pattern_class
                      )}`}
                    >
                      {formatPatternClass(pattern.pattern_class, language)}
                    </span>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {pattern.side} · {tp('lift')} {formatPct(pattern.lift_avg_pnl_pct)} ·{' '}
                    {pattern.sample_count} {tp('Deals')}
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {topDriftingPatterns.length > 0 && (
          <div className="nofx-glass rounded-xl p-5 space-y-3">
            <div className="text-xs uppercase tracking-[0.2em] text-orange-200">
              {tp('Biggest Recent Drifts')}
            </div>
            <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
              {topDriftingPatterns.map((pattern) => (
                <div
                  key={`drift-${pattern.id}`}
                  className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{pattern.pattern_signature}</span>
                    <span
                      className={`px-2 py-1 rounded-full text-[11px] border ${validationLabelClasses(
                        pattern.validation_label
                      )}`}
                    >
                      {formatValidationLabel(pattern.validation_label, language)}
                    </span>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {formatPatternScope(pattern, language)} · {tp('drift')}{' '}
                    {formatPct((pattern.drift_score || 0) * 100)} · {tp('recent support')}{' '}
                    {pattern.recent_support_count || 0}/{pattern.recent_sample_count || 0}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {((summary?.top_expiring_monitoring_rules &&
          summary.top_expiring_monitoring_rules.length > 0) ||
          (summary?.top_rollback_watch_patterns &&
            summary.top_rollback_watch_patterns.length > 0) ||
          (summary?.top_improving_monitoring_rules &&
            summary.top_improving_monitoring_rules.length > 0) ||
          (summary?.top_degrading_monitoring_rules &&
            summary.top_degrading_monitoring_rules.length > 0) ||
          (summary?.top_overblocking_monitoring_rules &&
            summary.top_overblocking_monitoring_rules.length > 0) ||
          (summary?.top_fragile_monitoring_rules &&
            summary.top_fragile_monitoring_rules.length > 0)) && (
          <div className="grid grid-cols-1 xl:grid-cols-3 2xl:grid-cols-6 gap-6">
            <div className="nofx-glass rounded-xl p-5 space-y-3">
              <div className="text-xs uppercase tracking-[0.2em] text-orange-200">
                {tp('Expiring Monitoring Rules')}
              </div>
              {!summary?.top_expiring_monitoring_rules ||
              summary.top_expiring_monitoring_rules.length === 0 ? (
                <div className="text-sm text-nofx-text-muted">
                  {tp('No monitoring rules are currently expiring in this slice.')}
                </div>
              ) : (
                summary.top_expiring_monitoring_rules.map((pattern) => (
                  <div
                    key={`expiring-${pattern.id}`}
                    className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{pattern.pattern_signature}</span>
                      <span
                        className={`px-2 py-1 rounded-full text-[11px] border ${lifecycleStatusClasses(
                          pattern.lifecycle?.status
                        )}`}
                      >
                        {formatLifecycleStatus(pattern.lifecycle?.status, language)}
                      </span>
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {tp('expiry')} {formatPct((pattern.lifecycle?.expiry_score || 0) * 100)} ·
                      {tp('validation')} {formatPct((pattern.validation_support_score || 0) * 100)} ·
                      {tp('recent')} {pattern.recent_support_count || 0}/
                      {pattern.recent_sample_count || 0}
                    </div>
                    {pattern.lifecycle?.summary && (
                      <div className="text-xs text-nofx-text-muted mt-2">
                        {pattern.lifecycle.summary}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5 space-y-3">
              <div className="text-xs uppercase tracking-[0.2em] text-emerald-200">
                {tp('Improving Rules')}
              </div>
              {!summary?.top_improving_monitoring_rules ||
              summary.top_improving_monitoring_rules.length === 0 ? (
                <div className="text-sm text-nofx-text-muted">
                  {tp(
                    'No monitoring rules are currently improving versus the prior window.'
                  )}
                </div>
              ) : (
                summary.top_improving_monitoring_rules.map((pattern) => (
                  <div
                    key={`improving-${pattern.id}`}
                    className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{pattern.pattern_signature}</span>
                      <span
                        className={`px-2 py-1 rounded-full text-[11px] border ${liveGuardTrendClasses(
                          pattern.live_guard_attribution_delta?.trend_label
                        )}`}
                      >
                        {formatLiveGuardTrendLabel(
                          pattern.live_guard_attribution_delta?.trend_label
                          ,
                          language
                        )}
                      </span>
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {tp('protective delta')}{' '}
                      {formatPct(
                        (pattern.live_guard_attribution_delta?.protective_rate_delta || 0) *
                          100
                      )}{' '}
                      · {tp('recent')}{' '}
                      {formatPct(
                        (pattern.live_guard_attribution_delta?.recent_protective_rate || 0) *
                          100
                      )}{' '}
                      · {tp('prior')}{' '}
                      {formatPct(
                        (pattern.live_guard_attribution_delta?.prior_protective_rate || 0) *
                          100
                      )}
                    </div>
                    {pattern.live_guard_attribution_delta?.summary && (
                      <div className="text-xs text-nofx-text-muted mt-2">
                        {pattern.live_guard_attribution_delta.summary}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5 space-y-3">
              <div className="text-xs uppercase tracking-[0.2em] text-amber-200">
                {tp('Degrading Rules')}
              </div>
              {!summary?.top_degrading_monitoring_rules ||
              summary.top_degrading_monitoring_rules.length === 0 ? (
                <div className="text-sm text-nofx-text-muted">
                  {tp(
                    'No monitoring rules are currently degrading versus the prior window.'
                  )}
                </div>
              ) : (
                summary.top_degrading_monitoring_rules.map((pattern) => (
                  <div
                    key={`degrading-${pattern.id}`}
                    className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{pattern.pattern_signature}</span>
                      <span
                        className={`px-2 py-1 rounded-full text-[11px] border ${liveGuardTrendClasses(
                          pattern.live_guard_attribution_delta?.trend_label
                        )}`}
                      >
                        {formatLiveGuardTrendLabel(
                          pattern.live_guard_attribution_delta?.trend_label
                          ,
                          language
                        )}
                      </span>
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {tp('overblocking delta')}{' '}
                      {formatPct(
                        (pattern.live_guard_attribution_delta?.overblocking_rate_delta || 0) *
                          100
                      )}{' '}
                      · {tp('recent')}{' '}
                      {formatPct(
                        (pattern.live_guard_attribution_delta?.recent_overblocking_rate || 0) *
                          100
                      )}{' '}
                      · {tp('prior')}{' '}
                      {formatPct(
                        (pattern.live_guard_attribution_delta?.prior_overblocking_rate || 0) *
                          100
                      )}
                    </div>
                    {pattern.live_guard_attribution_delta?.summary && (
                      <div className="text-xs text-nofx-text-muted mt-2">
                        {pattern.live_guard_attribution_delta.summary}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5 space-y-3">
              <div className="text-xs uppercase tracking-[0.2em] text-rose-200">
                {tp('Overblocking Rules')}
              </div>
              {!summary?.top_overblocking_monitoring_rules ||
              summary.top_overblocking_monitoring_rules.length === 0 ? (
                <div className="text-sm text-nofx-text-muted">
                  {tp('No monitoring rules currently look overblocking in this slice.')}
                </div>
              ) : (
                summary.top_overblocking_monitoring_rules.map((pattern) => (
                  <div
                    key={`overblocking-${pattern.id}`}
                    className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{pattern.pattern_signature}</span>
                      <span
                        className={`px-2 py-1 rounded-full text-[11px] border ${liveGuardAttributionClasses(
                          pattern.live_guard_attribution?.attribution_label
                        )}`}
                      >
                        {formatLiveGuardAttributionLabel(
                          pattern.live_guard_attribution?.attribution_label
                          ,
                          language
                        )}
                      </span>
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {tp('overblocking')}{' '}
                      {formatPct(
                        (pattern.live_guard_attribution?.overblocking_rate || 0) * 100
                      )}{' '}
                      · {tp('resolved')} {pattern.live_guard_attribution?.resolved_event_count || 0} ·
                      {tp('confidence')}{' '}
                      {formatPct(
                        (pattern.live_guard_attribution?.confidence_score || 0) * 100
                      )}
                    </div>
                    {pattern.live_guard_attribution?.summary && (
                      <div className="text-xs text-nofx-text-muted mt-2">
                        {pattern.live_guard_attribution.summary}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5 space-y-3">
              <div className="text-xs uppercase tracking-[0.2em] text-rose-200">
                {tp('Rollback watch')}
              </div>
              {!summary?.top_rollback_watch_patterns ||
              summary.top_rollback_watch_patterns.length === 0 ? (
                <div className="text-sm text-nofx-text-muted">
                  {tp('No live monitoring rules are on rollback watch in this slice.')}
                </div>
              ) : (
                summary.top_rollback_watch_patterns.map((pattern) => (
                  <div
                    key={`rollback-watch-${pattern.id}`}
                    className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{pattern.pattern_signature}</span>
                      <span
                        className={`px-2 py-1 rounded-full text-[11px] border ${lifecycleStatusClasses(
                          pattern.lifecycle?.status
                        )}`}
                      >
                        {formatLifecycleStatus(pattern.lifecycle?.status, language)}
                      </span>
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {tp('rollback')} {formatPct((pattern.lifecycle?.rollback_score || 0) * 100)} ·
                      {tp('guard hits')} {pattern.lifecycle?.recent_guard_event_count || 0} ·
                      {tp('hard block')} {pattern.lifecycle?.recent_hard_blocked_count || 0}
                    </div>
                    {pattern.lifecycle?.summary && (
                      <div className="text-xs text-nofx-text-muted mt-2">
                        {pattern.lifecycle.summary}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>

            <div className="nofx-glass rounded-xl p-5 space-y-3">
              <div className="text-xs uppercase tracking-[0.2em] text-amber-200">
                {tp('Fragile Monitoring Rules')}
              </div>
              {!summary?.top_fragile_monitoring_rules ||
              summary.top_fragile_monitoring_rules.length === 0 ? (
                <div className="text-sm text-nofx-text-muted">
                  {tp('No monitoring rules look structurally fragile in this slice.')}
                </div>
              ) : (
                summary.top_fragile_monitoring_rules.map((pattern) => (
                  <div
                    key={`fragile-${pattern.id}`}
                    className="rounded-lg border border-white/10 bg-black/20 px-3 py-3"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{pattern.pattern_signature}</span>
                      {pattern.lifecycle?.status && (
                        <span
                          className={`px-2 py-1 rounded-full text-[11px] border ${lifecycleStatusClasses(
                            pattern.lifecycle.status
                          )}`}
                        >
                          {formatLifecycleStatus(pattern.lifecycle.status, language)}
                        </span>
                      )}
                      <span className="px-2 py-1 rounded-full text-[11px] border border-amber-400/20 bg-amber-500/10 text-amber-200">
                        {tp('Fragile')}
                      </span>
                    </div>
                    <div className="text-xs text-nofx-text-muted mt-1">
                      {tp('degrade')}{' '}
                      {formatPct(
                        ((pattern.lifecycle_trend?.degrading_share || 0) +
                          (pattern.lifecycle_trend?.rollback_watch_share || 0)) *
                          100
                      )}{' '}
                      · {tp('stale lag')} {tp('snapshots')}{' '}
                      {pattern.lifecycle_trend?.stale_guard_snapshot_count || 0} ·
                      {tp('changes')} {pattern.lifecycle_trend?.status_change_count || 0}
                    </div>
                    {pattern.lifecycle_trend?.summary && (
                      <div className="text-xs text-nofx-text-muted mt-2">
                        {pattern.lifecycle_trend.summary}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>
        )}

        <div className="nofx-glass rounded-xl p-5 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold">{tp('Pattern corpus')}</div>
              <div className="text-xs text-nofx-text-muted mt-1">
                {tp(
                  'Full list for the current slice. Use these cards to inspect feature combinations and jump directly into deal review for the matching cohort.'
                )}
              </div>
            </div>
            <div className="text-xs text-nofx-text-muted">
              {loading
                ? tp('Refreshing…')
                : `${items.length} ${tp('item(s) loaded')}`}
            </div>
          </div>

          {error && (
            <div className="rounded-lg border border-rose-400/20 bg-rose-500/10 px-3 py-2 text-sm text-rose-200">
              {error}
            </div>
          )}

          {!loading && items.length === 0 ? (
            <div className="text-sm text-nofx-text-muted">
              {tp('No learned patterns matched the current filter slice yet.')}
            </div>
          ) : (
            <div className="space-y-4">
              {items.map((pattern) => (
                <div
                  key={pattern.id}
                  className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4"
                >
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-semibold">{pattern.pattern_signature}</span>
                        <span
                          className={`px-2 py-1 rounded-full text-[11px] border ${patternClassClasses(
                            pattern.pattern_class
                          )}`}
                        >
                          {formatPatternClass(pattern.pattern_class, language)}
                        </span>
                        <span
                          className={`px-2 py-1 rounded-full text-[11px] border ${validationLabelClasses(
                            pattern.validation_label
                          )}`}
                        >
                          {formatValidationLabel(pattern.validation_label, language)}
                        </span>
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                          {formatPatternScope(pattern, language)}
                        </span>
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                          {formatRecommendedUse(pattern.recommended_use, language)}
                        </span>
                        {pattern.manual_control?.control_state && (
                          <span
                            className={`px-2 py-1 rounded-full text-[11px] border ${manualControlStateClasses(
                              pattern.manual_control.control_state
                            )}`}
                          >
                            {tp('Analyst')}{' '}
                            {formatManualControlState(
                              pattern.manual_control.control_state,
                              language
                            )}
                          </span>
                        )}
                        {pattern.lifecycle?.status && (
                          <span
                            className={`px-2 py-1 rounded-full text-[11px] border ${lifecycleStatusClasses(
                              pattern.lifecycle.status
                            )}`}
                          >
                            {formatLifecycleStatus(pattern.lifecycle.status, language)}
                          </span>
                        )}
                      </div>
                      <div className="text-xs text-nofx-text-muted mt-1">
                        {pattern.regime_signature || tp('No regime signature stored')}
                      </div>
                    </div>
                    <div className="text-right text-xs text-nofx-text-muted">
                      <div>
                        {tp('built')} {formatTimestamp(pattern.built_at, language)}
                      </div>
                      <div className="mt-1">
                        {tp('observed')} {formatTimestamp(pattern.last_observed_at, language)}
                      </div>
                    </div>
                  </div>

                  <div className="text-sm text-nofx-text-muted">{pattern.summary}</div>

                  {pattern.validation_alert && (
                    <div className="rounded-lg border border-sky-400/15 bg-sky-500/5 px-3 py-2 text-xs text-sky-100">
                      {pattern.validation_alert}
                    </div>
                  )}

                  {pattern.lifecycle?.summary && (
                    <div
                      className={`rounded-lg border px-3 py-2 text-xs ${lifecycleStatusClasses(
                        pattern.lifecycle.status
                      )}`}
                    >
                      <div className="font-medium">
                        {formatLifecycleStatus(pattern.lifecycle.status, language)}
                      </div>
                      <div className="mt-1">{pattern.lifecycle.summary}</div>
                    </div>
                  )}

                  {pattern.lifecycle_trend?.summary && (
                    <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs text-nofx-text-muted">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-white/90">{tp('Lifecycle Trend')}</span>
                        {pattern.lifecycle_trend.fragile && (
                          <span className="px-2 py-1 rounded-full text-[11px] border border-amber-400/20 bg-amber-500/10 text-amber-200">
                            {tp('Fragile')}
                          </span>
                        )}
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                          {pattern.lifecycle_trend.snapshot_count || 0} {tp('snapshots')}
                        </span>
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                          {tp('span')} {formatHoursCompact(pattern.lifecycle_trend.span_hours)}
                        </span>
                      </div>
                      <div className="mt-2">{pattern.lifecycle_trend.summary}</div>
                      <div className="mt-2 flex flex-wrap gap-3">
                        <span>
                          {tp('active')} {formatHoursCompact(pattern.lifecycle_trend.active_hours)}
                        </span>
                        <span>
                          {tp('degrading')}{' '}
                          {formatHoursCompact(pattern.lifecycle_trend.degrading_hours)}
                        </span>
                        <span>
                          {tp('rollback')}{' '}
                          {formatHoursCompact(
                            pattern.lifecycle_trend.rollback_watch_hours
                          )}
                        </span>
                        <span>
                          {tp('stale lag')} {pattern.lifecycle_trend.stale_guard_snapshot_count || 0}
                        </span>
                      </div>
                    </div>
                  )}

                  {pattern.live_guard_attribution?.summary && (
                    <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs text-nofx-text-muted">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-white/90">
                          {tp('Live-guard attribution')}
                        </span>
                        <span
                          className={`px-2 py-1 rounded-full text-[11px] border ${liveGuardAttributionClasses(
                            pattern.live_guard_attribution.attribution_label
                          )}`}
                        >
                          {formatLiveGuardAttributionLabel(
                            pattern.live_guard_attribution.attribution_label,
                            language
                          )}
                        </span>
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                          {pattern.live_guard_attribution.event_count || 0} {tp('events')}
                        </span>
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                          {pattern.live_guard_attribution.resolved_event_count || 0} {tp('resolved')}
                        </span>
                      </div>
                      <div className="mt-2">{pattern.live_guard_attribution.summary}</div>
                      <div className="mt-2 flex flex-wrap gap-3">
                        <span>
                          {tp('protective')}{' '}
                          {formatPct(
                            (pattern.live_guard_attribution.protective_rate || 0) * 100
                          )}
                        </span>
                        <span>
                          {tp('overblocking')}{' '}
                          {formatPct(
                            (pattern.live_guard_attribution.overblocking_rate || 0) *
                              100
                          )}
                        </span>
                        <span>
                          {tp('pending')} {pattern.live_guard_attribution.pending_count || 0}
                        </span>
                        <span>
                          {tp('follow-up open')}{' '}
                          {pattern.live_guard_attribution.followup_open_count || 0}
                        </span>
                      </div>
                    </div>
                  )}

                  {pattern.live_guard_attribution_delta?.summary && (
                    <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-xs text-nofx-text-muted">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-white/90">{tp('Live-guard trend')}</span>
                        <span
                          className={`px-2 py-1 rounded-full text-[11px] border ${liveGuardTrendClasses(
                            pattern.live_guard_attribution_delta.trend_label
                          )}`}
                        >
                          {formatLiveGuardTrendLabel(
                            pattern.live_guard_attribution_delta.trend_label,
                            language
                          )}
                        </span>
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                          {tp('recent')}{' '}
                          {pattern.live_guard_attribution_delta.recent_resolved_event_count || 0}{' '}
                          {tp('resolved')}
                        </span>
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                          {tp('prior')}{' '}
                          {pattern.live_guard_attribution_delta.prior_resolved_event_count || 0}{' '}
                          {tp('resolved')}
                        </span>
                      </div>
                      <div className="mt-2">{pattern.live_guard_attribution_delta.summary}</div>
                      <div className="mt-2 flex flex-wrap gap-3">
                        <span>
                          {tp('recent')} {tp('overblocking')}{' '}
                          {formatPct(
                            (pattern.live_guard_attribution_delta.recent_overblocking_rate ||
                              0) * 100
                          )}
                        </span>
                        <span>
                          {tp('prior')} {tp('overblocking')}{' '}
                          {formatPct(
                            (pattern.live_guard_attribution_delta.prior_overblocking_rate ||
                              0) * 100
                          )}
                        </span>
                        <span>
                          {tp('delta')}{' '}
                          {formatPct(
                            (pattern.live_guard_attribution_delta.overblocking_rate_delta ||
                              0) * 100
                          )}
                        </span>
                        <span>
                          {tp('confidence')}{' '}
                          {formatPct(
                            (pattern.live_guard_attribution_delta.confidence_score || 0) *
                              100
                          )}
                        </span>
                      </div>
                    </div>
                  )}

                  {pattern.action_hint?.summary && (
                    <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 space-y-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-white/90">
                          {tp('Suggested analyst action')}
                        </span>
                        {pattern.action_hint.recommended_action && (
                          <span
                            className={`px-2 py-1 rounded-full text-[11px] border ${actionHintActionClasses(
                              pattern.action_hint.recommended_action
                            )}`}
                          >
                            {formatActionHintAction(
                              pattern.action_hint.recommended_action,
                              language
                            )}
                          </span>
                        )}
                        {pattern.action_hint.priority_label && (
                          <span
                            className={`px-2 py-1 rounded-full text-[11px] border ${actionHintPriorityClasses(
                              pattern.action_hint.priority_label
                            )}`}
                          >
                            {formatActionHintPriority(
                              pattern.action_hint.priority_label,
                              language
                            )}
                          </span>
                        )}
                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                          {tp('confidence')}{' '}
                          {formatPct(
                            (pattern.action_hint.confidence_score || 0) * 100
                          )}
                        </span>
                      </div>
                      <div className="text-xs text-nofx-text-muted">
                        {pattern.action_hint.summary}
                      </div>
                      {pattern.action_hint.auto_note && (
                        <div className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs text-nofx-text-muted">
                          {pattern.action_hint.auto_note}
                        </div>
                      )}
                      {pattern.action_hint.recommended_action && (
                        <div className="flex flex-wrap gap-2">
                          <button
                            onClick={() => void applySuggestedPatternAction(pattern)}
                            disabled={controlBusyKey !== ''}
                            className="h-9 px-3 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 text-sm text-nofx-gold disabled:opacity-50"
                          >
                            {controlBusyKey ===
                            `${patternControlKey(pattern)}:${pattern.action_hint.recommended_action}`
                              ? tp('Saving…')
                              : formatActionHintAction(
                                  pattern.action_hint.recommended_action,
                                  language
                                )}
                          </button>
                          <button
                            onClick={() =>
                              setControlNotes((current) => ({
                                ...current,
                                [patternControlKey(pattern)]:
                                  pattern.action_hint?.auto_note || '',
                              }))
                            }
                            className="h-9 px-3 rounded-lg border border-white/10 bg-white/5 text-sm text-white"
                          >
                            {tp('Use suggested note')}
                          </button>
                        </div>
                      )}
                    </div>
                  )}

                  {pattern.lifecycle_history &&
                    pattern.lifecycle_history.length > 0 && (
                      <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-2">
                        <div className="text-[11px] uppercase tracking-[0.18em] text-nofx-text-muted">
                          {tp('Lifecycle Trail')}
                        </div>
                        <div className="mt-2 flex flex-wrap gap-2">
                          {pattern.lifecycle_history.map((snapshot) => (
                            <div
                              key={snapshot.id}
                              className={`rounded-full border px-2 py-1 text-[11px] ${lifecycleStatusClasses(
                                snapshot.lifecycle_status
                              )}`}
                              title={snapshot.summary || undefined}
                            >
                              {formatLifecycleStatus(snapshot.lifecycle_status, language)} ·{' '}
                              {formatCompactTimestamp(snapshot.captured_at, language)} ·{' '}
                              {formatLifecycleSnapshotSource(
                                snapshot.snapshot_source,
                                language
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                  {canManagePattern(pattern) && (
                    <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 space-y-3">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div>
                          <div className="text-[11px] uppercase tracking-[0.18em] text-nofx-text-muted">
                            {tp('Analyst Control')}
                          </div>
                          <div className="text-xs text-nofx-text-muted mt-1">
                            {tp('Base')}{' '}
                            {formatRecommendedUse(pattern.base_recommended_use, language)} ·{' '}
                            {tp('effective')}{' '}
                            {formatRecommendedUse(pattern.recommended_use, language)}
                          </div>
                        </div>
                        {pattern.manual_control?.control_state ? (
                          <span
                            className={`px-2 py-1 rounded-full text-[11px] border ${manualControlStateClasses(
                              pattern.manual_control.control_state
                            )}`}
                          >
                            {formatManualControlState(
                              pattern.manual_control.control_state,
                              language
                            )}
                          </span>
                        ) : (
                          <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white">
                            {tp('Inherit')}
                          </span>
                        )}
                      </div>

                      {pattern.manual_control && (
                        <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-xs text-nofx-text-muted">
                          <div>
                            {tp('Last action')}{' '}
                            {formatManualControlAction(
                              pattern.manual_control.last_action,
                              language
                            )}{' '}
                            · {formatTimestamp(pattern.manual_control.applied_at, language)}
                          </div>
                          {pattern.manual_control.note && (
                            <div className="mt-1 text-white/85">{pattern.manual_control.note}</div>
                          )}
                        </div>
                      )}

                      {pattern.manual_control_history &&
                        pattern.manual_control_history.length > 0 && (
                          <div className="flex flex-wrap gap-2">
                            {pattern.manual_control_history.map((event) => (
                              <div
                                key={event.id}
                                className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-[11px] text-white/80"
                                title={event.note || undefined}
                              >
                                {formatManualControlAction(event.action, language)} ·{' '}
                                {formatCompactTimestamp(event.applied_at, language)}
                              </div>
                            ))}
                          </div>
                        )}

                      {pattern.intervention_history &&
                        pattern.intervention_history.length > 0 && (
                          <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 space-y-3">
                            <div className="text-[11px] uppercase tracking-[0.18em] text-nofx-text-muted">
                              {tp('Intervention History')}
                            </div>
                            <div className="space-y-2">
                              {pattern.intervention_history.map((event) => (
                                <div
                                  key={event.id}
                                  className="rounded-lg border border-white/10 bg-white/[0.03] px-3 py-2 text-xs text-nofx-text-muted"
                                >
                                  <div className="flex flex-wrap items-center gap-2">
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${interventionEventTypeClasses(
                                        event.event_type
                                      )}`}
                                    >
                                      {formatInterventionEventType(event.event_type, language)}
                                    </span>
                                    <span
                                      className={`px-2 py-1 rounded-full text-[11px] border ${interventionStatusClasses(
                                        event.event_status
                                      )}`}
                                    >
                                      {formatInterventionStatus(
                                        event.event_status,
                                        language
                                      )}
                                    </span>
                                    {event.suggested_action && (
                                      <span
                                        className={`px-2 py-1 rounded-full text-[11px] border ${actionHintActionClasses(
                                          event.suggested_action
                                        )}`}
                                      >
                                        {formatActionHintAction(event.suggested_action, language)}
                                      </span>
                                    )}
                                    {event.applied_action && (
                                      <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/80">
                                        {tp('Applied')}{' '}
                                        {formatManualControlAction(
                                          event.applied_action,
                                          language
                                        )}
                                      </span>
                                    )}
                                    <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/70">
                                      {formatCompactTimestamp(
                                        interventionTimestampLabel(event),
                                        language
                                      )}
                                    </span>
                                    {event.event_type === 'suggested' &&
                                      (event.seen_count || 0) > 1 && (
                                        <span className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-black/20 text-white/70">
                                          {tp('seen')} {event.seen_count}x
                                        </span>
                                      )}
                                  </div>
                                  {event.summary && (
                                    <div className="mt-2 text-white/85">
                                      {event.summary}
                                    </div>
                                  )}
                                  {event.note && (
                                    <div className="mt-2 rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-white/75">
                                      {event.note}
                                    </div>
                                  )}
                                </div>
                              ))}
                            </div>
                          </div>
                        )}

                      <textarea
                        value={controlNotes[patternControlKey(pattern)] || ''}
                        onChange={(event) =>
                          setControlNotes((current) => ({
                            ...current,
                            [patternControlKey(pattern)]: event.target.value,
                          }))
                        }
                        placeholder={tp(
                          'Why should this rule stay live, be suppressed, retired, or re-armed?'
                        )}
                        className="w-full min-h-[84px] rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-sm text-white placeholder:text-nofx-text-muted/70"
                      />

                      <div className="flex flex-wrap gap-2">
                        <button
                          onClick={() => void applyPatternControl(pattern, 'acknowledge_live')}
                          disabled={controlBusyKey !== ''}
                          className="h-9 px-3 rounded-lg border border-emerald-400/20 bg-emerald-500/10 text-sm text-emerald-200 disabled:opacity-50"
                        >
                          {controlBusyKey === `${patternControlKey(pattern)}:acknowledge_live`
                            ? tp('Refreshing…')
                            : tp('Keep live')}
                        </button>
                        <button
                          onClick={() => void applyPatternControl(pattern, 'suppress')}
                          disabled={controlBusyKey !== ''}
                          className="h-9 px-3 rounded-lg border border-amber-400/20 bg-amber-500/10 text-sm text-amber-200 disabled:opacity-50"
                        >
                          {controlBusyKey === `${patternControlKey(pattern)}:suppress`
                            ? tp('Refreshing…')
                            : tp('Suppress')}
                        </button>
                        <button
                          onClick={() => void applyPatternControl(pattern, 'retire')}
                          disabled={controlBusyKey !== ''}
                          className="h-9 px-3 rounded-lg border border-rose-400/20 bg-rose-500/10 text-sm text-rose-200 disabled:opacity-50"
                        >
                          {controlBusyKey === `${patternControlKey(pattern)}:retire`
                            ? tp('Refreshing…')
                            : tp('Retire')}
                        </button>
                        {(pattern.manual_control?.control_state === 'suppressed' ||
                          pattern.manual_control?.control_state === 'retired') && (
                          <button
                            onClick={() => void applyPatternControl(pattern, 'rearm')}
                            disabled={controlBusyKey !== ''}
                            className="h-9 px-3 rounded-lg border border-sky-400/20 bg-sky-500/10 text-sm text-sky-200 disabled:opacity-50"
                          >
                            {controlBusyKey === `${patternControlKey(pattern)}:rearm`
                              ? tp('Refreshing…')
                              : tp('Re-arm')}
                          </button>
                        )}
                      </div>
                    </div>
                  )}

                  {pattern.feature_set && pattern.feature_set.length > 0 && (
                    <div className="flex flex-wrap gap-2">
                      {pattern.feature_set.map((feature) => (
                        <button
                          key={`${pattern.id}-${feature}`}
                          onClick={() => {
                            const nextFeature = feature.replace(':', '_')
                            setFilters((current) => ({ ...current, feature: nextFeature }))
                            applyQuickFilter({ feature: nextFeature })
                          }}
                          className="px-2 py-1 rounded-full text-[11px] border border-white/10 bg-white/5 text-white"
                        >
                          {feature}
                        </button>
                      ))}
                    </div>
                  )}

                  <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-6 gap-3 text-xs">
                    <div>
                      {tp('Samples')}: {' '}
                      <span className="text-white">{pattern.sample_count}</span>
                    </div>
                    <div>
                      {tp('Support')}: {' '}
                      <span className="text-white">
                        {pattern.support_count} / {pattern.sample_count}
                      </span>
                    </div>
                    <div>
                      {tp('Avg PnL')}: {' '}
                      <span
                        className={
                          pattern.avg_pnl >= 0 ? 'text-emerald-300' : 'text-rose-300'
                        }
                      >
                        {formatMoney(pattern.avg_pnl)} / {formatPct(pattern.avg_pnl_pct)}
                      </span>
                    </div>
                    <div>
                      {tp('Lift')}: {' '}
                      <span
                        className={
                          pattern.lift_avg_pnl_pct >= 0
                            ? 'text-emerald-300'
                            : 'text-rose-300'
                        }
                      >
                        {formatPct(pattern.lift_avg_pnl_pct)}
                      </span>
                    </div>
                    <div>
                      {tp('Confidence')}: {' '}
                      <span className="text-white">
                        {formatPct((pattern.confidence_score || 0) * 100)}
                      </span>
                    </div>
                    <div>
                      {tp('Stability')}: {' '}
                      <span className="text-white">
                        {formatPct((pattern.stability_score || 0) * 100)}
                      </span>
                    </div>
                    <div>
                      {tp('Holdout')}: {' '}
                      <span className="text-white">
                        {pattern.validation_support_count || 0}/
                        {pattern.validation_sample_count || 0}
                      </span>
                    </div>
                    <div>
                      {tp('Recent')}: {' '}
                      <span className="text-white">
                        {pattern.recent_support_count || 0}/
                        {pattern.recent_sample_count || 0}
                      </span>
                    </div>
                    <div>
                      {tp('Drift')}: {' '}
                      <span
                        className={
                          (pattern.drift_score || 0) >= 0.5
                            ? 'text-orange-300'
                            : 'text-white'
                        }
                      >
                        {formatPct((pattern.drift_score || 0) * 100)}
                      </span>
                    </div>
                    <div>
                      {tp('Reverse')}: {' '}
                      <span className="text-amber-300">
                        {formatPct((pattern.reverse_risk_score || 0) * 100)}
                      </span>
                    </div>
                    <div>
                      FP:{' '}
                      <span className="text-rose-300">
                        {formatPct((pattern.false_positive_score || 0) * 100)}
                      </span>
                    </div>
                    <div>
                      {tp('Avg hold')}: {' '}
                      <span className="text-white">
                        {formatHold(pattern.avg_hold_ms || 0)}
                      </span>
                    </div>
                  </div>

                  <div className="flex flex-wrap gap-2">
                    <button
                      onClick={() => openDealReview(pattern)}
                      className="h-9 px-3 rounded-lg border border-white/10 bg-white/5 text-xs font-medium"
                    >
                      {tp('Open cohort in review')}
                    </button>
                    {pattern.symbol && (
                      <button
                        onClick={() =>
                          applyQuickFilter({
                            symbol: pattern.symbol,
                            side: pattern.side,
                            scopeType: pattern.scope_type,
                          })
                        }
                        className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-xs font-medium"
                      >
                        {tp('Focus')} {pattern.symbol}
                      </button>
                    )}
                    <button
                      onClick={() => {
                        navigator.clipboard
                          .writeText(pattern.pattern_signature)
                          .then(() => notify.success(tp('Pattern signature copied')))
                          .catch(() => notify.error(tp('Failed to copy pattern signature')))
                      }}
                      className="h-9 px-3 rounded-lg border border-white/10 bg-black/20 text-xs font-medium"
                    >
                      {tp('Copy signature')}
                    </button>
                  </div>

                  {pattern.evidence && pattern.evidence.length > 0 && (
                    <div className="space-y-2">
                      <div className="text-xs uppercase tracking-[0.2em] text-nofx-text-muted">
                        {tp('Evidence')}
                      </div>
                      <div className="space-y-2">
                        {pattern.evidence.map((evidence) => (
                          <div
                            key={`${pattern.id}-${evidence.case_id}`}
                            className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 flex flex-wrap items-center justify-between gap-3"
                          >
                            <div className="text-xs text-nofx-text-muted">
                              <span className="text-white">
                                {evidence.symbol} · {evidence.side}
                              </span>{' '}
                              · {evidence.outcome || '-'} · {formatMoney(evidence.realized_pnl)} /{' '}
                              {formatPct(evidence.realized_pnl_pct)} ·{' '}
                              {formatHold(evidence.hold_duration_ms || 0)}
                            </div>
                            <button
                              onClick={() => openDealReview(pattern, evidence.case_id)}
                              className="h-8 px-3 rounded-lg border border-white/10 bg-black/20 text-xs"
                            >
                              {tp('Open deal')}
                            </button>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </DeepVoidBackground>
  )
}
