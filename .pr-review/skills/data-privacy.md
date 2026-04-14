# Data Privacy Review Skill

Review for compliance with DPDP Act (India), GDPR (EU), CCPA (US), and general data protection principles.

## Universal Data Protection Principles

1. **Purpose limitation** → CRITICAL: collect data only for stated purpose. Don't repurpose without fresh consent.
2. **Data minimization** → IMPORTANT: collect only what's necessary. Don't ask for father's name if not needed.
3. **Storage limitation** → IMPORTANT: define retention period for every data type. Auto-delete after expiry.
4. **Accuracy** → IMPORTANT: provide mechanism for users to update their data. Act on corrections promptly.
5. **Integrity & confidentiality** → CRITICAL: encrypt PII at rest and in transit. Access controls on all personal data.
6. **Accountability** → IMPORTANT: document what data you collect, why, how long, who accesses it. Be audit-ready.

## DPDP Act 2023 (India)

### Data Principal Rights

- **Right to access** → IMPORTANT: customer can request summary of their personal data and processing activities.
- **Right to correction** → IMPORTANT: customer can request correction of inaccurate data. Must act without delay.
- **Right to erasure** → IMPORTANT: customer can request deletion when data is no longer needed for original purpose.
- **Right to grievance redressal** → IMPORTANT: must have Data Protection Officer or contact point. Respond within prescribed timeline.
- **Right to nominate** → NIT: customer can nominate someone to exercise rights in case of death/incapacity.

### Data Fiduciary Obligations

- **Consent notice** → CRITICAL: clear, plain-language notice describing: what data, why, how long, who you share with. Before or at time of collection.
- **Consent mechanism** → CRITICAL: explicit opt-in (not pre-checked boxes). Must be as easy to withdraw as to give.
- **Consent manager** → IMPORTANT: if using consent manager, must be registered with DPA. Track consent via consent manager platform.
- **Children's data** → CRITICAL: processing children's data (<18 in India) requires verifiable parental consent. No behavioral tracking/targeted ads.
- **Significant data fiduciary** → IMPORTANT: if classified as significant (by government notification), must appoint DPO based in India, conduct DPIA.
- **Cross-border transfer** → IMPORTANT: transfer outside India only to countries not restricted by government notification.
- **Breach notification** → CRITICAL: notify Data Protection Board and affected individuals of breach. No specified timeline yet — implement within 72 hours as best practice.
- **Data retention** → IMPORTANT: delete personal data when consent is withdrawn or purpose is fulfilled. Don't retain "just in case."

## GDPR (EU/UK)

### Lawful Basis

