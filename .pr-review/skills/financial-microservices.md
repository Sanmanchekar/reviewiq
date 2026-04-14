# Financial Microservices Review Skill

Patterns for distributed financial systems where money movement spans multiple services.

## Distributed Transaction Patterns

### Saga Pattern

- **Missing saga orchestrator** → CRITICAL: multi-service money movement without saga = partial failures with money stuck.
- **Missing compensating transactions** → CRITICAL: if step 3 of 5 fails, steps 1-2 must be reversed. Every step needs a compensation action.
- **Compensation ordering** → IMPORTANT: compensations must execute in reverse order of the original steps. A→B→C fails → compensate C→B→A.
- **Idempotent steps** → CRITICAL: every saga step must be idempotent. Retries happen during recovery.
- **Saga timeout** → IMPORTANT: saga must have a global timeout. Stuck sagas must be detected and resolved (complete or compensate).
- **Saga state persistence** → CRITICAL: saga state must survive service restarts. Store in durable storage, not memory.
- **Partial compensation** → IMPORTANT: if compensation itself fails, what happens? Must have a dead-letter/manual resolution path.

**Example — Loan Disbursement Saga:**
```
1. Create loan record → compensate: cancel loan
2. Debit from funding account → compensate: credit back
3. Credit to borrower account → compensate: debit back
4. Update ledger → compensate: reverse entry
5. Send confirmation → compensate: send cancellation
```

### Event Sourcing for Finance

- **Event immutability** → CRITICAL: financial events must never be modified or deleted. Corrections via new compensating events.
- **Event ordering** → CRITICAL: events must be processed in order per aggregate. Out-of-order = wrong balance.
- **Snapshot strategy** → IMPORTANT: rebuild from events can be slow. Snapshot every N events for performance.
- **Schema evolution** → IMPORTANT: event schema will change. Use versioning and upcasting. Never break old events.
- **Replay safety** → CRITICAL: replaying events must produce the same state. No side effects during replay (don't re-send emails, re-charge cards).

### Eventual Consistency

- **Consistency boundaries** → IMPORTANT: define which operations must be strongly consistent (balance check + debit) vs eventually consistent (notifications, analytics).
- **Stale read handling** → IMPORTANT: after transferring money, user may see old balance. Show pending state, not stale balance.
- **Conflict resolution** → IMPORTANT: when two services disagree on state, which wins? Define conflict resolution rules upfront.
- **Convergence guarantee** → CRITICAL: eventually consistent systems must actually converge. Monitor and alert on stuck state.

## Service Communication

### Synchronous (HTTP/gRPC)

- **Timeout chain** → CRITICAL: if Service A (timeout: 30s) calls B (timeout: 30s) calls C (timeout: 30s), actual timeout can be 90s. Set cascading timeouts: A=30s, B=20s, C=10s.
- **Retry amplification** → CRITICAL: A retries 3x → B retries 3x → C gets 9 requests. Set retry budgets across the chain.
- **Circuit breaker per dependency** → IMPORTANT: separate circuit breaker for each downstream service. One failure shouldn't cascade.
- **Fallback responses** → IMPORTANT: when dependency is down, what do you return? Cached data? Degraded response? Error? Define per endpoint.
- **Request correlation** → IMPORTANT: propagate trace ID across all services. Without it, debugging distributed failures is impossible.

### Asynchronous (Message Queue)

- **At-least-once delivery** → CRITICAL: queues deliver at least once. Every consumer must be idempotent. Deduplication by message/event ID.
- **Message ordering** → IMPORTANT: for financial events, ordering matters. Use partition keys (account ID) to ensure per-entity ordering.
- **Dead letter queue** → CRITICAL: messages that fail repeatedly must go to DLQ, not be retried forever. Monitor DLQ depth. Alert on growth.
- **Poison message handling** → IMPORTANT: malformed message that crashes consumer. Must not block other messages. Isolate and DLQ.
- **Consumer lag monitoring** → IMPORTANT: growing consumer lag = processing falling behind. Alert before it becomes critical.
- **Backpressure** → IMPORTANT: producer faster than consumer. Rate limit producers or scale consumers. Don't let queue grow unbounded.
- **Message TTL** → IMPORTANT: financial messages shouldn't be processed after too long (e.g., don't process a payment 3 days late). Set expiry.

