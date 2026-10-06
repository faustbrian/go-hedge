package hedge

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	resilience2 "github.com/faustbrian/go-resilience/v2"
)

func TestSelectedV2BudgetClassifiesRealAdmissionErrorsDirectly(t *testing.T) {
	budget, err := resilience2.NewBudget(resilience2.BudgetConfig{
		MaxResources: 1, MaxScopes: 1, MaxAdditionalPerExecution: 1,
		MaxConcurrentAdditional: 1, MaxAdditionalPerWindow: 1,
		AdditionalWindow: time.Minute, PermitTTL: time.Minute, Clock: RealClock{},
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := resilience2.NewMetadata("logical", "lookup", "dependency")
	if err != nil {
		t.Fatal(err)
	}
	scope, ctx, err := budget.Start(context.Background(), metadata)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = scope.Close() })
	_, original, originalPermit, err := resilience2.AdmitAttempt(ctx, resilience2.OriginOriginal, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = originalPermit.Complete() })
	_, _, additionalPermit, err := resilience2.AdmitAttempt(ctx, resilience2.OriginHedge, original.Ordinal, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = additionalPermit.Complete() })
	_, _, _, capacity := resilience2.AdmitAttempt(ctx, resilience2.OriginHedge, original.Ordinal, time.Now())
	if resilience2.RejectionReasonOf(capacity) != resilience2.ReasonExecutionLimit || !errors.Is(capacity, resilience2.ErrBudgetRejected) {
		t.Fatalf("capacity error=%v", capacity)
	}
	unknown, err := resilience2.NewAttempt(5, resilience2.OriginHedge, 4, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, lineage := scope.Acquire(ctx, unknown)
	if resilience2.RejectionReasonOf(lineage) != resilience2.ReasonUnknownParent {
		t.Fatalf("lineage error=%v", lineage)
	}
	for _, test := range []struct {
		name     string
		err      error
		capacity bool
	}{
		{"direct capacity", capacity, true},
		{"invalid lineage", lineage, false},
		{"wrapped capacity", fmt.Errorf("outer: %w", capacity), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := sharedBudgetV2.capacityDenial(test.err); got != test.capacity {
				t.Fatalf("capacity denial=%v want=%v", got, test.capacity)
			}
		})
	}
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, _, _, canceled := resilience2.AdmitAttempt(canceledCtx, resilience2.OriginHedge, original.Ordinal, time.Now())
	if canceled != context.Canceled || !sharedBudgetV2.cancellation(canceled) || sharedBudgetV2.cancellation(fmt.Errorf("outer: %w", canceled)) { //nolint:errorlint // The v2 contract requires direct sentinel identity; wrapped cancellation must not qualify.
		t.Fatalf("cancellation identity=%v", canceled)
	}
	if err := additionalPermit.Complete(); err != nil {
		t.Fatal(err)
	}
	if err := originalPermit.Complete(); err != nil {
		t.Fatal(err)
	}
	if snapshot := scope.Snapshot(); snapshot.AdditionalAdmitted != 1 || snapshot.AdditionalActive != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
}
