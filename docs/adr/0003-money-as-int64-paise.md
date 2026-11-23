# 0003. Money as int64 paise with one rounding point

- **Status:** Accepted
- **Date:** 2026-11-23
- **Issue:** #6

## Context

A billing engine has one non-negotiable property: it must never create or lose money. The usual ways it happens:

- Floating-point arithmetic: `0.1 + 0.2 != 0.3`, and the errors compound across thousands of invoices.
- Rounding scattered through the code, so a line rounds one way, its tax another, and the invoice total a third, leaving one-paisa mismatches between the invoice and the ledger.
- Splitting an amount (a ₹100 discount over three lines, CGST and SGST halves) and rounding each part independently, so the parts no longer add up to the whole.

GST makes this sharper: tax is computed per line, split into CGST and SGST for intra-state supplies, and must reconcile exactly with the invoice total and the ledger.

## Decision

- Every amount is a `money.Amount`: an **int64 count of the currency's minor unit** (paise for INR) plus an ISO 4217 currency code. The database column is `amount_paise bigint` plus `currency char(3)`.
- Rates (tax rates, discounts) are integers in **basis points** (`1800` = 18%). Calculations that need fractional intermediates (proration, tiered rates) use an exact decimal type (`cockroachdb/apd`) internally and convert back to minor units at the end.
- Rounding happens in **one function**, using **half-even** (banker's) rounding unless a rule requires otherwise.
- Splitting goes through `money.Allocate`, which distributes the remainder so the parts always sum exactly to the whole.
- Adding amounts in different currencies is an error, not a conversion.

## Alternatives considered

- **`float64`.** Rejected outright; it cannot represent most decimal amounts exactly.
- **`numeric` / arbitrary-precision decimal everywhere.** Exact, but slower, allocates on every operation, and does not stop scattered rounding: each call site still decides its own precision.
- **Integer rupees.** Too coarse; GST and proration produce paise.
- **A third-party money library.** Most Go options either wrap floats, are unmaintained, or do not model allocation. The type is small enough to own and test exhaustively.

## Consequences

- int64 paise covers about ±₹9.2 × 10¹⁶ (roughly ₹92,000 lakh crore) per amount, far beyond any single invoice or ledger balance.
- A lint rule and code review forbid `float32`/`float64` in money paths.
- Property tests (#7, #20) check that `Allocate` always sums to its input, that addition and subtraction detect overflow, and that invoice totals reconcile with the ledger for random inputs.
- Currencies with zero or three minor-unit digits work through the currency's exponent, but v1 is tested against INR first.
- Rounding policy lives in one place, so a regulatory change to rounding is a one-function change with a test.
