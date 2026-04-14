# The Commandments — Universal Review Laws

These apply to ALL code in ALL languages. Violations are findings.

## I. Correctness Commandments

1. **Every branch shall be covered.** If there is an `if`, there must be an `else` — or a clear reason why the else case is impossible. Unhandled branches are the #1 source of production incidents.

2. **Every error shall be handled.** Never swallow exceptions. Never ignore return codes. Every error path must either recover, propagate, or fail explicitly with context.

3. **Every input shall be validated.** Data crossing a trust boundary (user input, API responses, file reads, env vars, queue messages) must be validated before use. Assume all external data is hostile.

4. **Every resource shall be released.** Files, connections, locks, transactions, memory allocations — if you acquire it, you must release it, even on error paths. Use language-native resource management (try-with-resources, `with`, `defer`, RAII).

5. **Every assumption shall be documented.** If code depends on an invariant that isn't enforced by the type system, add an assertion or comment explaining why it's safe.

## II. Security Commandments

6. **Never trust user input.** Sanitize, validate, parameterize. SQL injection, XSS, command injection, path traversal — all start with trusting input.

7. **Never store secrets in code.** No API keys, passwords, tokens, or certificates in source files, config files, comments, or commit messages. Use secret managers or environment variables.

8. **Never expose internal errors to users.** Stack traces, database errors, internal paths — these are attack surface. Return generic error messages externally, log details internally.

9. **Never roll your own crypto.** Use established libraries for encryption, hashing, signing, and token generation. Custom implementations are almost always vulnerable.

10. **Never disable security controls.** Don't skip TLS verification, don't disable CSRF protection, don't set permissive CORS. If you must disable a control for local dev, it must be impossible to reach production that way.

## III. Reliability Commandments

11. **Every timeout shall be set.** Network calls, database queries, external API calls, lock acquisitions — all must have explicit timeouts. Hanging is worse than failing.

12. **Every retry shall have backoff.** Retries without backoff cause thundering herds. Exponential backoff with jitter is the minimum. Set max retries. Add circuit breakers for chronic failures.

13. **Every concurrent access shall be synchronized.** Shared mutable state requires synchronization. If two threads/goroutines/processes can touch the same data, there must be a lock, channel, or atomic operation protecting it.

14. **Every deployment shall be reversible.** Database migrations must have rollbacks. Feature flags must have off switches. Config changes must be revertable without a deployment.

15. **Every failure shall be observable.** If something can fail, it must emit a log, metric, or alert. Silent failures are the hardest bugs to find. Include context: what failed, why, what was the input, what was the expected outcome.

## IV. Performance Commandments

16. **Never do O(n) work inside O(n) loops.** Nested iteration over data structures is the most common performance bug. Use indexes, maps, sets, or pre-computation.

17. **Never query in a loop.** N+1 queries kill databases. Batch reads, use joins, pre-fetch related data. One query per collection, not one per item.

18. **Never hold locks during I/O.** Network calls, disk reads, and external API calls while holding a lock will serialize your entire system. Fetch data first, then lock, compute, unlock.

19. **Never allocate in hot paths.** Memory allocation in tight loops causes GC pressure and latency spikes. Pre-allocate buffers, reuse objects, pool connections.

20. **Never ignore pagination.** Unbounded queries return unbounded results. Always paginate database queries, API responses, and list operations. Set hard limits.

## V. Maintainability Commandments

21. **Names shall reveal intent.** Variables, functions, classes — their names must say what they do, not how they do it. `retryWithBackoff()` not `loop2()`. `userEmail` not `str1`.

22. **Functions shall do one thing.** If a function has `and` in its description, it does too much. If it requires scrolling to read, it's too long. Extract sub-functions.

23. **Dependencies shall be explicit.** No global state, no hidden singletons, no implicit initialization order. Every dependency should be injected or passed explicitly.

24. **Abstractions shall be earned.** Don't create interfaces for one implementation. Don't add factory patterns for one product. Duplication is cheaper than the wrong abstraction.

25. **Dead code shall be deleted.** Commented-out code, unused imports, unreachable branches, TODO-without-ticket — delete them. Version control remembers.

## VI. Data Commandments

26. **Every mutation shall be atomic.** If updating multiple records that must be consistent, use transactions. Partial updates are data corruption.

27. **Every schema change shall be backward-compatible.** Don't rename columns, don't change types, don't drop tables during deployment. Expand-then-contract migrations only.

28. **Every ID shall be opaque.** Don't expose auto-increment IDs — they leak entity count. Don't use predictable IDs — they enable enumeration attacks. Use UUIDs or random IDs.

29. **Every collection shall be bounded.** Arrays, maps, queues, caches, log files — everything that grows must have a size limit and an eviction policy.

30. **Every timestamp shall have a timezone.** Store UTC. Display local. Never assume timezone. Never use naive datetimes in distributed systems.

## VII. API Commandments

31. **Every API shall be versioned.** Breaking changes without versioning break clients. Use URL versioning, header versioning, or content negotiation.

32. **Every API shall validate requests.** Check required fields, validate types, enforce size limits, reject unknown fields. Invalid requests should fail fast with clear error messages.

33. **Every API shall be idempotent.** Retries happen. Duplicates happen. Network timeouts happen. PUT and DELETE must be idempotent. POST operations need idempotency keys.

34. **Every API response shall include error details.** Error code, human-readable message, request ID for correlation, and optional pointer to the offending field.

35. **Every API shall rate limit.** Protect against abuse, accidents, and cascading failures. Return 429 with Retry-After header. Different limits for different endpoints.

## VIII. Testing Commandments

36. **Tests shall test behavior, not implementation.** Test what the code does, not how it does it. Tests that break when you refactor internals are worse than no tests.

37. **Tests shall be deterministic.** No flaky tests. No time-dependent tests. No order-dependent tests. No network-dependent tests in unit suites.

38. **Test data shall be explicit.** No shared fixtures that mutate. No magic test databases. Every test sets up its own data and cleans up after itself.

39. **Edge cases shall have tests.** Empty inputs, null values, maximum sizes, Unicode, concurrent access, timezone boundaries — if it can happen in production, test it.

40. **Integration points shall be tested.** Mock internal dependencies, but test external integrations. Database queries, API calls, message queues — verify they work with real (or realistic) backends.
