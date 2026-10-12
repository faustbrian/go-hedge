# Source documentation audit

## Scope and acceptance

Documentation-only Tier A pass from `76e78bd6545d465898de307eddfc6b052983d9a5`.
The owner is the Hedge repository worker. Preserve every executable Go token,
compiler/linter directive, dependency and test. Clarify existing public contracts
for callers and composition consumers, not change scheduling or repair behavior.
Direct owned consumers in the catalog are Concurrency Limit
(`integration/resilience`) and Service (`integration/reference-external`); their
dependency routes remain unchanged. Resilience v1/v2 are production dependencies
and Failsafe-Go is imported only by benchmark fixtures.

## Reproducible inventory

Run from this repository:

```sh
git ls-files '*.go' 'go.mod' '**/go.mod'
cat modules.json packages.json .golib.yaml .go-version
rg -n '^func |^type |^const |^var ' --glob '*.go'
rg -n 'go:generate|Code generated|go:build|\+build|TODO|FIXME|HACK|NOTE|WARNING|Deprecated:|nolint' --glob '*.go'
golib source-docs inventory --json
golib source-docs check --json
```

There is one module, `github.com/faustbrian/go-hedge`, one public package `hedge`
and internal/external test packages. There are 26 tracked Go files (five
production and 21 test/helper files), with no nested modules, build-tag/platform
variants, generated Go files or generation directives. Existing linter directives
are retained. No unfinished-work or deprecation markers were found in Go source.
The shared inventory/check commands inspect tracked files, so stage the new audit
explicitly before invoking them. Shared-tooling adoption is coordinator-owned;
local checker results are not hosted CI qualification.

| Tracked file | Audited responsibility |
| --- | --- |
| `aaa_mutation_boundaries_internal_test.go` | Inclusive limits, publication ordering, cleanup/selection helpers and mutation-sensitive truth tables. |
| `benchmark_test.go` | Preconstructed policies and Failsafe comparison; no equivalent cleanup/replay guarantee. |
| `budget.go` | Atomic outstanding admissions, nil/zero behavior and idempotent permit release. |
| `budget_test.go` | Shared concurrent admission and repeated release. |
| `clock_test.go` | Mutex-owned deterministic time/timers and caller-side helpers. |
| `composition_semantics_test.go` | Selected error cause, loser ownership and callback cleanup semantics. |
| `composition_test.go` | Composition-facing reusable policy execution. |
| `coverage_contract_test.go` | Factory, classifier, dynamic delay and derived cancellation failure paths. |
| `example_test.go` | Public construction/Do example; executable text unchanged. |
| `execution.go` | Attempt admission, completion selection, publication/cleanup ownership, errors and reporting. |
| `execution_boundary_test.go` | Completion timing/ties, partial failures, deadlines and cleanup wait boundaries. |
| `execution_test.go` | Original/hedge success, cancellation and disposal. |
| `fuzz_test.go` | Validation and outstanding-budget seed targets; not executed for prose. |
| `internal_contract_test.go` | Private ordering, error cause and callback recovery helpers. |
| `leak_test.go` | Cooperative per-execution lifetime fixture; not universal shutdown proof. |
| `observation.go` | Bounded outcome labels and caller-owned metadata privacy. |
| `observation_test.go` | Metadata events and observer panic containment. |
| `policy.go` | Canonical package/API contracts, validation and copied schedule; borrowed collaborators. |
| `policy_test.go` | Validation, copying, typed nil dependencies and bound partitions. |
| `race_test.go` | Shared budget/permit fixture; not proof for arbitrary caller callbacks. |
| `resilience_budget.go` | Explicit v1/v2 scope selection, borrowed physical permit and direct v2 rejection classification. |
| `resilience_budget_test.go` | v1 original/hedge charges, composed lineage and denial. |
| `resilience_v2_internal_test.go` | Direct v2 errors, version selection and permit wrapper. |
| `resilience_v2_test.go` | v2 ownership/lineage, dual-scope rejection and composition. |
| `security_timer_test.go` | Caller cancellation through total-context path stops scheduled timer. |
| `strategy_factory_test.go` | Fixed/scheduled/dynamic slots, factory modes and admission denial. |

## Public and private contract coverage

Review all exported constants/variables/types/functions, enum members, interface
methods, struct fields and concrete methods in the five production files.
Important gaps are not just declaration coverage:

- `NewPolicy` validates bounds, replay declaration, exactly one delay strategy,
  required collaborators and budget ownership. It copies the schedule but borrows
  callback/dependency references. Construct policies through it; callbacks used
  by concurrent executions must themselves be safe and cooperative.
- Delay ordinals are scheduling slots, not guaranteed starts. Denied admission
  and continued factory failures consume a slot and advance the next delay.
- Success ordering compares completion time, then ordinal, only among results
  already published when selection closes publication. It cannot compare future
  completions. Terminal and all-failed paths can transfer a partial value on error.
- Caller cancellation and total/attempt deadlines are cooperative. Synchronous
  factories, clocks, observers and budget methods cannot be forcibly interrupted.
  Classification still sees the attempt error and context state; policy decisions
  are explicit, rather than inferred from an error or panic.
