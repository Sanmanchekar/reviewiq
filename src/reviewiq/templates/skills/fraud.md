# Fraud Detection & Prevention Review Skill

Review code for fraud vulnerabilities, detection gaps, and prevention controls.

## Transaction Fraud Prevention

- **Velocity checks missing** → CRITICAL: no limits on transaction count/amount per time window. Attacker can drain accounts.
  - Per-user: max N transactions per hour/day
  - Per-card/account: max amount per day/week
  - Per-device: max transactions per device fingerprint
  - Per-IP: max transactions per IP address
- **Amount anomaly detection** → IMPORTANT: transaction significantly larger than user's historical pattern. Flag for review.
- **Geographic anomaly** → IMPORTANT: transaction from country/region user has never transacted from. Especially same-day multi-country.
- **Time anomaly** → NIT: transaction at unusual hour for the user. Lower confidence signal, combine with others.
- **Device fingerprinting** → IMPORTANT: track device characteristics (browser, OS, screen, timezone, plugins). New device + high-value = flag.
- **Missing transaction risk scoring** → IMPORTANT: each transaction should have a risk score combining multiple signals. Don't rely on single rules.

## Account Fraud

- **Account takeover (ATO)** → CRITICAL: password change + email change + immediate withdrawal = ATO pattern. Add cooling period after credential changes.
- **SIM swap detection** → IMPORTANT: phone number recently ported + OTP-based auth = high risk. Check with telco APIs or add delay.
- **Synthetic identity** → IMPORTANT: fabricated identities using mix of real/fake data. Cross-check PAN-name-DOB consistency.
- **Multiple accounts per device/IP** → IMPORTANT: same device fingerprint creating multiple accounts. Flag for review.
- **Email pattern analysis** → NIT: disposable email domains, recently created email addresses. Lower confidence signal.
- **Referral abuse** → IMPORTANT: self-referral rings, fake referrals for bonus. Track referral chains, limit depth.

## Payment Fraud

- **Card testing** → CRITICAL: many small transactions to test stolen card numbers. Velocity check on declined transactions.
- **Chargeback pattern** → IMPORTANT: users with >1% chargeback rate. Flag for review, potentially block.
- **Friendly fraud** → IMPORTANT: legitimate customer claims they didn't make the purchase. Keep delivery proof, device fingerprint.
- **Refund abuse** → IMPORTANT: excessive refund requests, refund + keep pattern. Track refund rate per user.
- **Promo/coupon abuse** → IMPORTANT: same user using multiple promo codes via different accounts. Link via device/IP/payment method.
- **BIN attack** → CRITICAL: sequential card numbers being tested. Rate limit by BIN range.

## Lending Fraud

- **Income inflation** → IMPORTANT: stated income significantly higher than bureau/bank statement data. Cross-verify.
- **Document forgery** → IMPORTANT: tampered bank statements, salary slips, ITR. Use verified data sources (AA, DigiLocker, ITR-V).
- **Stacking** → CRITICAL: borrower taking multiple loans simultaneously from different lenders. Check bureau in real-time, not cached.
- **First-party fraud** → IMPORTANT: borrower with intent to default. Look for: new credit history, maximum borrowing, sudden address change.
- **Collusion** → IMPORTANT: DSA/agent colluding with borrower. Monitor DSA-level default rates, unusual patterns.
- **Bust-out** → CRITICAL: build good repayment history, then take maximum credit and disappear. Monitor sudden increase in credit utilization.

## Technical Controls

### Rule Engine

- **Hardcoded rules** → IMPORTANT: fraud rules in application code. Use configurable rules engine for fast updates without deployment.
- **Rule ordering** → IMPORTANT: rules should be evaluated in priority order. Fast-fail on high-confidence rules.
- **Rule versioning** → IMPORTANT: track which version of rules evaluated each transaction. Needed for disputes/audits.
- **False positive handling** → IMPORTANT: manual review queue for flagged transactions. SLA for review. Customer notification.
- **Rule bypass** → CRITICAL: no mechanism to bypass fraud rules without audit trail. Admin overrides must be logged.

### ML Model Review

- **Model bias** → CRITICAL: fraud models must not discriminate by protected attributes. Test for disparate impact.
- **Feature leakage** → IMPORTANT: using future data or target-correlated features in training. Invalidates model.
- **Model staleness** → IMPORTANT: fraud patterns evolve. Model performance must be monitored continuously. Retrain on schedule.
- **Explainability** → IMPORTANT: for disputes and regulatory queries, must be able to explain why a transaction was flagged.
- **Fallback on model failure** → IMPORTANT: if ML service is down, fall back to rule-based detection. Don't let all transactions through.
- **Score threshold** → IMPORTANT: define clear thresholds — auto-approve, manual review, auto-decline. Track and tune.
- **Champion-challenger** → NIT: run new model alongside current in shadow mode before switching. Compare performance.

### Data Pipeline

- **Real-time vs batch** → IMPORTANT: fraud detection must be real-time for transactions. Batch is only for pattern analysis.
- **Feature freshness** → IMPORTANT: features like "transactions in last 24h" must be computed in real-time, not from yesterday's batch.
- **Label accuracy** → IMPORTANT: confirmed fraud labels (not just chargebacks) needed for model training. Track investigation outcomes.
- **Data retention for investigation** → IMPORTANT: retain transaction context (device, IP, location, session) for 180+ days for investigation.

## Alert & Response

- **Alert fatigue** → IMPORTANT: too many alerts desensitize the team. Tune thresholds to keep alert-to-fraud ratio actionable.
- **Escalation path** → IMPORTANT: clear escalation: automated block → analyst review → investigation → law enforcement.
- **Customer communication** → IMPORTANT: notify customer of suspected fraud immediately. Don't just block silently.
- **Account lockdown** → IMPORTANT: ability to instantly freeze account/card on fraud detection. Must be automated for high-confidence signals.
- **Recovery process** → IMPORTANT: workflow for recovering funds, reversing transactions, restoring account access after false positive.

## Checklist

```
[ ] Velocity checks implemented (per-user, per-device, per-IP)
[ ] Device fingerprinting active on sensitive operations
[ ] Cooling period after credential changes (password, email, phone)
[ ] Transaction risk scoring combining multiple signals
[ ] Configurable rules engine (not hardcoded fraud rules)
[ ] ML model monitored for performance degradation and bias
[ ] Real-time detection for transactions (not batch-only)
[ ] Manual review queue with SLA for flagged transactions
[ ] Admin fraud rule overrides logged with audit trail
[ ] Account lockdown capability (instant freeze)
[ ] Customer notification on suspected fraud
[ ] Bureau pull at origination is real-time (not cached for stacking detection)
```
