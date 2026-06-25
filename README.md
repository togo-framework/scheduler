<div align="center">
  <img src=".github/assets/togo-mark.svg" alt="togo" height="64" />
  <h1>togo-framework/scheduler</h1>
  <p>
    <a href="https://to-go.dev/marketplace"><img src="https://img.shields.io/badge/marketplace-to--go.dev-1FC7DC" alt="marketplace" /></a>
    <a href="https://pkg.go.dev/github.com/togo-framework/scheduler"><img src="https://pkg.go.dev/badge/github.com/togo-framework/scheduler.svg" alt="pkg.go.dev" /></a>
    <img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT" />
  </p>
  <p><strong>Task scheduler for <a href="https://to-go.dev">togo</a> — cron & recurring jobs, dispatched over the queue.</strong></p>
</div>

## Install

```bash
togo install togo-framework/scheduler
```

`scheduler` is the togo answer to Laravel's Scheduler / Celery Beat / Rails recurring jobs. Register a job once, schedule it with a cron expression or a fluent helper, and the scheduler dispatches it over the kernel **Queue** on time — with **overlap prevention** and **run history** built in.

## How it works

- **Register** a named job handler (`scheduler.Register`).
- **Schedule** it with a cron expression or a helper (`Service.Schedule`).
- On each tick the job is **dispatched onto the togo Queue** so the work runs on a worker, not the cron goroutine. With no queue configured it runs **inline**.
- A job whose previous run is still in flight is **skipped** (overlap prevention) and recorded as `skipped`.
- Every run is recorded (`done` / `failed` / `skipped`) and panics are recovered into errors.

## Usage

```go
package jobs

import (
	"context"

	"github.com/togo-framework/scheduler"
	"github.com/togo-framework/togo"
)

func init() {
	// 1. Register handlers (from any plugin's init() or Boot).
	scheduler.Register("cleanup", func(ctx context.Context) error {
		return purgeExpired(ctx)
	})
	scheduler.Register("digest", func(ctx context.Context) error {
		return sendDailyDigest(ctx)
	})
}

// 2. Schedule them once the kernel is booted.
func Schedule(k *togo.Kernel) {
	s, ok := scheduler.FromKernel(k)
	if !ok {
		return
	}
	s.Schedule("cleanup", scheduler.EveryFiveMinutes) // "*/5 * * * *"
	s.Schedule("digest", scheduler.DailyAt(9, 0))     // 09:00 every day
}
```

### Cadence helpers

| Helper | Cron |
|---|---|
| `EveryMinute` | `* * * * *` |
| `EveryFiveMinutes` / `EveryTenMinutes` / `EveryFifteenMin` / `EveryThirtyMin` | `*/N * * * *` |
| `Hourly` / `Daily` / `Weekly` / `Monthly` | period start |
| `DailyAt(hour, minute)` | `m h * * *` |
| `HourlyAt(minute)` | `m * * * *` |
| `WeeklyOn(weekday, hour, minute)` | `m h * * wd` |
| `Every("30s")` | `@every 30s` (sub-minute / macros) |

Any standard 5-field cron expression or robfig macro (`@hourly`, `@every 1h30m`, …) works too.

### Managing schedules

```go
s.Pause("digest")                       // stop firing (keeps the schedule)
s.Resume("digest")
s.Trigger(ctx, "cleanup")               // run now, ad-hoc
s.Unschedule("cleanup")                 // remove the schedule
jobs := s.Jobs()                        // []ScheduledJob{name, spec, paused}
runs := s.Runs()                        // recent run history (done/failed/skipped)
```

## Configuration

No required env. The scheduler uses the kernel **Queue** when present (install `togo install togo-framework/queue` + a worker) so jobs run off-request; otherwise jobs run inline on the cron goroutine.

---

<div align="center">
  <h3>Premium sponsors</h3>
  <p>
    <a href="https://id8media.com"><strong>ID8 Media</strong></a> &nbsp;·&nbsp;
    <a href="https://one-studio.co"><strong>One Studio</strong></a>
  </p>
  <p><sub>Support togo — <a href="https://github.com/sponsors/fadymondy">become a sponsor</a>.</sub></p>
</div>
