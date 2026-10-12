package hedge

import "time"

// Outcome identifies one bounded lifecycle event. It is safe to use as a
// metric label after mapping unknown values to a bounded fallback.
type Outcome uint8

const (
	// OutcomeNoHedgeNeeded records an original winner before any hedge started.
	OutcomeNoHedgeNeeded Outcome = iota + 1
	// OutcomeHedgeStarted records one admitted additional attempt.
	OutcomeHedgeStarted
	// OutcomeBudgetDenied records rejected additional work.
	OutcomeBudgetDenied
	// OutcomeWinnerSelected records a winner after additional work started.
	OutcomeWinnerSelected
	// OutcomeAllAttemptsFailed records deterministic failed-result selection.
	OutcomeAllAttemptsFailed
	// OutcomeCallerCanceled records caller cancellation.
	OutcomeCallerCanceled
	// OutcomeTotalDeadline records total timeout expiry.
	OutcomeTotalDeadline
	// OutcomeAttemptCompleted records one result published to the coordinator.
	OutcomeAttemptCompleted
	// OutcomeCleanupFailed records one disposer failure.
	OutcomeCleanupFailed
)

// String returns a bounded label; unknown values collapse to "unknown".
func (outcome Outcome) String() string {
	switch outcome {
	case OutcomeNoHedgeNeeded:
		return "no_hedge_needed"
	case OutcomeHedgeStarted:
		return "hedge_started"
	case OutcomeBudgetDenied:
		return "budget_denied"
	case OutcomeWinnerSelected:
		return "winner_selected"
	case OutcomeAllAttemptsFailed:
		return "all_attempts_failed"
	case OutcomeCallerCanceled:
		return "caller_canceled"
	case OutcomeTotalDeadline:
		return "total_deadline"
	case OutcomeAttemptCompleted:
		return "attempt_completed"
	case OutcomeCleanupFailed:
		return "cleanup_failed"
	default:
		return "unknown"
	}
}

// Observation excludes returned values and raw errors. Resource and Endpoint
// are caller labels bounded in length, not automatically sanitized/redacted or
// cardinality-limited. Events from workers may arrive after Do returns.
type Observation struct {
	// Outcome identifies the event; not every terminal failure has an event.
	Outcome Outcome
	// Ordinal is local to Do: zero is original, positive values are hedge slots.
	Ordinal uint
	// Delay is the chosen slot delay, or zero for the original.
	Delay time.Duration
	// Duration measures attempt plus classification time where available,
	// including on cleanup-failure events; it is not disposal latency.
	Duration time.Duration
	// Resource is the policy's caller-provided budget/observation identity.
	Resource string
	// Endpoint is the factory's optional caller-provided endpoint identity.
	Endpoint string
	// Classification is set for OutcomeAttemptCompleted and zero otherwise.
	Classification Classification
	// Winner marks the selected-success event, not every successful completion.
	Winner bool
	// Loser marks cleanup-failure events, not every non-selected completion.
	Loser bool
}
