# Maintainability Review Skill

Code that works today must be understandable, changeable, and debuggable tomorrow. Review for long-term health, not just correctness.

## Complexity

- **Cyclomatic Complexity >10 per function** → IMPORTANT: too many branches, extract sub-functions or use polymorphism
- **Nesting Depth >3** → IMPORTANT: deeply nested `if/for/try` blocks are unreadable. Use early returns, guard clauses, extract methods.
- **Function Length >50 lines** → NIT: function does too much. Split by responsibility.
- **File Length >500 lines** → NIT: file covers too many concerns. Split into modules.
- **Parameter Count >5** → NIT: too many params indicates function does too much. Use parameter objects or builder pattern.
- **Boolean Parameters** → NIT: `process(data, true, false)` is unreadable. Use enums, named constants, or separate functions.
- **Deeply Nested Callbacks** → IMPORTANT: callback hell. Use async/await, promises, or extract named functions.
- **Switch/Case >7 branches** → NIT: consider strategy pattern, lookup table, or polymorphism.

## Naming

- **Single-letter variables** (outside loop counters) → NIT: `x`, `d`, `tmp` reveal nothing. Name for intent.
- **Misleading names** → IMPORTANT: `isReady` that returns a count, `getUser` that deletes records. Names must match behavior.
- **Abbreviations** → NIT: `usr_mgr_svc` → `userManagerService`. Clarity over brevity.
- **Generic names** → NIT: `data`, `info`, `temp`, `result`, `item`, `thing` — name for what it actually represents.
- **Inconsistent naming** → NIT: mixing `camelCase` and `snake_case` in same module, or `get`/`fetch`/`retrieve` for same concept.
- **Negated booleans** → NIT: `isNotValid`, `disableCache` — double negatives confuse. Use positive names: `isValid`, `enableCache`.

## Code Organization

- **God classes/modules** → IMPORTANT: one class that handles auth, validation, persistence, and email. Split by responsibility.
- **Feature envy** → NIT: method that accesses another object's fields more than its own. Move the method.
- **Circular dependencies** → IMPORTANT: module A imports B imports A. Restructure, extract shared interface, or use dependency injection.
- **Dead code** → NIT: unused functions, unreachable branches, commented-out blocks. Delete — git remembers.
- **Copy-paste code** → IMPORTANT: same logic in 3+ places. Extract to shared function/module.
- **Inconsistent abstractions** → NIT: mixing raw SQL and ORM in same service, or manual HTTP and API client in same module.
- **Magic numbers/strings** → NIT: `if status == 3` or `timeout = 30000`. Use named constants.
- **Premature abstraction** → NIT: interface with one implementation, factory for one product. Wait for the second use case.

## Dependencies

- **Tight coupling** → IMPORTANT: class directly instantiates its dependencies. Use dependency injection.
- **Hidden dependencies** → IMPORTANT: function uses global state, singleton, or module-level variable. Make dependencies explicit via parameters.
- **Inappropriate intimacy** → NIT: class reaching into another's internals. Use public API.
- **Unused dependencies** → NIT: imported but never used packages. Remove to reduce attack surface and build times.
- **Transitive dependency reliance** → IMPORTANT: using a type/function from a library you don't directly depend on. Add explicit dependency.

## Error Messages & Logging

- **Generic error messages** → NIT: `"An error occurred"` helps nobody. Include: what failed, what was the input, what to do about it.
- **Missing correlation IDs** → IMPORTANT: no way to trace a request across logs. Add request ID to all log entries.
- **Log-and-throw** → NIT: logging an error then rethrowing it produces duplicate log entries. Do one or the other.
- **Inconsistent log levels** → NIT: ERROR for non-errors, INFO for debugging details. ERROR = needs human attention.
- **String interpolation in logs** → NIT: `log(f"User {user_id}")` allocates even when log level is disabled. Use lazy formatting.

## Documentation (only flag when missing on complex/non-obvious code)

- **Complex algorithm without explanation** → NIT: if the logic took you more than 30 seconds to understand, it needs a comment explaining *why*, not *what*.
- **Public API without doc** → NIT: public functions/classes that other teams consume need parameter/return documentation.
- **Non-obvious side effects** → IMPORTANT: function that looks pure but modifies global state, sends email, or writes to disk. Document the side effect.
- **Workarounds without context** → IMPORTANT: hack with no comment explaining why. Future devs will either break it or be afraid to touch it.

## Testability

- **Untestable code** → IMPORTANT: static methods calling static methods, direct filesystem/network access in business logic. Must be injectable.
- **Time-dependent code** → IMPORTANT: `DateTime.Now` / `time.time()` hardcoded. Inject a clock for deterministic testing.
- **Random-dependent code** → NIT: using `Math.random()` without seed injection. Makes tests non-deterministic.
- **Large setup requirements** → NIT: test requires 20 lines of setup. Indicates the class has too many dependencies.

## Refactoring Signals

When you see these, suggest a refactoring direction (not just flag the problem):

| Signal | Suggested Direction |
|---|---|
| Duplicated blocks | Extract shared function |
| Long parameter list | Introduce parameter object |
| Switch on type | Replace with polymorphism |
| Temporary fields | Extract class |
| Parallel inheritance | Merge hierarchies |
| Data clumps | Group into value object |
| Primitive obsession | Introduce domain types |
| Shotgun surgery | Move related code together |

## Checklist

```
[ ] No function exceeds 50 lines or cyclomatic complexity 10
[ ] No nesting deeper than 3 levels
[ ] All names reveal intent (no single-letter, no abbreviations, no generics)
[ ] No copy-paste code (DRY across 3+ occurrences)
[ ] No magic numbers or strings — all use named constants
[ ] No circular dependencies between modules
[ ] Dead code removed (unused imports, functions, commented blocks)
[ ] Complex/non-obvious logic has comments explaining WHY
[ ] Dependencies are explicit (no hidden globals/singletons)
[ ] Error messages include context (what failed, why, what input)
```
