# Security Review Skill

Apply these checks to every PR. Flag violations as CRITICAL or IMPORTANT.

## Injection

- **SQL Injection**: String concatenation in queries → parameterized queries. Check ORM raw queries too.
- **Command Injection**: User input in `exec`, `system`, `popen`, subprocess calls → whitelist or escape.
- **XSS**: Unescaped user input rendered in HTML → context-aware escaping or templating engine auto-escape.
- **SSRF**: User-controlled URLs in server-side requests → allowlist domains, block private IPs (10.x, 172.16-31.x, 192.168.x, 169.254.x, localhost).
- **Path Traversal**: User input in file paths → normalize, validate against base directory, reject `..`.
- **Template Injection**: User input in template engines → never pass user data as template source.
- **LDAP/XML/NoSQL Injection**: Same principle as SQL — parameterize, never concatenate.
- **Log Injection**: User data in logs → sanitize newlines and control characters to prevent log forging.

## Authentication & Authorization

- **Broken Authentication**: Weak password policies, missing rate limiting on login, session tokens in URLs.
- **Missing Authorization Checks**: Endpoint exists but doesn't verify user has permission. Check every state-changing endpoint.
- **IDOR (Insecure Direct Object References)**: User can access resources by changing IDs. Verify ownership at the data layer.
- **Privilege Escalation**: User can promote their own role. Admin endpoints accessible without admin check.
- **Session Management**: Session fixation, missing regeneration after login, no expiration, no secure/httponly flags.
- **JWT Pitfalls**: Algorithm confusion (none/HS256 vs RS256), missing expiry, missing audience validation, secret in code.
- **API Key Management**: Keys in source code, no rotation policy, overly permissive scopes.

## Cryptography

- **Weak Algorithms**: MD5/SHA1 for passwords → bcrypt/scrypt/argon2. DES/3DES → AES-256-GCM. RSA-1024 → RSA-2048+.
- **Hardcoded Secrets**: API keys, passwords, private keys in source → secret manager, env vars.
- **Insecure Random**: `Math.random()`, `rand()`, `random.random()` for security-sensitive values → crypto-secure PRNG.
- **Missing Encryption**: PII/sensitive data stored or transmitted in plaintext → encrypt at rest and in transit.
- **Certificate Validation**: Disabled TLS verification → never in production, even for internal services.

## Data Protection

- **PII Exposure**: Personal data in logs, error messages, URLs, analytics. Mask or redact.
- **Data Retention**: Storing data forever without deletion policy → define retention, implement cleanup.
- **Sensitive Data in Errors**: Stack traces, internal IPs, database names, query details in user-facing errors.
- **Insecure Deserialization**: Deserializing untrusted data (pickle, Java serialization, YAML load) → use safe loaders, validate types.

## Infrastructure Security

- **Overpermissive IAM**: Wildcard permissions, admin roles for services → principle of least privilege.
- **Public S3/Storage Buckets**: Default public access → block public access, use signed URLs.
- **Exposed Debug Endpoints**: Debug mode in production, profiling endpoints, health checks with sensitive info.
- **Missing Security Headers**: HSTS, CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy.
- **CORS Misconfiguration**: Wildcard origins, credentialed requests with wildcard → specific origins only.
- **Container Security**: Running as root, unnecessary capabilities, privileged mode, writable root filesystem.

## Dependency Security

- **Known Vulnerabilities**: Check versions against CVE databases. Flag outdated packages with known vulns.
- **Typosquatting**: Verify package names are correct (not misspelled variants).
- **License Compliance**: GPL in proprietary code, incompatible license combinations.
- **Pinned Versions**: Floating versions in production → pin to specific versions with lockfile.

## Checklist (CRITICAL if violated)

```
[ ] No secrets in code, config, or commit messages
[ ] All user input validated and sanitized
[ ] All queries parameterized (no string concatenation)
[ ] All endpoints have authorization checks
[ ] All sensitive data encrypted at rest and in transit
[ ] No debug/test code in production paths
[ ] All dependencies on known-safe versions
[ ] Error messages don't leak internal details
[ ] HTTPS enforced, certificates validated
[ ] Rate limiting on authentication endpoints
```
