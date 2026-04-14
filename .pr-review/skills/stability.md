# Stability Review Skill

Review for reliability, resilience, and observability. Code that works isn't enough — it must keep working.

## Error Handling

- **Swallowed Exceptions**: Empty catch blocks, `except: pass`, `catch(e) {}` → handle, log, or propagate.
- **Generic Catches**: Catching `Exception`/`Throwable`/`Error` → catch specific exceptions. Generic catches mask bugs.
- **Missing Rollback**: Error after partial state change → transactional operations, compensation logic.
- **Error Storms**: One failure triggers error logs/alerts in a loop → rate-limit error reporting, deduplicate.
- **Panic/Exit in Libraries**: Libraries should return errors, not crash the process → `panic` only in truly unrecoverable states.
- **Missing Context in Errors**: `throw new Error("failed")` → include what failed, why, and what input caused it.
- **Error Type Confusion**: HTTP 500 for validation errors, HTTP 200 for failures → correct status codes, structured errors.

## Resilience Patterns

- **Circuit Breakers**: Detect repeated failures to an external dependency → stop calling, return fallback, periodically probe.
  - Open after N failures in window W
  - Half-open: test with single request
  - Closed: resume normal traffic
- **Bulkheads**: Isolate failures to subsystems → separate thread pools, connection pools, process boundaries.
- **Timeouts at Every Boundary**: HTTP, database, cache, message queue, file system → explicit timeouts. Default is "hang forever."
- **Graceful Degradation**: When a non-critical service fails → disable feature, serve cached data, return defaults.
- **Health Checks**: Liveness (process alive), readiness (can serve traffic), startup (initialization complete) → separate endpoints.
- **Retry with Backoff**: Exponential backoff with jitter. Cap max retries. Set overall deadline. Never retry non-idempotent operations blindly.
- **Idempotency**: All retry-able operations must be safe to repeat → idempotency keys, upserts, deduplication.

## State Management

- **Distributed State Consistency**: Multiple instances modifying shared state → distributed locks, CRDTs, event sourcing.
- **Cache/DB Inconsistency**: Cache not invalidated after write → write-through, invalidate-on-write, short TTLs.
- **Session Stickiness**: State stored in memory → externalize to Redis/DB, or use stateless design.
- **Clock Skew**: Relying on system clock for ordering → logical clocks, vector clocks, or accept clock skew bounds.
- **Split Brain**: Network partition causes divergent state → quorum writes, leader election, conflict resolution.

## Deployment Safety

- **Backward Compatibility**: New code must work with old data, old clients, old messages in queue → expand-then-contract.
- **Feature Flags**: Big changes behind flags → gradual rollout, instant rollback without deployment.
- **Database Migrations**: Non-locking, backward-compatible, rollback-ready → no column renames/drops in same deploy.
- **Rolling Updates**: Old and new versions running simultaneously → both must work correctly together.
- **Canary/Blue-Green**: Route percentage of traffic to new version → automatic rollback on error rate spike.
- **Rollback Plan**: Every deployment must have a documented rollback path tested before the deployment.

## Observability

- **Structured Logging**: JSON/key-value logs with correlation IDs → not printf-style strings. Include: timestamp, level, service, trace_id, span_id.
- **Meaningful Metrics**: RED (Rate, Errors, Duration) for services. USE (Utilization, Saturation, Errors) for resources. SLIs tied to SLOs.
- **Distributed Tracing**: Cross-service request tracing → propagate trace context (W3C TraceContext, B3).
- **Alerting on Symptoms**: Alert on user-facing symptoms (error rate, latency) not causes (CPU, memory) → actionable alerts.
- **Log Levels**: ERROR = needs human attention. WARN = unusual but handled. INFO = business events. DEBUG = development only.
- **Sensitive Data in Logs**: No PII, no secrets, no tokens in logs → redact/mask before logging.
- **Metric Cardinality**: High-cardinality labels (user IDs, URLs) in metrics → cardinality explosion. Use bounded labels.

## Failure Modes

Review every external dependency and ask: "What happens when this fails?"

| Dependency | Failure Mode | Required Handling |
|---|---|---|
| Database | Connection refused / timeout / slow query | Circuit breaker, connection pool, query timeout |
| Cache | Miss / unavailable / stale | Fallback to DB, rebuild cache, serve stale |
| External API | Timeout / 5xx / rate limited | Retry with backoff, circuit breaker, fallback |
| Message Queue | Full / unavailable / duplicate | Dead letter queue, backpressure, idempotency |
| File System | Full / permission denied / file locked | Quota check, cleanup, retry, graceful error |
| DNS | Resolution failure / stale cache | TTL-respecting cache, fallback IPs |
| Clock | Skew / NTP failure | Monotonic clocks for durations, tolerance for comparisons |

## Checklist

```
[ ] Every external call has a timeout
[ ] Every error is logged with context (what, why, input)
[ ] No empty catch blocks or swallowed exceptions
[ ] Critical paths have circuit breakers
[ ] Retry logic is idempotent-safe with backoff
[ ] Logs are structured with correlation IDs
[ ] Health check endpoints exist (liveness + readiness)
[ ] Graceful shutdown handles in-flight requests
[ ] No single points of failure in architecture
[ ] Deployment is backward-compatible and rollback-ready
```
