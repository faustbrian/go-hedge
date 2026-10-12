// Package hedge executes explicitly replay-safe operations with bounded,
// delayed concurrent attempts. It does not infer idempotency or clone request
// state; an AttemptFactory must create independently owned mutable state for
// every attempt.
//
// Construct a Policy with NewPolicy, then call Do for each logical operation.
// The selected value belongs to the caller, even when returned with an error;
// Report.Wait observes cleanup of all other returned values. Cancellation and
// cleanup deadlines require cooperative caller-supplied callbacks. There are
// no package-wide workers to shut down.
//
// A compatibility Budget bounds additional attempts. Opt-in Resilience v1/v2
// context scopes coordinate original and hedge admission across composed policies.
// Endpoint diversity, replay safety, label privacy and aggregate cleanup limits
// remain caller responsibilities. The package has no cgo bindings or required
// external services.
package hedge

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"
)

// MaxHedges bounds policy allocation, retained failure metadata, and fan-out.
const MaxHedges uint = 64

// MaxResourceLength bounds resource and endpoint identities in bytes. It does
// not sanitize their contents or limit the number of distinct caller labels.
const MaxResourceLength = 128

// MaxBudgetCapacity prevents a nominally finite budget from acting as an
// operationally unbounded admission policy.
const MaxBudgetCapacity uint = 1_000_000

// ErrInvalidPolicy identifies implicit, contradictory or unbounded policy
// configuration, missing Do inputs and invalid callback metadata/decisions.
var ErrInvalidPolicy = errors.New("hedge: invalid policy")

// Timer is owned by one execution. Stop releases its resources; callers must
// not close C.
type Timer interface {
	// C returns the timer notification channel; the execution receives but never closes it.
	C() <-chan time.Time
	// Stop releases timer resources without blocking or panicking and reports
	// whether it prevented the timer from firing.
	Stop() bool
}

// Clock supplies monotonic policy time, timers, and deadline contexts so
// scheduling can be tested deterministically. Implementations must be safe for
// concurrent use and must not panic.
type Clock interface {
	// Now supplies comparable completion times and elapsed-time measurements.
	Now() time.Time
	// NewTimer starts a non-nil timer with a usable notification channel.
	NewTimer(time.Duration) Timer
	// WithTimeout returns a non-nil derived context and cancel function, preserving
	// parent cancellation/deadlines. Cleanup passes Background as the parent.
	WithTimeout(context.Context, time.Duration) (context.Context, context.CancelFunc)
}

// RealClock uses the standard library's monotonic clock and timers. Its zero
// value is ready for concurrent use.
type RealClock struct{}

// Now returns the current time.
func (RealClock) Now() time.Time { return time.Now() }

// NewTimer starts a standard-library timer; non-positive delays fire immediately.
func (RealClock) NewTimer(delay time.Duration) Timer { return realTimer{Timer: time.NewTimer(delay)} }

// WithTimeout derives a standard-library timeout context. The caller must call
// the returned cancel function to release resources; parent must not be nil.
func (RealClock) WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

type realTimer struct{ *time.Timer }

// C exposes the standard-library timer channel without transferring closure ownership.
func (timer realTimer) C() <-chan time.Time { return timer.Timer.C }

// Classification is the explicit policy decision for one completed attempt.
type Classification uint8

const (
	// ClassificationSuccess makes the result eligible for winner selection.
	ClassificationSuccess Classification = iota + 1
	// ClassificationFailure retains the failure and permits active or scheduled
	// attempts to continue.
	ClassificationFailure
	// ClassificationCanceled records a non-downstream cancellation and permits
	// other attempts to continue.
	ClassificationCanceled
	// ClassificationTerminal stops the logical operation without a winner.
	ClassificationTerminal
)

// AttemptResult is passed to a Classifier. Value and Err are never included in
// observations or the package's wrapper error strings; unwrapped causes may
// retain the original error data.
type AttemptResult[T any] struct {
	// Value is the returned value, including partial/zero values on failure.
	// Classification does not transfer ownership or copy referenced state.
	Value T
	// Err is the attempt error, or a payload-free error for an attempt panic.
	Err error
	// ContextErr captures the attempt context error before classification.
	ContextErr error
	// Ordinal is zero for the original and the scheduling slot for a hedge.
	Ordinal uint
	// Hedge distinguishes additional attempts from the original.
	Hedge bool
}

// Classifier decides whether one result wins, fails, was canceled, or
// terminally stops the logical operation. It must be safe for concurrent use.
type Classifier[T any] interface {
	// Classify runs in the attempt worker, potentially concurrently with other
	// attempts and even after cancellation. It must finish cooperatively.
	// The decision is explicit: a non-nil attempt error does not automatically
	// prevent success. A returned error, invalid decision or panic is terminal.
	Classify(context.Context, AttemptResult[T]) (Classification, error)
}

// ClassifyFunc adapts a function to Classifier.
type ClassifyFunc[T any] func(context.Context, AttemptResult[T]) (Classification, error)

