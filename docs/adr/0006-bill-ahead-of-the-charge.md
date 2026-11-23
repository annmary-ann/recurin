# 0006. Bill ahead of the charge to meet the pre-debit rule

- **Status:** Accepted
- **Date:** 2026-11-23
- **Issue:** #6

## Context

Card-on-file billing engines create the renewal invoice and charge the customer at the same moment, at the start of the new period.

Indian recurring payments do not allow that. Under the RBI e-mandate framework and NPCI's UPI AutoPay rules, the customer must receive a **pre-debit notification** before each recurring debit (at least 24 hours before, as currently understood; issue #14 verifies the exact requirements against the circulars). The notice must state the amount, so the amount has to be final before the notice goes out, which means the invoice, including GST, proration and usage, has to be final before the notice too.

Other constraints point the same way:

- Debits above a limit need the customer to authenticate again, so the engine must know the amount in advance to choose between an auto-debit and a payment link.
- e-NACH presentations settle a day or more later, so the charge has to start earlier to land on time.
- Indian SaaS customers expect access to continue without interruption on the renewal date.

## Decision

recurin separates **billing** from **charging**:

1. At `period_end − lead_time`, the billing tick creates and **finalises** the renewal invoice. The lead time is configured per payment rail; the default is 48 hours.
2. The mandates module sends the pre-debit notice and records `debit_after`.
3. The charge job runs at `max(period_end, debit_after)` and refuses to run earlier.
4. Changes made after finalisation (an upgrade, a new add-on) go on the next invoice or a separate prorated invoice; they never alter a finalised, already-announced amount.

## Alternatives considered

- **Bill and charge at renewal, as card-on-file engines do.** Breaks the notice rule: there is no final amount to announce 24 hours earlier.
- **Send a notice with an estimated amount, then bill at renewal.** The notice would not match the debit. That fails the rule's intent and confuses customers.
- **Delay the period start until the charge clears.** Compliant, but every customer's access would lapse at each renewal, and anniversary dates would drift.

## Consequences

- recurin is compliant by construction, and a test asserts that no charge is ever attempted before `debit_after`.
- Usage-based charges must be cut off at finalisation. Usage that arrives during the lead time rolls into the next invoice, and the docs must explain this clearly.
- Invoices carry a finalisation time that is earlier than their period start. Reports and the admin console label this correctly.
- Card-on-file markets can set the lead time to zero, which gives classic charge-at-renewal behaviour from the same code path.