- Per-execution publication and cleanup mutexes own separate state. The bounded
  completion channel is never closed; closed publication redirects late values
  to their runner for disposal. Cleanup admission is sealed before its done channel
  closes once its active count reaches zero. No package-wide worker or Close API
  exists. Report.Wait is the caller's cleanup/shutdown observation boundary,
  not a general goroutine join: accounting finishes before deferred runner cancel.
- Every returned non-selected value, including an errored/zero value, is disposed.
  Selected success or partial failure values belong to the caller. Each disposal
  has a fresh timeout derived from Background, not caller values/cancellation.
  Uncooperative callbacks can outlive that timeout; Wait cancellation stops only
  waiting. Permits release before disposal, so admission bounds do not bound all
  cleanup work across executions.
- Original compatibility attempts are uncharged; additional ones share the
  explicit Budget. Resilience scopes instead own original/hedge admission;
  borrowed physical-attempt permits remain outer-owned. Exactly one supported
  v1/v2 scope is required and mixed scopes fail before factory/charging.
- Resource/endpoint lengths are bounded in bytes, not automatically redacted or
  cardinality-limited. Raw result/error data are absent from observations, but
  caller labels and unwrapped execution causes retain their own privacy boundary.

Private helpers were reviewed for synchronization, capacity, ownership transfer,
callback panic containment, error precedence, timer stopping and late-completion
failure windows. Existing tests supply source evidence, not newly executed
runtime proof. Fuzz targets, benchmark fixtures, mutation boundaries, cooperative
leak/race fixtures and the unchanged example were included in the file review.

## Rationale and remaining questions

Useful comments explain why slots can be skipped, why publication closes instead
of the channel, why cleanup is detached from caller cancellation, why shared
permits are idempotent, and why v2 avoids legacy error traversal. Simple local
helpers need no restatement of syntax. No public claim should turn a finite
configuration bound into forced termination, arbitrary callback safety, global
cleanup limits, automatic request replay safety or automatic label sanitization.
No source contract question requires a runtime change in this pass; replay safety,
callback cooperation, endpoint selection and aggregate cleanup admission remain
caller obligations rather than promises inferred from narrow fixtures.

## Smallest sufficient checks

- Shared bounded source inventory/check: package/API/member documentation and
  AST structure; record concrete results below.
- Go formatting; comment-excluding token equivalence for every tracked Go file,
  plus exact build/generation/linter directive preservation.
- Render root package documentation with the module-catalog Go 1.27.0 toolchain,
  isolated task-owned caches and workspace disabled. Inspect all changed contracts.
- Shared `api/baseline.txt` is apidiff input and stays unchanged for comments.
  Also regenerate the repository-owned `compat/public-api.txt` oracle using
  `scripts/check-api-compat.sh` semantics: ordered `go list ./...`, PACKAGE header,
  complete native `go doc -all` output and one trailing newline. Repeat to prove
  stability and compare declaration bodies with the old oracle.
- Maintained Markdown links/structure, source markers/generation, changed-content
  secrets, and one ordinary complete base-to-head diff review. No behavioral,
  race, fuzz, mutation, performance or broad aggregate campaign is applicable.
- Coordinator owns remote-main delivery and exact-source required CI. A local
  documentation commit does not establish those states or require a release.

## Results

- Shared `golib source-docs inventory --json` and `check --json` passed with
  one module, 26 tracked files and zero findings; 145 exported declarations and
  members were also enumerated during structural comparison.
- Module-catalog `GOTOOLCHAIN=go1.27.0`, `GOWORK=off`, bounded parallelism and
  owned GOCACHE/GOMODCACHE/GOTMPDIR were used for one local session. Pinned
  `gofmt -w`/`gofmt -l` passed for all eight affected Go files.
- A disposable `go/scanner`/`go/parser` comparison against the recorded base
  passed for all 26 files: code tokens, comment-free/source-position-free AST
  and compiler/linter directives match. Only optional inferred newline
  semicolons before closing delimiters are normalized (documenting formerly
  inline struct/interface members changes those separators, not execution).
- Ordered `go list ./...` returned only `github.com/faustbrian/go-hedge`.
  Two complete `go doc -all` snapshot generations, including the native PACKAGE
  header and one final newline, matched byte-for-byte. All 61 declaration blocks
  match the old oracle after removing comments/whitespace. The shared apidiff
  baseline is unchanged. Complete rendered API contracts were inspected.
- Native maintained Markdown link validation passed across the repository.
  Generation/build variants and the full source marker set were checked; no
  undocumented variant, generator, deprecation or unfinished marker remains.
- Complete ordinary final-diff review and changed-content secret checks passed;
  ACK profile/message checks passed; normal safe-wrapper hooks run at commit.
- No executable example changed. Tests, race/leak checks, active fuzzing,
  mutation, benchmarks, broad aggregate checks and release qualification were
  not run: the change is non-runtime prose and its generated API reference.
- Task-owned caches and temporary helper/output are disposed after evidence
  capture. The committed worktree is preserved for coordinator delivery;
  remote-main qualification and CI are not local completion claims.

## Objective enforcement adoption

The local `source-docs-check` target is required by ordinary/full checks and CI.
Shared CI uses `source_documentation: check` with the immutable Tools v2.4.0
workflow and matching authenticated binary/checksum declaration. Structural
checks do not replace this maintained semantic audit or native docs/examples.
