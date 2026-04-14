# Credit Bureau Integration Review Skill

Rules for integrating with credit bureaus (CIBIL/TransUnion, Experian, Equifax, CRIF) and handling credit data responsibly.

## Bureau API Integration

- **API credentials in code** → CRITICAL: bureau credentials are highly sensitive. Secret manager only.
- **Request timeout** → IMPORTANT: bureau APIs can be slow (5-15s). Set appropriate timeouts, don't block user flow.
- **Fallback strategy** → IMPORTANT: if primary bureau is down, fall back to secondary. Don't block origination entirely.
- **Request deduplication** → IMPORTANT: multiple bureau pulls for same customer in short window = unnecessary hard inquiry. Cache recent pulls.
- **Response validation** → IMPORTANT: validate bureau response schema. Handle partial responses, missing segments gracefully.
- **Error code handling** → IMPORTANT: distinguish between "no record found" (thin file) and "system error." Different business treatment.
- **Bureau selection logic** → NIT: choose bureau based on coverage for customer segment/geography.

## Data Handling

- **Hard vs Soft inquiry** → CRITICAL: hard inquiries affect credit score. Only pull hard inquiry with customer consent for actual credit decision. Use soft pull for pre-qualification.
- **Consent before pull** → CRITICAL: explicit customer consent required before bureau inquiry. Store consent proof with timestamp.
- **Data retention** → CRITICAL: bureau data has strict retention limits (typically 90 days for decisioning). Auto-purge after allowed period.
- **Data sharing restrictions** → CRITICAL: bureau data cannot be shared with third parties. Cannot be used for marketing.
- **Score caching** → IMPORTANT: cache score within allowed window to avoid repeated pulls. Invalidate on new credit events.
- **PII in bureau data** → CRITICAL: name, DOB, PAN, address from bureau response — treat as PII. Encrypt at rest, mask in UI.

## Credit Score Processing

- **Score interpretation** → IMPORTANT: different bureaus have different score ranges. CIBIL: 300-900, Experian: 300-900, Equifax: 1-999. Normalize before comparison.
- **Score version** → IMPORTANT: bureau score models have versions. Track which version was used for the decision. Model changes affect score distribution.
- **No-hit / thin file** → IMPORTANT: customer with no credit history. Don't reject outright — use alternative data or manual review.
- **Multiple scores** → NIT: customer can have different scores across bureaus. Define which bureau is primary for decisioning.
- **Score refresh** → IMPORTANT: scores update monthly. Don't use stale scores for new decisions.

## Credit Report Parsing

- **Account status codes** → IMPORTANT: current, overdue, written-off, settled, closed. Map correctly to internal risk categories.
- **DPD (Days Past Due)** → IMPORTANT: parse DPD history correctly. Look for patterns (improving vs deteriorating).
- **Inquiry section** → IMPORTANT: count recent hard inquiries. High inquiry count = risk signal (credit-hungry behavior).
- **Address/employment history** → NIT: stability signals. Frequent changes may indicate instability.
- **Suit-filed / willful defaulter** → CRITICAL: legal flags in bureau report. Must be caught and flagged in underwriting.
- **Guarantor obligations** → IMPORTANT: existing guarantor obligations affect repayment capacity. Include in DTI calculation.
- **Joint accounts** → IMPORTANT: joint account liabilities must be included at appropriate percentage in debt calculation.

## Reporting to Bureau

- **Monthly reporting** → CRITICAL: report all account data monthly to bureaus. Missing reports = stale bureau data for your customers.
- **Accurate status reporting** → CRITICAL: report correct DPD, balance, status. Inaccurate reporting = regulatory action + customer disputes.
- **Account closure reporting** → IMPORTANT: report account closure/settlement promptly. Customers shouldn't have stale open accounts.
- **Dispute handling** → IMPORTANT: when customer disputes bureau data, investigate within 30 days. Correct and re-report if error found.
- **Data format compliance** → IMPORTANT: follow bureau-specific data submission format (CIBIL: TUEF, Experian: specific format). Validation before submission.
- **Negative reporting responsibility** → IMPORTANT: before reporting negative (NPA/default), verify accuracy. False negative reporting = legal liability.

## Regulatory Requirements

- **CICRA (Credit Information Companies Regulation Act)** → IMPORTANT: governs bureau operations in India. Compliance mandatory for credit institutions.
- **Fair lending** → CRITICAL: credit decisions must not discriminate. Prohibited factors: religion, caste, gender, marital status, ethnicity.
- **Adverse action notice** → IMPORTANT: if credit denied based on bureau data, must inform customer with bureau name and contact.
- **Right to dispute** → IMPORTANT: customer has right to dispute bureau information. Must facilitate dispute process.
- **Annual free report** → NIT: customers entitled to one free credit report per year from each bureau.

## Checklist

```
[ ] Bureau API credentials in secret manager (not code)
[ ] Explicit customer consent obtained before bureau pull
[ ] Hard inquiry only for actual credit decision, soft pull for pre-qualification
[ ] Bureau data retention within allowed limits (auto-purge)
[ ] Bureau data not shared with third parties or used for marketing
[ ] Credit score version tracked for each decision
[ ] Monthly reporting to bureaus with accurate data
[ ] Dispute handling process implemented (30-day resolution)
[ ] Adverse action notice sent when credit denied on bureau data
[ ] Suit-filed / willful defaulter flags caught in underwriting
```
