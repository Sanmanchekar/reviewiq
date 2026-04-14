# Fintech Review Skill

Specialized rules for financial services: payments, lending, insurance, and regulatory compliance. Financial code has zero tolerance for data integrity bugs and strict regulatory requirements.

## Universal Financial Commandments

1. **Money must NEVER be floating point.** Use integer cents/paise, `Decimal`, `BigDecimal`, or fixed-point. `0.1 + 0.2 != 0.3` in IEEE 754 → CRITICAL.
2. **Every financial transaction must be idempotent.** Network retries, webhook replays, queue redelivery — duplicate processing must be impossible. Use idempotency keys → CRITICAL.
3. **Every financial transaction must be atomic.** Debit without credit, partial transfers, orphaned records — use database transactions with proper isolation → CRITICAL.
4. **Every financial mutation must be auditable.** Who changed what, when, why. Append-only audit trail. Never delete or overwrite financial records → CRITICAL.
5. **Every amount must carry its currency.** No implicit currencies. `Money(100, "USD")` not bare `100`. Currency mismatch is silent data corruption → CRITICAL.
6. **Every calculation must define rounding rules.** ROUND_HALF_UP, ROUND_HALF_EVEN (banker's rounding), truncation — inconsistent rounding accumulates into real discrepancies → IMPORTANT.

---

## Payments & Payment Gateways

### Payment Processing

- **Double charging** → CRITICAL: submitting payment twice on retry/timeout. Require idempotency key on every payment request.
- **Amount tampering** → CRITICAL: amount passed from frontend/client. Always recompute on server from source-of-truth (cart, invoice, order).
- **Currency mismatch** → CRITICAL: charging USD when customer selected INR. Validate currency at every boundary.
- **Partial capture without tracking** → IMPORTANT: authorized $100, captured $60, no record of remaining $40 auth. Track auth/capture/void lifecycle.
- **Missing webhook verification** → CRITICAL: accepting payment gateway webhooks without signature verification. Attacker can fake payment confirmations.
- **Webhook replay vulnerability** → IMPORTANT: processing same webhook event twice. Store processed event IDs, check before processing.
- **Missing payment state machine** → IMPORTANT: payments have states (initiated → processing → success/failed/refunded). Invalid transitions must be rejected.
- **Refund exceeding original amount** → CRITICAL: sum of refunds must never exceed original payment. Check before processing.
- **Settlement reconciliation** → IMPORTANT: internal records must match gateway settlement reports. Build reconciliation from day one.

### Payment Gateway Integration

- **Hardcoded gateway credentials** → CRITICAL: API keys, merchant IDs in source code. Use secret manager.
- **Missing timeout on gateway calls** → CRITICAL: payment API call hangs → customer charged but system doesn't know. Set 30s max timeout.
- **Missing retry with idempotency** → IMPORTANT: gateway timeout doesn't mean payment failed. Retry with same idempotency key to get status.
- **PCI DSS violation — storing card data** → CRITICAL: never store full card numbers, CVV, or magnetic stripe data. Use tokenization.
- **Passing sensitive data in URL params** → CRITICAL: card numbers, account numbers in query strings. Logged everywhere.
- **Missing 3DS/SCA handling** → IMPORTANT: Strong Customer Authentication required in EU/UK. Handle redirect/challenge flows.
- **Test/Live mode confusion** → CRITICAL: test API keys in production, live keys in staging. Environment-specific configuration.
- **Missing callback/return URL validation** → IMPORTANT: redirect URLs must be whitelisted. Open redirect after payment = phishing.

### PCI DSS Compliance

- **Storing PAN (Primary Account Number)** → CRITICAL: only last 4 digits. Full PAN requires PCI Level 1.
- **CVV storage** → CRITICAL: NEVER store CVV/CVC. Not even encrypted. Not even temporarily.
- **Logging card data** → CRITICAL: card numbers in logs, error messages, analytics. Mask to last 4.
- **Card data in client-side storage** → CRITICAL: localStorage, sessionStorage, cookies. Use gateway-hosted fields or iframes.
- **Missing encryption in transit** → CRITICAL: TLS 1.2+ required for all card data transmission.
- **Missing access controls** → IMPORTANT: restrict who can access payment processing systems. Principle of least privilege.

---

## Lending & Loan Management

### Loan Origination

- **Interest calculation errors** → CRITICAL: simple vs compound, daily vs monthly accrual, day-count convention (ACT/360, ACT/365, 30/360). Must match loan agreement.
- **APR calculation** → CRITICAL: Annual Percentage Rate must include all fees per regulatory requirements (TILA in US, similar elsewhere). Incorrect APR = regulatory violation.
- **EMI calculation** → IMPORTANT: reducing balance method vs flat rate. Formula: `EMI = P * r * (1+r)^n / ((1+r)^n - 1)`. Use Decimal arithmetic.
- **Rounding in amortization** → IMPORTANT: rounding each EMI independently can cause last payment to differ. Adjust final payment to close balance exactly.
- **Prepayment handling** → IMPORTANT: partial/full prepayment must recalculate remaining schedule. Apply to principal first or per loan terms.
- **Late fee calculation** → IMPORTANT: grace period, fee caps, compounding rules. Must match regulatory limits.
- **Disbursement before all checks** → CRITICAL: funds released before KYC/credit check/agreement signed. Enforce workflow gates.

### Loan Servicing

- **Payment allocation** → IMPORTANT: payment received — apply to fees/interest/principal in correct order per agreement.
- **Due date calculation** → IMPORTANT: business day conventions (modified following, preceding), month-end handling (Feb 28/29).
- **Balance reconciliation** → CRITICAL: principal outstanding + interest accrued + fees must reconcile with all transactions.
- **Write-off handling** → IMPORTANT: partial/full write-off must be auditable, approved, and reflected in GL.
- **Collection workflow** → IMPORTANT: DPD (Days Past Due) triggers must be configurable, not hardcoded.
- **Moratorium/restructuring** → IMPORTANT: loan restructuring must create new amortization schedule with full audit trail.

### Credit Decision

- **Hardcoded credit rules** → IMPORTANT: credit score thresholds, DTI limits in code. Use configurable rules engine.
- **Missing adverse action notice** → IMPORTANT: rejected applications require reason codes per ECOA/FCRA (US) or equivalent regulation.
- **Discriminatory variables** → CRITICAL: race, gender, religion, zip code (as proxy) in credit models. Fair lending violation.
- **Bureau data handling** → CRITICAL: credit bureau data has strict usage/storage/retention requirements. Don't cache beyond allowed period.

---

## Insurance

### Policy Management

- **Premium calculation** → CRITICAL: must be actuarially sound and auditable. All factors, tables, and adjustments traced.
- **Coverage overlap/gap** → IMPORTANT: multiple policies must not double-cover or leave gaps. Check exclusions and sub-limits.
- **Endorsement handling** → IMPORTANT: mid-term changes must adjust premium pro-rata and maintain policy integrity.
- **Renewal logic** → IMPORTANT: auto-renewal terms, rate changes, coverage modifications. Must notify policyholder per regulation.
- **Cancellation/refund** → IMPORTANT: pro-rata vs short-rate refund calculation. Must match policy terms and regulatory requirements.

### Claims Processing

- **Claim amount exceeding coverage** → CRITICAL: payout must not exceed sum insured minus deductible minus previous claims.
- **Duplicate claim detection** → IMPORTANT: same incident, same policyholder. Requires deduplication logic.
- **Claim state machine** → IMPORTANT: filed → under_review → approved/denied → paid/appealed. Invalid transitions must be rejected.
- **Fraud indicators** → IMPORTANT: flag claims shortly after policy inception, claims near coverage limits, multiple claims in short period.
- **Reserve calculation** → IMPORTANT: claim reserves (IBNR, case reserves) affect financial statements. Must follow actuarial standards.

### Underwriting

- **Risk assessment** → IMPORTANT: all risk factors must be documented and version-controlled. Model changes need approval.
- **Exclusion handling** → CRITICAL: pre-existing conditions, war clauses, etc. Must be explicitly checked during claims.
- **Regulatory compliance** → IMPORTANT: rates must be filed with regulators where required. Using unfiled rates = violation.

---

## Personal Loans / Consumer Finance

- **Usury laws** → CRITICAL: interest rate must not exceed legal maximum per jurisdiction. Check state/country-specific caps.
- **Fee disclosure** → CRITICAL: all fees (origination, late, prepayment) must be disclosed per TILA/regulation. Hidden fees = regulatory violation.
- **Cooling-off period** → IMPORTANT: many jurisdictions require right-to-cancel within N days. Must be implemented.
- **Auto-debit authorization** → IMPORTANT: ECS/NACH/ACH mandates must have explicit customer consent. Missing consent = unauthorized debit.
- **Collection practices** → CRITICAL: contact hours, frequency limits, harassment prevention per FDCPA (US) or equivalent. Violations carry penalties.
- **Data minimization** → IMPORTANT: collect only data needed for lending decision. Store only what's legally required.
- **Right to access/delete** → IMPORTANT: GDPR, CCPA, DPDP Act rights. Customer must be able to access and request deletion of their data.

---

## Cross-Cutting Financial Concerns

### Reconciliation

- **Every financial system needs reconciliation** → IMPORTANT: internal ledger vs bank statement, internal vs gateway, internal vs GL. Discrepancies must trigger alerts.
- **End-of-day reconciliation** → IMPORTANT: daily trial balance, sum of debits = sum of credits.
- **Inter-system reconciliation** → IMPORTANT: if two systems track the same money (payment service and ledger), they must reconcile.
- **Reconciliation break handling** → IMPORTANT: what happens when numbers don't match? Escalation path, resolution workflow, audit trail.

### Regulatory & Compliance

- **KYC/AML checks** → CRITICAL: Know Your Customer and Anti-Money Laundering checks before account opening/transactions. Missing = regulatory violation.
- **Transaction monitoring** → IMPORTANT: suspicious transaction reporting (STR/SAR). Unusual patterns must be flagged.
- **Sanctions screening** → CRITICAL: screen against OFAC, EU, UN sanctions lists. Processing sanctioned transactions = severe penalties.
- **Data localization** → IMPORTANT: financial data must reside in jurisdiction-required location (India: RBI data localization, EU: GDPR).
- **Regulatory reporting** → IMPORTANT: periodic filings (call reports, RBI returns, etc.) must be automated and auditable.
- **Record retention** → IMPORTANT: financial records must be retained per regulatory requirements (typically 5-10 years). Immutable storage.

### Ledger & Accounting

- **Double-entry bookkeeping** → CRITICAL: every debit has a credit. Ledger must always balance. Single-entry = data integrity risk.
- **Immutable ledger entries** → CRITICAL: never modify/delete ledger entries. Corrections via reversing entries only.
- **Chart of accounts** → IMPORTANT: proper GL account structure. New transaction types need proper account mapping.
- **Multi-currency support** → IMPORTANT: if supporting multiple currencies — exchange rate at transaction time, unrealized gain/loss tracking, proper GL entries.
- **End-of-period close** → IMPORTANT: period close must prevent backdated entries. Soft close → hard close workflow.

### Date & Time in Finance

- **Business day calculation** → IMPORTANT: holiday calendars per market/jurisdiction. Settlement T+1, T+2 conventions.
- **Timezone handling** → CRITICAL: transaction timestamps in UTC, display in user timezone. Cut-off times are timezone-sensitive.
- **Day-count conventions** → IMPORTANT: ACT/360, ACT/365, 30/360, ACT/ACT. Must match contract terms. Wrong convention = wrong interest.
- **Leap year handling** → IMPORTANT: Feb 29 affects daily interest calculations. Must handle correctly.
- **Month-end conventions** → IMPORTANT: loan due on Jan 31 — what's the due date in Feb? Follow contract terms or modified following.

## Checklist

```
[ ] No floating-point money — all amounts use Decimal/integer arithmetic
[ ] All financial transactions are idempotent with idempotency keys
[ ] All financial mutations are atomic (database transactions)
[ ] Audit trail captures every financial event (append-only)
[ ] Every amount paired with currency code
[ ] Rounding rules defined and consistent
[ ] Payment gateway webhooks verified by signature
[ ] No PCI data stored (no full PAN, no CVV, ever)
[ ] Interest/EMI calculations use correct formulas and day-count conventions
[ ] Reconciliation exists for every financial integration
[ ] KYC/AML checks enforced before financial operations
[ ] Regulatory rate limits and fee caps enforced
[ ] Double-entry bookkeeping with balanced ledger
[ ] All timestamps in UTC with proper timezone handling
```
