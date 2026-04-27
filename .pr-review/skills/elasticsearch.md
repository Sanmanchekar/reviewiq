# Elasticsearch / OpenSearch Review Skill

Review search indexing, mappings, query DSL, and operational settings.

## Mappings

**Anti-Patterns → Findings**:
- Dynamic mapping enabled in production (`dynamic: true`, the default) → CRITICAL: arbitrary fields from user input explode the mapping. Set `dynamic: "strict"` or `"runtime"` on production indices.
- Field mapped as both `text` and `keyword` left at default multi-field config without confirming intent → NIT: usually fine, but a wide schema doubles index size.
- Numeric ID field mapped as `long` but always queried as exact match → IMPORTANT: use `keyword` for exact lookups; `long` is for range/aggregation.
- Date field stored as `text` → CRITICAL: range queries impossible, aggregations break. Map as `date` with explicit format.
- High-cardinality `keyword` field used in aggregations without `eager_global_ordinals` → IMPORTANT: cold-cache aggregations are slow.
- `nested` fields used heavily without understanding the per-doc child overhead → IMPORTANT: nested docs are separate Lucene docs; size and cost can balloon.
- `_source` disabled to save space → IMPORTANT: breaks reindex, partial update, highlighting. Almost never worth it.
- Reindex required to change a mapping (mappings are mostly immutable) but no plan for it → CRITICAL: PR ships dead code.

## Index Design

- Single huge index that grows forever → IMPORTANT: shards become huge, slow to recover. Use time-based or rollover indices with ILM.
- Default 1 primary shard with no sizing thought → IMPORTANT: target ~10–50GB per shard. Wrong shard count is hard to fix later.
- Too many shards (oversharding) for low-volume indices → CRITICAL: cluster state grows, master overloaded. Aim for hundreds of shards per node, not thousands.
- Replicas = 0 in prod → CRITICAL: any node failure = data loss + index unavailable.
- Index naming without rollover/alias pattern → IMPORTANT: rotating indices manually is error-prone.

## Queries

- Deep pagination with `from + size` past a few thousand → CRITICAL: O(from+size) per shard, then merge. Use `search_after` or scroll/PIT.
- `scroll` API for live user-facing pagination → IMPORTANT: scroll holds a snapshot, not for interactive UX. Use `search_after` + PIT.
- `wildcard` query with leading `*` (`*foo`) → CRITICAL: full term scan. Use reversed-field index or n-grams if needed.
- `regexp` / `wildcard` on `text` analyzed field → IMPORTANT: matches against analyzed terms, often surprises. Use `keyword` subfield.
- `match` query expected to do exact match → IMPORTANT: `match` runs analyzer; for exact use `term` on a `keyword` field.
- `term` on a `text` field → IMPORTANT: matches against tokenized output, usually returns nothing or wrong results.
- `bool` query with many `should` clauses and no `minimum_should_match` → NIT: scoring works but recall expectations vary.
- `function_score` with expensive script_score on every hit → IMPORTANT: per-doc script eval, slow.
- `query_string` exposed to user input → CRITICAL: query DSL injection — users can craft regex/wildcard/range to DoS the cluster. Use `simple_query_string` and restrict fields.

## Aggregations

- `terms` aggregation on high-cardinality `text` field → CRITICAL: expects `keyword`. On `text`, requires fielddata which OOMs.
- `size: 10000` (max bucket size) on `terms` with high cardinality → IMPORTANT: huge memory + network. Use composite aggregation for full enumeration.
- Nested aggregations several levels deep without bucket sizing → IMPORTANT: cartesian-style blowup, OOM circuit breaker.
- `cardinality` agg expected to be exact → IMPORTANT: HyperLogLog approximation. Tune `precision_threshold` if needed.
- Date histogram with `fixed_interval` 1m over years of data → IMPORTANT: huge bucket count. Use `calendar_interval` or downsample.

## Indexing & Bulk Writes

- Single-doc `index` requests in a loop → CRITICAL: round-trip + refresh per doc. Use `_bulk` with batches of MBs (5–15MB typical).
- `_bulk` batch too large (>100MB) → IMPORTANT: can hit `http.max_content_length` (default 100MB), node memory pressure.
- `refresh: true` after each write → CRITICAL: forces segment flush; cripples throughput. Default `refresh_interval` (1s) is fine.
- Indexing with `refresh_interval: -1` and forgetting to re-enable → IMPORTANT: search can't see writes.
- Throttling done client-side without watching `bulk` rejections (queue full) → IMPORTANT: silent drops without retry-with-backoff. Always handle 429.
- Updating a doc with `update` API many times rapidly → IMPORTANT: each update marks old doc deleted, segment churn. Consider partial updates only when needed.

## Concurrency & Versioning

- Optimistic concurrency (`if_seq_no` / `if_primary_term`) not used for read-modify-write → CRITICAL: lost updates.
- External versioning (`version_type: external`) without a monotonic source → IMPORTANT: silent rejection of out-of-order updates.

## Cluster & Operations

- Single master-eligible node → CRITICAL: split-brain or no quorum on failure. Always use 3 (or odd number).
- Heap > 32GB → IMPORTANT: loses compressed oops, worse perf than 30GB. Cap at ~30GB; add nodes instead.
- Heap not 50% of system memory (and the rest left for OS file cache) → IMPORTANT: file cache is critical for ES.
- Disk watermark not configured / disk fills up → CRITICAL: cluster goes red, indices read-only.
- Snapshots not configured → CRITICAL: no DR.
- ILM policy missing → IMPORTANT: indices grow unbounded.
- `discovery.seed_hosts` / `cluster.initial_master_nodes` misconfigured → CRITICAL: split-brain on restart.

## Security

- Cluster reachable on public network without auth (legacy ES <8) → CRITICAL: open data exposure / RCE history.
- `xpack.security.enabled: false` in prod → CRITICAL.
- TLS not enabled on transport layer → IMPORTANT.
- API keys / `elastic` superuser used by app → CRITICAL: least-privilege role per service.
- Document-level security assumed but not enabled → IMPORTANT.

## Search Quality (often missed in code review)

- Analyzer mismatch between index time and query time → IMPORTANT: hits 0 results inexplicably. Test with `_analyze`.
- Stop words not aligned with language → NIT: surprising recall gaps.
- No synonyms or phonetic handling for use cases that need them → NIT.
- Boosting hardcoded in app code instead of indexed-time → NIT: harder to tune.
- Score sort assumed stable → IMPORTANT: scores tie often; add a tiebreaker (`_id`) for deterministic order.

## Client Library Specifics

- **elasticsearch-py / @elastic/elasticsearch**: official 8.x client requires version match with cluster — silent compatibility checks. `RequestError` not caught for `429` (queue full) → no retry-with-backoff.
- **Spring Data Elasticsearch**: lazy serialization with `@Field(type = FieldType.Auto)` causes mapping drift.
- **Logstash / Beats**: pipeline backpressure ignored — beats can fall behind silently. Watch `pipeline.workers` and queue size.
