package hedge_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/faustbrian/go-hedge"
)

// Gate the caller error check after the total deadline is selected. The
// underlying context still owns cancellation and all returned context state.
type gatedCallerError struct {
	context.Context
	reads   atomic.Uint32
	entered chan struct{}
	release chan struct{}
}

func (caller *gatedCallerError) Err() error {
	if caller.reads.Add(1) == 2 {
		close(caller.entered)
		<-caller.release
	}
	return caller.Context.Err()
}

func TestCallerCancellationAfterTotalDeadlineStopsScheduledTimer(t *testing.T) {
	clock := newManualClock()
	config := validConfig()
	config.Clock = clock
	config.Delay = time.Hour
	config.TotalTimeout = time.Minute
	policy, err := hedge.NewPolicy(config)
	if err != nil {
		t.Fatal(err)
	}
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	caller := &gatedCallerError{Context: parent, entered: make(chan struct{}), release: make(chan struct{})}
	defer close(caller.release)
	type outcome struct {
		report hedge.Report
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		_, report, doErr := hedge.Do(caller, policy, hedge.AttemptFactoryFunc[string](func(hedge.AttemptInfo) (hedge.Attempt[string], string, error) {
			return func(ctx context.Context) (string, error) {
				<-ctx.Done()
				return "", ctx.Err()
			}, "endpoint", nil
		}))
		done <- outcome{report: report, err: doErr}
	}()
	setupCtx, setupCancel := context.WithTimeout(context.Background(), time.Second)
	defer setupCancel()
	for {
		clock.mu.Lock()
		ready := len(clock.timers) >= 2
		clock.mu.Unlock()
		if ready {
			break
		}
		select {
		case <-clock.changed:
		case <-setupCtx.Done():
			t.Fatal("execution did not create its deadline and hedge timers")
		}
	}
	clock.mu.Lock()
	scheduled := clock.timers[1]
	clock.mu.Unlock()
	clock.Advance(config.TotalTimeout)
	select {
	case <-caller.entered:
	case <-time.After(time.Second):
		t.Fatal("total deadline did not reach the caller error check")
	}
	cancel()
	caller.release <- struct{}{}
	select {
	case result := <-done:
		if !errors.Is(result.err, context.Canceled) || result.report.Reason != hedge.ReasonCallerCanceled || result.report.HedgesStarted != 0 {
			t.Fatalf("cancellation outcome = (%+v, %v)", result.report, result.err)
		}
		waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
		defer waitCancel()
		if err := result.report.Wait(waitCtx); err != nil {
			t.Fatalf("canceled attempt cleanup = %v", err)
		}
		select {
		case <-scheduled.stopped:
		default:
			t.Fatal("caller cancellation left the scheduled hedge timer active")
		}
	case <-time.After(time.Second):
		t.Fatal("caller cancellation did not terminate execution")
	}
}
