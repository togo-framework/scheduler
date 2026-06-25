---
name: scheduler
description: Define, inspect and manage cron/recurring scheduled jobs in a togo app using the scheduler plugin (register handlers, schedule by cron/helper, pause/resume/trigger, view run history).
---

# togo scheduler

Use this skill to work with the `togo-framework/scheduler` plugin in a togo app.

## Register a job
Add a handler in a plugin `init()` or `Boot`:
```go
scheduler.Register("<name>", func(ctx context.Context) error { /* work */ return nil })
```

## Schedule it
```go
s, _ := scheduler.FromKernel(k)
s.Schedule("<name>", scheduler.EveryFiveMinutes) // or scheduler.DailyAt(9,0), scheduler.Every("30s"), or a raw "*/5 * * * *"
```

## Cadences
`EveryMinute`, `EveryFiveMinutes`, `EveryTenMinutes`, `EveryThirtyMin`, `Hourly`, `Daily`, `Weekly`, `Monthly`, `DailyAt(h,m)`, `HourlyAt(m)`, `WeeklyOn(wd,h,m)`, `Every("<dur>")`. Any 5-field cron or robfig macro works.

## Manage
- `s.Pause(name)` / `s.Resume(name)` — toggle without removing the schedule
- `s.Trigger(ctx, name)` — run now
- `s.Unschedule(name)` — remove the schedule
- `s.Jobs()` — configured schedules; `s.Runs()` — run history (done/failed/skipped)

## Notes
- Jobs run over the kernel **Queue** when present (install `queue` + a worker); otherwise inline.
- **Overlap is prevented** — a job is skipped if its previous run is still running.
- Prefer idempotent handlers; long jobs belong on the queue.
