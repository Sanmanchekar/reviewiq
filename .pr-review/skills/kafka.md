# Kafka Review Skill

Review Apache Kafka producers, consumers, Kafka Streams, and Connect configurations.

## Producer

**Producer Anti-Patterns → Findings**:
- Fire-and-forget (no callback or `.get()`) → CRITICAL: messages silently lost on broker failure
- Missing `acks=all` for financial/critical data → CRITICAL: data loss if leader crashes before replication
- `retries=0` or no retry config → IMPORTANT: transient network errors cause permanent loss
- Missing `enable.idempotence=true` → IMPORTANT: retries can produce duplicates without idempotence
- `max.in.flight.requests.per.connection > 5` with retries → IMPORTANT: message reordering risk
- Serializer without schema registry → IMPORTANT: schema drift breaks consumers silently
- Large messages (>1MB default) without compression → IMPORTANT: use `compression.type=snappy|lz4`
- Missing `linger.ms` and `batch.size` tuning → NIT: default batching is suboptimal for throughput
- Hardcoded topic names → NIT: use config/env vars for environment-specific topics
- `key=None` for ordered data → CRITICAL: without key, messages spread across partitions — no ordering guarantee

## Consumer

**Consumer Anti-Patterns → Findings**:
- `enable.auto.commit=true` with at-least-once requirement → CRITICAL: auto-commit before processing = message loss on crash
- Missing `auto.offset.reset` config → IMPORTANT: default is `latest` — new consumer group misses all existing messages
- `poll()` timeout too short (<100ms) → NIT: excessive rebalancing, CPU waste
- No error handling in poll loop → CRITICAL: single bad message crashes entire consumer
- Missing dead-letter queue (DLQ) for poison messages → IMPORTANT: one bad message blocks partition forever
- `max.poll.records` too high without matching `max.poll.interval.ms` → IMPORTANT: consumer kicked from group for slow processing
- Committing offsets before processing completes → CRITICAL: message loss — commit AFTER successful processing
- Missing consumer lag monitoring → IMPORTANT: silent backlog buildup
- Single-threaded consumer for high-throughput topic → IMPORTANT: consider partition-level parallelism
- `group.id` shared across unrelated services → CRITICAL: consumers steal each other's partitions

## Consumer Group & Rebalancing

- Missing `session.timeout.ms` tuning → NIT: default (45s) may be too long for fast failure detection
- `heartbeat.interval.ms` > `session.timeout.ms / 3` → IMPORTANT: consumer evicted before heartbeat sent
- No `ConsumerRebalanceListener` → IMPORTANT: in-progress work lost on rebalance without cleanup callback
- Static group membership not used for stable consumers → NIT: `group.instance.id` prevents unnecessary rebalances on restart
- Missing graceful shutdown (no `consumer.close()`) → IMPORTANT: triggers rebalance instead of clean leave

## Topic Design

- Single partition for high-throughput topic → CRITICAL: can't scale consumers beyond 1
- Too many partitions (>50 per topic without reason) → IMPORTANT: increases end-to-end latency, metadata overhead
- Missing retention policy → IMPORTANT: disk fills up or data lost too early
- `cleanup.policy=compact` without key → CRITICAL: compaction requires message keys
- Missing `min.insync.replicas=2` for critical topics → IMPORTANT: `acks=all` alone doesn't guarantee durability if replicas < 2
- Replication factor = 1 → CRITICAL: single broker failure = data loss

## Exactly-Once Semantics

- Producer without transactions for multi-topic writes → IMPORTANT: partial writes across topics on failure
- Missing `transactional.id` for transactional producer → CRITICAL: transactions require stable ID across restarts
- Consumer read_committed without producer transactions → NIT: `isolation.level=read_committed` has no effect without transactional producers
- Mixing transactional and non-transactional producers on same topic → IMPORTANT: non-transactional messages bypass isolation

## Schema & Serialization

- JSON without schema validation → IMPORTANT: schema drift breaks consumers. Use Avro/Protobuf + Schema Registry.
- Missing schema compatibility mode (BACKWARD/FORWARD) → IMPORTANT: incompatible schema changes break running consumers
- Custom serializer without error handling → IMPORTANT: serialization failure crashes producer
- Deserializer trusting all input → CRITICAL: malformed messages crash consumer. Always handle `SerializationException`.

## Operational

- Missing consumer lag alerting (`kafka.consumer:type=consumer-fetch-manager-metrics`) → IMPORTANT: silent processing delays
- No monitoring of under-replicated partitions → CRITICAL: data durability at risk
- Missing broker disk usage alerts → IMPORTANT: full disk = broker crash
- Kafka Connect with `tasks.max=1` for high-volume connector → NIT: can parallelize with more tasks
- Connect without dead-letter queue (`errors.deadletterqueue.topic.name`) → IMPORTANT: bad records block entire connector
