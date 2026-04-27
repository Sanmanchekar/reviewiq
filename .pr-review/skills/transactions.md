# Transactions, Isolation & Locking Review Skill

Review transactional code for correctness under concurrency: isolation level, lock ordering, retry, idempotency.

## Isolation Level

**Anti-Patterns → Findings**:
- Read-modify-write under `READ COMMITTED` (the default) without `SELECT ... FOR UPDATE` → CRITICAL: lost update. Two transactions both read, both write, the second silently overwrites.
- Aggregate-then-decide pattern (`SELECT SUM(...)` then `INSERT` if under limit) without `SERIALIZABLE` or explicit locking → CRITICAL: classic phantom — concurrent inserts blow past the limit. Money flows often hit this.
- Using `REPEATABLE READ` and assuming it prevents lost updates — it does, but only by **aborting** the second tx with serialization failure. No retry loop → IMPORTANT: user sees error.
- Mixing isolation levels per query inside one transaction → IMPORTANT: `SET TRANSACTION ISOLATION LEVEL` only works at start; later statements ignore it.
- Long-running `SERIALIZABLE` transactions → IMPORTANT: high abort rate under load. Keep them short.

## Locking

- `SELECT ... FOR UPDATE` without `NOWAIT` / `SKIP LOCKED` on a worker queue pattern → IMPORTANT: workers serialize on the same row. Use `SKIP LOCKED` for parallel pickup.
- `SELECT ... FOR UPDATE` on a wide range without index → CRITICAL: locks far more rows than intended. Confirm predicate is index-driven.
- Inconsistent lock ordering across code paths (path A locks `users` then `accounts`; path B does the reverse) → CRITICAL: deadlock. Establish global lock order.
- Advisory locks without paired release in `finally` → CRITICAL: leaked lock blocks all future work in that key space.
- Long-held locks across an external HTTP/S3 call → CRITICAL: holds the row + connection for whatever the external service decides. Move external I/O outside the tx.
- `LOCK TABLE` in app code → IMPORTANT: nuclear option, rarely the right answer in OLTP.

## Retry & Deadlock

- `try/except SerializationFailure` / `DeadlockDetected` not retried → CRITICAL: user-visible error for a transient condition. Retry with bounded attempts + backoff.
- Retry loop without idempotency → CRITICAL: side-effects (emails, charges) fire twice. Pair retry with idempotency keys.
- Retry that re-reads inside the **same** session/connection without resetting state → IMPORTANT: must `ROLLBACK` and start fresh.
- Unbounded retry → IMPORTANT: amplifies overload. Cap attempts (3–5) and backoff.

## Transaction Scope

- Transaction wrapping an external call (HTTP, S3, Stripe) → CRITICAL: external latency = held DB connection + locks + replication backpressure. Pattern: persist intent → release tx → external call → second tx to record result (saga / outbox).
- Transaction held across a request boundary (open in handler A, committed in handler B) → CRITICAL: any failure between leaks the tx.
- Spring `@Transactional` on a private method or self-call → CRITICAL: AOP proxy bypassed; method runs without a tx silently.
- Spring `@Transactional(propagation = REQUIRES_NEW)` without understanding suspension → IMPORTANT: outer tx pauses, inner can commit while outer rolls back → partial state.
- Django `transaction.atomic` swallowing the exception inside (`try: ... atomic ... except`) → CRITICAL: catching after the rollback marker is set raises `TransactionManagementError` on next query.
- Async code holding a connection across `await` on unrelated I/O → IMPORTANT: starves the pool. Release before, reacquire after.

## Distributed / Multi-Resource

- Two-phase commit assumed across DB + message broker → CRITICAL: there is no XA in most modern stacks. Use outbox or saga pattern.
- Writing to DB and publishing to Kafka in the same code block, expecting both-or-neither → CRITICAL: one will fail. Outbox: write to DB only, separate process publishes.
- "Transactional outbox" without exactly-once consumer (idempotency key) → IMPORTANT: at-least-once delivery means duplicates downstream.
- Saga without compensating actions for each step → CRITICAL: partial failure leaves system in inconsistent state.
- Distributed lock (Redis SETNX, ZooKeeper) without TTL → CRITICAL: dead holder blocks all callers. With TTL → must handle the race when TTL expires mid-work (use fencing token).

## Idempotency

- Money / external-side-effect handler without an idempotency key → CRITICAL: at-least-once delivery + retries = duplicate charges/emails/notifications.
- Idempotency key derived from request body without dedup window → IMPORTANT: long-lived dedup table grows unbounded.
- Idempotency check after the side effect, not before → CRITICAL: defeats the purpose.
- Using the DB primary key as idempotency key → IMPORTANT: works only if the client controls the PK; otherwise reuse a business identifier (order_id, txn_ref).

## Common Specific Issues

- Postgres: `SELECT FOR UPDATE` on a row that's part of a CTE in the same statement → IMPORTANT: locking rules in CTEs are subtle; verify lock actually applied.
- Postgres advisory locks shared between unrelated subsystems on overlapping integer keys → CRITICAL: collision = silent serialization. Namespace with the two-arg form `pg_advisory_lock(class, id)`.
- MySQL InnoDB gap locks under `REPEATABLE READ` (default) blocking concurrent inserts unexpectedly → IMPORTANT: surprising deadlocks. Audit gap-lock interactions on hot tables.
- Aurora multi-AZ failover mid-transaction → IMPORTANT: in-flight tx rolled back, app must surface and retry.
- Read replicas + read-after-write expectation → CRITICAL: replication lag means just-written row not visible. Route read-after-write to primary.
