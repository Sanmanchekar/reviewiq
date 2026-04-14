# Language-Specific Review Rules

Only the sections matching detected languages are loaded.

## Python

**Anti-Patterns → Findings**:
- Mutable default arguments (`def f(x=[])`) → IMPORTANT: shared state between calls
- Bare `except:` or `except Exception:` → IMPORTANT: catches KeyboardInterrupt, SystemExit
- `eval()`, `exec()`, `pickle.loads()` on user input → CRITICAL: code execution
- `os.system()`, `subprocess.call(shell=True)` with user input → CRITICAL: command injection
- `import *` → NIT: pollutes namespace, breaks IDE support
- `global` keyword → IMPORTANT: hidden state, breaks testability
- String formatting for SQL (`f"SELECT {user_input}"`) → CRITICAL: SQL injection
- `time.sleep()` in async code → IMPORTANT: blocks event loop, use `asyncio.sleep()`
- Missing `__init__.py` in packages → NIT: implicit namespace packages are fragile
- `datetime.now()` without timezone → IMPORTANT: naive datetimes cause timezone bugs

**Performance**:
- List comprehension vs generator for large datasets → use generators to avoid memory explosion
- `+` for string concatenation in loops → use `join()` or `io.StringIO`
- `in` on list vs set for lookups → set for O(1) vs O(n)
- Missing `__slots__` for data-heavy classes → saves 40-50% memory per instance
- Sync I/O in async functions → blocks entire event loop

**Type Safety**:
- Missing type hints on public API → NIT: use `def f(x: int) -> str:`
- `Any` type overuse → NIT: defeats type checking
- `Optional` without None check → IMPORTANT: potential NoneType error

**Testing**:
- `unittest.mock.patch` without `autospec=True` → tests pass but mock doesn't match real interface
- `assert` statements (disabled with `python -O`) → use `pytest.raises` or explicit checks

## Java

**Anti-Patterns → Findings**:
- Raw types (`List` instead of `List<String>`) → IMPORTANT: type safety lost at runtime
- `catch (Exception e) {}` empty catch → CRITICAL: silent failures
- `==` for String comparison → IMPORTANT: reference equality, use `.equals()`
- `synchronized` on non-final field → IMPORTANT: lock can be changed
- `finalize()` method → IMPORTANT: deprecated, unreliable, use try-with-resources
- Mutable static fields → IMPORTANT: shared state across threads
- Missing `@Override` annotation → NIT: catches signature mismatches at compile time
- `new Date()` → IMPORTANT: mutable, use `java.time.Instant`

**Performance**:
- String concatenation in loops → `StringBuilder`
- `HashMap` without initial capacity for known sizes → avoids rehashing
- Autoboxing in hot paths → primitive types for performance-critical code
- `synchronized` blocks too large → minimize critical section scope
- Missing connection pooling (JDBC) → HikariCP or similar

**Concurrency**:
- `ConcurrentHashMap.putIfAbsent()` vs `.computeIfAbsent()` → latter is atomic
- `volatile` without understanding visibility guarantees → prefer `AtomicReference`
- `Thread.sleep()` in production code → use `ScheduledExecutorService`
- Double-checked locking without `volatile` → broken pattern without volatile

**Spring-Specific**:
- `@Autowired` on fields → use constructor injection for testability
- Missing `@Transactional` on service methods that modify data
- `@RequestMapping` without method restriction → specify GET/POST/etc.
- Repository methods without `@Query` returning `List` → unbounded result sets

## Golang

**Anti-Patterns → Findings**:
- Ignoring errors (`val, _ := f()`) → IMPORTANT: must check all errors
- `defer` in loops → IMPORTANT: deferred calls accumulate until function returns
- Goroutine leak (no termination signal) → CRITICAL: unbounded goroutine growth
- Race conditions (shared state without mutex/channel) → CRITICAL: data corruption
- `interface{}` / `any` overuse → NIT: loses type safety
- `panic()` in library code → IMPORTANT: libraries should return errors
- `init()` function side effects → IMPORTANT: makes testing/ordering fragile
- String-keyed maps for known key sets → use constants or typed keys

**Performance**:
- Unbuffered channels in hot paths → buffered channels for throughput
- Excessive allocations (slice append without pre-allocation) → `make([]T, 0, expectedSize)`
- `fmt.Sprintf` in hot paths → `strconv` functions are faster
- Missing `sync.Pool` for frequently allocated objects
- Large structs passed by value → pass by pointer for structs > 64 bytes

**Concurrency**:
- Missing `context.Context` propagation → required for cancellation and deadlines
- `sync.Mutex` protecting map without `sync.RWMutex` → readers block each other
- `select` without `default` blocking forever → add timeout case
- WaitGroup misuse (Add after Go) → always Add before launching goroutine

## TypeScript

**Anti-Patterns → Findings**:
- `any` type → IMPORTANT: defeats TypeScript's purpose
- `as` type assertions → NIT: prefer type guards, assertions bypass checking
- `!` non-null assertion → IMPORTANT: runtime error if wrong
- `== null` instead of `=== null` → NIT: type coercion surprises
- `var` instead of `let`/`const` → NIT: hoisting and scope issues
- Missing `readonly` on properties that shouldn't change → NIT: prevents accidental mutation
- `Promise` without error handling → IMPORTANT: unhandled rejection crashes Node.js
- Circular imports → IMPORTANT: causes undefined at runtime

