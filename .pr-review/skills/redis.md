# Redis Review Skill

Review Redis usage as cache, queue, lock, rate-limiter, and session store.

## Dangerous Commands

**Anti-Patterns → Findings**:
- `KEYS *` in app code → CRITICAL: O(N), blocks single-threaded server. Use `SCAN` with cursor.
- `FLUSHDB` / `FLUSHALL` reachable from app code path → CRITICAL: nukes data.
- `DEBUG SLEEP` left in code → CRITICAL.
- `MIGRATE` from app → IMPORTANT: rare; usually a footgun.
- Lua script using `KEYS *` or unbounded loops → CRITICAL: blocks server (Lua runs atomically).
- Long-running Lua script (>50ms) → IMPORTANT: blocks all other commands.

## Caching

- No TTL on cache key (`SET k v` without `EX`) → IMPORTANT: leaks memory until eviction kicks in. Set explicit TTL.
- All keys with the same TTL → IMPORTANT: cache stampede on synchronized expiry. Add jitter.
- Cache stampede unmitigated (read miss → many concurrent fetches → all write back) → CRITICAL under load. Use single-flight (one fetch, others wait) or probabilistic early refresh.
- Cache stores raw DB rows including PII without policy → IMPORTANT: data lifecycle / privacy.
- Negative-cache miss not stored → NIT: every request re-hits the DB for the same missing key. Cache "not found" with shorter TTL.
- Read-modify-write on a hash without `WATCH` / Lua → CRITICAL: lost updates.
- Cache invalidation via `DEL` racing with read-then-write → CRITICAL: stale value re-cached after delete. Use versioned keys or explicit invalidation token.
- Caching very large values (>100KB) → IMPORTANT: network + serialization cost can exceed DB hit. Profile.
- Storing user-scoped cache without a namespace prefix → IMPORTANT: cross-tenant leak risk with shared keys.

## Memory & Eviction

- `maxmemory` not set → CRITICAL: Redis can OOM-kill the host.
- `maxmemory-policy = noeviction` for a cache use case → IMPORTANT: writes start failing when full. Use `allkeys-lru` or `allkeys-lfu` for caches.
- `volatile-lru` chosen but most keys lack TTL → IMPORTANT: nothing to evict, defeats policy.
- Storing collections that grow unboundedly (lists, sorted sets) without size cap → CRITICAL: memory blowout.
- Big-key hot-spot (single key holds GBs) → IMPORTANT: single-shard bottleneck in cluster mode, AOF rewrite stalls. Shard the key.
- `DEBUG OBJECT` / `MEMORY USAGE` ignored when in doubt about size → NIT: easy diagnostic.

## Persistence & Durability

- Treating Redis as a durable database without RDB or AOF strategy understood → CRITICAL: data loss on restart.
- AOF `appendfsync = no` for critical data → CRITICAL: up to 30s loss on crash. Use `everysec` or `always`.
- AOF `everysec` accepted as "durable" for money flows → IMPORTANT: still up to 1s loss. Use `always` or another store for the source of truth.
- Replica without `replica-read-only = yes` accepting writes → CRITICAL: split-brain.
- No snapshot/AOF backup off-host → IMPORTANT.

## Locking

- Distributed lock via `SETNX` without TTL → CRITICAL: dead holder blocks all callers forever. Use `SET key val NX PX <ms>`.
- Distributed lock TTL but no fencing token → CRITICAL: TTL expires mid-work, second holder writes concurrently with first. Use fencing.
- Releasing lock without checking ownership (just `DEL`) → CRITICAL: deletes someone else's lock. Use Lua `if GET == mine then DEL`.
- Redlock across non-cluster Redis instances assumed safer than single-node → IMPORTANT: contested correctness; for hard correctness use a real consensus system (etcd, ZK).

## Pipelining & Latency

- Many sequential round-trips (`for x in items: redis.get(x)`) → CRITICAL: latency × N. Use `MGET`, pipeline, or Lua.
- Pipeline mixing reads and writes that depend on each other → IMPORTANT: pipeline doesn't see intra-pipeline state. Use Lua / `MULTI`.
- `MULTI`/`EXEC` used for atomicity but reads inside aren't reflected back → IMPORTANT: `EXEC` returns all results; reads return queued, not live values.
- Blocking commands (`BLPOP`, `BRPOP`, `XREAD BLOCK`) on the shared connection → CRITICAL: blocks all other commands. Use a dedicated connection per blocker.

## Pub/Sub vs Streams

- Pub/Sub used where durability matters → CRITICAL: messages dropped if no subscriber. Use Streams (`XADD` / `XREADGROUP`) for durable delivery.
- Streams without consumer group → IMPORTANT: no progress tracking; restarts replay everything.
- Streams `MAXLEN` not set → IMPORTANT: unbounded growth.
- `XACK` missing → IMPORTANT: messages stay in PEL forever. Recovered consumers see duplicates.

## Cluster Mode

- Multi-key command (`MSET`, `SUNIONSTORE`, transactions, Lua with multiple KEYS) across slots → CRITICAL: `CROSSSLOT` error. Use hash tags `{user:42}:cart` to colocate.
- Hash-tag overuse causes uneven shard load → IMPORTANT: mega-tag tenant pinned to one shard.
- Cluster client without `MOVED`/`ASK` redirect handling → CRITICAL: errors during resharding.
- Read scaling via replicas without `READONLY` flag → IMPORTANT: replicas reject reads by default.

## Rate Limiting

- Naive `INCR + EXPIRE` rate limiter where the EXPIRE is conditional → IMPORTANT: race between INCR and EXPIRE leaves a key with no TTL.
- Sliding-window rate limiter via sorted set without trim → IMPORTANT: unbounded growth.
- Token-bucket implemented per-request without Lua atomicity → IMPORTANT: race condition allows burst above limit.

## Security

- Redis exposed to public network without `requirepass` / ACL → CRITICAL: open RCE via `CONFIG SET dir + SAVE` historically. Lock down.
- ACLs not used; everything runs as `default` user → IMPORTANT: blast radius.
- TLS not enabled for cross-AZ / cross-VPC traffic → IMPORTANT.
- Credentials in code/env without rotation → IMPORTANT.

## Client Library Specific

- **node-redis / ioredis**: not awaiting pipeline `.exec()` → IMPORTANT: silent failure. `enableOfflineQueue: true` (default) buffers during disconnect — confirm intent.
- **redis-py**: connection pool not shared across workers, or `decode_responses=True` mismatch with binary data.
- **go-redis**: `ctx` not threaded through, missed cancellation.
- **Lettuce/Jedis**: shared `StatefulRedisConnection` is fine; pooling `RedisCommands` is wrong (per-thread Jedis vs shared Lettuce).
