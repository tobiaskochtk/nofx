# German Localization Scope

**Goal:** Add first-class German support across NOFX so a German-speaking user can choose `de` as the product language and get a coherent experience across core onboarding, strategy configuration, dashboard flows, and other visible UI surfaces.

**Important scope constraint:** German is a user-interface locale only. Internal config language and AI prompt language remain English unless a specific feature explicitly requires otherwise.

**Definition of done for full support:**

- `de` is a selectable and persisted UI language
- core UI labels, feedback states, and prompts render in German
- strategy defaults and internal prompt templates may remain English
- date/time and locale-sensitive formatting respect German locale rules where applicable
- no critical user path silently falls back to Chinese-specific logic

## Current state

The repo already has partial i18n infrastructure:

- centralized UI strings in `web/src/i18n/translations.ts`
- strategy-specific helper translations in `web/src/i18n/strategy-translations.ts`
- persisted language state in `web/src/contexts/LanguageContext.tsx`
- backend strategy defaults keyed primarily by `zh` and `en`

But German is not yet a first-class locale. The current implementation has two different localization models:

- centralized translation keys via `t(...)`
- many direct `language === 'zh' ? ... : ...` branches embedded in components

That means adding `de` is not just a dictionary task. It requires both locale plumbing and cleanup of hard-coded language branching.

## Findings from the codebase

### Centralized i18n is available

- `web/src/i18n/translations.ts` contains the main UI dictionary for `en`, `zh`, and `id`
- `web/src/i18n/strategy-translations.ts` already falls back to English for unknown languages via `ts(...)`

### German is currently blocked in multiple places

- `Language` type only allows `en | zh | id`
- language pickers only expose Chinese, English, and Indonesian
- `LanguageContext` persistence whitelist rejects `de`
- strategy editor code narrows language to `zh | en`
- backend strategy defaults normalize to `zh` or `en`
- registration/default strategy creation logic does not define German names/descriptions

### Hard-coded language branching is widespread

Observed in `web/src`:

- roughly 28 files contain direct language branching or locale assumptions
- roughly 136 matched hotspots from `rg` for patterns like:
  - `language === 'zh'`
  - `lang === 'zh'`
  - hard-coded `zh-CN`
  - `zh/en`-only casts

Representative files:

- `web/src/components/common/HeaderBar.tsx`
- `web/src/components/common/Header.tsx`
- `web/src/pages/SettingsPage.tsx`
- `web/src/components/landing/*`
- `web/src/components/trader/*`
- `web/src/components/strategy/*`
- `web/src/components/charts/*`

## Scope

### Phase 1: Locale foundation

In scope:

- add `de` to frontend language types
- persist and restore `de`
- add German option to all language switchers
- introduce shared locale helpers for:
  - HTML `lang`
  - date/time formatting
  - widget locale mapping
- add German base translations with English fallback

Out of scope for this phase:

- exhaustive translation of every hard-coded string in every page
- copywriting refinement for every long-form marketing section

### Phase 2: Core product flows

In scope:

- auth/login/register shell
- landing page shared text driven by `t(...)`
- header/navigation shell
- strategy studio core labels
- dashboard/charts/common status strings
- backend strategy default config remains English for `de` users where it feeds AI prompts
- German default strategy names/descriptions for newly registered users

### Phase 3: Hard-coded component cleanup

In scope:

- convert direct `zh/en` ternaries into translation keys or locale helpers
- remove `zh-CN` assumptions from chart/date formatting
- remove `zh | en`-only casts and narrowings

Priority targets:

- `web/src/pages/SettingsPage.tsx`
- `web/src/components/landing/*`
- `web/src/components/trader/*`
- `web/src/components/faq/*`
- `web/src/components/strategy/*`

### Phase 4: QA and completeness pass

In scope:

- smoke-test every page in `de`
- verify fallback behavior for untranslated keys
- check layout overflow in long German labels
- check prompt generation, strategy creation, and strategy save/update

## Initial implementation slice in this change

This first pass should cover:

- scope document
- `de` as an official UI language
- German base dictionary with English fallback for centralized translations
- locale helpers for dates/document language
- German support in visible language switchers and header shell
- English prompt/config language retained behind the scenes for German UI users
- German default strategy names/descriptions at registration time

## Implementation status

Completed in the current pass:

- `de` wired into frontend language types, persistence, switchers, and document locale handling
- German translation overlay added with English fallback for centralized `t(...)` keys
- German added across landing, onboarding selector, settings shell, FAQ layout, and multiple landing sections
- strategy/config internals kept English for `de` users in:
  - frontend strategy save/load flow
  - backend strategy default config normalization
  - strategy API `lang` normalization
  - user registration strategy-config creation
- chart locale/date handling updated to respect German UI locale
- visible German coverage extended across trader-facing surfaces:
  - beginner quickstart cards
  - AI traders page alerts and quick-setup messaging
  - config status grid chips/status labels
  - traders list controls/tooltips
  - trader config modal strategy summary labels
  - model config wallet/deposit/help text
  - position history stats/date formatting/pagination labels
  - metric tooltips
  - strategy editor toast messages
  - FAQ special content blocks

Explicitly kept English by design:

- internal strategy config language for `de` users
- AI prompt generation language and backend prompt templates
- any backend-only/internal comments or identifiers not rendered to end users

Remaining follow-up candidates:

- broad manual UX sweep for any isolated hard-coded English strings outside the audited priority surfaces
- mobile overflow pass for longer German labels in compact cards/modals
- optional cleanup of residual `language === 'zh'` branches that are now harmless because they already route `de` correctly

## Risks

- German labels are longer than English and may overflow compact controls
- some pages will still show English because they bypass `t(...)`
- strategy prompts need to remain semantically equivalent across languages
- TradingView/widget locale support may still need an explicit second pass if German-specific locale codes are required beyond the current safe fallback

## Recommended follow-up order

1. Finish `SettingsPage` hard-coded text conversion.
2. Finish landing page component ternaries.
3. Convert trader modals/toasts/status chips to translation keys.
4. Sweep remaining `zh-CN` and `zh/en` assumptions.
5. Run a German-language UX smoke test across mobile and desktop.