- **Consent** → CRITICAL: freely given, specific, informed, unambiguous. Pre-ticked boxes invalid. Must record proof.
- **Contract** → IMPORTANT: processing necessary for contract performance. Document which data is needed for which contract obligation.
- **Legitimate interest** → IMPORTANT: requires balancing test (your interest vs data subject's rights). Document the assessment.
- **Legal obligation** → IMPORTANT: processing required by law (e.g., tax records, AML). Cite the specific law.

### Data Subject Rights

- **Right to access (SAR)** → IMPORTANT: respond within 30 days. Provide all personal data in portable format.
- **Right to rectification** → IMPORTANT: correct inaccurate data without undue delay.
- **Right to erasure (RTBF)** → IMPORTANT: delete when no longer necessary, consent withdrawn, or unlawful processing. Exceptions for legal obligations.
- **Right to portability** → IMPORTANT: provide data in machine-readable format. API endpoint for data export.
- **Right to object** → IMPORTANT: stop processing for direct marketing immediately. Other objections: assess and respond.
- **Automated decision-making** → CRITICAL: right to human review of decisions made solely by automated processing with legal/significant effects.

### Technical Requirements

- **Privacy by design** → IMPORTANT: data protection built into system design, not bolted on. Default to most privacy-protective settings.
- **Data Protection Impact Assessment (DPIA)** → IMPORTANT: required for high-risk processing (large-scale, systematic monitoring, sensitive data).
- **Breach notification** → CRITICAL: notify supervisory authority within 72 hours. Notify data subjects if high risk.
- **Records of processing** → IMPORTANT: maintain records of all processing activities (Article 30). Purpose, categories, recipients, retention, safeguards.
- **Data Processing Agreement** → IMPORTANT: written agreement with all processors (cloud providers, analytics, etc.) covering Article 28 requirements.

## CCPA/CPRA (California, US)

- **Right to know** → IMPORTANT: disclose categories of data collected, sources, purposes, third parties shared with.
- **Right to delete** → IMPORTANT: delete on request. Exceptions for legal obligations, security, contract performance.
- **Right to opt-out of sale** → CRITICAL: "Do Not Sell or Share My Personal Information" link required. Honor immediately.
- **Non-discrimination** → CRITICAL: cannot deny service or charge more for exercising privacy rights.
- **Sensitive personal information** → IMPORTANT: SSN, financial account numbers, precise geolocation, biometrics — limit use to what's necessary.

## Technical Implementation

### PII Detection & Classification

- **PII in logs** → CRITICAL: scan logs for PII (email, phone, PAN, Aadhaar, SSN, card numbers). Mask or exclude.
- **PII in error messages** → CRITICAL: stack traces and error responses must not contain personal data.
- **PII in URLs** → CRITICAL: personal data in query parameters gets logged in web servers, proxies, analytics. Use POST body.
- **PII in analytics** → IMPORTANT: don't send personal data to analytics platforms (Google Analytics, Mixpanel) without anonymization.
- **PII in caches** → IMPORTANT: cached PII must have TTL and be invalidated on deletion request.
- **PII in backups** → IMPORTANT: backups contain PII. Retention policy must apply to backups too. Encryption mandatory.
- **PII in test/staging** → CRITICAL: production PII in non-production environments. Use anonymized/synthetic data.

### Data Encryption

- **At rest** → CRITICAL: all PII encrypted at rest. AES-256 minimum. Key management via KMS, not code.
- **In transit** → CRITICAL: TLS 1.2+ for all data transmission. No plaintext APIs for personal data.
- **Application-level encryption** → IMPORTANT: for sensitive fields (Aadhaar, PAN, bank account), encrypt at application level in addition to storage encryption.
- **Key rotation** → IMPORTANT: encryption keys must be rotatable without data loss. Plan and test key rotation.
- **Tokenization** → IMPORTANT: for high-sensitivity data (card numbers, Aadhaar), replace with tokens. Store mapping in secure vault.

### Consent Management Technical

- **Consent storage** → CRITICAL: store: what was consented, when, version of privacy policy, method (click/checkbox/API). Immutable.
- **Consent version tracking** → IMPORTANT: when privacy policy changes, re-consent required. Track which version each user consented to.
- **Consent propagation** → IMPORTANT: consent change must propagate to all systems processing that data. Event-driven propagation.
- **Consent proof** → IMPORTANT: be able to prove consent was given. Timestamp, IP, session, UI screenshot/hash.

### Data Deletion

- **Hard vs soft delete** → IMPORTANT: GDPR/DPDP erasure generally requires hard delete. Soft delete (is_deleted flag) may not comply.
- **Cascade deletion** → IMPORTANT: deleting user must cascade to all related records across all services/databases.
- **Backup deletion** → IMPORTANT: deleted data must eventually be purged from backups. Define backup rotation aligned with retention.
- **Third-party deletion** → IMPORTANT: if data was shared with third parties, notify them to delete too.
- **Deletion verification** → IMPORTANT: verify deletion completed across all systems. Audit log of deletion.
- **Legal hold exception** → IMPORTANT: don't delete data under legal hold or regulatory retention requirement. Flag and exclude from deletion.

## Checklist

```
[ ] Privacy notice provided before/at data collection
[ ] Explicit consent obtained (not pre-checked, not bundled)
[ ] Consent withdrawal mechanism exists and works
[ ] Data access/export API exists for data subject requests
[ ] Data deletion cascades across all systems and services
[ ] PII encrypted at rest (AES-256) and in transit (TLS 1.2+)
[ ] No PII in logs, error messages, URLs, or analytics
[ ] No production PII in test/staging environments
[ ] Data retention periods defined and auto-enforced
[ ] Breach notification process documented and tested
[ ] Records of processing activities maintained
[ ] Children's data handled with parental consent
[ ] Cross-border transfer compliance verified
[ ] Third-party data processing agreements in place
```
