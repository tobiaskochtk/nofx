# PostgreSQL Migration Scope

Date: 2026-04-18
Owner: Codex draft for implementation
Status: Audit complete, slice 1 in progress

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
  - global defaults still point to `DBType=sqlite` and `DBPath=data/data.db`
- `main.go`
  - startup still contains explicit SQLite-first compatibility handling for CLI override and local directory creation
- `store/driver.go`
  - `DB_TYPE` defaults to `sqlite`
- `store/gorm.go`
  - both backends exist, but SQLite remains a full first-class dialector in the live store bootstrap
- `store/sqlite_runtime.go`
  - WAL, busy timeout, and retry logic are SQLite-specific operational patches
- `store/position_history.go`
  - write paths still call `retrySQLiteWrite(...)`
- `store/deal_review.go`
  - some live paths explicitly swallow or downgrade SQLite corruption errors
- `store/equity.go`
  - historical migration detection still queries `sqlite_master`

### B. Deploy / Packaging

- `docker-compose.yml`
  - main app had no bundled PostgreSQL service or DB env wiring
- `docker-compose.prod.yml`
  - same gap in production compose
- `docker-compose.stable.yml`
  - same gap in stable compose
- `Dockerfile.railway`
  - still hardwires `DB_PATH=/app/data/data.db`
- `docker/Dockerfile.backend`
  - still installs the `sqlite` runtime package in the backend image

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
- [x] add PostgreSQL service wiring to compose manifests while keeping SQLite as the default until cutover is explicit
- [ ] remove the legacy SQLite-only config layer
- [ ] migrate or intentionally isolate `selfhosted-ai500`
- [ ] rewrite docs/frontend text that still present SQLite as the normal production datastore

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

## Cutover Rule

Do not change the main runtime default to PostgreSQL in code until at least one verified migration path exists and the deployment manifests can boot PostgreSQL cleanly.
