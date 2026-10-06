package hedge

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/faustbrian/go-resilience"
	resilience2 "github.com/faustbrian/go-resilience/v2"
)

// sharedBudgetVersion is selected once from scope ownership, not inferred from
// an error or borrowed attempt. The two public context namespaces are distinct.
type sharedBudgetVersion uint8

const (
	sharedBudgetV1 sharedBudgetVersion = iota + 1
	sharedBudgetV2
)

func selectSharedBudget(ctx context.Context) (sharedBudgetVersion, error) {
	_, legacy := resilience.BudgetScopeFromContext(ctx)
	_, current := resilience2.BudgetScopeFromContext(ctx)
	switch {
	case legacy && current:
		return 0, fmt.Errorf("%w: multiple resilience budget versions attached", ErrInvalidPolicy)
	case current:
		return sharedBudgetV2, nil
	case legacy:
		return sharedBudgetV1, nil
	default:
		return 0, resilience.ErrBudgetScopeRequired
	}
}

func (version sharedBudgetVersion) original(ctx context.Context, now time.Time) (context.Context, uint64, Permit, error) {
	if version == sharedBudgetV2 {
		if current, ok := resilience2.AttemptFromContext(ctx); ok {
			return ctx, current.Ordinal, nil, nil
		}
		admittedCtx, attempt, permit, err := resilience2.AdmitAttempt(ctx, resilience2.OriginOriginal, 0, now)
		return admittedCtx, attempt.Ordinal, wrapResiliencePermit(permit), err
	}
	if current, ok := resilience.AttemptFromContext(ctx); ok {
		return ctx, current.Ordinal, nil, nil
	}
	admittedCtx, attempt, permit, err := resilience.AdmitAttempt(ctx, resilience.OriginOriginal, 0, now)
	return admittedCtx, attempt.Ordinal, wrapResiliencePermit(permit), err
}

func (version sharedBudgetVersion) hedge(ctx context.Context, parent uint64, now time.Time) (context.Context, Permit, error) {
	if version == sharedBudgetV2 {
		admittedCtx, _, permit, err := resilience2.AdmitAttempt(ctx, resilience2.OriginHedge, parent, now)
		return admittedCtx, wrapResiliencePermit(permit), err
	}
	admittedCtx, _, permit, err := resilience.AdmitAttempt(ctx, resilience.OriginHedge, parent, now)
	return admittedCtx, wrapResiliencePermit(permit), err
}

func (version sharedBudgetVersion) capacityDenial(err error) bool {
	if version == sharedBudgetV2 {
		switch resilience2.RejectionReasonOf(err) {
		case resilience2.ReasonExecutionLimit, resilience2.ReasonConcurrentLimit, resilience2.ReasonWindowLimit:
			return true
		case resilience2.ReasonResourceLimit, resilience2.ReasonScopeLimit, resilience2.ReasonDuplicateWork, resilience2.ReasonOriginalRequired, resilience2.ReasonUnknownParent:
			return false
		default:
			return false
		}
	}
	return isCapacityDenial(err)
}

func (version sharedBudgetVersion) cancellation(err error) bool {
	if version == sharedBudgetV2 {
		// Only direct cancellation qualifies on the v2 route; do not invoke
		// custom scope error hooks through legacy traversal.
		return err == context.Canceled || err == context.DeadlineExceeded //nolint:errorlint // The selected v2 route requires direct sentinel identity, without caller error traversal.
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

type sharedWorkPermit struct {
	permit resilience.Permit
	once   sync.Once
}

func wrapResiliencePermit(permit resilience.Permit) Permit {
	if permit == nil {
		return nil
	}
	return &sharedWorkPermit{permit: permit}
}

func (permit *sharedWorkPermit) Release() {
	if permit == nil {
		return
	}
	permit.once.Do(func() { _ = permit.permit.Complete() })
}
