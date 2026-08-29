# Changelog

## Unreleased

### Changed

- Replace the repository-local verification implementation with the pinned
  `go-library-tools` v1.0.4 CLI and reusable workflow while preserving package
  policy and content-addressed verification evidence.
- Make architecture verification portable to CI runners without `ripgrep` and
  restrict it to repository-owned Go sources.

### Documentation

- Replace the archived monorepo link with package-owned documentation.

## 1.0.0 - 2026-08-25

### Changed

- Exclude intentional nested modules from root local-proxy archives so local,
  bootstrap, CI, and public module checksums describe the same source
  boundary.

- Track the pinned documentation-tool lockfile so clean CI checkouts install
  the exact validated cspell dependency.

- Reconcile standalone dependency checksums against deterministic current
  module archives so CI, local verification, and release consumers resolve
  identical content.

- Harden standalone documentation validation with deterministic spelling and
  link checks, package-specific documentation gates, and repository-local
  contributor guidance.

### Changed

- Publish the module from its standalone `github.com/faustbrian/go-hedge` identity while preserving its documented API and behavior.

### Documentation

- Link the package README to package-owned documentation.

### Added

- Add opt-in consumption of the shared `resilience` budget with coordinated
  nested attempt lineage and bounded retry-plus-hedge amplification.
- Add an explicitly replay-safe, deadline-bounded hedged execution policy with
  fixed, scheduled, and dynamic delays.
- Add shared outstanding-work budgets, deterministic result selection,
  cooperative loser cancellation, explicit result disposal, cleanup waiting,
  and bounded lifecycle observations.
- Require budgets to declare a validated finite capacity and add pinned
  Failsafe-Go comparison benchmarks as a development-only dependency.
- Add explicit deterministic-scheduling, fault-path, clean-consumer, and
  supply-chain gates to the module contract.
- Replace the narrow mutation smoke check with the repository's complete viable
  mutant contract.

No migration is required because this is the first release.
