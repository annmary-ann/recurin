# Security policy

recurin moves money and stores customer data, so security reports get priority over everything else.

## Reporting a vulnerability

**Do not open a public issue.** Report it privately through GitHub: go to the [Security tab](https://github.com/annmary-ann/recurin/security) and choose **Report a vulnerability**.

Please include:

- what an attacker could do (for example, read another account's invoices, or trigger a charge without a valid mandate)
- steps or a proof of concept against a local install
- the version or commit you tested

You will get an acknowledgement within 3 days and a first assessment within 7 days. Please give us a reasonable time to fix the issue before you disclose it. With your permission, we will credit you in the release notes.

## Supported versions

recurin is pre-release. Until v1.0, only the latest commit on `main` gets security fixes.

## What recurin never stores

recurin never receives or stores card numbers, CVVs or UPI PINs. Payment details are collected by the payment gateway, and recurin keeps only the gateway's tokens and mandate IDs.
