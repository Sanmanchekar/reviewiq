# Communication & Notification Compliance Review Skill

Rules for SMS, email, push notifications, and WhatsApp in regulated industries.

## SMS Compliance

### TRAI DND (India)

- **DND registry check** → CRITICAL: before sending promotional SMS, check TRAI DND registry. Sending to DND-registered numbers = penalty.
- **Transactional vs Promotional** → IMPORTANT: transactional SMS (OTP, alerts, confirmations) exempt from DND. Promotional requires consent.
- **Sender ID (Header)** → IMPORTANT: registered 6-character header required. Unregistered headers blocked by telecom.
- **Template registration** → CRITICAL: SMS templates must be registered with DLT platform. Unregistered templates blocked.
- **Content type matching** → IMPORTANT: DLT template variables must match actual content type. Entity ID must match sender.
- **Time restrictions** → IMPORTANT: promotional SMS only between 9 AM and 9 PM. Transactional: any time.
- **Opt-out mechanism** → IMPORTANT: every promotional SMS must include opt-out instruction.
- **Scrubbing** → IMPORTANT: number scrubbing against DND list before campaign dispatch. Real-time for transactional exemption.

### RBI SMS Mandates (Financial)

- **Transaction alerts** → CRITICAL: banks/NBFCs must send SMS for every financial transaction. Amount, account (masked), date/time, balance.
- **OTP delivery** → CRITICAL: OTP for financial transactions must be via SMS (or in-app). Email-only OTP not acceptable for high-value.
- **Loan disbursement notification** → IMPORTANT: SMS confirming loan disbursement with amount, account, terms reference.
- **EMI reminder** → IMPORTANT: pre-debit notification at least 24h before auto-debit.
- **Account statement** → NIT: periodic account summary via SMS/email per customer preference.

## Email Compliance

- **CAN-SPAM / GDPR consent** → CRITICAL: marketing emails require explicit opt-in. Include unsubscribe link. Honor unsubscribe within 10 days.
- **Transactional email authentication** → IMPORTANT: SPF, DKIM, DMARC configured. Prevents spoofing and improves deliverability.
- **PII in email** → IMPORTANT: don't include full account numbers, PAN, Aadhaar in email body. Mask sensitive data.
- **Email template injection** → CRITICAL: user-controlled data in email templates. Sanitize to prevent HTML/JS injection.
- **Reply-to address** → NIT: don't use noreply@ for transaction disputes. Provide actionable reply path.
- **Attachment security** → IMPORTANT: password-protect sensitive attachments (statements, tax documents). Send password via separate channel.
- **Bounce handling** → IMPORTANT: track hard bounces, remove from list. Continued sending to invalid addresses = spam flag.

## Push Notifications

- **Permission handling** → IMPORTANT: request push permission contextually, not on first launch. Explain value.
- **Silent push for data sync** → NIT: don't abuse silent push. iOS throttles aggressive background pushes.
- **Payload size** → NIT: keep payload small. APNs: 4KB limit, FCM: 4KB limit. Large payloads = delivery failure.
- **Sensitive data in push** → CRITICAL: push notification content visible on lock screen. Don't include OTP, balance, transaction amounts.
- **Deep linking** → IMPORTANT: deep links in push must validate authentication state. Don't deep link to authenticated pages without session check.
- **Rate limiting** → IMPORTANT: too many pushes = user disables notifications. Max 2-3 per day for non-critical.
- **Token refresh** → IMPORTANT: handle FCM/APNs token refresh. Stale tokens = undelivered notifications.

## WhatsApp Business API

- **Template approval** → CRITICAL: message templates must be pre-approved by WhatsApp/Meta. Unapproved templates rejected.
- **Opt-in requirement** → CRITICAL: explicit customer opt-in for WhatsApp messaging. Cannot message without consent.
- **24-hour window** → IMPORTANT: free-form messages only within 24h of customer's last message. After that, only approved templates.
- **Session vs template messages** → IMPORTANT: session messages (within 24h window) can be free-form. Template messages (outside window) must use approved templates.
- **Media handling** → NIT: WhatsApp supports images, documents, videos. Size limits apply. Validate before sending.
- **Read receipts** → NIT: track delivery and read status for important notifications.
- **Fallback** → IMPORTANT: if WhatsApp delivery fails, fall back to SMS/email. Don't silently fail.

## Consent Management

- **Consent recording** → CRITICAL: record what the customer consented to, when, via what channel. Immutable audit trail.
- **Granular consent** → IMPORTANT: separate consent for SMS, email, push, WhatsApp, phone calls. Don't bundle.
- **Consent withdrawal** → CRITICAL: customer must be able to withdraw consent easily. Effect within 24-48 hours.
- **Consent for third-party** → IMPORTANT: if sharing data with partners for communication, separate consent required.
- **Preference center** → NIT: provide UI for customers to manage communication preferences.
- **Consent expiry** → IMPORTANT: marketing consent may have validity period per regulation. Track and re-consent.

## Timing & Frequency

- **Quiet hours** → IMPORTANT: no promotional messages between 9 PM and 9 AM (India), or per local regulation.
- **Frequency capping** → IMPORTANT: max messages per channel per day/week. Prevent notification fatigue.
- **Critical vs promotional** → IMPORTANT: critical alerts (fraud, security) exempt from quiet hours and frequency caps.
- **Timezone awareness** → IMPORTANT: send messages in customer's local timezone, not server timezone.
- **Batch sending** → NIT: stagger bulk sends to avoid spike. Telecom/email providers may throttle.

## Checklist

```
[ ] SMS templates registered on DLT platform
[ ] DND registry checked before promotional SMS
[ ] No promotional SMS outside 9 AM - 9 PM
[ ] Transaction alerts sent for every financial transaction
[ ] Marketing emails have unsubscribe link and honor opt-out
[ ] Email SPF/DKIM/DMARC configured
[ ] No sensitive data (OTP, balance) in push notification content
[ ] WhatsApp templates pre-approved by Meta
[ ] Explicit opt-in obtained per channel (SMS/email/push/WhatsApp)
[ ] Consent withdrawal mechanism exists and works within 48h
[ ] Frequency caps implemented per channel
[ ] Fallback chain configured (WhatsApp → SMS → email)
```