// Classify invokes the adapted function. Direct invocation of a nil function
// panics; NewPolicy rejects typed nil classifier dependencies.
func (function ClassifyFunc[T]) Classify(ctx context.Context, result AttemptResult[T]) (Classification, error) {
	return function(ctx, result)
}

// Disposer releases resources owned by every non-selected returned value. It
// must be concurrency-safe and honor its cleanup context. Selected partial
// failure values, like selected successes, belong to the caller.
type Disposer[T any] interface {
	// Dispose handles values from successful, failed or panicking attempts,
	// including zero values. Each call has a fresh Background-derived cleanup
	// deadline, without caller context values. Errors/panics are counted by Wait.
	Dispose(context.Context, T) error
}

// DisposeFunc adapts a function to Disposer.
type DisposeFunc[T any] func(context.Context, T) error

// Dispose invokes the adapted function. Direct invocation of a nil function
// panics; NewPolicy rejects typed nil disposer dependencies.
func (function DisposeFunc[T]) Dispose(ctx context.Context, value T) error {
	return function(ctx, value)
}

// Permit accounts for one additional attempt. Release must be idempotent,
// concurrency-safe, non-blocking, and must not panic.
type Permit interface {
	// Release relinquishes admission when a result is consumed or reclaimed,
	// before its disposal. Admission therefore does not bound cleanup work.
	Release()
}

// Budget bounds shared additional work. Implementations may account globally,
// per resource, or over a documented recent-work window. Capacity must be
// immutable, finite, and consistent with non-blocking, panic-free TryAcquire.
type Budget interface {
	// Capacity returns the immutable positive bound, no greater than MaxBudgetCapacity.
	Capacity() uint
	// TryAcquire returns a non-nil permit and true on admission, or nil and false
	// on denial. Implementations must be safe for concurrent use.
	TryAcquire(resource string) (Permit, bool)
}

// FactoryFailureMode defines what happens when a hedge attempt cannot be
// constructed. Original-attempt factory failure always stops execution.
type FactoryFailureMode uint8

const (
	// FactoryFailureStop stops the logical operation and cancels active attempts.
	FactoryFailureStop FactoryFailureMode = iota + 1
	// FactoryFailureContinue records the failed hedge and permits later work.
	FactoryFailureContinue
)

// DelayInput gives a dynamic delay policy bounded metadata. Dynamic functions
// must not retain hidden unbounded latency history.
type DelayInput struct {
	// Hedge is the next scheduling slot in [1, Config.MaxHedges], not a start count.
	Hedge uint
	// Previous is the preceding chosen delay, or zero for the first hedge slot.
	Previous time.Duration
}

// DelayFunc chooses a positive delay for the next scheduling slot. Denied
// admission and continued construction failure also advance slots. It runs
// synchronously, must return promptly, and must be safe for concurrent policies
// sharing it. An error, panic or non-positive duration stops the operation.
type DelayFunc func(DelayInput) (time.Duration, error)

// Observer receives bounded lifecycle metadata through a non-blocking method.
// TryObserve must return immediately and must not call back into an execution.
type Observer interface {
	// TryObserve may run concurrently from coordination, attempt and cleanup
	// paths. Its result is ignored; delivery/storage policy belongs to the observer.
	// Panics are contained, but blocking can delay execution or cleanup.
	TryObserve(Observation) bool
}

// Config defines every safety decision and dependency for a Policy. Exactly
// one of Delay, Schedule, or DynamicDelay must be configured.
type Config[T any] struct {
	// MaxHedges bounds additional scheduling slots, from one through MaxHedges.
	MaxHedges uint
	// ReplaySafe must be true: the caller explicitly permits concurrent replay.
	// The package cannot verify idempotency or clone request state.
	ReplaySafe bool
	// Delay is the positive repeated delay, or zero when another strategy is used.
	Delay time.Duration
	// Schedule contains exactly MaxHedges positive successive slot delays.
	// NewPolicy copies this slice; these are not offsets from the original start.
	Schedule []time.Duration
	// DynamicDelay chooses each positive slot delay instead of Delay or Schedule.
	DynamicDelay DelayFunc
	// TotalTimeout is a positive logical deadline. It does not forcibly interrupt
	// synchronous factories or other uncooperative caller callbacks.
	TotalTimeout time.Duration
	// AttemptTimeout is zero to use the logical context, or positive and at most
	// TotalTimeout. It begins after construction and covers the attempt/classifier.
	AttemptTimeout time.Duration
	// CleanupTimeout is a positive, separate cooperative deadline per disposal.
	CleanupTimeout time.Duration
	// Clock is the required shared time/deadline provider.
	Clock Clock
	// Budget owns additional admission unless UseResilienceBudget is true.
	Budget Budget
	// Classifier is the required explicit result decision policy.
	Classifier Classifier[T]
	// Disposer is required even when the value type has no resources to release.
	Disposer Disposer[T]
	// Observer optionally receives events; a typed nil observer is treated as absent.
	Observer Observer
	// Resource is a non-empty identity of at most MaxResourceLength bytes.
	// The caller must keep it credential-free and suitably low-cardinality.
	Resource string
	// FactoryFailureMode explicitly chooses stop or continue for hedge construction.
	FactoryFailureMode FactoryFailureMode
	// UseResilienceBudget requires exactly one v1 or v2 scope in the Do context
	// and an absent Budget. Shared scopes own original/hedge admission; an existing
	// physical attempt is borrowed and its permit remains owned by the outer layer.
	UseResilienceBudget bool
}

