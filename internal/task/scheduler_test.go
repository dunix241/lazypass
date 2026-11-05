package task

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func scheduler(t *testing.T, options Options) *Scheduler {
	t.Helper()
	s := New(context.Background(), options)
	t.Cleanup(func() {
		s.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := s.Wait(ctx); err != nil {
			t.Error(err)
		}
	})
	return s
}

func await(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for task")
	}
}

func TestReadyGateAndLateRegistration(t *testing.T) {
	s := scheduler(t, Options{})
	var calls atomic.Int32
	h, err := s.OnReady(Task{ID: "first", Run: func(context.Context) error { calls.Add(1); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-h.Done():
		t.Fatal("ran before readiness")
	default:
	}
	for i := 0; i < 10; i++ {
		s.Ready()
	}
	await(t, h.Done())
	if calls.Load() != 1 {
		t.Fatal("ready task repeated")
	}
	h, err = s.OnReady(Task{ID: "late", Run: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	await(t, h.Done())
}

func TestCapacityDuplicateAndCancellation(t *testing.T) {
	s := scheduler(t, Options{MaxTasks: 1})
	h, err := s.OnReady(Task{ID: "pending", Run: func(context.Context) error { t.Error("cancelled task ran"); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Submit(Task{ID: "pending", Run: func(context.Context) error { return nil }}); !errors.Is(err, ErrDuplicate) {
		t.Fatal(err)
	}
	if _, err := s.Submit(Task{ID: "other", Run: func(context.Context) error { return nil }}); !errors.Is(err, ErrCapacity) {
		t.Fatal(err)
	}
	h.Cancel()
	await(t, h.Done())
	if err := h.Wait(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	s.Close()
	if _, err := s.Submit(Task{ID: "closed", Run: func(context.Context) error { return nil }}); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestRecurringTimeoutAndNoOverlap(t *testing.T) {
	errorsSeen := make(chan error, 8)
	s := scheduler(t, Options{OnError: func(_ string, err error) { errorsSeen <- err }})
	var active, maximum, calls atomic.Int32
	h, err := s.Submit(Task{ID: "repeat", Interval: time.Millisecond, Timeout: 5 * time.Millisecond, Run: func(ctx context.Context) error {
		n := active.Add(1)
		if n > maximum.Load() {
			maximum.Store(n)
		}
		defer active.Add(-1)
		calls.Add(1)
		<-ctx.Done()
		return ctx.Err()
	}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case err := <-errorsSeen:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout not reported")
		}
	}
	h.Cancel()
	await(t, h.Done())
	if maximum.Load() != 1 || calls.Load() < 2 {
		t.Fatalf("overlap=%d calls=%d", maximum.Load(), calls.Load())
	}
}

func TestShutdownCancelsReadyDelayedAndRunningTasks(t *testing.T) {
	s := scheduler(t, Options{})
	var handles []*Handle
	for _, job := range []Task{
		{ID: "delayed", Delay: time.Hour, Run: func(context.Context) error { t.Error("delay ignored"); return nil }},
		{ID: "running", Run: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }},
	} {
		h, err := s.Submit(job)
		if err != nil {
			t.Fatal(err)
		}
		handles = append(handles, h)
	}
	h, err := s.OnReady(Task{ID: "ready", Run: func(context.Context) error { t.Error("ready gate ignored"); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	handles = append(handles, h)
	s.Close()
	for _, h := range handles {
		await(t, h.Done())
	}
}

func TestConcurrentRegistrationsAndClose(t *testing.T) {
	s := scheduler(t, Options{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Ready()
			_, _ = s.Submit(Task{ID: "same", Run: func(context.Context) error { return nil }})
			s.Close()
		}()
	}
	wg.Wait()
}

func TestDelayAndIndependentExecution(t *testing.T) {
	s := scheduler(t, Options{})
	started := make(chan struct{})
	slow, err := s.Submit(Task{ID: "slow", Run: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}})
	if err != nil {
		t.Fatal(err)
	}
	await(t, started)
	begin := time.Now()
	fast, err := s.Submit(Task{ID: "delayed", Delay: 10 * time.Millisecond, Run: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	await(t, fast.Done())
	if time.Since(begin) < 10*time.Millisecond {
		t.Fatal("task ran before delay")
	}
	slow.Cancel()
	await(t, slow.Done())
}
