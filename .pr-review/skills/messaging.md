# Messaging & Queue Review Skill

Review message brokers, queues, and async processing patterns (RabbitMQ, SQS, Celery, Redis queues, etc.).

## RabbitMQ

**Anti-Patterns → Findings**:
- Missing `durable=True` on queues with important messages → CRITICAL: queue lost on broker restart
- Publisher without `mandatory=True` → IMPORTANT: messages to non-existent queues silently dropped
- Missing `delivery_mode=2` (persistent) on critical messages → CRITICAL: messages lost on broker restart
- Missing publisher confirms → IMPORTANT: no guarantee message reached broker
- Unbounded queue without TTL or max-length → IMPORTANT: memory exhaustion on slow consumers
- Missing dead-letter exchange (DLX) → IMPORTANT: rejected messages disappear forever
- `basic_get` (pull) in a loop instead of `basic_consume` (push) → NIT: inefficient polling
- Missing `prefetch_count` → IMPORTANT: single slow consumer blocks entire queue
- Acking before processing → CRITICAL: message loss on crash
- Missing connection/channel recovery → IMPORTANT: transient network errors kill consumer permanently

## Celery

**Anti-Patterns → Findings**:
- Task without `bind=True` and `self.retry()` → IMPORTANT: no automatic retry on failure
- Missing `max_retries` → IMPORTANT: infinite retry loop on permanent failure
- Missing `acks_late=True` for at-least-once → CRITICAL: task lost if worker crashes during execution
- `task_always_eager=True` in production → CRITICAL: bypasses broker, runs synchronously — defeats purpose of async
- Result backend enabled but results never consumed → IMPORTANT: result store fills up. Disable `ignore_result=True` if unused.
- Large task arguments (>100KB) → IMPORTANT: serialized through broker. Pass reference (DB ID, S3 URI) instead.
- Missing `task_time_limit` and `task_soft_time_limit` → IMPORTANT: hung tasks block workers
- Task importing heavy modules at function level → NIT: move to top-level or lazy import
- Missing `task_reject_on_worker_lost=True` → IMPORTANT: task lost if worker killed (OOM, SIGKILL)
- Celery beat schedule without `solar`/`crontab` (using timedelta) → NIT: drift over time

## SQS / Cloud Queues

- Missing visibility timeout configuration → IMPORTANT: default 30s may be too short for long-running tasks
- `ReceiveMessage` without `WaitTimeSeconds` (short polling) → IMPORTANT: high cost, empty responses. Use long polling.
- Missing DLQ configuration (`RedrivePolicy`) → IMPORTANT: poison messages retried forever
- `maxReceiveCount` too high in redrive policy → NIT: excessive retries before DLQ
- Deleting message before processing completes → CRITICAL: message lost on crash
- FIFO queue without `MessageGroupId` → CRITICAL: ordering not guaranteed without group ID
- Missing message deduplication for non-FIFO → IMPORTANT: at-least-once delivery means duplicates

## Redis Queues (Bull, BullMQ, RQ)

- Missing job timeout → IMPORTANT: stuck jobs block workers
- No failed job handler → IMPORTANT: failed jobs disappear silently
- Missing `removeOnComplete`/`removeOnFail` TTL → IMPORTANT: completed jobs fill Redis memory
- Large job payloads in Redis → IMPORTANT: Redis is in-memory — pass references, not data
- Missing concurrency limit → IMPORTANT: all workers process same queue, starving others
- No rate limiting on high-volume producers → IMPORTANT: Redis OOM

## General Async Patterns

- Missing idempotency on message handlers → CRITICAL: at-least-once delivery means duplicates. Use idempotency keys.
- No poison message handling → IMPORTANT: one bad message blocks entire queue/partition
- Missing message ordering guarantee when required → CRITICAL: out-of-order processing corrupts state
- Synchronous call inside async handler → IMPORTANT: blocks worker thread, reduces throughput
- Missing correlation ID for request-reply → IMPORTANT: can't trace messages across services
- No backpressure mechanism → IMPORTANT: fast producer + slow consumer = unbounded queue growth
- Missing message schema versioning → IMPORTANT: schema changes break running consumers
