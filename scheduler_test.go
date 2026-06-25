package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

// newTestService builds a Service without a kernel (queue=nil → inline runs).
func newTestService() *Service {
	return &Service{
		cron:    cron.New(),
		scheds:  map[string]*schedule{},
		running: map[string]bool{},
		maxRuns: 50,
	}
}

func TestRegisterAndScheduleValidation(t *testing.T) {
	s := newTestService()
	// Scheduling an unregistered job fails.
	if err := s.Schedule("missing", EveryMinute); err == nil {
		t.Fatal("expected error scheduling an unregistered job")
	}
	Register("reg-test", func(ctx context.Context) error { return nil })
	// Invalid cron spec fails.
	if err := s.Schedule("reg-test", "not a cron"); err == nil {
		t.Fatal("expected error for invalid cron spec")
	}
	// Valid spec + macro succeed.
	if err := s.Schedule("reg-test", EveryFiveMinutes); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	if err := s.Schedule("reg-test", Every("30s")); err != nil {
		t.Fatalf("@every macro rejected: %v", err)
	}
	if jobs := s.Jobs(); len(jobs) != 1 || jobs[0].Name != "reg-test" {
		t.Fatalf("Jobs() = %+v", jobs)
	}
}

func TestRunRecordsSuccessAndFailure(t *testing.T) {
	s := newTestService()
	Register("ok-job", func(ctx context.Context) error { return nil })
	Register("bad-job", func(ctx context.Context) error { return errors.New("boom") })

	if err := s.run(context.Background(), "ok-job"); err != nil {
		t.Fatalf("ok-job returned %v", err)
	}
	if err := s.run(context.Background(), "bad-job"); err == nil {
		t.Fatal("bad-job should have errored")
	}
	runs := s.Runs()
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	if runs[0].Status != StatusDone || runs[0].Finished == nil {
		t.Errorf("run0 = %+v", runs[0])
	}
	if runs[1].Status != StatusFailed || runs[1].Error == "" {
		t.Errorf("run1 = %+v", runs[1])
	}
}

func TestPanicIsRecovered(t *testing.T) {
	s := newTestService()
	Register("panic-job", func(ctx context.Context) error { panic("kaboom") })
	if err := s.run(context.Background(), "panic-job"); err == nil {
		t.Fatal("panicking job should surface an error, not crash")
	}
	if r := s.Runs(); len(r) != 1 || r[0].Status != StatusFailed {
		t.Fatalf("panic run not recorded as failed: %+v", r)
	}
}

func TestOverlapPrevention(t *testing.T) {
	s := newTestService()
	var started, blockReleased int32
	release := make(chan struct{})
	Register("slow-job", func(ctx context.Context) error {
		atomic.AddInt32(&started, 1)
		<-release // hold the job "running"
		atomic.AddInt32(&blockReleased, 1)
		return nil
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = s.run(context.Background(), "slow-job") }()

	// Wait until the first run is in flight.
	for atomic.LoadInt32(&started) == 0 {
		time.Sleep(time.Millisecond)
	}
	// A second concurrent run must be skipped (returns nil immediately).
	if err := s.run(context.Background(), "slow-job"); err != nil {
		t.Fatalf("overlapping run returned %v", err)
	}
	if got := atomic.LoadInt32(&started); got != 1 {
		t.Fatalf("job ran %d times concurrently; overlap not prevented", got)
	}
	close(release)
	wg.Wait()
}

func TestPauseResume(t *testing.T) {
	s := newTestService()
	var n int32
	Register("pausable", func(ctx context.Context) error { atomic.AddInt32(&n, 1); return nil })
	if err := s.Schedule("pausable", EveryMinute); err != nil {
		t.Fatal(err)
	}
	s.Pause("pausable")
	s.fire("pausable") // paused → no run
	if atomic.LoadInt32(&n) != 0 {
		t.Fatal("paused job should not fire")
	}
	s.Resume("pausable")
	s.fire("pausable") // queue=nil → inline run
	if atomic.LoadInt32(&n) != 1 {
		t.Fatalf("resumed job should fire once, got %d", n)
	}
}

func TestHelpers(t *testing.T) {
	cases := map[string]string{
		DailyAt(9, 30):       "30 9 * * *",
		HourlyAt(15):         "15 * * * *",
		WeeklyOn(1, 8, 0):    "0 8 * * 1",
		Every("45s"):         "@every 45s",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("helper = %q, want %q", got, want)
		}
	}
	// Every cadence constant must parse as a valid cron spec.
	p := cron.New()
	for _, spec := range []string{EveryMinute, EveryFiveMinutes, Hourly, Daily, Weekly, Monthly} {
		if _, err := p.AddFunc(spec, func() {}); err != nil {
			t.Errorf("cadence %q invalid: %v", spec, err)
		}
	}
}