// Policy is immutable and safe for concurrent execution. Its Budget,
// Classifier, Disposer, Clock, and Observer dependencies must also be safe for
// concurrent use, as must shared DynamicDelay functions and attempt factories.
// The zero value is not an executable policy; use NewPolicy.
type Policy[T any] struct{ config Config[T] }

// NewPolicy validates finite amplification, time, replay, ownership, and
// dependency contracts. It copies Schedule but retains collaborator/function
// references. Validation errors wrap ErrInvalidPolicy; no workers are started.
func NewPolicy[T any](config Config[T]) (*Policy[T], error) {
	delayModes := 0
	if config.Delay != 0 {
		delayModes++
	}
	if len(config.Schedule) != 0 {
		delayModes++
	}
	if config.DynamicDelay != nil {
		delayModes++
	}

	switch {
	case config.MaxHedges == 0 || config.MaxHedges > MaxHedges:
		return nil, fmt.Errorf("%w: max hedges must be between 1 and %d", ErrInvalidPolicy, MaxHedges)
	case !config.ReplaySafe:
		return nil, fmt.Errorf("%w: concurrent replay safety must be declared", ErrInvalidPolicy)
	case delayModes != 1:
		return nil, fmt.Errorf("%w: exactly one delay strategy is required", ErrInvalidPolicy)
	case config.Delay < 0:
		return nil, fmt.Errorf("%w: fixed delay must be positive", ErrInvalidPolicy)
	case config.TotalTimeout <= 0:
		return nil, fmt.Errorf("%w: total timeout must be positive", ErrInvalidPolicy)
	case config.AttemptTimeout < 0 || config.AttemptTimeout > config.TotalTimeout:
		return nil, fmt.Errorf("%w: attempt timeout must be non-negative and no greater than total timeout", ErrInvalidPolicy)
	case config.CleanupTimeout <= 0:
		return nil, fmt.Errorf("%w: cleanup timeout must be positive", ErrInvalidPolicy)
	case nilLike(config.Clock):
		return nil, fmt.Errorf("%w: clock is required", ErrInvalidPolicy)
	case config.UseResilienceBudget && !nilLike(config.Budget):
		return nil, fmt.Errorf("%w: resilience and compatibility budgets cannot both own admission", ErrInvalidPolicy)
	case !config.UseResilienceBudget && nilLike(config.Budget):
		return nil, fmt.Errorf("%w: budget is required", ErrInvalidPolicy)
	case nilLike(config.Classifier):
		return nil, fmt.Errorf("%w: classifier is required", ErrInvalidPolicy)
	case nilLike(config.Disposer):
		return nil, fmt.Errorf("%w: disposer is required", ErrInvalidPolicy)
	case config.Resource == "" || len(config.Resource) > MaxResourceLength:
		return nil, fmt.Errorf("%w: bounded resource identity is required", ErrInvalidPolicy)
	case config.FactoryFailureMode != FactoryFailureStop && config.FactoryFailureMode != FactoryFailureContinue:
		return nil, fmt.Errorf("%w: factory failure mode is required", ErrInvalidPolicy)
	}
	if !config.UseResilienceBudget {
		capacity := safeBudgetCapacity(config.Budget)
		if capacity == 0 || capacity > MaxBudgetCapacity {
			return nil, fmt.Errorf("%w: budget capacity must be between 1 and %d", ErrInvalidPolicy, MaxBudgetCapacity)
		}
	}

	if len(config.Schedule) != 0 {
		if uint(len(config.Schedule)) != config.MaxHedges {
			return nil, fmt.Errorf("%w: schedule must define every hedge delay", ErrInvalidPolicy)
		}
		for _, delay := range config.Schedule {
			if delay <= 0 {
				return nil, fmt.Errorf("%w: scheduled delays must be positive", ErrInvalidPolicy)
			}
		}
		config.Schedule = append([]time.Duration(nil), config.Schedule...)
	}
	if config.Observer != nil && nilLike(config.Observer) {
		config.Observer = nil
	}

	return &Policy[T]{config: config}, nil
}

func safeBudgetCapacity(budget Budget) (capacity uint) {
	defer func() {
		if recover() != nil {
			capacity = 0
		}
	}()
	return budget.Capacity()
}

func nilLike(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() { //nolint:exhaustive // Only nil-capable kinds require distinct handling.
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
