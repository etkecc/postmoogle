# crontab

[![Go Reference](https://pkg.go.dev/badge/github.com/etkecc/go-kit.svg)](https://pkg.go.dev/github.com/etkecc/go-kit/crontab)

An in-process cron scheduler. Point it at `0 3 * * *`, hand it a func, and it fires at 3am every day for as long as your process is alive. No reflection, no regex engine, no goroutine that outlives the scheduler: a standard-library ticker and nothing else.

Ships inside the dependency-free root module.

```go
go get github.com/etkecc/go-kit
```

```go
import "github.com/etkecc/go-kit/crontab"

c := crontab.New()
c.MustAddJob("0 3 * * *", func() { runNightlyReport() })

// on shutdown, drain in-flight jobs, bounded by a deadline:
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := c.Shutdown(ctx); err != nil {
    log.Printf("crontab drain timed out, orphaned jobs finishing on their own: %v", err)
}
```

`MustAddJob` panics on a bad spec, so use it only for compile-time-constant schedules at startup. `AddJob` returns the error for anything dynamic. A `Job` is a bare `func()`: no args, no context, no return. If it needs state, close over it.

## Fires once. Except in one timezone.

A job runs at most once per minute. The guard keys on the absolute instant, so a drifting ticker or an off-boundary re-tick can't double-fire it. Out of the box the scheduler runs in **UTC**, which has no daylight-saving fall-back, so on the default there is exactly one edge case here and it is "none." Good. Leave it in UTC and skip the next paragraph.

Still reading? Then you passed `WithLocation` and picked a zone that observes DST, and you bought the one deliberate double-fire that comes with it. When the clock rolls 02:00 back to 01:00, the wall-clock minute 01:30 comes around **twice**, an hour apart in real time, and a job scheduled for 01:30 fires at both. That is on purpose. The guard keys on the absolute instant, not the label on the clock, and on fall-back night 01:30 points at two genuinely different instants. The scheduler refuses to pretend they're the same one. If firing once across that boundary matters, make the job idempotent, because the scheduler will not do it for you.

A wall-clock step is the same wound with no calendar to warn you. The ticker runs on the monotonic clock, but the minute a job matches comes off the wall clock, so an NTP step, a paused-and-resumed VM, or someone typing `date -s` splits the two apart: step back and a minute that already ran fires again, step forward and one silently never runs, and nothing logs either way. You find out when a job double-fires and the timestamps don't add up, or when it just didn't run and nobody can say why. The fix isn't in the scheduler, it's in your job: **make it idempotent** and it shrugs all of this off.

## The dom/dow footgun

Five numeric fields, `min hour dom month dow`. No `JAN`/`MON` names, dow is 0-7 (both 0 and 7 are Sunday). Supports `*/N`, `a-b`, `a-b/N`, lists. `*/0` is rejected outright, because it's an infinite-loop trap.

The Vixie day-of-month / day-of-week union is the trap everyone steps in once: when **both** dom and dow are restricted (neither is `*`), a day matches if it satisfies **either** one. `1 0 1 * 1` fires on the 1st of the month **and** on every Monday, not only when the 1st happens to be a Monday. If at least one field is `*`, both must match, the way you'd expect. This is standard cron behavior and it trips everyone once.

## Overlap, panics, shutdown

By default a job still running when its next tick arrives **skips** that tick. Pass `WithOverlap()` to let it run concurrently with itself instead. A job that panics gets recovered, so one bad job can't take the whole scheduler down; its panic lands on stderr, or wherever `WithPanicHandler` points it. `Shutdown` stops the ticker and waits for in-flight jobs, bounded by the context; miss the deadline and it returns `ctx.Err()` and leaves the stragglers to finish unwatched. A stopped scheduler does not restart, and `AddJob` after `Shutdown` returns `ErrClosed`.

Full option list and spec grammar in [godoc](https://pkg.go.dev/github.com/etkecc/go-kit/crontab).

## License

GNU LGPL-3.0. See [../LICENSE](../LICENSE).
