# Security policy

## Supported versions

The latest stable v1 release receives security fixes. Older releases and the
`main` branch are unsupported; upgrade before reporting unless the issue is a
regression under active development.

| Version | Supported |
| --- | --- |
| Latest stable v1 release | Yes |
| Older releases | No |
| `main` | No |

## Reporting a vulnerability

Do not disclose a suspected vulnerability in a public issue. Use the
repository's private security reporting facility when available. If private
reporting is unavailable, ask a maintainer for a private contact channel
without disclosing the vulnerability.

Do not include credentials, production payloads, request bodies, URLs, or raw
customer errors in a public report or initial contact request.

## Hedging boundary

Unsafe replay can duplicate side effects. Use hedging only after reviewing
every downstream hop's idempotency and authorization contract. Endpoint
diversity must preserve tenant routing, data residency, consistency, and
credentials. Bound amplification, labels, deadlines, cleanup, and shutdown.
