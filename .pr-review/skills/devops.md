# DevOps Review Skill

Review infrastructure-as-code, deployment configs, and CI/CD pipelines.

## Docker

**Dockerfile Anti-Patterns → Findings**:
- `FROM latest` tag → IMPORTANT: non-reproducible builds, pin specific version
- Running as root (no `USER` instruction) → CRITICAL: container escape risk
- `COPY . .` before installing deps → NIT: busts cache on every code change. Copy lockfile first, install, then copy code.
- Missing `.dockerignore` → IMPORTANT: sending unnecessary context (node_modules, .git, secrets)
- `RUN apt-get install` without `--no-install-recommends` → NIT: bloated image
- Missing `apt-get clean && rm -rf /var/lib/apt/lists/*` → NIT: layer bloat
- Multiple `RUN` statements that should be combined → NIT: excessive layers
- `ADD` instead of `COPY` → NIT: COPY is more explicit (ADD can untar/download)
- Secrets in build args or environment → CRITICAL: visible in image history
- `EXPOSE` without matching app config → NIT: misleading documentation
- Missing health check (`HEALTHCHECK`) → IMPORTANT: orchestrator can't detect unhealthy containers
- Writable root filesystem → IMPORTANT: use `--read-only` in production
- Unnecessary capabilities → IMPORTANT: drop all, add only needed ones

**Docker Compose**:
- Missing restart policy → IMPORTANT: containers don't recover from crashes
- Hardcoded ports that may conflict → NIT: use env vars or dynamic ports
- Missing resource limits (memory, CPU) → IMPORTANT: single container can starve host
- `privileged: true` → CRITICAL: full host access
- Missing network isolation → IMPORTANT: all services on default network
- Volumes mounting sensitive host paths → CRITICAL: data exposure risk
- Missing dependency health checks (`depends_on` with `condition`) → NIT: startup race conditions

**Image Security**:
- Base image from untrusted registry → IMPORTANT: supply chain risk
- Known CVEs in base image → CRITICAL: scan with Trivy/Snyk
- Multi-stage builds not used → NIT: build tools in production image
- Debug tools in production image → IMPORTANT: attack surface

## Kubernetes

**Workload Configuration**:
- Missing resource requests/limits → CRITICAL: unbounded resource use, eviction risk
- `replicas: 1` for production workloads → IMPORTANT: single point of failure
- Missing liveness/readiness probes → CRITICAL: K8s can't detect/recover from failures
- `imagePullPolicy: Always` without image pinning → IMPORTANT: non-reproducible deployments
- `latest` tag in pod spec → CRITICAL: non-deterministic deployments
- Missing `PodDisruptionBudget` → IMPORTANT: all pods can be evicted simultaneously
- `hostNetwork: true` → CRITICAL: bypasses network policies
- `hostPID: true` or `hostIPC: true` → CRITICAL: process namespace leakage
- Missing anti-affinity rules → NIT: all replicas on same node = single point of failure
- Missing `topologySpreadConstraints` → NIT: uneven distribution across zones

**Security**:
- Running as root (`runAsNonRoot: false` or missing) → CRITICAL
- `privileged: true` in securityContext → CRITICAL: full host access
- Missing `readOnlyRootFilesystem` → IMPORTANT: writable container filesystem
- Missing `seccompProfile` → NIT: no syscall filtering
- `allowPrivilegeEscalation: true` → IMPORTANT: container can gain more privileges
- Missing `NetworkPolicy` → IMPORTANT: all pods can communicate with all pods
- Secrets in plain ConfigMaps → CRITICAL: use Secrets (still base64, use sealed-secrets/vault for real encryption)
- Missing RBAC (ClusterRoleBinding to default SA) → CRITICAL: over-permissioned workloads
- `automountServiceAccountToken: true` without need → NIT: unnecessary credential exposure

**Networking**:
- Missing `Service` for pod-to-pod communication → NIT: direct pod IP is ephemeral
- `type: NodePort` in production → NIT: use LoadBalancer or Ingress
- Missing Ingress TLS configuration → CRITICAL: unencrypted external traffic
- Missing rate limiting annotations on Ingress → IMPORTANT: DoS risk
- Missing connection draining (`terminationGracePeriodSeconds`) → IMPORTANT: in-flight requests dropped

**Storage**:
- `emptyDir` for persistent data → CRITICAL: data lost on pod restart
- Missing `storageClassName` → NIT: uses default, may not be appropriate
- Missing backup strategy for PersistentVolumes → IMPORTANT: data loss risk
- `ReadWriteMany` when `ReadWriteOnce` suffices → NIT: unnecessary complexity

