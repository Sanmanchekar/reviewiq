# MongoDB Review Skill

Review MongoDB schema design, queries, aggregation pipelines, and write concerns.

## Schema & Documents

**Anti-Patterns → Findings**:
- Unbounded array embedded in a document (e.g. `comments: [...]` for a popular post) → CRITICAL: hits 16MB document limit, every update rewrites entire doc. Use a separate collection with parent ref.
- Modeling as one giant `users` doc with embedded everything → IMPORTANT: write amplification, locking on doc-level. Split aggregates by access pattern.
- Polymorphic collection without a discriminator field → IMPORTANT: queries scan all variants.
- Storing money as JS `Number` (double) → CRITICAL: precision loss. Use `Decimal128`.
- `_id` not tuned to access pattern → NIT: ObjectId is fine for most; for time-series consider compound `_id` like `{user: ..., ts: ...}`.
- Schemaless used as license to skip validation → IMPORTANT: enable schema validation (`$jsonSchema`) on critical collections.

## Queries

- `$regex` without anchor (`/foo/`) on a large collection → CRITICAL: collection scan. Anchor at start (`/^foo/`) and the index can be used.
- `$regex` case-insensitive (`/foo/i`) on a regular index → IMPORTANT: index ignored. Use a case-folded duplicate field with normal index, or text index.
- Query without supporting index → CRITICAL: collection scan. Always run `.explain('executionStats')` and confirm `IXSCAN`, not `COLLSCAN`.
- Compound index column order not matching ESR rule (Equality, Sort, Range) → IMPORTANT: sort/range portion can't use index efficiently.
- `$ne`, `$nin`, `$not` against an indexed field → IMPORTANT: typically can't use index efficiently.
- `$where` with JS function → CRITICAL: JS execution per doc, no index, security surface.
- `find().sort()` without an index covering the sort → IMPORTANT: in-memory sort, capped at 32MB then errors.
- Returning full doc when only a few fields needed → IMPORTANT: use projection (`{field: 1}`).
- `count()` on full collection for pagination total → IMPORTANT: scans all matching. Use estimated count or skip total.
- `skip()` for deep pagination → IMPORTANT: server still walks them. Use range-based pagination on indexed field.

## Indexes

- Too many indexes per collection (>10–15) → IMPORTANT: write amplification. Audit usage with `$indexStats`.
- Single-field index where compound would replace several → NIT: prefix-of-compound is reusable.
- Missing partial index for hot subset (`{status: "active"}`) → IMPORTANT: full index larger than needed.
- Index intersection assumed → IMPORTANT: planner uses it but compound is usually better.
- Index built without `background: true` (pre-4.2) on a busy collection → CRITICAL: blocks writes. (4.2+ builds online by default.)
- Unique index added to existing collection without dedup pre-check → CRITICAL: index build fails on duplicates.
- TTL index on a field that's sometimes null/missing → IMPORTANT: those docs never expire.

## Aggregation Pipeline

- `$match` after `$lookup` / `$group` instead of before → CRITICAL: processes way more docs than necessary. Push `$match` as early as possible.
- `$lookup` on an unindexed `foreignField` → CRITICAL: per-doc collection scan. Index it.
- `$lookup` followed by `$unwind` then `$match` to filter joined docs → IMPORTANT: heavy. Often a different schema is the answer.
- Pipeline returning unbounded docs without `$limit` → IMPORTANT: memory growth on aggregation server.
- Pipeline using `allowDiskUse: false` (default) on a large pipeline → IMPORTANT: 100MB stage memory limit triggers error. Set `allowDiskUse: true` deliberately.
- `$group` over high-cardinality field → IMPORTANT: may exceed 100MB stage limit. Use `allowDiskUse` and partition.
- `$out` / `$merge` racing with consumers reading the target collection → IMPORTANT: data flips between states.

## Writes & Consistency

- Default `w: 1` for critical data → CRITICAL: not durable across replica failover. Use `w: "majority"`.
- `j: false` (default in some configs) → IMPORTANT: ack before journal flush. Set `j: true` for durability.
- `readConcern: "local"` while reading just-written data after `w: "majority"` → IMPORTANT: replica may lag. Use `readConcern: "majority"` or read from primary.
- `readPreference: secondaryPreferred` for read-after-write paths → CRITICAL: stale reads. Use primary for read-after-write.
- Multi-document logical operation without a multi-document transaction → CRITICAL: partial writes. Wrap in `session.startTransaction()` (4.0+ replica sets, 4.2+ sharded).
- Transaction holding a read-modify-write across an external HTTP call → CRITICAL: contention + retries.
- `findOneAndUpdate` for a counter with `upsert: true` but no unique index on the filter → IMPORTANT: race creates duplicates.
- `bulkWrite` mixing ordered `true`/`false` without considering partial-failure semantics → IMPORTANT: ordered stops at first error; unordered continues.

## Sharding

- Monotonically-increasing shard key (`_id` ObjectId, timestamp) → CRITICAL: hot shard for all writes. Use hashed shard key or compound including hash.
- Low-cardinality shard key → CRITICAL: jumbo chunks, can't split. Pick high-cardinality.
- Query without the shard key → IMPORTANT: scatter-gather across all shards. Always include the shard key in hot queries.
- Range queries without the shard key as prefix → IMPORTANT: same scatter-gather issue.
- Changing shard key not planned for (pre-4.4 immutable) → IMPORTANT: very disruptive. Pick carefully day one.

## Operational

- No `maxTimeMS` on long-running queries → IMPORTANT: queries can run forever, holding cursors.
- Cursor not closed (long-lived find without `toArray` / iteration completion) → IMPORTANT: server resources held.
- `tailable` cursor on capped collection without idle handling → IMPORTANT: connection assumed stable.
- Backups without encryption at rest → IMPORTANT.
- Connection string with credentials in code/log → CRITICAL.
- `SCRAM-SHA-1` instead of `SCRAM-SHA-256` → IMPORTANT.
- Atlas IP allowlist `0.0.0.0/0` → CRITICAL.

## Driver Specifics

- **Mongoose**: `strict: false` silently drops unknown fields. `lean()` returns POJOs (faster) — use for read paths. `populate()` is N+1 in disguise — confirm or use `$lookup`. Schema-level virtuals don't survive `lean()`.
- **PyMongo**: `MongoClient` per-call (not pooled) → CRITICAL: handshake per request. Reuse the client.
- **Node MongoDB driver**: not closing the client on shutdown → IMPORTANT: lingering connections.
- **Spring Data**: `@DBRef` for "joins" → IMPORTANT: N+1 lookups, no indexing help.
