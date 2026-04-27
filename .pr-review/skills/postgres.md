# PostgreSQL Review Skill

Review Postgres-specific code: data types, indexing strategies, MVCC behavior, replication, and operational config.

## Data Types

**Anti-Patterns → Findings**:
- `JSON` instead of `JSONB` → IMPORTANT: `JSON` stores as text, no indexing, no operator support. Use `JSONB` unless you specifically need to preserve key order/whitespace.
- `TIMESTAMP WITHOUT TIME ZONE` for events → CRITICAL: ambiguity across regions/DST. Use `TIMESTAMPTZ` (stored as UTC, returned in client TZ).
- `VARCHAR(n)` with arbitrary `n` → NIT: PG stores `VARCHAR` and `TEXT` identically. Use `TEXT` unless there's a real business limit; `n` adds a `CHECK`-style constraint with no perf benefit.
- `CHAR(n)` for non-fixed-width data → NIT: pads with spaces. Almost never what you want.
- Money as `FLOAT8`/`REAL` → CRITICAL: precision loss. Use `NUMERIC(p, s)`.
- `SERIAL`/`INTEGER` PK on a high-volume table → IMPORTANT: 2.1B ceiling. Use `BIGSERIAL`/`BIGINT` or `IDENTITY` from day one.
- `UUID` stored as `VARCHAR(36)` → NIT: native `UUID` is half the storage and faster compare.
- Enum (`CREATE TYPE ... AS ENUM`) when values may change → IMPORTANT: removing/reordering enum values requires careful migration. Lookup table is more flexible for evolving sets.

## JSONB

- Querying JSONB by inner field without index (`WHERE doc->>'status' = 'active'`) → IMPORTANT: full scan + JSON parse per row. Use expression index or GIN.
- GIN index on whole document when only a few fields are queried → IMPORTANT: oversized index. Use expression indexes on the actual query paths.
- Storing arrays in JSONB and `ANY(...)`-querying them → IMPORTANT: GIN with `jsonb_path_ops` is faster than default for containment queries.
- Mutating JSONB by reading, modifying in app, writing back → IMPORTANT: lost-update under concurrency. Use `jsonb_set` server-side.
- Schemaless JSONB used as a substitute for proper columns on hot fields → IMPORTANT: forfeits type safety, indexing, and stats. Promote to columns.

## Indexing

- Btree on low-cardinality boolean alone → NIT: rarely useful. Use partial: `WHERE is_active = true`.
- No partial index for hot subset (`WHERE deleted_at IS NULL`) → IMPORTANT: smaller, faster index for the dominant query.
- Composite index column order not matching the query's leftmost-prefix usage → IMPORTANT: index unused.
- `CREATE INDEX` (not `CONCURRENTLY`) on a hot table → CRITICAL: blocks writes for the build duration.
- Index on a function/expression query without expression index → IMPORTANT: planner can't use the index. Add `CREATE INDEX ... ON t (LOWER(email))`.
- Duplicate / overlapping indexes (`(a)`, `(a, b)`, `(a, b, c)` all present) → NIT: write amplification. Often only the widest is needed.
- Missing FK index → IMPORTANT: parent updates/deletes scan child fully and hold locks.

## Query Planning

- `WHERE` with `OR` across columns → IMPORTANT: planner may switch to seq scan. Often `UNION ALL` of indexed branches is faster.
- Subquery using parameterized prepared statement causing generic plan → IMPORTANT: generic vs custom plan choice can flip after PG picks a bad plan. Consider `plan_cache_mode = force_custom_plan` for sensitive paths.
- `LIMIT` with `OFFSET` for deep pagination → IMPORTANT: scans + discards. Use keyset pagination.
- `EXPLAIN` not consulted on a non-trivial new query → IMPORTANT: best-effort review wants `EXPLAIN (ANALYZE, BUFFERS)` posted in the PR.

## MVCC, Bloat & Vacuum

- Frequently-updated wide row table with no fillfactor tuning → IMPORTANT: high bloat. Set `fillfactor < 100` to leave room for HOT updates.
- Long-running transaction (idle in transaction) → CRITICAL: blocks vacuum, accumulates bloat across the whole DB. Set `idle_in_transaction_session_timeout`.
- `VACUUM FULL` in a migration → CRITICAL: takes ACCESS EXCLUSIVE lock, rewrites table.
- Heavy `UPDATE` workload without expectation that autovacuum keeps up → IMPORTANT: tune `autovacuum_vacuum_scale_factor` per-table.
- TOAST'd columns updated in every UPDATE even when unchanged → IMPORTANT: consider splitting large/rarely-changing columns.
- `pg_repack` invoked from app code path → CRITICAL: maintenance op, not request-path.

## Concurrency Specific

- `INSERT ... ON CONFLICT DO UPDATE` without confirming the unique constraint matches the conflict target → IMPORTANT: silent insert-as-no-op when constraint doesn't match.
- Advisory locks shared across unrelated subsystems on the same integer key → CRITICAL: namespace collision. Use the two-arg form `pg_advisory_lock(class, id)`.
- `SELECT ... FOR UPDATE SKIP LOCKED` not used for worker-queue patterns → IMPORTANT: workers serialize on the same row.
- Forgetting `SELECT ... FOR NO KEY UPDATE` for non-key updates → NIT: blocks fewer concurrent operations than `FOR UPDATE`.

## Replication & HA

- Read after write routed to a streaming replica → CRITICAL: replication lag, just-written row missing. Route read-after-write to primary.
- Logical replication enabled but DDL changes not replicated (Postgres logical doesn't replicate DDL) → IMPORTANT: schema must be deployed to the subscriber separately.
- Long replication slot left orphaned → CRITICAL: WAL piles up, disk fills.
- Synchronous replication (`synchronous_commit = on` + `synchronous_standby_names`) without standby → CRITICAL: every commit blocks indefinitely.
- Failover assumed seamless mid-transaction → IMPORTANT: in-flight tx rolled back; app must retry.

## Connection & Pooling

- App opening a new connection per request → CRITICAL: PG fork-per-conn model, expensive. Pool via PgBouncer / RDS Proxy / app pool.
- Pool size > `max_connections / replicas` → CRITICAL: thundering herd to DB. Right-size and prefer transaction pooling.
- PgBouncer in `transaction` mode + app using session-level features (advisory locks across statements, `SET LOCAL` outside tx, prepared statements) → CRITICAL: features silently broken. Use `session` mode or refactor.
- No `statement_timeout` on user-facing connection → IMPORTANT: one slow query holds a connection.
- No `idle_in_transaction_session_timeout` → IMPORTANT: hung clients pin tx-snapshot, block vacuum.

## Security

- `pg_hba.conf` with `trust` auth → CRITICAL: passwordless. `md5` is also weak — prefer `scram-sha-256`.
- App connecting as superuser → CRITICAL: blast radius. Use a least-privilege role.
- Row-level security policies bypassed by `BYPASSRLS` role used in app → CRITICAL: defeats the purpose.
- Backups without encryption at rest → IMPORTANT.

## Operational

- New extension installed without confirming it's allowed in managed PG (RDS/Cloud SQL only allow specific list) → IMPORTANT: deploy will fail.
- Triggers doing heavy work in row-level handler → IMPORTANT: per-row cost amplifies bulk operations. Consider statement-level or async via outbox.
- Stored procedures used as primary business logic without testing path → IMPORTANT: hard to test, monitor, and version. Confirm intentional.
- Large `text` / `bytea` columns retrieved on every read → IMPORTANT: wasted I/O. Project them out unless needed.
