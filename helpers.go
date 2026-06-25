package scheduler

import "fmt"

// Common cron cadences (standard 5-field cron, minute resolution).
const (
	EveryMinute      = "* * * * *"
	EveryFiveMinutes = "*/5 * * * *"
	EveryTenMinutes  = "*/10 * * * *"
	EveryFifteenMin  = "*/15 * * * *"
	EveryThirtyMin   = "*/30 * * * *"
	Hourly           = "0 * * * *"
	Daily            = "0 0 * * *"
	Weekly           = "0 0 * * 0"
	Monthly          = "0 0 1 * *"
)

// Every builds a sub-minute/second cadence macro, e.g. Every("30s") → "@every 30s".
func Every(dur string) string { return "@every " + dur }

// DailyAt builds a cron spec that runs once a day at hour:minute (24h clock).
func DailyAt(hour, minute int) string { return fmt.Sprintf("%d %d * * *", minute, hour) }

// HourlyAt builds a cron spec that runs once an hour at the given minute.
func HourlyAt(minute int) string { return fmt.Sprintf("%d * * * *", minute) }

// WeeklyOn builds a cron spec that runs weekly on weekday (0=Sun..6=Sat) at hour:minute.
func WeeklyOn(weekday, hour, minute int) string {
	return fmt.Sprintf("%d %d * * %d", minute, hour, weekday)
}
