# Internal Pgvector Semantic Memory Scope

Date: 2026-04-18
Owner: Codex draft for implementation
Status: Phase 6 retrieval benchmark slice live

## Goal

Add an internal semantic-memory layer on top of PostgreSQL so NOFX can search similar review cases, optimizer runs, strategy changes, and backlog findings without depending on an external hosted vector store.

The target architecture is:

- PostgreSQL remains the source of truth
- `pgvector` provides internal vector indexing
- OpenAI embeddings are used first for embedding generation
- semantic memory is fed from compact, curated knowledge documents instead of raw full-fidelity trading telemetry

## Why This Scope Exists

We explicitly decided against indexing the whole raw `decision_records` corpus first.

That remains possible later, but it is not the right first slice because:

- it is materially larger than the review and optimizer corpus
- it is noisier and more operational than analytical
- it would raise embedding cost and retrieval noise before we even prove the retrieval UX

The right first corpus is:

- `deal_review_cases`
- `autonomous_optimizer_runs`
- `autonomous_optimizer_backlog_items`
- later `deal_review_strategy_versions`

## Design Principles

### 1. PostgreSQL First

- Postgres remains the canonical data store
- no semantic document should become the only source of truth
- semantic memory is a derived projection of existing structured data

### 2. Curated Knowledge Documents

- we index compact, human-readable documents
- we do not dump full raw prompts, giant JSON blobs, or every market snapshot by default
- documents should be stable enough for retrieval and cheap enough to rebuild

### 3. Rebuildable Pipeline

- semantic docs can be backfilled from live tables at any time
- content hashes determine whether a document needs re-embedding
- embedding state is explicit and recoverable

### 4. Pgvector As Local Infra, Not Magic

- retrieval quality still depends on document design and filters
- vector search should be combined with structured filters like trader, symbol, outcome, regime, and time window
- we should not wire semantic retrieval into live trading decisions before review and optimizer use cases prove value

## Target Use Cases

### A. Deal Review Similarity

- find similar historical losses, give-back exits, regime mismatches, and trailing-stop cases
- help analysts see what similar cases looked like before and after strategy changes

### B. Optimizer Memory

- find similar prior `blocked_by_gate`, `deferred`, `kept`, and `rolled_back` runs
- reuse earlier lessons instead of relying only on the most recent few runs

### C. Strategy Change Memory

- retrieve similar patches, similar target cohorts, and similar outcome patterns
- later compare whether a new patch resembles previous winners or losers

### D. Analyst Search

- query semantic memory over the review corpus with trader and cohort filters
- surface exact source records and links back into review and optimizer UI

## Data Model

### Semantic Documents

Each semantic document should store:

- document id
- user id
- trader id
- document type
- source id
- source updated timestamp
- title
- summary
- body
- metadata JSON
- content hash
- token estimate
- embedding provider/model metadata
- embedding status and timestamps

### Sync Runs

Each backfill or rebuild run should store:

- run id
- scope
- status
- counts for inserted, updated, unchanged, failed docs
- summary
- metadata JSON
- started/completed timestamps

### Future Pgvector Layer

Once `pgvector` is enabled, add:

- vector column or vector-backed chunk table
- ANN indexes
- chunk and retrieval metadata
- optional hybrid lexical + vector ranking

## Phased Delivery Plan

## Phase 0. Corpus And Infra Decisions

- [x] choose internal PostgreSQL + `pgvector` strategy over hosted vector store
- [x] cost-check current live corpus size against OpenAI embedding pricing
- [x] decide to start with compact review and optimizer corpora instead of full `decision_records`
- [ ] finalize whether initial retrieval stays document-level or starts with chunk-level indexing

## Phase 1. Semantic Document Registry

- [x] add scope document
- [x] add semantic document table
- [x] add semantic sync run table
- [x] add status model for `pending_embedding`, `embedded`, `failed`, `skipped`
- [x] add content-hash based upsert logic
- [x] add token estimation for cost visibility
- [x] add internal backfill command for the first core corpora
- [x] add first document builders for:
  - `deal_review_cases`
  - `autonomous_optimizer_runs`
  - `autonomous_optimizer_backlog_items`
- [x] add `deal_review_strategy_versions` document builder

## Phase 2. Pgvector Runtime Enablement

