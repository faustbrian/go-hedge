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
[private vulnerability reporting form](https://github.com/faustbrian/go-hedge/security/advisories/new).

Do not include credentials, production payloads, request bodies, URLs, or raw
customer errors in a public report or initial contact request.

Include the affected module and version, a minimal synthetic reproduction,
expected and observed behavior, and the relevant policy or callback boundary.
Maintainers target acknowledgment within two business days and an initial
severity and remediation assessment within seven days. If no acknowledgment
arrives, follow up through the same private reporting channel.

## Triage and coordinated disclosure

The repository maintainer owns severity, remediation and publication decisions.
Assess impact and reachability: credential exposure, authorization bypass,
unsafe replay and persistent corruption have higher priority than a bounded
local failure. Account for required caller-controlled configuration and
cooperative callback contracts when classifying resource-exhaustion reports.

Keep reports and reproductions private while investigating. For confirmed
vulnerabilities, identify affected versions, agree a disclosure date with the
reporter, add a regression test, and publish the smallest compatible fix with
upgrade guidance. Seek a shorter coordinated window for active exploitation
or severe impact; otherwise target remediation within 90 days. If remediation
cannot meet the agreed date, explain the remaining risk and mitigation to the
reporter and revise the date explicitly.

Publish a GitHub Security Advisory when warranted, together with affected and
fixed versions and a privacy-safe explanation. Never include reporter-private
data or exploit-enabling credentials in an advisory, release or test fixture.

## Hedging boundary

Unsafe replay can duplicate side effects. Use hedging only after reviewing
every downstream hop's idempotency and authorization contract. Endpoint
diversity must preserve tenant routing, data residency, consistency, and
credentials. Bound amplification, labels, deadlines, cleanup, and shutdown.