**React-Specific**:
- Missing dependency arrays in `useEffect`/`useMemo`/`useCallback` → IMPORTANT: stale closures or infinite loops
- State mutation (`state.push(item)`) → CRITICAL: React won't re-render
- Large components (>300 lines) → NIT: extract sub-components
- Missing `key` prop in lists → IMPORTANT: reconciliation bugs
- Inline object/function props → NIT: causes unnecessary re-renders

**Node.js**:
- `process.exit()` without cleanup → IMPORTANT: in-flight requests dropped
- Missing error event handler on streams → CRITICAL: crashes process
- Blocking the event loop (sync I/O, CPU-heavy computation) → CRITICAL: freezes all requests
- `require()` inside functions → NIT: modules should be top-level imports

## C++ / C

**Anti-Patterns → Findings**:
- Raw `new`/`delete` → CRITICAL: use smart pointers (`unique_ptr`, `shared_ptr`)
- Buffer overflow (`strcpy`, `sprintf`, `gets`) → CRITICAL: use bounds-checked alternatives
- Use after free → CRITICAL: undefined behavior, security vulnerability
- Uninitialized variables → CRITICAL: undefined behavior
- Missing virtual destructor in base class → IMPORTANT: memory leak in polymorphic types
- `const_cast` → IMPORTANT: usually indicates design problem
- Macro overuse → NIT: use `constexpr`, templates, `inline` functions
- C-style casts → NIT: use `static_cast`, `dynamic_cast`, `reinterpret_cast`

**Memory Safety**:
- Array bounds not checked → CRITICAL: buffer overflow
- Integer overflow without check → IMPORTANT: undefined behavior in signed, wrapping in unsigned
- Pointer arithmetic without bounds → CRITICAL: out-of-bounds access
- `memcpy`/`memmove` with wrong sizes → CRITICAL: buffer overflow
- Missing null check after allocation → IMPORTANT: null dereference

## Rust

**Anti-Patterns → Findings**:
- Excessive `.unwrap()` / `.expect()` → IMPORTANT: panics in production, use `?` or match
- `unsafe` block without safety comment → CRITICAL: must document why invariants hold
- `.clone()` overuse → NIT: may indicate ownership design issue
- `Arc<Mutex<T>>` where `Arc<RwLock<T>>` suffices → NIT: readers shouldn't block each other
- `Box<dyn Error>` losing error type info → NIT: use typed errors or `thiserror`

## C# / .NET

**Anti-Patterns → Findings**:
- `async void` methods → CRITICAL: exceptions can't be caught, use `async Task`
- Missing `ConfigureAwait(false)` in libraries → IMPORTANT: deadlocks in non-async callers
- `IDisposable` without `using` → IMPORTANT: resource leak
- `lock(this)` or `lock(typeof(T))` → IMPORTANT: external code can deadlock
- String concatenation in loops → use `StringBuilder`
- LINQ `.ToList()` when only iterating → NIT: unnecessary allocation

## Ruby

**Anti-Patterns → Findings**:
- `eval()` / `send()` with user input → CRITICAL: code execution
- Missing `freeze` on string constants → NIT: mutable string constants
- N+1 queries in Rails (`.each` without `includes`) → IMPORTANT: use eager loading
- `rescue => e` (catches all StandardError) → IMPORTANT: too broad, catch specific errors
- Missing strong parameters in controllers → CRITICAL: mass assignment vulnerability

## PHP

**Anti-Patterns → Findings**:
- `eval()` / `preg_replace` with `/e` → CRITICAL: code execution
- `mysql_*` functions → CRITICAL: deprecated, use PDO with prepared statements
- `$_GET`/`$_POST` without sanitization → CRITICAL: injection risk
- `include`/`require` with user input → CRITICAL: local file inclusion
- `==` for security comparisons → IMPORTANT: type juggling, use `===`

## SQL

**Anti-Patterns → Findings**:
- `SELECT *` → NIT: fetch only needed columns, breaks on schema change
- Missing indexes on WHERE/JOIN columns → IMPORTANT: full table scan
- `LIKE '%pattern%'` → IMPORTANT: can't use index, consider full-text search
- Functions on indexed columns in WHERE → IMPORTANT: prevents index use
- Missing transaction for multi-statement writes → CRITICAL: partial updates
- `DELETE` without `WHERE` → CRITICAL: deletes all rows
- `UPDATE` without `WHERE` → CRITICAL: updates all rows
- Cartesian joins (missing JOIN condition) → CRITICAL: result set explosion

## Shell

**Anti-Patterns → Findings**:
- Unquoted variables (`$var` vs `"$var"`) → IMPORTANT: word splitting, globbing
- Missing `set -euo pipefail` → IMPORTANT: errors silently ignored
- `eval` with user input → CRITICAL: command injection
- `curl | bash` → CRITICAL: remote code execution without verification
- Hardcoded paths → NIT: use `$(dirname "$0")` or configurable paths
- Missing error handling on critical commands → IMPORTANT: script continues after failure

## Legacy (COBOL/Fortran/ABAP)

**Review Focus**:
- Fixed-width field overflow → CRITICAL: data truncation without warning
- Missing bounds checking on arrays → CRITICAL: buffer overflow
- Implicit type conversions → IMPORTANT: precision loss
- GO TO spaghetti → NIT: restructure to structured programming where possible
- Copybook changes affecting multiple programs → IMPORTANT: check all consumers
- JCL/batch job dependencies → IMPORTANT: verify upstream/downstream job compatibility
- Character encoding assumptions (EBCDIC vs ASCII) → IMPORTANT: conversion errors
