# Airflow Review Skill

Review Apache Airflow DAGs, operators, and pipeline configurations.

## DAG Design

**DAG Anti-Patterns → Findings**:
- DAG file does heavy computation at module level → CRITICAL: runs on every scheduler heartbeat (~5s), blocks all DAG parsing
- Missing `default_args` with `retries`, `retry_delay` → IMPORTANT: tasks fail permanently on transient errors
- `schedule_interval` as timedelta without `catchup=False` → IMPORTANT: creates backfill storm on deploy
- Missing `max_active_runs=1` on DAGs that aren't idempotent → CRITICAL: parallel runs corrupt data
- DAG without `tags` → NIT: hard to filter in Airflow UI
- `start_date` set to `datetime.now()` → CRITICAL: DAG never triggers — start_date must be static
- Missing `dagrun_timeout` → IMPORTANT: stuck DAGs consume worker slots forever
- DAG file imports heavy libraries (pandas, spark) at top level → IMPORTANT: slows scheduler parsing. Import inside task functions.

## Task Design

**Task Anti-Patterns → Findings**:
- `PythonOperator` with lambda → NIT: not serializable, breaks with certain executors
- Hardcoded connection strings instead of Airflow Connections → CRITICAL: secrets in code, not rotatable
- Tasks that return large XCom values (>48KB) → IMPORTANT: XCom stored in metadata DB, large values cause DB bloat/OOM
- Missing `task_id` naming convention → NIT: use `verb_noun` pattern (e.g., `extract_orders`, `load_warehouse`)
- `BashOperator` with user input in command string → CRITICAL: shell injection risk
- `trigger_rule='all_success'` (default) when partial success is acceptable → IMPORTANT: entire pipeline fails if optional task fails
- Missing `execution_timeout` on tasks → IMPORTANT: hung tasks block worker slots
- `depends_on_past=True` without `wait_for_downstream=True` → IMPORTANT: can skip failed downstream tasks

## Operator Best Practices

- Using `PythonOperator` for database queries → IMPORTANT: use `PostgresOperator`/`MySqlOperator` with connection pooling
- `S3ToRedshiftOperator` without `TRUNCATECOLUMNS` → IMPORTANT: schema changes cause load failures
- `EmailOperator` in task flow (not `on_failure_callback`) → NIT: email should be alert, not pipeline step
- `SubDagOperator` → IMPORTANT: deprecated, use TaskGroup instead — SubDags cause deadlocks
- `BranchPythonOperator` without `trigger_rule='none_failed_min_one_success'` on downstream join → CRITICAL: skipped branch permanently blocks join task
- Custom operator without `template_fields` → IMPORTANT: Jinja templating won't work for those parameters

## Data Pipeline Patterns

- Missing idempotency (no `INSERT ... ON CONFLICT` or `MERGE`) → CRITICAL: re-runs create duplicates
- No partition pruning on date-based queries → IMPORTANT: full table scan on each run
- Loading data without schema validation → IMPORTANT: bad data propagates downstream silently
- Missing data quality checks between extract and load → IMPORTANT: garbage in, garbage out
- `Variable.get()` called in DAG file body (not inside task) → IMPORTANT: hits metadata DB on every scheduler parse
- Sensor with `mode='poke'` and short `poke_interval` → IMPORTANT: wastes worker slot. Use `mode='reschedule'` for long waits.
- Missing SLA/alerting on critical pipelines → IMPORTANT: silent failures

## XCom & Communication

- Passing DataFrames through XCom → CRITICAL: serializes entire DataFrame to metadata DB. Use S3/GCS intermediate storage.
- XCom without explicit `key` → NIT: default key makes debugging harder
- Relying on XCom for large data transfer between tasks → IMPORTANT: use object storage, pass reference (S3 URI)

## Security

- Connections with plaintext passwords in environment vars → CRITICAL: use Airflow's encrypted connection store or secrets backend
- `fernet_key` not set or default → CRITICAL: connections stored unencrypted in metadata DB
- Missing RBAC configuration → IMPORTANT: all users have admin access by default
- DAGs accessing resources outside their scope → IMPORTANT: use Airflow roles to restrict DAG-level access
