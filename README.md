# hedge

[![CI](https://github.com/faustbrian/go-hedge/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/faustbrian/go-hedge/actions/workflows/ci.yml)
[![CodeQL](https://img.shields.io/badge/CodeQL-required-blue)](https://github.com/faustbrian/go-hedge/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Mutation](https://img.shields.io/badge/mutation-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Documentation](https://img.shields.io/badge/docs-checked_in_CI-blue)](docs/)
[![Go Reference](https://pkg.go.dev/badge/github.com/faustbrian/go-hedge.svg)](https://pkg.go.dev/github.com/faustbrian/go-hedge)
[![Release](https://img.shields.io/github/v/release/faustbrian/go-hedge?sort=semver)](https://github.com/faustbrian/go-hedge/releases)
[![Go](https://img.shields.io/badge/go-1.27.0-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`hedge` reduces eligible tail latency by starting a finite number of duplicate
attempts after explicit delays. Unlike retry, an earlier attempt is still
running when a hedge starts. The package makes replay safety, amplification,
deadlines, shared budgets, cancellation, result ownership, and endpoint-safe
observability explicit.

The design follows [Failsafe-Go hedge semantics](https://failsafe-go.dev/hedge/)
and the delayed-duplicate technique described in
[The Tail at Scale](https://research.google/pubs/the-tail-at-scale/). Go
contexts make cancellation cooperative: canceling a context asks work to stop;
it does not wait for it to stop.

The module is a stable v1 public library. It requires Go 1.27.0 or newer.

## Install

```sh
go get github.com/faustbrian/go-hedge@v1
```

## Quick start

```go
budget, err := hedge.NewOutstandingBudget(20)
if err != nil {
	return err
}
policy, err := hedge.NewPolicy(hedge.Config[*http.Response]{
	MaxHedges:          1,
	ReplaySafe:         true, // a reviewed downstream duplicate-suppression contract
	Delay:              40 * time.Millisecond,
	TotalTimeout:       300 * time.Millisecond,
	AttemptTimeout:     200 * time.Millisecond,
	CleanupTimeout:     50 * time.Millisecond,
	Clock:              hedge.RealClock{},
	Budget:             budget,
	Classifier: (hedge.ClassifyFunc[*http.Response])(func(_ context.Context, r hedge.AttemptResult[*http.Response]) (hedge.Classification, error) {
		if r.Err == nil && r.Value.StatusCode < 500 {
			return hedge.ClassificationSuccess, nil
		}
		return hedge.ClassificationFailure, nil
	}),
	Disposer: (hedge.DisposeFunc[*http.Response])(func(_ context.Context, response *http.Response) error {
		if response == nil || response.Body == nil {
			return nil
		}
		return response.Body.Close()
	}),
	Resource:           "catalogue-read",
	FactoryFailureMode: hedge.FactoryFailureStop,
})
if err != nil {
	return err
}

response, report, err := hedge.Do(ctx, policy,
	(hedge.AttemptFactoryFunc[*http.Response])(func(info hedge.AttemptInfo) (hedge.Attempt[*http.Response], string, error) {
		req, err := newIndependentlyOwnedRequest(info) // including a fresh Body
		if err != nil {
			return nil, "", err
		}
		return func(attemptCtx context.Context) (*http.Response, error) {
			return client.Do(req.WithContext(attemptCtx))
		}, safeEndpointID(info), nil
	}))
if err != nil {
	// A non-nil response is the deterministic selected failed result and is now
	// caller-owned. All other returned results are disposed by the policy.
}
// During shutdown, wait with a bounded context for cooperative losers.
_ = report.Wait(shutdownCtx)
```

`http.Request.Clone` only shallow-copies `Body`; use `GetBody` or an
application-owned replay factory to create a fresh body for every attempt.

## Safety boundary

`ReplaySafe: true` is a declaration, not an inferred property. Do not hedge
payments, non-idempotent writes, queue acknowledgements, transactions, or
non-replayable streams unless every downstream hop provides and honors a
reviewed idempotency key and duplicate suppression. An idempotency key by
itself does not prove this.

The package never clones requests, discovers endpoints, load balances, retries
sequentially, chooses fallbacks, or combines retry and hedge presets. See
[replay safety](docs/replay-safety.md), [composition](docs/composition.md), and
[Kubernetes sizing](docs/kubernetes.md).

## API and ownership

A `Policy` copies its schedule and is immutable after construction. Its
callback dependencies are borrowed and must remain concurrency-safe for the
policy's lifetime. The policy itself starts no work. Each `Do` call owns a
bounded set of attempt goroutines, timers, and cancellation scopes. Cancellation
is cooperative, so retain the returned `Report` and call `Wait` with a bounded
context during shutdown. A shared `OutstandingBudget` is concurrency-safe and
starts no goroutines.

- Ordinal `0` is the original; `1..MaxHedges` are delayed hedges.
- Exactly one published success wins. Published equal-clock successes use the
  lower ordinal; publication itself is linearized by the execution.
- Winner cancellation is immediate but cooperative. `Report.Wait` exposes
  attempts that ignore cancellation.
- The returned value is caller-owned. On all-failure, it is the lowest-ordinal
  failed result. Every other returned value is passed exactly once to the
  configured disposer.
- `ExecutionError.Error` and observations exclude raw downstream messages.
- `OutstandingBudget` bounds concurrent additional attempts across every
  execution sharing it. Use a distinct shared instance per resource when
  independent bounds are required.

## Documentation

- [Documentation index](docs/README.md)
- [API reference](docs/api.md)
- [Composition and adoption guidance](docs/composition.md)
- [Operations guide](docs/operations.md)
- [Performance methodology](docs/performance.md)
- [FAQ](docs/faq.md)
- [Support](SUPPORT.md)
- [Security policy and reporting guidance](SECURITY.md)
- [Compatibility policy](COMPATIBILITY.md)
- [Release history](CHANGELOG.md)

For ecosystem-wide selection and ownership guidance, see the versioned
[Golib ecosystem index](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/README.md)
and its [Resilience family](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/design-language.md#package-families-and-selection).
