# Scalability Review Skill

Review code for behaviors that break under load. Flag issues before they page someone at 3am.

## Database

- **N+1 Queries**: Loading a list then querying per item → batch/join/eager-load. Count queries, not just results.
- **Missing Indexes**: WHERE/ORDER BY/JOIN on unindexed columns → add indexes. Check EXPLAIN plans mentally.
- **Full Table Scans**: SELECT * without WHERE, LIKE '%prefix%', functions on indexed columns → rewrite for index use.
- **Unbounded Queries**: No LIMIT on SELECT → always paginate. Set hard caps (max 1000 rows).
- **Lock Contention**: Long transactions holding row/table locks → minimize transaction scope, use optimistic locking.
- **Schema Migrations**: Locking migrations (ALTER TABLE on large tables) → online schema change tools, expand-then-contract.
- **Connection Exhaustion**: Creating connections per request → connection pooling. Pool size must match concurrency.
- **Hot Partitions**: Monotonic keys (timestamps, auto-increment) as partition keys → add randomness or use composite keys.
- **Read/Write Split**: All queries hitting primary → read replicas for read-heavy paths, accept eventual consistency.

## Caching

- **Cache Stampede**: Cache expires, 1000 requests simultaneously rebuild it → lock/singleflight on cache miss.
- **Cache Invalidation**: Stale data after writes → invalidate or use write-through/write-behind patterns.
- **Unbounded Caches**: Cache grows forever → set max size, TTL, eviction policy (LRU/LFU).
- **Cache Key Collisions**: Insufficient key granularity → include all varying parameters in cache key.
- **Serialization Cost**: Serializing/deserializing large objects per request → cache pre-serialized responses.
- **Thundering Herd**: Many processes wake on same event → staggered expiry, jitter on TTLs.

## Concurrency

- **Thread Pool Exhaustion**: Blocking I/O in shared thread pools → dedicated pools for I/O, async I/O, virtual threads.
- **Queue Backpressure**: Unbounded queues → bounded queues with rejection policies. Monitor queue depth.
- **Worker Starvation**: Long-running tasks blocking short tasks → separate queues by task duration, priority queues.
- **Race Conditions**: Check-then-act without atomicity → atomic operations, compare-and-swap, distributed locks.
- **Deadlocks**: Inconsistent lock ordering → always acquire locks in the same global order. Set lock timeouts.
- **Connection Pool Starvation**: Slow queries hold connections → query timeouts, pool monitoring, circuit breakers.

## Network & I/O

- **Chatty APIs**: Many small requests → batch APIs, aggregate endpoints, GraphQL for flexible queries.
- **Large Payloads**: Unbounded response sizes → pagination, streaming, compression, field selection.
- **Missing Timeouts**: HTTP calls, DNS lookups, socket reads without timeouts → set explicit timeouts everywhere.
- **Synchronous Chains**: Service A calls B calls C calls D synchronously → async where possible, timeout budgets.
- **Missing Retry Budget**: Every service retries 3x → exponential fanout. Set system-wide retry budgets.
- **DNS Caching**: Resolved IPs cached forever → respect TTL, use connection pooling that re-resolves.

## Compute

- **CPU-Bound in Request Path**: Crypto, image processing, ML inference in sync request → offload to background workers.
- **Memory Leaks**: Growing maps/caches/listeners without cleanup → weak references, TTL, periodic cleanup.
- **GC Pressure**: Excessive allocation in hot paths → object pools, pre-allocation, struct-of-arrays.
- **Algorithmic Complexity**: O(n^2) or worse on user-controlled input sizes → sublinear algorithms or hard size limits.
- **Regex Catastrophe**: Backtracking regex on user input → bounded/non-backtracking regex, input size limits.

## Architecture

- **Single Point of Failure**: One instance of critical service → redundancy, failover, load balancing.
- **Missing Circuit Breakers**: Failing dependency takes down caller → circuit breakers with fallback behavior.
- **Tight Coupling**: Service A can't function without Service B → graceful degradation, async communication.
- **Missing Backpressure**: Producer faster than consumer → rate limiting, flow control, load shedding.
- **Shared Database**: Multiple services writing to same tables → event-driven, eventually consistent, bounded contexts.
- **Synchronous Writes**: Writes on critical path → write-behind, event sourcing, async processing.

## Checklist

```
[ ] All database queries are bounded (LIMIT/pagination)
[ ] No N+1 query patterns
[ ] All external calls have timeouts
[ ] Retry logic has backoff and max retries
[ ] Caches have TTL and max size
[ ] Connection pools are sized for expected concurrency
[ ] No O(n^2+) algorithms on user-controlled input sizes
[ ] Background jobs are idempotent (safe to retry)
[ ] Queue consumers have dead-letter queues
[ ] No unbounded in-memory growth
```
