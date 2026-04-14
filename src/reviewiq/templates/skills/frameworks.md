# Framework-Specific Review Rules

Only the sections matching detected frameworks are loaded.

## Django

- Missing `select_related()` / `prefetch_related()` → IMPORTANT: N+1 queries
- Raw SQL without parameterization → CRITICAL: SQL injection
- Missing CSRF protection on state-changing views → CRITICAL
- `DEBUG = True` in production settings → CRITICAL: exposes internals
- Missing `@login_required` or permission checks → IMPORTANT: unprotected views
- `Model.objects.all()` without pagination → IMPORTANT: loads entire table
- Missing database indexes on filtered/ordered fields → IMPORTANT
- Signals for business logic → NIT: hard to trace, prefer explicit calls
- Fat models without service layer → NIT: single responsibility violation
- Missing migrations after model changes → CRITICAL: schema out of sync
- `SECRET_KEY` in settings file → CRITICAL: must be env var
- `ALLOWED_HOSTS = ['*']` → CRITICAL: host header injection

## FastAPI

- Missing response model (`response_model=`) → NIT: no output validation/documentation
- Sync functions for I/O operations → IMPORTANT: blocks event loop, use `async`
- Missing dependency injection for database sessions → IMPORTANT: connection leaks
- Missing request validation (Pydantic models) → IMPORTANT: unvalidated input
- Background tasks without error handling → IMPORTANT: silent failures
- Missing CORS configuration → NIT: if serving frontend
- Circular imports between routers → IMPORTANT: startup failures

## Flask

- Missing CSRF protection (no Flask-WTF) → CRITICAL on forms
- `app.run(debug=True)` in production → CRITICAL: code execution via debugger
- Missing input validation → IMPORTANT: all `request.args`/`request.form` must validate
- Global state in module scope → IMPORTANT: shared across requests in multi-worker
- Missing error handlers (`@app.errorhandler`) → NIT: raw stack traces to users

## Spring

- `@Autowired` field injection → NIT: use constructor injection for testability
- Missing `@Transactional` on service methods → IMPORTANT: no rollback on failure
- `@RequestMapping` without method → IMPORTANT: accepts all HTTP methods
- Missing `@Valid` on request body → IMPORTANT: no input validation
- `@Async` without custom executor → NIT: default pool may be undersized
- Missing exception handler (`@ControllerAdvice`) → IMPORTANT: raw errors to clients
- `@Cacheable` without TTL → IMPORTANT: unbounded cache growth
- Missing security configuration (`@PreAuthorize`) → CRITICAL: unprotected endpoints
- `JpaRepository.findAll()` without pagination → IMPORTANT: loads entire table
- Missing `@Entity` index annotations → IMPORTANT: missing database indexes

## React

- Missing error boundary → IMPORTANT: one component crash takes down entire app
- State mutation (array.push, object property assignment) → CRITICAL: won't re-render
- `useEffect` without cleanup → IMPORTANT: memory leaks (subscriptions, timers)
- `useEffect` with missing/wrong deps → IMPORTANT: stale state or infinite loops
- Prop drilling >3 levels → NIT: use context or state management
- Inline handlers in render (`onClick={() => ...}`) → NIT: new function every render
- Missing `React.memo` on expensive child components → NIT: unnecessary re-renders
- `dangerouslySetInnerHTML` with user data → CRITICAL: XSS
- Missing `key` prop on list items → IMPORTANT: reconciliation bugs
- Large component files (>300 lines) → NIT: extract components
- `localStorage`/`sessionStorage` for sensitive data → CRITICAL: XSS can read it
- Missing loading/error states → NIT: poor UX on slow/failed requests

## Next.js

- `getServerSideProps` for static data → NIT: use `getStaticProps` for performance
- Missing `revalidate` in `getStaticProps` → NIT: stale data until rebuild
- API routes without auth → CRITICAL: open endpoints
- Client-side data fetching for SEO content → IMPORTANT: not indexed by crawlers
- Missing image optimization (`next/image`) → NIT: large unoptimized images
- Missing `middleware.ts` for auth → IMPORTANT: route protection gaps
- Hydration mismatch (server/client render different content) → IMPORTANT: UI bugs

## Express

- Missing input validation middleware → IMPORTANT: raw user input reaches handlers
- Missing error-handling middleware → IMPORTANT: unhandled errors crash process
- Missing rate limiting → IMPORTANT: DoS vulnerability
- `req.params`/`req.query` used without validation → IMPORTANT: injection risk
- Missing helmet middleware → NIT: missing security headers
- Sync I/O in request handlers → CRITICAL: blocks all other requests
- Missing CORS configuration → NIT: if serving API to browser clients
- `app.use(express.json())` without body size limit → IMPORTANT: DoS via large payload

## NestJS

- Missing `ValidationPipe` → IMPORTANT: no input validation
- Missing `AuthGuard` on controllers → CRITICAL: unprotected endpoints
- Circular dependency injection → IMPORTANT: runtime errors
- Missing `@ApiTags` / `@ApiResponse` → NIT: incomplete Swagger docs
- `@Injectable()` without scope specification → NIT: default singleton may not be appropriate

## Vue

- Direct state mutation (`this.items.push()`) → IMPORTANT: reactivity not triggered in Vue 2
- Missing `key` on `v-for` → IMPORTANT: rendering bugs
- Computed properties with side effects → IMPORTANT: unpredictable behavior
- Missing error handling in async operations → IMPORTANT: silent failures
- Watchers instead of computed → NIT: computed is more efficient for derived state

## Angular

- Missing `OnDestroy` for subscription cleanup → IMPORTANT: memory leaks
- Direct DOM manipulation → IMPORTANT: bypasses Angular change detection
- Missing `trackBy` in `*ngFor` → NIT: entire list re-rendered on change
- Synchronous HTTP calls → CRITICAL: blocks UI thread
- Missing lazy loading for feature modules → NIT: large initial bundle

## Rails

- Missing `strong_parameters` → CRITICAL: mass assignment vulnerability
- N+1 queries (`.each` without `.includes`) → IMPORTANT: use eager loading
- Missing CSRF token verification → CRITICAL
- `find_by_sql` with interpolation → CRITICAL: SQL injection
- Missing database indexes on foreign keys → IMPORTANT
- Callbacks for business logic → NIT: hard to trace, prefer service objects
- Missing background job for slow operations → IMPORTANT: request timeout risk
- `rescue => e` in controllers → IMPORTANT: too broad, catch specific errors

## .NET / ASP.NET

- `async void` event handlers → CRITICAL: unobserved exceptions
- Missing `[Authorize]` on controllers → CRITICAL: unprotected endpoints
- Missing `[ValidateAntiForgeryToken]` → CRITICAL: CSRF vulnerability
- `IDisposable` without `using` → IMPORTANT: resource leaks
- `ConfigureAwait(false)` missing in library code → IMPORTANT: deadlock risk
- Missing model validation (`ModelState.IsValid`) → IMPORTANT: unvalidated input
- EF Core lazy loading without awareness → IMPORTANT: N+1 queries
- Missing exception filters → IMPORTANT: raw errors to clients