## Helm

**Chart Quality**:
- Hardcoded values in templates → IMPORTANT: use `{{ .Values.xxx }}` for configurability
- Missing `values.yaml` defaults → IMPORTANT: chart fails without overrides
- Missing `NOTES.txt` → NIT: no post-install instructions
- Missing `Chart.yaml` version bump → IMPORTANT: same version = cache hits on old chart
- Templates without `{{ include "chart.fullname" . }}` naming → NIT: name collisions in cluster
- Missing `{{ .Release.Namespace }}` in cross-resource references → IMPORTANT: wrong namespace
- Hardcoded `namespace` in templates → IMPORTANT: prevents deployment to different namespaces

**values.yaml**:
- Missing resource requests/limits defaults → CRITICAL: see Kubernetes section
- Missing replica count → IMPORTANT: defaults to 1
- Secrets in values.yaml → CRITICAL: committed to git
- Missing image.tag default → NIT: chart unusable without override
- Missing ingress.enabled toggle → NIT: can't disable ingress
- Missing nodeSelector/tolerations/affinity defaults → NIT: scheduling gaps

**Hooks & Tests**:
- Missing `helm test` definitions → NIT: no automated validation
- Pre-install hooks without `--wait` → IMPORTANT: race condition with main resources
- Missing hook deletion policy (`hook-delete-policy`) → NIT: orphaned hook resources
- Missing `pre-upgrade` hook for migrations → IMPORTANT: schema not updated before new code

## Terraform

- Missing state backend (local state) → CRITICAL: state lost, can't collaborate
- Missing state locking → CRITICAL: concurrent applies corrupt state
- Hardcoded credentials → CRITICAL: use provider auth or environment variables
- Missing `terraform.tfvars` from `.gitignore` → IMPORTANT: secrets in repo
- `count` instead of `for_each` for conditional resources → NIT: index-based = order-dependent
- Missing `lifecycle { prevent_destroy }` on stateful resources → IMPORTANT: accidental deletion
- `*` in security group rules → CRITICAL: open to world
- Missing tags on resources → NIT: unattributed cloud costs
- Modules without version pinning → IMPORTANT: breaking changes on next apply
- Missing output values → NIT: consumers can't reference without hardcoding

## CI/CD

**GitHub Actions**:
- Secrets in workflow files → CRITICAL: use `${{ secrets.XXX }}`
- `pull_request_target` with checkout of PR code → CRITICAL: code execution from forks
- Missing `permissions` block → IMPORTANT: default permissions too broad
- `actions/checkout` without `persist-credentials: false` → NIT: token persists
- `npm install` without lockfile → IMPORTANT: non-reproducible builds
- Missing caching for dependencies → NIT: slow builds
- `continue-on-error: true` on critical steps → IMPORTANT: failures silently pass
- Self-hosted runners without security hardening → CRITICAL: code execution on your infra

**General CI/CD**:
- Missing build reproducibility (floating deps, latest tags) → IMPORTANT
- Missing artifact signing/verification → IMPORTANT: supply chain risk
- Deployment without approval gate → IMPORTANT: for production environments
- Missing rollback mechanism → IMPORTANT: failed deploy needs manual intervention
- Parallel jobs without dependency management → NIT: flaky pipeline
- Missing notification on failure → NIT: failures go unnoticed
- Test stage skippable → IMPORTANT: can deploy untested code

## Ansible

- Hardcoded passwords/keys → CRITICAL: use Ansible Vault
- Missing `become: yes` for privileged tasks → NIT: task fails silently
- Missing handlers for service restarts → IMPORTANT: config change without restart
- Missing idempotency (command/shell without `creates`/`when`) → IMPORTANT: not safe to re-run
- Missing `ansible-lint` in CI → NIT: style and best practice violations

## Checklist

```
[ ] No secrets in any config files or manifests
[ ] All containers run as non-root with minimal capabilities
[ ] Resource limits set on all workloads
[ ] Health checks configured (liveness + readiness)
[ ] TLS/HTTPS enforced on all external endpoints
[ ] Network policies restrict pod-to-pod communication
[ ] Images pinned to specific versions (no :latest)
[ ] State stored in proper persistent storage (not emptyDir)
[ ] CI/CD pipeline includes security scanning
[ ] Rollback mechanism documented and tested
[ ] Infrastructure changes go through code review
[ ] Terraform state is remote with locking
```
