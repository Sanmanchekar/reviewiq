# ORM Review Skill

Review ORM usage (Django ORM, SQLAlchemy, Prisma, TypeORM, Sequelize, Mongoose, GORM, Hibernate) for query shape, transaction discipline, and lazy-load traps.

## N+1 & Eager Loading

**Anti-Patterns → Findings**:
- Iterating a queryset and accessing a relation inside the loop without prefetch → CRITICAL: classic N+1. One query per row.
  - Django: missing `select_related()` (FK/OneToOne) / `prefetch_related()` (M2M, reverse FK).
  - SQLAlchemy: missing `joinedload()` / `selectinload()`. Default lazy `select` strategy in a loop.
  - Prisma: missing `include` / `select`.
  - TypeORM: missing `relations: [...]` in `find` or `leftJoinAndSelect`.
  - Sequelize: missing `include`.
  - Mongoose: missing `.populate()`.
  - GORM: missing `Preload` / `Joins`.
  - Hibernate: lazy associations accessed outside session, or `@OneToMany(fetch = LAZY)` in a loop.
- `len(queryset)` to check existence → IMPORTANT: hydrates full result set. Use `.exists()` (Django), `EXISTS` (SQL), or `.first() is not None`.
- `count()` on a queryset already loaded into memory → NIT: use `len(list)` or cache the count.
- Loading full objects to read one field → IMPORTANT: use `.values()`, `.only()`, `.defer()`, or projection.

## Bulk Operations

- Row-by-row `save()` / `INSERT` in a loop → IMPORTANT: use `bulk_create` / `bulk_insert_mappings` / `createMany` / batch `INSERT`.
- `bulk_create` without `batch_size` on huge lists → IMPORTANT: single statement may exceed driver/PG limits.
- `bulk_update` without `update_fields` → NIT: writes more columns than needed.
- Looping `delete()` per row instead of `.delete()` on a queryset → IMPORTANT: one statement vs N.
- Django `bulk_create(ignore_conflicts=True)` without confirming intent → IMPORTANT: silently skips, may hide a real bug.

## Lazy Load & Session Boundaries

- Returning lazy ORM object from a closed session/request scope (Hibernate `LazyInitializationException`, SQLAlchemy `DetachedInstanceError`) → CRITICAL: blow up at template/serializer time.
- Accessing relations in serializer/template after the view returned → IMPORTANT: triggers query outside transaction; can also N+1 silently.
- ORM session leaked across requests (web app holds a single global session) → CRITICAL: stale identity map, transaction interleave.

## Transactions

- Multi-statement business operation without `transaction.atomic` / `@Transactional` / `db.transaction(...)` → CRITICAL: partial writes on error.
- `atomic`/transaction wrapping a slow external call (HTTP, S3) → CRITICAL: holds DB connection + locks; cascades into pool exhaustion.
- Catching exception inside `atomic` and continuing → CRITICAL: marks transaction broken; subsequent statements raise.
- Nested `atomic` without savepoint awareness → IMPORTANT: outer rollback discards inner "successful" work; usually a misunderstanding.
- ORM autoflush surprising the developer (SQLAlchemy autoflush firing in middle of read) → NIT: disable for the block if it interferes.

## Query Shape via ORM

- `.filter().filter().filter()` chains hiding a complex query → IMPORTANT: read the generated SQL; sometimes one expression is clearer and faster.
- ORM-built query with `OR` across two columns → IMPORTANT: often non-indexable. Confirm planner uses index.
- `Q(...) | Q(...)` joining unrelated tables → IMPORTANT: produces unexpected joins.
- `.distinct()` added to fix duplicate rows from a JOIN → IMPORTANT: usually masks a wrong join. Investigate.
- `.order_by('?')` (Django) / random order → CRITICAL: full scan + sort. Avoid in hot paths.
- `.annotate()` followed by `.filter()` on the annotation when a `WHERE` would suffice → NIT: extra GROUP BY, slower.
- `raw()` / `text()` with f-string interpolation → CRITICAL: SQL injection. Use bind params.

## Migrations via ORM

- `makemigrations` / `alembic revision --autogenerate` committed without reading the diff → IMPORTANT: autogen frequently picks the wrong op.
- Schema change reliant on importing the current model class → IMPORTANT: model drifts; migration won't replay correctly later. Use raw SQL or copy model inline.
- Squashing migrations after some envs already applied them → CRITICAL: drift between envs.

## Connection Management

- Opening a connection per call instead of using the pool → CRITICAL: TCP/TLS handshake per request, exhausts DB.
- Holding a connection across an `await` on an unrelated I/O (async ORMs) → IMPORTANT: connection starvation. Release first, re-acquire.
- Pool size too high relative to DB `max_connections` → CRITICAL: thundering herd on DB. Use a connection proxy (PgBouncer, RDS Proxy).
- `lifo` vs `fifo` pool strategy without rationale → NIT.

## Specific ORM Footguns

- **Django**: `update()` on queryset doesn't call `save()` / signals — confirm if signals are required. `.values()` returns dicts, not objects — easy to lose type checks.
- **SQLAlchemy**: 1.x vs 2.x style mixing — `Query` vs `select()` patterns. `Session.merge()` overuse instead of explicit identity handling.
- **Prisma**: `findFirst` vs `findUnique` confusion — `findUnique` is faster (uses unique constraint) but throws if not unique.
- **TypeORM**: `save()` does upsert by primary key — accidental updates if PK collides. `cascade: true` everywhere → unintended deep writes.
- **Sequelize**: `paranoid: true` (soft delete) silently filters all reads — confirm intent on join-heavy queries.
- **Mongoose**: schema `strict: false` → silent dropped fields. `lean()` returns POJOs (faster, no virtuals/methods) — use for read paths.
- **GORM**: `.Updates(map)` vs `.Updates(struct)` — struct skips zero values. Easy to "lose" updates that set fields to false/0/"".
- **Hibernate**: `@OneToMany` without `fetch = LAZY` defaults to EAGER for `@ManyToOne` — surprise joins. `cascade = ALL` with orphan removal can wipe related data.
