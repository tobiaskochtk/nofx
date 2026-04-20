# PostgreSQL Migration Scope

Date: 2026-04-18
Owner: Codex draft for implementation
Status: Main app and selfhosted-ai500 locally cut over to PostgreSQL; legacy SQLite config layer isolated from default build

## Goal

Stop recurring SQLite corruption in the main NOFX runtime by moving persistent application data to PostgreSQL with a controlled cutover and verifiable data migration.

This scope is about the primary NOFX app first. Sidecar services and pure test fixtures are tracked separately so we do not confuse "all SQLite strings in the repo" with "what still keeps the production system on SQLite".

## Audit Summary

A fresh repository audit found four different SQLite buckets:

- the primary app runtime is already partially PostgreSQL-capable, but still defaults to SQLite
- deployment manifests still do not provide a first-class PostgreSQL path for the main app
- a large legacy SQLite-only config database layer still exists in the repo, but appears disconnected from the live startup path
- sidecars, repair scripts, tests, and docs still contain significant SQLite assumptions

## Where SQLite Is Still In Use

### A. Main App Runtime

- `config/config.go`
  - runtime now defaults to PostgreSQL; `DBPath=data/data.db` remains only as a legacy SQLite fallback
- `main.go`
  - positional CLI path override and local directory creation now apply only when SQLite is explicitly selected
- `store/driver.go`
  - `DB_TYPE` now defaults to `postgres`
- `store/gorm.go`
  - both backends still exist, but SQLite is now an explicit legacy fallback instead of the runtime default
- `store/sqlite_runtime.go`
  - only explicit SQLite pragma configuration remains; write-retry logic was removed from live runtime paths
- `store/position_history.go`
  - closed-PnL sync writes now fail normally instead of using SQLite-specific retry wrappers
- `store/deal_review.go`
  - live deal-review queries now fail normally instead of swallowing SQLite corruption errors
- `store/equity.go`
  - legacy equity migration now uses a backend-neutral table existence check

### B. Deploy / Packaging

- `docker-compose.yml`
  - main app had no bundled PostgreSQL service or DB env wiring
- `docker-compose.prod.yml`
  - same gap in production compose
- `docker-compose.stable.yml`
  - same gap in stable compose
- `Dockerfile.railway`
  - Railway bootstrap now defaults to PostgreSQL and maps `PG*` env vars into `DB_*`
- `docker/Dockerfile.backend`
  - backend runtime image no longer installs the `sqlite` system package

### C. Legacy SQLite-Only Database Layer

- `config/database.go`
  - large raw `database/sql` implementation bound directly to SQLite pragmas and SQLite schema details
- `config/deals.go`
  - legacy deal accessors attached to that database type
- `config/database_test.go`
  - legacy tests still validate SQLite WAL/synchronous behaviour

Current assessment:

- this layer appears to be repository baggage rather than the active app path
- it still matters because it preserves old tables like `beta_codes` and `user_signal_sources` that are not represented in the current `store` schema

### D. Sidecars And Tooling

- `internal/selfhostedai500/store.go`
  - separate SQLite store with WAL pragmas
- `scripts/migrate_encryption/main.go`
- `scripts/migrate_close_reason.go`
- `scripts/fix_trader_userid/main.go`
- `scripts/fix_closed_deals/main.go`
- root-level repair / recovery PowerShell and shell scripts

These are not the main NOFX runtime, but they still anchor operational habits around SQLite files.

### E. Tests, Docs, And Frontend Text

- many `*_test.go` files intentionally use SQLite for temporary fixtures
- troubleshooting and FAQ docs still describe SQLite lock/corruption behaviour
- frontend translations still tell users that secrets/config live in `data.db`

These should be cleaned up after runtime cutover, not before.

## Important Schema Gap

The current live `store` package does not create every table that exists in the old SQLite config layer.

Notable examples:

- `beta_codes`
- `user_signal_sources`

That means a blind "create PostgreSQL schema from current store models, then copy every SQLite table" migration would either fail or silently drop those legacy tables.

Current assessment:

- they look unused by the live startup/API path today
- they still need explicit handling in migration tooling and the final cleanup plan

## Recommended Migration Path

### Phase 1. Provision PostgreSQL Beside SQLite

