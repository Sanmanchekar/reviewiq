# India Regulatory & Payment Rails Review Skill

India-specific financial regulations, payment infrastructure, and compliance requirements.

## RBI Digital Lending Guidelines

- **Direct disbursement to borrower** → CRITICAL: loan must be disbursed directly to borrower's bank account, not through third-party pool. RBI 2022 guidelines.
- **FLDG (First Loss Default Guarantee) cap** → CRITICAL: FLDG cannot exceed 5% of the loan portfolio. Verify cap enforcement in code.
- **LSP disclosure** → IMPORTANT: Lending Service Provider (LSP) identity must be disclosed upfront. No white-labeling without disclosure.
- **KFS (Key Fact Statement)** → CRITICAL: must be presented before loan agreement. Contains APR, all fees, total repayment amount, cooling-off period.
- **Cooling-off period** → CRITICAL: borrower must have right to exit without penalty within look-up period. Enforce in code.
- **Grievance redressal** → IMPORTANT: must have nodal officer details, escalation matrix, 30-day resolution timeline.
- **Data access restrictions** → CRITICAL: apps cannot access contacts, photos, media, call logs. Only camera (for KYC), location (for geo-tagging), storage (for KYC docs).
- **Interest rate cap** → IMPORTANT: RBI doesn't set explicit caps for NBFCs but requires fair practice code. Usurious rates flagged by SRO.
- **Penal charges** → IMPORTANT: RBI 2023 circular — penal charges must not be added to principal for interest calculation. Track separately.

## NBFC Compliance

- **Capital adequacy (CRAR)** → IMPORTANT: maintain minimum 15% Capital to Risk-weighted Assets Ratio. Report calculations must be accurate.
- **Asset classification** → CRITICAL: NPA recognition on day 90 (or 120 for microfinance). Automated DPD tracking must be exact.
- **Provisioning norms** → IMPORTANT: standard (0.40%), sub-standard (10%), doubtful (20-100%), loss (100%). Auto-provision on classification change.
- **Priority Sector Lending** → IMPORTANT: if applicable, PSL target tracking and reporting to RBI.
- **CERSAI filing** → IMPORTANT: Central Registry filing for secured loans. Must file within 30 days of creation/modification/satisfaction.
- **SMA reporting** → IMPORTANT: Special Mention Accounts (SMA-0, SMA-1, SMA-2) reporting based on overdue days.
- **Co-lending model** → IMPORTANT: if co-lending with bank, 80:20 ratio, minimum 20% on NBFC books, joint underwriting.
- **RBI returns** → IMPORTANT: monthly/quarterly returns (ALM, NPA, sectoral exposure). Automated report generation must match manual format.

## UPI (Unified Payments Interface)

- **NPCI guidelines compliance** → CRITICAL: follow NPCI circular specifications for PSP/TPAP integration.
- **Transaction limits** → IMPORTANT: per-transaction limit (Rs 1L default, Rs 5L for specific categories). Enforce at application layer.
- **Mandate/AutoPay** → IMPORTANT: UPI AutoPay limit Rs 1L for recurring. Pre-debit notification 24hrs before. Customer can revoke anytime.
- **VPA validation** → IMPORTANT: validate VPA format and verify through collect request before crediting.
- **Callback handling** → CRITICAL: handle all UPI callback statuses: SUCCESS, FAILURE, PENDING, DEEMED. Don't assume non-response = failure.
- **PENDING status** → CRITICAL: UPI transaction can be PENDING for up to 48 hours. Do NOT reverse or retry. Poll for status.
- **Refund via UPI** → IMPORTANT: refunds go through original PSP. Track original transaction reference.
- **UPI Lite** → NIT: small-value offline transactions, separate balance management.
- **Dispute resolution** → IMPORTANT: NPCI dispute mechanism with defined TATs. Auto-raise disputes for failed-but-debited scenarios.

## IMPS/NEFT/RTGS

- **NEFT batch windows** → IMPORTANT: NEFT settles in half-hourly batches (24x7 since Dec 2019). Don't assume instant settlement.
- **RTGS for large values** → IMPORTANT: minimum Rs 2L. Use for time-critical large transfers. Available 24x7.
- **IMPS availability** → IMPORTANT: 24x7 instant transfer up to Rs 5L. Verify beneficiary before transfer.
- **IFSC validation** → IMPORTANT: validate IFSC code format and existence before initiating transfer. IFSC codes get deprecated.
- **Beneficiary management** → IMPORTANT: cooling period for new beneficiaries, per-beneficiary limits, activation verification.
- **Return/reject handling** → IMPORTANT: NEFT returns within T+1. Handle credit-back to sender automatically.
- **Reconciliation** → CRITICAL: bank statement reconciliation for NEFT/RTGS/IMPS. Match UTR numbers.

