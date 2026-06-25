# scheduler — usage

## 1. Register a job

Register handlers from any plugin's `init()` or `Boot`:

```go
scheduler.Register("cleanup", func(ctx context.Context) error {
    return purgeExpired(ctx)
})
```

## 2. Schedule it

Resolve the service from the kernel and schedule by cron spec or helper:

```go
s, _ := scheduler.FromKernel(k)
s.Schedule("cleanup", scheduler.EveryFiveMinutes) // "*/5 * * * *"
s.Schedule("digest",  scheduler.DailyAt(9, 0))    // 09:00 daily
s.Schedule("ping",    scheduler.Every("30s"))     // @every 30s
```

Accepts any standard 5-field cron expression or robfig macro (`@hourly`,
`@every 1h30m`, …).

## Cadence helpers

| Helper | Cron |
|---|---|
| `EveryMinute`, `EveryFiveMinutes`, `EveryTenMinutes`, `EveryFifteenMin`, `EveryThirtyMin` | `*/N * * * *` |
| `Hourly`, `Daily`, `Weekly`, `Monthly` | period start |
| `DailyAt(hour, minute)` | `m h * * *` |
| `HourlyAt(minute)` | `m * * * *` |
| `WeeklyOn(weekday, hour, minute)` | `m h * * wd` |
| `Every("30s")` | `@every 30s` |

## Execution model

- With a **queue** configured, due jobs are dispatched onto `scheduler.run` and
  executed by a worker (off the cron goroutine). Install `queue` + run a worker.
- With **no queue**, jobs run **inline** on the cron goroutine.
- **Overlap prevention:** a job is skipped (recorded `skipped`) if its previous
  run is still in flight.
- **Panics** are recovered and recorded as `failed`.

## Managing schedules & runs

```go
s.Pause("digest"); s.Resume("digest")
s.Trigger(ctx, "cleanup")   // run now, ad-hoc
s.Unschedule("cleanup")
s.Jobs()                    // []ScheduledJob{name, spec, paused}
s.Runs()                    // recent Run history (done/failed/skipped)
```