- [x] enable `vector` extension in the runtime PostgreSQL image
- [x] add store capability checks for vector extension presence/version
- [x] add migration path for vector-backed storage tables
- [x] decide initial embedding dimension strategy
- [ ] verify local compose and production compose both support the extension in deployed environments

## Phase 3. Embedding Pipeline

- [x] add internal embedding client for OpenAI embeddings
- [x] default to `text-embedding-3-small`
- [x] store provider, model, dimensions, and embed timestamps per document
- [x] add incremental embedding worker that only processes changed docs
- [x] add retry/error state for failed embedding jobs
- [x] add optional CLI flags for `re-embed`, `only-pending`, `doc-type`, `trader-id`

## Phase 4. Retrieval Layer

- [x] add similarity search store methods with structured filters
- [x] support filter combinations for trader, symbol, outcome, close reason, exit origin, regime, and time window
- [x] add optimizer-memory retrieval for similar prior runs
- [x] add review-memory retrieval for similar deal cases
- [x] add source links from hits back into review or optimizer records

## Phase 5. UI And Analyst Workflows

- [x] show similar historical cases in deal-review detail
- [x] show similar prior optimizer runs in optimizer run detail
- [x] add analyst semantic search page or module
- [x] expose retrieval scores and source metadata
- [x] expose embedding model, sync status, and corpus counts in UI

## Phase 6. Expansion And Hardening

- [x] add strategy-version corpus
- [ ] evaluate whether selected `decision_records` summaries should be indexed later
- [x] add backfill scheduling and periodic refresh jobs
- [x] add observability for embedding cost estimates and sync failures
- [x] add benchmarks for retrieval quality before broader rollout
- [ ] evaluate optional future self-hosted embedding models if OpenAI should later be removed from the embedding path

## Current Delivery Slice

The first safe slice has now moved past corpus-only groundwork. Runtime `pgvector`, embedding backfill, retrieval APIs, and the first dedicated analyst UI are live. The current delivery line now stops after the first free-text analyst search workflow and before broader corpus expansion.

Completed in this slice:

- [x] semantic-memory scope document
- [x] persistent semantic document registry
- [x] sync-run tracking for corpus rebuilds
- [x] curated document builders for review cases, optimizer runs, and backlog items
- [x] content-hash based incremental backfill
- [x] CLI backfill entry point
- [x] tests for semantic document backfill/update behavior
- [x] `pgvector` runtime image in compose
- [x] vector extension bootstrap and vector table creation
- [x] OpenAI embedding pipeline using `text-embedding-3-small`
- [x] idempotent embedding backfill against live Postgres
- [x] similar-case and similar-run backend retrieval endpoints
- [x] similar-case UI block in deal-review detail
- [x] similar-run UI block in optimizer run detail
- [x] free-text analyst semantic search backend endpoints
- [x] dedicated `/memory` analyst retrieval page
- [x] corpus-status, sync-run, and embedding-model visibility in UI
- [x] retrieval score and source-link visibility in UI
- [x] strategy-version corpus builder and live backfill
- [x] targeted CLI controls for `doc-type`, `only-pending`, and `re-embed`
- [x] saved semantic-memory search presets on `/memory`
- [x] direct similar-strategy-version retrieval in `/deal-review`
- [x] periodic semantic-memory refresh supervisor with automatic backfill and embed passes
- [x] semantic-memory observability for token estimates, cost estimates, recent embed usage, and sync failures
- [x] heuristic similarity benchmark runner with persisted runs, API access, CLI entry point, and `/memory` visibility

Deliberately left for the next slice:

- [ ] production-environment pgvector verification beyond local compose/runtime
- [ ] evaluate selected `decision_records` summaries before expanding corpus
- [ ] calibrate benchmark heuristics and thresholds after more strategy-version corpus coverage

## Notes For The Next Slice

- start retrieval at document level, not chunk level
- keep the document body compact and opinionated
- only bring `decision_records` in later if we define curated summaries instead of raw full-prompt indexing
- keep analyst retrieval scoped to authenticated internal review workflows
- current benchmarking is heuristic and strongest on deal-review cases; optimizer/strategy corpora should be recalibrated as they grow
- next focus should be selected `decision_records` evaluation, heuristic calibration, and wider pgvector validation outside the local compose runtime
