# Performance Review Skill

Measure first, optimize second. But catch the obvious performance bugs before they reach production.

## Algorithmic Complexity

- **O(n^2) or worse on user-controlled input** → CRITICAL: attacker can DoS with large input. Hard limit input size or use O(n log n) algorithm.
- **Nested loops over collections** → IMPORTANT: `for x in A: for y in B: if x == y` → use set intersection. O(n*m) → O(n+m).
- **Linear search where hash lookup works** → IMPORTANT: searching a list for membership → use set/map. O(n) → O(1).
- **Sorting when only min/max needed** → NIT: `sorted(list)[0]` → `min(list)`. O(n log n) → O(n).
- **Repeated computation** → NIT: calculating same value inside a loop. Hoist invariants out of loops.
- **String concatenation in loops** → IMPORTANT: `result += s` in a loop creates O(n^2) copies. Use StringBuilder/join/buffer.
- **Regex compilation in loops** → NIT: compile regex once, reuse. Compilation is expensive.

## Memory

- **Unbounded collections** → CRITICAL: lists/maps/caches that grow without limit. Set max size, implement eviction.
- **Loading entire dataset into memory** → IMPORTANT: `SELECT *` into a list. Stream/paginate for large datasets.
- **Memory leaks** → CRITICAL: event listeners not removed, closures retaining large objects, cache without TTL.
- **Large object duplication** → NIT: deep-copying when a reference/view suffices. Clone only when mutation isolation is needed.
- **Buffer over-allocation** → NIT: pre-allocating 1MB when 1KB is typical. Use adaptive sizing or start small.
- **Holding references to unused objects** → IMPORTANT: objects kept alive by collection membership after logical removal.
- **String interning opportunities** → NIT: repeated identical strings (status codes, enum names) can be interned/shared.

## Database

- **N+1 queries** → IMPORTANT: querying per-item in a loop. Batch into single query with IN clause or JOIN.
- **Missing indexes** → IMPORTANT: WHERE/ORDER BY/JOIN on unindexed columns. Verify with EXPLAIN.
- **SELECT * when few columns needed** → NIT: transfers unnecessary data. Select only needed columns.
- **Unbounded queries** → IMPORTANT: no LIMIT clause. Always paginate.
- **Full table scan via function on indexed column** → IMPORTANT: `WHERE YEAR(created_at) = 2024` can't use index. Rewrite as range.
- **Connection pool exhaustion** → CRITICAL: creating connections per-request or holding them during I/O. Pool and release promptly.
- **Missing query timeout** → IMPORTANT: no statement timeout. Slow query blocks connection forever.
- **Write amplification** → NIT: updating entire row when one column changed. Use partial updates.
- **Missing batch inserts** → NIT: inserting rows one-by-one. Batch insert for bulk operations.

## Network & I/O

- **Synchronous I/O in hot paths** → CRITICAL: blocking the event loop or request thread on file/network I/O.
- **Missing connection reuse** → IMPORTANT: creating new HTTP/TCP connections per request. Use connection pooling, keep-alive.
- **Missing compression** → NIT: large payloads without gzip/brotli. Compress responses >1KB.
- **Chatty protocols** → IMPORTANT: many small requests when one batch request works. Aggregate API calls.
- **Missing caching headers** → NIT: static resources without Cache-Control/ETag. Saves bandwidth and latency.
- **Large response payloads** → NIT: returning full objects when client needs 3 fields. Use field selection or separate endpoints.
- **DNS lookup per request** → NIT: connection pooling handles this. Check if custom HTTP clients bypass pooling.
- **Uncompressed assets** → NIT: serving unminified JS/CSS, uncompressed images. Use build-time optimization.

## CPU

- **JSON serialization in hot paths** → NIT: JSON parse/stringify is CPU-intensive. Cache parsed results, use binary protocols for internal services.
- **Crypto in request path** → NIT: bcrypt/scrypt in sync request handler. Offload to worker thread/pool.
- **Excessive object creation** → NIT: creating objects in tight loops. Pre-allocate, use object pools.
- **Reflection/introspection in hot paths** → NIT: `getattr`, reflection API calls. Resolve once, cache the reference.
- **Unnecessary deep copies** → NIT: cloning objects when read-only access suffices. Use immutable references.
- **Inefficient data structures** → NIT: linked list for random access, array for frequent insertions. Match structure to access pattern.

## Frontend Performance

- **Bundle size** → IMPORTANT: initial JS >500KB, total >2MB. Code-split, lazy-load, tree-shake.
- **Render blocking resources** → IMPORTANT: synchronous CSS/JS in head. Use async/defer, critical CSS inlining.
- **Layout thrashing** → IMPORTANT: reading layout properties then writing DOM in a loop. Batch reads, then batch writes.
- **Missing image optimization** → NIT: serving raw images without sizing, format optimization (WebP/AVIF), lazy loading.
- **Unnecessary re-renders** → NIT: React components re-rendering on every parent render. Use memo, useMemo, useCallback.
- **Missing virtual scrolling** → IMPORTANT: rendering 10K+ DOM nodes for a list. Use virtualization (react-window, etc).
- **Third-party script blocking** → NIT: analytics/tracking scripts blocking page load. Load async, defer non-critical.
- **Core Web Vitals** → LCP <2.5s, FID <100ms, CLS <0.1. Flag changes likely to regress these.

## Concurrency Performance

- **Lock contention** → IMPORTANT: hot locks serializing concurrent work. Reduce critical section scope, use reader-writer locks, lock-free structures.
- **Thread pool sizing** → IMPORTANT: CPU-bound: N_cores. I/O-bound: N_cores * (1 + wait/compute). Wrong sizing = underutilization or thrashing.
- **Context switching overhead** → NIT: thousands of OS threads. Use async I/O, green threads, or thread pool.
- **False sharing** → NIT: independent variables on same cache line causing cache invalidation across cores. Pad or separate.
- **Blocking in async context** → CRITICAL: sync I/O in async function. Blocks entire event loop. Use async alternatives.

## Caching

- **Missing caching for expensive operations** → NIT: repeated identical computation/query. Add caching with appropriate TTL.
- **Cache key too broad** → NIT: caching by user ID when data varies by user+locale. Include all varying dimensions.
- **Cache key too narrow** → NIT: cache key includes timestamp. Hit rate near 0%. Remove volatile components.
- **Missing cache warming** → NIT: cold cache after deploy causes latency spike. Pre-warm critical caches.
- **Serialization cost in cache** → NIT: serializing complex objects per cache hit. Store pre-serialized or use binary format.

## Profiling & Measurement

When suggesting performance improvements, recommend:

| Concern | Measurement Tool/Approach |
|---|---|
| API latency | P50, P95, P99 response times (not averages) |
| Database | EXPLAIN ANALYZE on suspicious queries |
| Memory | Heap snapshots, allocation profiling |
| CPU | CPU profiling/flame graphs |
| Frontend | Lighthouse, Core Web Vitals, Network waterfall |
| Bundle size | webpack-bundle-analyzer, source-map-explorer |
| Concurrency | Thread dump analysis, lock contention profiling |

## Checklist

```
[ ] No O(n^2+) algorithms on user-controlled input
[ ] No N+1 query patterns
[ ] No unbounded collections or queries
[ ] No synchronous I/O in hot paths or async contexts
[ ] No memory leaks (unremoved listeners, unbounded caches)
[ ] Connection pooling used for all external services
[ ] Database queries have indexes for WHERE/JOIN/ORDER columns
[ ] Frontend bundle <500KB initial load
[ ] String concatenation uses builder/join, not += in loops
[ ] Expensive computations cached with appropriate TTL
```
