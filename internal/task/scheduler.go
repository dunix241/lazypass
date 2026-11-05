// Package task manages cancellable background work scoped to an application.
// Callbacks never run on the submitting goroutine or a UI event loop.
package task

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrClosed    = errors.New("task scheduler is closed")
	ErrDuplicate = errors.New("task ID is already registered")
	ErrCapacity  = errors.New("task scheduler capacity reached")
)

// Task describes a job. Delay applies before its first run. Interval repeats
// after each completed run, so executions never overlap or accumulate a backlog.
// Timeout, when set, applies separately to each invocation.
type Task struct {
	ID       string
	Run      func(context.Context) error
	Delay    time.Duration
	Interval time.Duration
	Timeout  time.Duration
}

type Options struct {
	// MaxTasks bounds pending and running jobs. Zero selects 64.
	MaxTasks int
	// OnError runs in the job's goroutine and must be concurrency-safe. Cancelled
	// invocations are excluded. Recurring jobs continue after ordinary errors.
	OnError func(id string, err error)
}

type Scheduler struct {
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	ready     chan struct{}
	readyOnce sync.Once
	closed    bool
	jobs      map[string]struct{}
	done      chan struct{}
	options   Options
}

func New(ctx context.Context, options Options) *Scheduler {
	ctx, cancel := context.WithCancel(ctx)
	if options.MaxTasks <= 0 {
		options.MaxTasks = 64
	}
	return &Scheduler{ctx: ctx, cancel: cancel, ready: make(chan struct{}), done: make(chan struct{}), jobs: make(map[string]struct{}), options: options}
}

// Ready opens the lifecycle gate once. It only signals workers; no task callback
// runs here. OnReady jobs registered after this call may start immediately.
func (s *Scheduler) Ready() { s.readyOnce.Do(func() { close(s.ready) }) }

func (s *Scheduler) Submit(job Task) (*Handle, error)  { return s.submit(job, false) }
func (s *Scheduler) OnReady(job Task) (*Handle, error) { return s.submit(job, true) }

func (s *Scheduler) submit(job Task, ready bool) (*Handle, error) {
	if job.ID == "" || job.Run == nil || job.Delay < 0 || job.Interval < 0 || job.Timeout < 0 {
		return nil, errors.New("task requires an ID, callback, and nonnegative durations")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.ctx.Err() != nil {
		return nil, ErrClosed
	}
	if _, exists := s.jobs[job.ID]; exists {
		return nil, ErrDuplicate
	}
	if len(s.jobs) >= s.options.MaxTasks {
		return nil, ErrCapacity
	}
	ctx, cancel := context.WithCancel(s.ctx)
	h := &Handle{cancel: cancel, done: make(chan struct{})}
	s.jobs[job.ID] = struct{}{}
	go s.run(ctx, job, h, ready)
	return h, nil
}

func (s *Scheduler) run(ctx context.Context, job Task, h *Handle, ready bool) {
	defer func() {
		h.cancel()
		s.mu.Lock()
		delete(s.jobs, job.ID)
		close(h.done)
		s.finish()
		s.mu.Unlock()
	}()
	if ready {
		select {
		case <-ctx.Done():
			h.err = ctx.Err()
			return
		case <-s.ready:
		}
	}
	if !wait(ctx, job.Delay) {
		h.err = ctx.Err()
		return
	}
	for {
		if ctx.Err() != nil {
			h.err = ctx.Err()
			return
		}
		invocation := ctx
		cancel := func() {}
		if job.Timeout > 0 {
			invocation, cancel = context.WithTimeout(ctx, job.Timeout)
		}
		err := job.Run(invocation)
		if err == nil {
			err = invocation.Err()
		}
		cancel()
		if err != nil && ctx.Err() == nil && s.options.OnError != nil {
			s.options.OnError(job.ID, err)
		}
		if job.Interval == 0 {
			h.err = err
			return
		}
		if !wait(ctx, job.Interval) {
			h.err = ctx.Err()
			return
		}
	}
}

func wait(ctx context.Context, delay time.Duration) bool {
	if delay == 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}

// Close prevents new registrations and cancels all work without waiting. Tasks
// must honor their context; neither Close nor a timeout forcibly stops Go code.
func (s *Scheduler) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.cancel()
	s.finish()
}

// Called with mu held, after registration has been closed.
func (s *Scheduler) finish() {
	if s.closed && len(s.jobs) == 0 {
		close(s.done)
	}
}

// Wait joins tasks after Close, or returns when ctx expires. Do not call it from
// a task callback or an event loop needed by an outstanding callback.
func (s *Scheduler) Wait(ctx context.Context) error {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if !closed {
		return errors.New("close scheduler before waiting")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return nil
	}
}

type Handle struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func (h *Handle) Cancel()               { h.cancel() }
func (h *Handle) Done() <-chan struct{} { return h.done }

// Wait returns the final result; for recurring jobs this is normally cancellation.
func (h *Handle) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-h.done:
		return h.err
	}
}
