# SQL Review Skill

Review raw SQL and ORM-generated queries for correctness, indexability, and cost.

## Indexability & Predicates

**Anti-Patterns → Findings**:
- Function on indexed column in `WHERE` (`WHERE LOWER(email) = ?`, `WHERE DATE(created_at) = ?`) → IMPORTANT: non-SARGable, full scan. Use functional/expression indexes or normalize at write time.
- Implicit type coercion (`WHERE varchar_col = 123`, `WHERE bigint_id = '42'`) → IMPORTANT: coercion blocks index use. Cast on the literal, not the column.
- Leading-wildcard `LIKE '%foo'` → IMPORTANT: btree can't use it. Use trigram (`pg_trgm`), reverse index, or full-text search.
- `OR` across columns without composite index → IMPORTANT: planner often falls back to scan. Rewrite as `UNION ALL` or add covering index.
- `NOT IN (subquery)` with NULLable column → CRITICAL: returns empty if any NULL. Use `NOT EXISTS` instead.
- `IN (...)` with thousands of literals → IMPORTANT: parser/planner overhead, may exceed limits. Use temp table or `= ANY($array)`.
- Missing index on FK column → IMPORTANT: parent updates/deletes do full child scan, lock contention.
- Index on low-cardinality boolean alone → NIT: rarely useful unless partial (`WHERE is_active = true`).

## Query Shape

- `SELECT *` in app code → IMPORTANT: bloats network/memory, breaks on schema change, defeats covering indexes. Enumerate columns.
- N+1 in raw SQL (loop issuing `SELECT ... WHERE id = ?`) → CRITICAL: latency × N. Batch with `WHERE id = ANY(?)` or join.
- Missing `LIMIT` on user-facing list queries → IMPORTANT: unbounded result sets, OOM risk.
- `OFFSET` for deep pagination (`OFFSET 100000`) → IMPORTANT: scans+discards. Use keyset/seek pagination (`WHERE id > ?`).
- `COUNT(*)` on large tables for pagination total → IMPORTANT: full scan. Use approximate (`pg_class.reltuples`) or estimated counts.
- `ORDER BY RAND()` / `ORDER BY RANDOM()` → CRITICAL: full scan + sort. Use `TABLESAMPLE` or random offset by id.
- `DISTINCT` masking a JOIN bug → IMPORTANT: usually a sign of duplicate rows from missing JOIN condition. Investigate root cause.
- Correlated subquery in `SELECT` list → IMPORTANT: per-row execution. Rewrite as JOIN or LATERAL.

## Joins

- Cross join (missing `ON`) → CRITICAL: cartesian product.
- Implicit join via `WHERE` instead of `ON` → NIT: harder to read; modern style uses explicit `JOIN ... ON`.
- `LEFT JOIN` followed by `WHERE right_col = ?` → CRITICAL: silently turns into INNER JOIN. Move predicate into `ON`.
- Joining on differently-typed columns → IMPORTANT: forces coercion, blocks index. Align types.
- Joining without an index on the join column → CRITICAL: hash join over scan in hot path.

## Writes

- `UPDATE` / `DELETE` without `WHERE` → CRITICAL: rewrites entire table.
- Bulk write in a row-by-row loop → IMPORTANT: use multi-row `INSERT`, `COPY`, or `INSERT ... SELECT`.
- `INSERT ... ON CONFLICT DO NOTHING` masking real errors → IMPORTANT: confirm it's truly idempotent intent, not error swallowing.
- `RETURNING` ignored when caller needs the id → NIT: extra round-trip via separate `SELECT lastval()`.
- Mass `DELETE` without batching on large tables → IMPORTANT: long lock, replication lag, bloat. Delete in chunks with `LIMIT`.

## SQL Injection & Safety

- String concatenation building queries (`"SELECT ... WHERE id = " + user_input`) → CRITICAL: SQL injection. Always parameterize.
- Dynamic table/column names from user input → CRITICAL: not parameterizable. Validate against an allowlist.
- ORM `raw()` / `text()` with f-strings or `+` → CRITICAL: same injection risk. Use bind params.
- `LIKE` with user input not escaped (`%`, `_`) → IMPORTANT: pattern injection or scan blowups. Escape wildcards or use literal match.

## Aggregation & Analytics

- `GROUP BY` ordinals (`GROUP BY 1, 2`) → NIT: brittle to column reorder. Group by names.
- `HAVING` doing what `WHERE` should (filter on non-aggregated col) → NIT: HAVING runs after aggregation; predicate may run on far more rows.
- Window function on entire table without partitioning when only a slice is needed → IMPORTANT: scans more than necessary.
- `SELECT col FROM t GROUP BY col` instead of `DISTINCT` → NIT: equivalent; pick whichever the team uses.

## Schema Smells (caught at query time)

- `VARCHAR(255)` everywhere with no real limit → NIT: cargo-culted. Use `TEXT` or actual business limit.
- Storing JSON blobs queried by inner fields without index → IMPORTANT: every query parses JSON. Promote hot fields to columns or add expression index.
- Boolean stored as `VARCHAR('Y'/'N')` → NIT: use `BOOLEAN`.
- Timestamps without timezone (`TIMESTAMP` instead of `TIMESTAMPTZ`) → IMPORTANT: ambiguity across DST/regions. Use TZ-aware type.
- Money as `FLOAT`/`DOUBLE` → CRITICAL: precision loss. Use `NUMERIC`/`DECIMAL` with explicit scale.

## Observability

- No query timeout (`statement_timeout` / `SET LOCAL`) on user-facing endpoints → IMPORTANT: one slow query holds a connection.
- Missing query tagging / `pg_stat_statements`-friendly literals → NIT: makes hot-query analysis harder.
- Logging full query text with PII bound → IMPORTANT: leaks PII into log pipeline.
