package util

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSafeGo_NormalExecution(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	executed := false
	panicked := false

	SafeGo(context.Background(), "test-normal-worker", func(ctx context.Context) {
		defer wg.Done()
		executed = true
	}, func(ctx context.Context, worker string, r any, stack []byte) {
		panicked = true
	})

	wg.Wait()

	if !executed {
		t.Errorf("expected worker func to execute")
	}
	if panicked {
		t.Errorf("expected onPanic not to be called on normal execution")
	}
}

func TestSafeGo_PanicInterception(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	var recoveredWorker string
	var recoveredVal any
	var recoveredStack []byte

	SafeGo(context.Background(), "test-panic-worker", func(ctx context.Context) {
		panic("simulated fatal crash")
	}, func(ctx context.Context, worker string, r any, stack []byte) {
		defer wg.Done()
		recoveredWorker = worker
		recoveredVal = r
		recoveredStack = stack
	})

	wg.Wait()

	if recoveredWorker != "test-panic-worker" {
		t.Errorf("expected worker 'test-panic-worker', got %q", recoveredWorker)
	}
	if recoveredVal != "simulated fatal crash" {
		t.Errorf("expected panic val 'simulated fatal crash', got %v", recoveredVal)
	}
	if len(recoveredStack) == 0 {
		t.Errorf("expected non-empty stack trace")
	}
}

func TestSafeGo_PanicInHookIsRecovered(t *testing.T) {
	// Ensure that even if the onPanic handler itself panics, the process does not terminate
	done := make(chan struct{})

	SafeGo(context.Background(), "test-double-panic", func(ctx context.Context) {
		panic("first panic")
	}, func(ctx context.Context, worker string, r any, stack []byte) {
		defer close(done)
		panic("panic inside onPanic hook")
	})

	select {
	case <-done:
		// success: hook ran and finished without taking down the test process
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for safe go hook execution")
	}
}
