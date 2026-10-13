# Verification

[Documentation index](README.md)

Ordinary development starts with `go test ./...` or a focused test selector.
`make check` and `make ci` use the public verifier declared in `.golib.yaml`
for bounded local checks, including one named race selection for shared-budget
release, loser cancellation/disposal and scheduled-timer cleanup. Documentation
changes need relevant structure, links and examples, not unrelated runtime work.
Applicable security checks and fail-closed required results remain enforced.

`make check-full` retains the explicit full module gate: formatting, module
tidiness, vet, architecture checks,
unit and example tests, race tests, exact statement coverage, fuzz targets,
mutation checks, leak tests, benchmarks, documentation links, API compatibility,
lint, static analysis, vulnerability scanning, secret and license checks, and a
clean external consumer. Selected release qualification uses
`golib release dry-run --module .` and retains applicable SBOM and provenance
checks. A bounded local pass is not release qualification, and CI-only changes
do not require a module release.

Deterministic clock tests control hedge and deadline races. Concurrency tests
exercise shared budgets, cancellation, late publication, disposal, and cleanup
waiting. Green coverage alone is not proof. Mutation is optional developer-local
or selected scheduled assurance, not a PR prerequisite. A selected campaign must
not hide viable survivors, timeouts or missing coverage; investigate real defects
and preserve narrowly reviewed equivalent/invalid dispositions. See
[contributor verification](../CONTRIBUTING.md#verification) for focused 2s fuzzing,
optional one-package mutation and selected API/composition requirements.

Named deterministic-scheduling and fault-path gates repeat timing-independent
selection tests and exercise bounded factory, attempt, delay, cleanup, and
observer failures. The clean-consumer gate compiles and executes the public API
from a fresh module with workspace mode disabled. Supply-chain checks verify
module checksums, read-only dependency resolution, license disclosure, and the
absence of committed secrets.