## Data Consistency

### Dual-Write Problem

- **Database + Event publish** → CRITICAL: writing to database AND publishing event is not atomic. If either fails, systems diverge.
- **Solutions**:
  - Transactional outbox: write event to outbox table in same DB transaction. Separate process publishes.
  - Change Data Capture (CDC): capture DB changes and publish as events.
  - Event-first: publish event, then let consumer update DB.
- **Missing outbox pattern** → CRITICAL: direct write to DB + direct publish to Kafka/RabbitMQ = lost events on failure.

### Cross-Service Data

- **Shared database** → IMPORTANT: multiple services writing same tables. Breaks service autonomy. Each service owns its data.
- **Data replication** → IMPORTANT: if service B needs service A's data, replicate via events. Don't share DB.
- **Cache invalidation across services** → IMPORTANT: service A updates data, service B has cached old version. Use event-driven invalidation.

## Ledger as a Service

- **Single source of truth** → CRITICAL: the ledger is the source of truth for money. All other systems derive from it.
- **Write-ahead logging** → IMPORTANT: log entry intent before execution. Enables recovery after crash.
- **Idempotent posting** → CRITICAL: same entry posted twice must not create duplicate records. Use entry reference + idempotency key.
- **Balance check atomicity** → CRITICAL: balance check and debit must be atomic. Without this: race condition → negative balance.
- **Multi-currency ledger** → IMPORTANT: if supporting multiple currencies, each account has per-currency balances. FX entries must balance in both currencies.

## Deployment Patterns

### Zero-Downtime for Financial Services

- **Database migration strategy** → CRITICAL: never lock tables with active financial transactions. Use expand-then-contract.
- **Feature flags for financial logic** → IMPORTANT: new interest calculation method behind flag. Gradual rollout. Instant rollback.
- **Canary with financial validation** → IMPORTANT: canary deployment must validate financial calculations match. Reconcile canary vs stable results.
- **Blue-green with state** → IMPORTANT: both environments must point to same database. No split-brain during cutover.
- **Rollback plan** → CRITICAL: every deployment affecting money movement must have tested rollback procedure.

### Service Mesh Considerations

- **mTLS between services** → IMPORTANT: financial data in transit between services must be encrypted. Mutual TLS mandatory.
- **Service-to-service authorization** → IMPORTANT: not every service should call every other service. Define and enforce service-level permissions.
- **Rate limiting between services** → NIT: prevent cascade failures. Limit requests per service pair.

## Monitoring for Financial Services

- **Reconciliation alerts** → CRITICAL: automated alerts when internal ledger diverges from external (bank, gateway).
- **Transaction success rate** → IMPORTANT: monitor per payment method, per gateway, per amount range. Alert on drops.
- **Settlement delay** → IMPORTANT: monitor time between transaction and settlement. Alert on increasing delays.
- **Money in transit** → IMPORTANT: monitor total money in intermediate states (processing, pending). Alert if growing over time.
- **End-of-day balance** → IMPORTANT: automated daily balance check — sum of transactions = expected balance change.

## Checklist

```
[ ] Multi-service money movement uses saga pattern with compensating transactions
[ ] Every saga step and compensation is idempotent
[ ] Saga state persisted in durable storage (not memory)
[ ] Timeout chain configured across service calls (cascading, not additive)
[ ] Retry budget set to prevent retry amplification
[ ] Transactional outbox pattern used (no dual-write DB + event publish)
[ ] Message consumers are idempotent with deduplication
[ ] Dead letter queue configured with monitoring
[ ] Financial events are immutable (corrections via compensating events)
[ ] Balance check + debit is atomic (no race condition)
[ ] mTLS enabled between financial services
[ ] Reconciliation alerts configured for all external integrations
[ ] Zero-downtime deployment with rollback plan tested
```
