package hedge_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/faustbrian/go-hedge"
	resilience1 "github.com/faustbrian/go-resilience"
	resilience2 "github.com/faustbrian/go-resilience/v2"
)

func newV2Scope(ctx context.Context, t *testing.T, additional uint64) (resilience2.WorkBudgetScope, context.Context) {
	t.Helper()
	budget, err := resilience2.NewBudget(resilience2.BudgetConfig{
		MaxResources: 1, MaxScopes: 1, MaxAdditionalPerExecution: additional,
		MaxConcurrentAdditional: additional, MaxAdditionalPerWindow: additional,
		AdditionalWindow: time.Minute, PermitTTL: time.Minute, Clock: hedge.RealClock{},
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := resilience2.NewMetadata("logical", "lookup", "dependency")
	if err != nil {
		t.Fatal(err)
	}
	scope, attached, err := budget.Start(ctx, metadata)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = scope.Close() })
	return scope, attached
}

func newSharedPolicy(t *testing.T, clock hedge.Clock, hedges uint) *hedge.Policy[string] {
	t.Helper()
	config := validConfig()
	config.Clock = clock
	config.MaxHedges = hedges
	config.Budget = nil
	config.UseResilienceBudget = true
	policy, err := hedge.NewPolicy(config)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func TestHedgeResilienceV2AdmitsOneAdditionalAndRefusesTheNext(t *testing.T) {
	scope, ctx := newV2Scope(context.Background(), t, 1)
	clock := newManualClock()
	config := validConfig()
	config.Clock, config.MaxHedges, config.Budget, config.UseResilienceBudget = clock, 2, nil, true
	denied := make(outcomeSignal, 1)
	config.Observer = denied
	policy, err := hedge.NewPolicy(config)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan resilience2.Attempt, 2)
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	factory := hedge.AttemptFactoryFunc[string](func(info hedge.AttemptInfo) (hedge.Attempt[string], string, error) {
		return func(ctx context.Context) (string, error) {
			attempt, ok := resilience2.AttemptFromContext(ctx)
			if !ok {
				return "", errors.New("missing v2 attempt")
			}
			started <- attempt
			if info.Ordinal == 0 {
				select {
				case <-release:
					return "original", nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}
			<-ctx.Done()
			return "", ctx.Err()
		}, "pod", nil
	})
	type result struct {
		value  string
		report hedge.Report
		err    error
	}
	done := make(chan result, 1)
	go func() { value, report, err := hedge.Do(ctx, policy, factory); done <- result{value, report, err} }()
	original := receiveWithin(t, started)
	if original.Ordinal != 1 || original.Origin != resilience2.OriginOriginal || original.ParentOrdinal != 0 {
		t.Fatalf("original lineage = %+v", original)
	}
	clock.WaitTimers(2)
	clock.Advance(time.Millisecond)
	additional := receiveWithin(t, started)
	if additional.Ordinal != 2 || additional.Origin != resilience2.OriginHedge || additional.ParentOrdinal != original.Ordinal {
		t.Fatalf("hedge lineage = %+v", additional)
	}
	clock.WaitTimers(3)
	clock.Advance(time.Millisecond)
	if got := receiveWithin(t, denied); got != hedge.OutcomeBudgetDenied {
		t.Fatalf("outcome = %v", got)
	}
	close(release)
	got := receiveWithin(t, done)
	if got.err != nil || got.value != "original" || got.report.AttemptsStarted != 2 || got.report.HedgesStarted != 1 || got.report.BudgetDenied != 1 {
		t.Fatalf("result = %+v", got)
	}
	if err := got.report.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if snapshot := scope.Snapshot(); snapshot.AdditionalAdmitted != 1 || snapshot.AdditionalActive != 0 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	if !scope.Snapshot().Closed {
		t.Fatal("scope did not close")
	}
}

func TestHedgeResilienceV2BorrowsAnOuterAdditionalAttempt(t *testing.T) {
	scope, ctx := newV2Scope(context.Background(), t, 1)
	_, original, originalPermit, err := resilience2.AdmitAttempt(ctx, resilience2.OriginOriginal, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := originalPermit.Complete(); err != nil {
		t.Fatal(err)
	}
	borrowedCtx, borrowed, permit, err := resilience2.AdmitAttempt(ctx, resilience2.OriginRetry, original.Ordinal, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = permit.Complete() })
	policy := newSharedPolicy(t, hedge.RealClock{}, 1)
	seen := make(chan resilience2.Attempt, 1)
	value, report, err := hedge.Do(borrowedCtx, policy, hedge.AttemptFactoryFunc[string](func(hedge.AttemptInfo) (hedge.Attempt[string], string, error) {
		return func(ctx context.Context) (string, error) {
			current, _ := resilience2.AttemptFromContext(ctx)
			seen <- current
			return "ok", nil
		}, "pod", nil
	}))
	if err != nil || value != "ok" || report.AttemptsStarted != 1 || report.HedgesStarted != 0 {
		t.Fatalf("value=%q report=%+v err=%v", value, report, err)
	}
	if current := receiveWithin(t, seen); current != borrowed {
		t.Fatalf("borrowed lineage=%+v want=%+v", current, borrowed)
	}
	if err := report.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if snapshot := scope.Snapshot(); snapshot.AdditionalAdmitted != 1 || snapshot.AdditionalActive != 1 {
		t.Fatalf("borrowed ownership changed: %+v", snapshot)
	}
	if err := permit.Complete(); err != nil {
		t.Fatalf("Hedge completed borrowed permit: %v", err)
	}
	if snapshot := scope.Snapshot(); snapshot.AdditionalActive != 0 {
		t.Fatalf("active=%+v", snapshot)
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestHedgeResilienceV2MissingScopeAndPreCanceledIdentity(t *testing.T) {
	policy := newSharedPolicy(t, hedge.RealClock{}, 1)
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "pre-canceled"}[canceled], func(t *testing.T) {
			ctx := context.Background()
			var scope resilience2.WorkBudgetScope
			if canceled {
				scope, ctx = newV2Scope(ctx, t, 1)
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			called := false
			_, report, err := hedge.Do(ctx, policy, hedge.AttemptFactoryFunc[string](func(hedge.AttemptInfo) (hedge.Attempt[string], string, error) {
				called = true
				return func(context.Context) (string, error) { return "", nil }, "pod", nil
			}))
			if called || report.AttemptsStarted != 0 {
				t.Fatalf("dispatched: called=%v report=%+v", called, report)
			}
			if canceled {
				if !errors.Is(err, context.Canceled) || report.Reason != hedge.ReasonCallerCanceled {
					t.Fatalf("cancellation=%v report=%+v", err, report)
				}
				if snapshot := scope.Snapshot(); snapshot.AdditionalAdmitted != 0 || snapshot.AdditionalActive != 0 {
					t.Fatalf("canceled charges=%+v", snapshot)
				}
			} else if !errors.Is(err, resilience1.ErrBudgetScopeRequired) || report.Reason != hedge.ReasonBudgetFailure {
				t.Fatalf("missing scope=%v report=%+v", err, report)
			}
		})
	}
}

func TestHedgeRejectsBothResilienceScopeVersionsBeforeDispatch(t *testing.T) {
	for _, firstV2 := range []bool{false, true} {
		t.Run(map[bool]string{false: "v1-then-v2", true: "v2-then-v1"}[firstV2], func(t *testing.T) {
			legacy, err := resilience1.NewBudget(resilience1.BudgetConfig{MaxResources: 1, MaxAdditionalPerExecution: 1, MaxConcurrentAdditional: 1, MaxAdditionalPerWindow: 1, AdditionalWindow: time.Minute, PermitTTL: time.Minute, Clock: hedge.RealClock{}})
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := resilience1.NewMetadata("logical", "lookup", "dependency")
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			var oldScope resilience1.WorkBudgetScope
			var newScope resilience2.WorkBudgetScope
			if firstV2 {
				newScope, ctx = newV2Scope(ctx, t, 1)
				oldScope, ctx, err = legacy.Start(ctx, metadata)
			} else {
				oldScope, ctx, err = legacy.Start(ctx, metadata)
				if err == nil {
					newScope, ctx = newV2Scope(ctx, t, 1)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = oldScope.Close() })
			called := false
			_, report, err := hedge.Do(ctx, newSharedPolicy(t, hedge.RealClock{}, 1), hedge.AttemptFactoryFunc[string](func(hedge.AttemptInfo) (hedge.Attempt[string], string, error) {
				called = true
				return func(context.Context) (string, error) { return "ok", nil }, "pod", nil
			}))
			if called || !errors.Is(err, hedge.ErrInvalidPolicy) || report.Reason != hedge.ReasonBudgetFailure || report.AttemptsStarted != 0 {
				t.Fatalf("called=%v report=%+v error=%v", called, report, err)
			}
			if oldScope.Snapshot().AdditionalAdmitted != 0 || newScope.Snapshot().AdditionalAdmitted != 0 {
				t.Fatal("dual owners were charged")
			}
			// No original may have been charged either: both owners must still admit it.
			for _, version := range []int{1, 2} {
				if version == 1 {
					_, _, permit, err := resilience1.AdmitAttempt(ctx, resilience1.OriginOriginal, 0, time.Now())
					if err != nil {
						t.Fatal(err)
					}
					if err := permit.Complete(); err != nil {
						t.Fatal(err)
					}
				} else {
					_, _, permit, err := resilience2.AdmitAttempt(ctx, resilience2.OriginOriginal, 0, time.Now())
					if err != nil {
						t.Fatal(err)
					}
					if err := permit.Complete(); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := oldScope.Close(); err != nil {
				t.Fatal(err)
			}
			if err := newScope.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHedgeResilienceV2ClosedScopeRetainsItsErrorIdentity(t *testing.T) {
	scope, ctx := newV2Scope(context.Background(), t, 1)
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	called := false
	_, report, err := hedge.Do(ctx, newSharedPolicy(t, hedge.RealClock{}, 1), hedge.AttemptFactoryFunc[string](func(hedge.AttemptInfo) (hedge.Attempt[string], string, error) {
		called = true
		return func(context.Context) (string, error) { return "ok", nil }, "pod", nil
	}))
	if called || !errors.Is(err, resilience2.ErrBudgetClosed) || errors.Is(err, resilience1.ErrBudgetClosed) || report.Reason != hedge.ReasonBudgetFailure || report.AttemptsStarted != 0 {
		t.Fatalf("called=%v report=%+v err=%v", called, report, err)
	}
	if snapshot := scope.Snapshot(); snapshot.AdditionalAdmitted != 0 || snapshot.AdditionalActive != 0 || !snapshot.Closed {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestHedgeResilienceV2AdditionalDescendsFromBorrowedAttempt(t *testing.T) {
	scope, ctx := newV2Scope(context.Background(), t, 2)
	_, original, originalPermit, err := resilience2.AdmitAttempt(ctx, resilience2.OriginOriginal, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := originalPermit.Complete(); err != nil {
		t.Fatal(err)
	}
	borrowedCtx, borrowed, borrowedPermit, err := resilience2.AdmitAttempt(ctx, resilience2.OriginRetry, original.Ordinal, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = borrowedPermit.Complete() })
	clock := newManualClock()
	policy := newSharedPolicy(t, clock, 1)
	started := make(chan resilience2.Attempt, 2)
	factory := hedge.AttemptFactoryFunc[string](func(info hedge.AttemptInfo) (hedge.Attempt[string], string, error) {
		return func(ctx context.Context) (string, error) {
			attempt, ok := resilience2.AttemptFromContext(ctx)
			if !ok {
				return "", errors.New("missing v2 attempt")
			}
			started <- attempt
			if info.Ordinal == 0 {
				<-ctx.Done()
				return "", ctx.Err()
			}
			return "hedged", nil
		}, "pod", nil
	})
	type result struct {
		value  string
		report hedge.Report
		err    error
	}
	done := make(chan result, 1)
	go func() {
		value, report, err := hedge.Do(borrowedCtx, policy, factory)
		done <- result{value, report, err}
	}()
	if seen := receiveWithin(t, started); seen != borrowed {
		t.Fatalf("original=%+v want borrowed=%+v", seen, borrowed)
	}
	clock.WaitTimers(2)
	clock.Advance(time.Millisecond)
	child := receiveWithin(t, started)
	if child.Ordinal != 3 || child.ParentOrdinal != borrowed.Ordinal || child.Origin != resilience2.OriginHedge {
		t.Fatalf("child lineage=%+v borrowed=%+v", child, borrowed)
	}
	got := receiveWithin(t, done)
	if got.err != nil || got.value != "hedged" || got.report.AttemptsStarted != 2 || got.report.HedgesStarted != 1 || got.report.BudgetDenied != 0 {
		t.Fatalf("result=%+v", got)
	}
	if err := got.report.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if snapshot := scope.Snapshot(); snapshot.AdditionalAdmitted != 2 || snapshot.AdditionalActive != 1 {
		t.Fatalf("borrowed ownership=%+v", snapshot)
	}
	if err := borrowedPermit.Complete(); err != nil {
		t.Fatalf("borrowed permit completed by Hedge: %v", err)
	}
	if snapshot := scope.Snapshot(); snapshot.AdditionalActive != 0 {
		t.Fatalf("active=%+v", snapshot)
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
}