## NACH/ECS (Auto-Debit)

- **Mandate registration** → CRITICAL: explicit customer authorization required. E-mandate via Aadhaar OTP/net banking/debit card.
- **Pre-debit notification** → CRITICAL: notify customer at least 24 hours before auto-debit with amount and date.
- **Mandate modification** → IMPORTANT: amount/frequency changes require customer re-authorization.
- **Insufficient funds handling** → IMPORTANT: handle NACH reject codes properly. Don't retry immediately — bank may charge.
- **Mandate cancellation** → IMPORTANT: customer can cancel anytime. Must stop future debits immediately.
- **Maximum amount in mandate** → IMPORTANT: actual debit must not exceed mandate maximum amount.
- **UMRN tracking** → IMPORTANT: Unique Mandate Reference Number must be stored and used for all operations.

## eKYC (Electronic KYC)

- **Aadhaar eKYC** → CRITICAL: only through UIDAI-authorized KUA/ASA. Direct Aadhaar storage prohibited for private entities.
- **Aadhaar number masking** → CRITICAL: display only last 4 digits. Never store full Aadhaar in regular databases. Use VID (Virtual ID) where possible.
- **CKYC (Central KYC)** → IMPORTANT: upload/download customer records to CKYC registry. 14-digit CKYC number tracking.
- **Video KYC** → IMPORTANT: RBI guidelines — live video, geo-tagging, AI-based liveliness check, randomized questions, recording retained.
- **PAN verification** → IMPORTANT: NSDL/UTIITSL PAN verification before onboarding. Name matching with tolerance.
- **Digilocker integration** → NIT: for pulling verified documents. Use DigiLocker APIs, not screenshots.
- **Re-KYC** → IMPORTANT: periodic KYC update required (2/8/10 years based on risk). Track and trigger re-KYC workflows.
- **Consent management** → CRITICAL: explicit consent before collecting biometric/Aadhaar data. Consent must be auditable.

## e-Sign

- **Aadhaar e-Sign** → IMPORTANT: through certified ESP (e-Sign Provider). OTP-based consent for each document.
- **DSC (Digital Signature Certificate)** → IMPORTANT: for high-value/regulated documents. Class 2/3 certificates.
- **Audit trail** → CRITICAL: e-sign event log — who signed, when, IP, device, document hash. Tamper-evident.
- **Document integrity** → CRITICAL: signed document must be tamper-proof. Use PDF digital signatures with hash verification.
- **Multi-party signing** → IMPORTANT: handle signing order, partial completion, expiry of unsigned documents.

## Account Aggregator (AA)

- **Consent architecture** → CRITICAL: FIP → AA → FIU flow. Data can only be fetched with explicit customer consent via AA.
- **Consent artefact** → IMPORTANT: includes purpose, data types, date range, frequency, expiry. Must be stored and honored.
- **Data fetch** → IMPORTANT: only fetch data types and date ranges specified in consent. No over-fetching.
- **Consent revocation** → IMPORTANT: customer can revoke anytime. Must stop data access immediately and delete fetched data per terms.
- **FI Types** → IMPORTANT: deposit, insurance, mutual fund, pension, etc. Handle each type's data schema.

## GST Compliance (for B2B fintech)

- **GSTIN validation** → IMPORTANT: validate format and check active status via GST API.
- **E-invoicing** → IMPORTANT: mandatory for turnover >5Cr. Generate IRN via NIC portal. QR code on invoice.
- **E-way bill** → IMPORTANT: for movement of goods >Rs 50K. Integrate with NIC e-way bill system.
- **Input Tax Credit** → IMPORTANT: ITC matching between GSTR-2A/2B. Auto-reconciliation logic.
- **TDS/TCS on payments** → IMPORTANT: applicable rates, threshold tracking, automatic deduction, certificate generation.

## Checklist

```
[ ] RBI digital lending guidelines followed (direct disbursement, KFS, cooling-off)
[ ] FLDG cap enforced at 5%
[ ] App permissions restricted (no contacts/photos/call logs access)
[ ] Penal charges not added to principal
[ ] NPA classification automated at 90 DPD
[ ] UPI PENDING status handled correctly (no premature reversal)
[ ] NACH pre-debit notification sent 24hrs before
[ ] Aadhaar number masked (last 4 only), no full storage
[ ] eKYC consent explicitly obtained and auditable
[ ] Account Aggregator consent artefacts stored and honored
[ ] CERSAI filing within 30 days
[ ] All regulatory returns data accurate and automated
```
