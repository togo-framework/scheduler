// Package scheduler runs recurring jobs on a cron schedule, dispatched over the
// togo queue (Laravel Scheduler / Celery Beat for togo).
//
// Register a job handler once, then schedule it with a cron expression or a
// fluent helper. On each tick the scheduler dispatches the job over the kernel
// Queue (so long jobs never block) — or runs it inline when no queue is
// configured. Overlap is prevented (a job is skipped if its previous run is
// still in flight) and every run is recorded for inspection.
//
//	scheduler.Register("cleanup", func(ctx context.Context) error { return purge(ctx) })
//	s, _ := scheduler.FromKernel(k)
//	s.Schedule("cleanup", scheduler.EveryFiveMinutes)
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/togo-framework/togo"
)

// JobFunc is a scheduled job handler.
type JobFunc func(ctx context.Context) error

var (
	regMu sync.RWMutex
	jobs  = map[string]JobFunc{}
)

// Register registers a named job handler. Call it from a plugin's init() or
// Boot; schedule the job later with Service.Schedule.
func Register(name string, fn JobFunc) {
	regMu.Lock()
	jobs[name] = fn
	regMu.Unlock()
}

// Registered reports whether a job handler with this name exists.
func Registered(name string) bool {
	regMu.RLock()
	_, ok := jobs[name]
	regMu.RUnlock()
	return ok
}

// Run status values.
const (
	StatusDone    = "done"
	StatusFailed  = "failed"
	StatusSkipped = "skipped" // previous run still in flight (overlap prevented)
)

// Run is a single execution of a scheduled job.
type Run struct {
	Job      string     `json:"job"`
	Status   string     `json:"status"`
	Started  time.Time  `json:"started"`
	Finished *time.Time `json:"finished,omitempty"`
	Error    string     `json:"error,omitempty"`
}

type schedule struct {
	Name    string
	Spec    string
	entryID cron.EntryID
	paused  bool
}

const queueName = "scheduler.run"

// Service is the scheduler runtime stored on the kernel (k.Get("scheduler")).
type Service struct {
	k       *togo.Kernel
	cron    *cron.Cron
	mu      sync.Mutex
	scheds  map[string]*schedule
	running map[string]bool
	runs    []Run
	maxRuns int
}

func init() {
	togo.RegisterProviderFunc("scheduler", togo.PriorityLate+10, func(k *togo.Kernel) error {
		s := &Service{
			k:       k,
			cron:    cron.New(),
			scheds:  map[string]*schedule{},
			running: map[string]bool{},
			maxRuns: 500,
		}
		// When a queue exists, due jobs are dispatched onto it and executed by
		// this handler (so the work runs on a worker, not the cron goroutine).
		if k.Queue != nil {
			k.Queue.Handle(queueName, func(ctx context.Context, payload any) error {
				name, _ := payload.(string)
				return s.run(ctx, name)
			})
		}
		k.Set("scheduler", s)
		s.cron.Start()
		return nil
	})
}

// FromKernel returns the scheduler Service registered on the kernel.
func FromKernel(k *togo.Kernel) (*Service, bool) {
	v, ok := k.Get("scheduler")
	if !ok {
		return nil, false
	}
	s, ok := v.(*Service)
	return s, ok
}

// Schedule adds (or replaces) a cron schedule for a registered job. spec is a
// standard 5-field cron expression ("*/5 * * * *") or a macro ("@every 30s",
// "@daily"). Use the Every*/Daily* helpers for common cadences.
func (s *Service) Schedule(name, spec string) error {
	if !Registered(name) {
		return fmt.Errorf("scheduler: no job registered named %q", name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.scheds[name]; ok {
		s.cron.Remove(old.entryID)
	}
	id, err := s.cron.AddFunc(spec, func() { s.fire(name) })
	if err != nil {
		return fmt.Errorf("scheduler: invalid spec %q for %q: %w", spec, name, err)
	}
	s.scheds[name] = &schedule{Name: name, Spec: spec, entryID: id}
	return nil
}

// Unschedule removes a job's schedule (the handler stays registered).
func (s *Service) Unschedule(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sc, ok := s.scheds[name]; ok {
		s.cron.Remove(sc.entryID)
		delete(s.scheds, name)
	}
}

// fire is the cron callback: dispatch onto the queue, or run inline.
func (s *Service) fire(name string) {
	s.mu.Lock()
	sc := s.scheds[name]
	switch {
	case sc == nil || sc.paused:
		s.mu.Unlock()
		return
	case s.running[name]:
		s.record(Run{Job: name, Status: StatusSkipped, Started: time.Now()})
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	ctx := context.Background()
	if s.k != nil && s.k.Queue != nil {
		_ = s.k.Queue.Dispatch(ctx, queueName, name)
		return
	}
	_ = s.run(ctx, name)
}

// run executes a job with an overlap guard and records the outcome.
func (s *Service) run(ctx context.Context, name string) error {
	regMu.RLock()
	fn := jobs[name]
	regMu.RUnlock()
	if fn == nil {
		return fmt.Errorf("scheduler: no job registered named %q", name)
	}

	s.mu.Lock()
	if s.running[name] {
		s.mu.Unlock()
		return nil // already running — skip (overlap prevention)
	}
	s.running[name] = true
	s.mu.Unlock()

	started := time.Now()
	err := safeRun(ctx, fn)
	finished := time.Now()

	s.mu.Lock()
	delete(s.running, name)
	r := Run{Job: name, Status: StatusDone, Started: started, Finished: &finished}
	if err != nil {
		r.Status = StatusFailed
		r.Error = err.Error()
	}
	s.record(r)
	s.mu.Unlock()
	return err
}

func safeRun(ctx context.Context, fn JobFunc) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("scheduler: job panicked: %v", r)
		}
	}()
	return fn(ctx)
}

// record appends to the bounded run history. Caller holds s.mu.
func (s *Service) record(r Run) {
	s.runs = append(s.runs, r)
	if len(s.runs) > s.maxRuns {
		s.runs = s.runs[len(s.runs)-s.maxRuns:]
	}
}

// Trigger runs a job immediately (ad-hoc), independent of its schedule.
func (s *Service) Trigger(ctx context.Context, name string) error {
	return s.run(ctx, name)
}

// Pause stops a job from firing without removing its schedule.
func (s *Service) Pause(name string) {
	s.mu.Lock()
	if sc := s.scheds[name]; sc != nil {
		sc.paused = true
	}
	s.mu.Unlock()
}

// Resume re-enables a paused job.
func (s *Service) Resume(name string) {
	s.mu.Lock()
	if sc := s.scheds[name]; sc != nil {
		sc.paused = false
	}
	s.mu.Unlock()
}

// ScheduledJob describes a configured schedule.
type ScheduledJob struct {
	Name   string `json:"name"`
	Spec   string `json:"spec"`
	Paused bool   `json:"paused"`
}

// Jobs returns the configured schedules.
func (s *Service) Jobs() []ScheduledJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ScheduledJob, 0, len(s.scheds))
	for _, sc := range s.scheds {
		out = append(out, ScheduledJob{Name: sc.Name, Spec: sc.Spec, Paused: sc.paused})
	}
	return out
}

// Runs returns recent run history, most recent last.
func (s *Service) Runs() []Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Run, len(s.runs))
	copy(out, s.runs)
	return out
}

// Stop halts the cron loop (call on shutdown).
func (s *Service) Stop() {
	s.cron.Stop()
}
