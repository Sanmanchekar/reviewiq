# Database Migrations Review Skill

Review schema changes for safety under rolling deploys, replication, and large data volumes.

## Locking & Online Safety (Postgres-centric, MySQL noted)

**Anti-Patterns → Findings**:
- `ADD COLUMN ... NOT NULL` without `DEFAULT` (PG <11) or with volatile default (any version) → CRITICAL: rewrites entire table under `ACCESS EXCLUSIVE` lock.
- `ALTER COLUMN ... TYPE` (narrowing, or any type change PG can't do in-place) → CRITICAL: full table rewrite + lock. Use new column + backfill + swap.
- Adding index without `CONCURRENTLY` (`CREATE INDEX CONCURRENTLY`) on large/hot tables → CRITICAL: blocks writes for the duration of the build.
- Dropping index without `CONCURRENTLY` → IMPORTANT: brief but blocking lock; use `DROP INDEX CONCURRENTLY`.
- `ADD FOREIGN KEY` without `NOT VALID` + later `VALIDATE CONSTRAINT` → IMPORTANT: validates synchronously under heavy lock. Two-step it.
- `ADD CHECK` constraint without `NOT VALID` then `VALIDATE` → IMPORTANT: same — synchronous validation locks the table.
- `ALTER TABLE` taking `AccessExclusiveLock` without a `lock_timeout` / `statement_timeout` set in the migration → IMPORTANT: long autovacuum or long-running tx will pile up behind it and block reads.
- MySQL `ALTER TABLE` without `ALGORITHM=INPLACE, LOCK=NONE` (or pt-online-schema-change / gh-ost) → IMPORTANT: full table copy with metadata lock.

## Rolling-Deploy Compatibility

- Dropping a column still read by the previous app version → CRITICAL: old replicas/canary pods 500. Two-phase: stop reading → deploy → drop in next migration.
- Renaming a column in a single migration → CRITICAL: same issue as drop. Add new + dual-write + cutover + drop old.
- Adding `NOT NULL` column without app code writing it first → CRITICAL: old code's INSERT fails.
- Removing an enum value still produced by old code → IMPORTANT: writes from old pods fail.
- Tightening a `CHECK` constraint that historical data violates → CRITICAL: migration fails or blocks all writes failing the check.
- Changing default value and assuming existing rows get the new default → IMPORTANT: defaults only apply to new rows. Backfill explicitly if needed.

## Reversibility

- Missing `down()` / `downgrade()` in a non-trivial migration → IMPORTANT: can't roll back. Required for risky changes.
- `down()` that drops a column with data → IMPORTANT: rollback = data loss. Document explicitly.
- Squashing prod migrations → CRITICAL: breaks environments mid-deploy. Squash only after all envs caught up.
- Editing an already-applied migration file → CRITICAL: drift between envs. Always add a new migration.

## Backfills

- Single-statement `UPDATE` over a huge table inside the migration → CRITICAL: long lock, bloat, replication lag. Backfill in batches outside the schema migration.
- Backfill in same migration as the structural change → IMPORTANT: structural changes should be fast; data moves separately.
- Missing progress / checkpointing on long backfills → IMPORTANT: a crash mid-backfill leaves unknown state.
- Reading and writing the same row in a hot loop without `LIMIT`/PK ordering → IMPORTANT: backfill may never finish under live writes.

## Indexes

- New index without `IF NOT EXISTS` in idempotent pipelines → NIT: re-runs fail.
- Composite index column order not matching dominant query predicate → IMPORTANT: index unused.
- Index covering single boolean → NIT: usually unused unless partial (`WHERE is_active = true`).
- Forgetting to drop the old index after replacing → NIT: dead weight, slows writes.

## Foreign Keys & Constraints

- New FK with `ON DELETE CASCADE` on a large parent → IMPORTANT: deletes can cascade unboundedly. Confirm intent.
- New FK without `ON DELETE` action specified → NIT: defaults to `NO ACTION`; be explicit.
- Constraint name not specified → NIT: auto-generated names differ across envs, harder to reference in later migrations.

## Data Types & Storage

- `SERIAL` / `INTEGER` PK on a table that may exceed 2.1B rows → IMPORTANT: silent overflow. Use `BIGSERIAL`/`BIGINT` from day one.
- `JSON` instead of `JSONB` (PG) → IMPORTANT: `JSON` is text — no indexing, no operators.
- `TIMESTAMP WITHOUT TIME ZONE` for events crossing regions → CRITICAL: ambiguous. Use `TIMESTAMPTZ`.
- Adding `UUID` PK as `VARCHAR(36)` → NIT: use native `UUID` type — half the storage, faster compare.
- Money as `FLOAT`/`DOUBLE` → CRITICAL: precision loss. Use `NUMERIC(p, s)`.

## Multi-DB / Replication

- `pg_repack` / vacuum-full inside a migration → CRITICAL: holds full lock; not for app deploy pipelines.
- DDL that breaks logical replication (e.g., dropping a column on a published table) → IMPORTANT: confirm with replication topology.
- Shard-aware schema changes applied to one shard only → CRITICAL: divergence. Migration runner must fan out.

## Tooling Hygiene (Alembic / Django / Flyway / Liquibase / Rails)

- Auto-generated migration left unread (`makemigrations` / `alembic revision --autogenerate`) → IMPORTANT: autogen often picks the wrong op. Read line-by-line before merging.
- Migration depending on app code (importing models) for a column rename/refactor → IMPORTANT: today's model may not match the migration's intent six months from now. Use raw SQL or copy the model definition.
- Flyway migration filename gap (`V5__`, `V7__`) → IMPORTANT: V6 may exist on another branch. Confirm sequencing.
- Liquibase changeset without `id` + `author` discipline or `runOnChange` misused → IMPORTANT: changesets must be immutable once shipped.
- Rails migration without `change` reversibility helpers when reversible → NIT: define `up`/`down` explicitly.