- add a first-class PostgreSQL service to compose files
- wire `DB_TYPE`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`
- do not force immediate cutover by changing defaults in code before data migration exists

### Phase 2. Create A Deterministic SQLite -> PostgreSQL Migrator

- initialize an empty PostgreSQL schema using the current `store` package
- inspect the SQLite source schema
- copy only tables that exist in both source and target
- preserve primary keys/IDs during copy
- reset PostgreSQL sequences after import
- emit a warning for legacy SQLite-only tables that were skipped

### Phase 3. Cut Over The Main Runtime

- run the migration against the live SQLite DB
- verify row counts table by table
- switch deployment env to `DB_TYPE=postgres`
- observe live startup and deal/trader loading against PostgreSQL

### Phase 4. Retire SQLite-First Runtime Code

- remove SQLite corruption fallbacks from hot paths where PostgreSQL is now authoritative
- delete or quarantine the legacy `config/database.go` layer
- update docs/translations from SQLite operational guidance to PostgreSQL backup/restore guidance

### Phase 5. Handle Remaining Exceptions

- decide whether `selfhosted-ai500` stays on SQLite as an intentional local cache or gets its own PostgreSQL path
- prune obsolete repair scripts that only exist because the main app used SQLite files

## Risks And Constraints

- the repo contains many uncommitted changes, so migration work should avoid broad refactors in already-hot files unless required
- blindly flipping the global default from SQLite to PostgreSQL would make existing installs look empty before their data is migrated
- sequence-backed integer IDs in PostgreSQL must be reseeded after copying explicit IDs from SQLite
- old legacy tables that no longer exist in `store` must be surfaced, not silently ignored

## Current Delivery Slice

This implementation pass starts with the minimum slice that materially moves us toward PostgreSQL without a dangerous forced cutover.

- [x] audit all SQLite usage buckets and separate runtime from baggage
- [x] add this scope document
- [x] add a dedicated `cmd/migrate_sqlite_to_postgres` tool
- [x] add PostgreSQL service wiring to compose manifests
- [x] run a local SQLite snapshot backup and confirm it was still corrupt
- [x] rebuild a clean migration source via SQLite `.recover`
- [x] migrate the main overlapping runtime tables into PostgreSQL
- [x] verify migrated row counts table-by-table for the main runtime schema
- [x] cut the local runtime over to PostgreSQL and verify `/api/health`
- [x] isolate the legacy SQLite-only `config/database.go` stack from the default build via build tags
- [x] migrate `selfhosted-ai500` cache/state tables into PostgreSQL
- [x] rewrite docs/frontend text that still present SQLite as the normal production datastore
- [x] flip main runtime defaults and compose fallbacks from SQLite to PostgreSQL
- [x] remove SQLite-specific write retries, corruption suppression, and runtime image packaging from the active runtime path

## Migration Tooling In This Slice

Planned usage:

```bash
go run ./cmd/migrate_sqlite_to_postgres \
  -sqlite-path ./data/data.db \
  -pg-host localhost \
  -pg-port 5432 \
  -pg-user nofx \
  -pg-password nofx \
  -pg-db nofx \
  -pg-sslmode disable
```

Behaviour of the first slice:

- creates/validates the PostgreSQL schema using the live `store` package
- requires the target tables to be empty
- copies the intersection of source and target tables in foreign-key-safe order
- resets PostgreSQL sequences afterward
- reports skipped SQLite-only legacy tables explicitly

## Execution Notes

Local execution completed on 2026-04-18 against:

- source runtime DB snapshot: `data/data.db.before_postgres_cutover_20260418_122314.db`
- recovered migration source: `data/data.db.recovered_for_postgres_20260418_122314.db`
- archived SQLite-only table dump: `data/sqlite_only_tables_20260418_122314.sql`
- selfhosted-ai500 source snapshot: `data/selfhosted-ai500/selfhosted-ai500.db.before_postgres_cutover_20260418_124310.db`

Observed result:

- the stopped runtime snapshot still failed `PRAGMA integrity_check`, confirming recurring SQLite corruption in the active dataset
- the rebuilt `.recover` database passed integrity check
- the main runtime table set was migrated successfully into PostgreSQL
- row counts were verified table-by-table between recovered SQLite and PostgreSQL for the migrated runtime schema
- local runtime was cut over and `GET /api/health` returned `{"status":"ok","time":null}`
- the legacy SQLite-only `config/database.go` path is now behind the `legacy_sqlite_config` build tag and no longer participates in default builds/tests
- `selfhosted-ai500` was rebuilt with PostgreSQL support, its SQLite cache tables were migrated, and `/health` now reports `status":"ok"` while the container runs with `SELFHOSTED_AI500_DB_TYPE=postgres`

## Runtime Tables Verified

- `ai_charges`
- `ai_models`
- `autonomous_optimizer_backlog_items`
- `autonomous_optimizer_configs`
- `autonomous_optimizer_runs`
- `deal_review_ai_scans`
- `deal_review_cases`
- `deal_review_challenger_compares`
- `deal_review_classifier_feedback`
- `deal_review_cycle_points`
- `deal_review_events`
- `deal_review_exit_intents`
- `deal_review_filter_presets`
- `deal_review_market_points`
- `deal_review_strategy_versions`
- `deal_review_trailing_updates`
- `decision_records`
- `exchanges`
- `grid_configs`
- `grid_events`
- `grid_instances`
- `grid_levels`
- `grid_regime_assessments`
- `strategies`
- `system_config`
- `telegram_configs`
- `trader_equity_snapshots`
- `trader_fills`
- `trader_orders`
- `trader_positions`
- `traders`
- `users`

## SQLite-Only Tables Still Outside The Store Schema

These were not dropped. They were preserved in the SQLite backups and exported separately to `data/sqlite_only_tables_20260418_122314.sql`.

- `backtest_checkpoints` with `1` row
- `backtest_decisions` with `24` rows
- `backtest_equity` with `480` rows
- `backtest_metrics` with `1` row
- `backtest_runs` with `1` row
- `backtest_trades` with `0` rows
- `debate_messages` with `0` rows
- `debate_participants` with `0` rows
- `debate_sessions` with `0` rows
- `debate_votes` with `0` rows
- `lost_and_found` with `15` rows

## selfhosted-ai500 Migration Notes

The sidecar cache/state store now supports both SQLite and PostgreSQL. Docker compose defaults were switched to PostgreSQL for this service.

Migration execution on 2026-04-18:

- SQLite backup used: `data/selfhosted-ai500/selfhosted-ai500.db.before_postgres_cutover_20260418_124310.db`
- imported counts at migration time:
  - `market_snapshots = 518279`
  - `score_state = 312`
- after service restart, PostgreSQL counts changed slightly during normal retention pruning and fresh refreshes:
  - `market_snapshots = 516839`
  - `score_state = 312`

This is expected because the service prunes old snapshots on refresh.

## Cutover Rule

Do not change the main runtime default to PostgreSQL in code until at least one verified migration path exists and the deployment manifests can boot PostgreSQL cleanly.
