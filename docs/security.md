# Security threat model

## Scope and trust boundaries

This model covers the root `github.com/faustbrian/go-hedge` module. The package
opens no network connections, reads no files or environment variables, and
starts no work until `Do` is called. Applications own operation authorization,
destinations, request bodies, credentials, and the lifetime of concurrent calls.

Attackers may cause slow or failed downstream operations, trigger concurrent
logical requests, or influence application data and identities. Policy and
callback implementations are trusted application dependencies, not sandboxed
plugins. An attempt factory must create independently owned mutable state and
must not start external work during construction.

Replay safety is an explicit declaration, not an authorization check or proof
of idempotency. Duplicate writes can cause financial loss, repeated queue
acknowledgements, inconsistent transactions, or cross-tenant routing. Review
every downstream hop before enabling concurrent replay.

## Enforced boundaries and ownership

- Policies reject missing replay declarations, more than 64 hedges, oversized
  schedules and identities, invalid deadlines, and unusable shared budgets
  before copying the schedule or launching work.
- One execution starts at most one original and 64 additional attempts.
  Completion storage and retained failure metadata are bounded by this count.
  The built-in outstanding budget bounds additional attempts across its shared
  scope; it does not bound the application's original requests or cleanup work.
- Total and optional attempt contexts reach attempt execution and
  classification. Caller cancellation takes precedence when observed after
  the total deadline. Both cancellation outcomes stop the scheduled hedge
  timer. Cancellation requests cooperation; it cannot force callback return.
- Published results have explicit ownership: one selected value transfers to
  the caller and every other returned value goes to the disposer. Cleanup gets
  a separate bounded context. `Report.Wait` with a bounded caller context
  observes unfinished attempts and cleanup without delaying winner delivery.
- Execution error strings and observations exclude raw values, request data,
  downstream messages, and recovered panic values. The selected error cause is
  deliberately available through `Unwrap`; this is not a redaction boundary
  for callers that inspect or log causes. Resource and endpoint strings are
  length-bounded but their content and label cardinality remain caller-owned.

## Conditional residual risks

Each risk is accepted only while its owner maintains the stated condition.

| Risk | Owner and rationale | Mitigation | Review condition |
| --- | --- | --- | --- |
| Duplicate side effects or cross-tenant routing | Application owner; the library cannot infer replay authorization or destination equivalence. | Use reviewed downstream duplicate suppression and independently owned requests; preserve tenant, residency and credential routing for every attempt. | A new operation, destination, body source or downstream retry policy. |
| Aggregate work exceeds the per-call bound | Application owner; a hedge budget covers additional attempts, not original request admission. | Bound logical request concurrency and request sizes; use a shared budget per resource and the resilience scope when composing amplification policies. | Concurrency limits, composition depth or budget scope changes. |
| Callbacks ignore cancellation, block, or panic outside contained hooks | Callback owner; Go cannot safely interrupt arbitrary synchronous code. | Keep factories, clocks, budgets, permits and observers non-blocking; bound dynamic delay computation; make attempts, classifiers and disposers honor their contexts. | A callback implementation or dependency changes. |
| Slow cleanup retains values or external resources | Application owner; timely disposal depends on cooperative caller code. | Bound result sizes and cleanup duration, close losing response bodies, cap concurrent calls, and retain reports for bounded shutdown waits. | Resource type, disposer, or shutdown policy changes. |
| Labels or selected causes expose secrets or unbounded identities | Application observability owner; identity content and deliberate cause inspection are application contracts. | Use finite credential-free labels; never use URLs, tokens, payloads or tenant identifiers directly; redact causes before logging. | Logging, tracing, label sources or cause inspection changes. |
| Dependency, action, release or maintainer compromise | Repository maintainer; supply-chain execution is a separate trust boundary. | Review dependency and immutable action pins, require hosted checks, verify release signatures and checksums, and use private coordinated disclosure. | Dependency update, advisory or release. |

## Verification and release verdict

The security audit found a cancellation race that could leave the scheduled
hedge timer active when caller cancellation was observed through the total
deadline path. A deterministic context and clock test checks timer release,
unchanged caller-cancellation identity, and bounded attempt cleanup. Existing
tests cover replay and fan-out limits, shared budget admission, panic-message
privacy, selected cause ownership, result disposal, cancellation and shutdown.

The compatible correction targets v1.0.2. Publication requires exact-source
required CI and the applicable security and release gates. Public APIs and selected
cause semantics remain unchanged. The conditional risks above are not claims
that arbitrary caller code or downstream systems are bounded by the package.

Private reporting and disclosure procedures are in [SECURITY.md](../SECURITY.md).
