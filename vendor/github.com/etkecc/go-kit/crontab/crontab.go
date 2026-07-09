// Package crontab runs functions on a standard five-field cron schedule, in-process, at
// minute granularity. Point it at "0 3 * * *", hand it a func, and it fires at 3am every
// day for as long as your process lives. No reflection, no regex, no goroutine that outlives
// the scheduler: the standard library and a ticker, nothing else.
//
// # At-most-once, and the one time it fires twice
//
// A job fires at most once per minute. The guard keys on the absolute instant truncated to
// the minute, so a drifting ticker or an off-boundary re-tick cannot double-fire a job.
//
// The one deliberate exception is the one that pages you: DST fall-back under WithLocation.
// When the clock rolls 02:00 back to 01:00, the wall-clock minute 01:30 comes around twice, an
// hour apart in real time, and a job scheduled for 01:30 fires at both. That is on purpose. The
// guard keys on the absolute instant, not the wall-clock label: on fall-back night 01:30 points
// at two different instants, and the scheduler refuses to pretend they are the same one. Run in
// the default UTC and it never comes up, since UTC has no fall-back. If a job must fire exactly
// once across that boundary, make it idempotent, because the scheduler will not do it for you.
//
// A wall-clock step is the same wound with no calendar to warn you. The ticker runs on the
// monotonic clock, but the minute a job matches comes off the wall clock, so an NTP step
// correction, a paused-and-resumed VM, or someone typing "date -s" splits the two apart: step
// back and a minute that already ran fires again, step forward and one silently never runs, and
// nothing logs either. You find out when a job double-fires and the timestamps do not add up, or
// when it just did not run and no one can say why. Small NTP slews are harmless; a hard step is
// the job's problem to survive, not the ticker's, and idempotent jobs shrug it off.
//
// # Overlap, panics, shutdown
//
// By default, a job still running when its next tick comes around skips that tick. Pass
// WithOverlap to let it run concurrently with itself instead. A job that panics gets recovered,
// so one bad job cannot take the whole scheduler down; its panic lands on stderr by default, or
// wherever WithPanicHandler points it.
//
// Shutdown stops the ticker and waits, bounded by a context, for in-flight jobs to finish. If
// the deadline hits first it returns ctx.Err() and leaves the still-running jobs orphaned: they
// finish on their own, unwatched, and get reaped at process exit. A stopped scheduler does not
// restart; AddJob after Shutdown returns ErrClosed.
package crontab

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// ErrClosed is returned by AddJob once Shutdown has been called.
var ErrClosed = errors.New("crontab: closed")

// Job is the function a schedule runs. It takes nothing and returns nothing: a cron job
// carries whatever it needs in its closure. A panic inside a Job is recovered and routed
// to the panic handler.
type Job func()

// Option configures a Crontab at construction. See WithLocation, WithOverlap, and
// WithPanicHandler.
type Option func(*Crontab)

type job struct {
	sched   *schedule
	fn      Job
	spec    string      // original schedule text, handed to the panic handler
	running atomic.Bool // non-overlap guard: true while a fire is in flight
	last    time.Time   // last minute this job fired, as an absolute instant; see runDue
}

// Crontab is an in-process cron scheduler. Construct it with New, add jobs with AddJob, and
// stop it with Shutdown. It starts ticking the moment New returns. AddJob and Shutdown are
// safe to call concurrently. The zero value is not usable.
type Crontab struct {
	loc     *time.Location
	overlap bool
	onPanic func(spec string, recovered any)
	now     func() time.Time // clock seam; time.Now in production, a fake in tests

	mu      sync.RWMutex
	jobs    []*job
	stop    chan struct{}
	once    sync.Once
	stopped atomic.Bool
	wg      sync.WaitGroup
}

// New builds a Crontab and starts its ticker. By default jobs run in UTC, a job that
// overruns its interval skips the overlapping tick, and a panicking job's details go to
// stderr. The options override those.
func New(opts ...Option) *Crontab {
	c := &Crontab{
		loc:     time.UTC,
		onPanic: defaultPanicHandler,
		now:     time.Now,
		stop:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	go c.run()
	return c
}

// WithLocation runs every schedule in loc instead of UTC. Mind the DST caveat in the package
// doc: under a location with daylight saving, a job can fire twice on fall-back night.
func WithLocation(loc *time.Location) Option {
	return func(c *Crontab) { c.loc = loc }
}

// WithOverlap lets a job run concurrently with itself. Without it, a job still in flight when
// its next tick arrives skips that tick.
func WithOverlap() Option {
	return func(c *Crontab) { c.overlap = true }
}

// WithPanicHandler replaces the default stderr handler. It receives the schedule text and the
// recovered value of any job that panics. Keep it quick and non-blocking; it runs on the
// job's own goroutine.
func WithPanicHandler(h func(spec string, recovered any)) Option {
	return func(c *Crontab) { c.onPanic = h }
}

// AddJob parses spec and registers fn to run on it. It returns a parse error if the schedule
// is malformed, or ErrClosed if the scheduler has been shut down. A closed scheduler rejects
// jobs rather than silently swallowing ones that would never fire.
func (c *Crontab) AddJob(spec string, fn Job) error {
	sched, err := parse(spec)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped.Load() { // checked under the lock Shutdown flips it under, so no TOCTOU
		return ErrClosed
	}
	c.jobs = append(c.jobs, &job{sched: sched, fn: fn, spec: spec})
	return nil
}

// MustAddJob is AddJob that panics instead of returning an error. Use it only with
// compile-time-constant schedules at startup, where a bad spec is a bug worth crashing on;
// never with anything user-supplied.
func (c *Crontab) MustAddJob(spec string, fn Job) {
	if err := c.AddJob(spec, fn); err != nil {
		panic(err)
	}
}

// run aligns to the next minute boundary, then ticks every minute until stop is closed. The
// ticker is a local, stopped on return, so this goroutine cannot outlive the loop and there
// is no struct-stored ticker to race with Shutdown.
func (c *Crontab) run() {
	start := c.now()
	first := start.Truncate(time.Minute).Add(time.Minute)
	timer := time.NewTimer(first.Sub(start))
	select {
	case <-c.stop:
		timer.Stop()
		return
	case <-timer.C:
	}
	c.runDue(c.now())

	tk := time.NewTicker(time.Minute)
	defer tk.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-tk.C:
			c.runDue(c.now())
		}
	}
}

// runDue fires every job whose schedule matches the given minute. It is the seam the tests
// drive directly, bypassing the real ticker.
//
// Single-writer invariant: runDue must not be called concurrently. Only run's goroutine calls
// it, which is why the write to j.last below needs no atomic and the RLock only has to guard
// the jobs slice against AddJob's append. Add a second concurrent caller and you introduce a
// data race on j.last; don't, without moving that write under full synchronization.
//
// The stopped check is the other half of Shutdown's drain safety: Shutdown flips stopped under
// c.mu.Lock before it starts wg.Wait, so once a runDue sees stopped it dispatches nothing, and
// wg.Add can never run concurrently with the waiter. Without it, sync.WaitGroup panics the
// moment the counter passes through zero while a job is being dispatched.
func (c *Crontab) runDue(now time.Time) {
	minute := now.In(c.loc).Truncate(time.Minute)
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.stopped.Load() {
		return
	}
	for _, j := range c.jobs {
		if !j.sched.match(minute) {
			continue
		}
		if j.last.Equal(minute) { // this exact instant already fired
			continue
		}
		if !c.overlap && !j.running.CompareAndSwap(false, true) {
			continue // previous run still in flight
		}
		j.last = minute
		c.wg.Add(1)
		go c.fire(j)
	}
}

// fire runs one job behind the guards that have to survive a panic: recover, so a panicking
// job cannot crash the process; clear the non-overlap flag, so the slot does not wedge shut;
// and wg.Done, so Shutdown's drain cannot hang. The defers unwind last-in-first-out, so
// recover runs first and the flag and counter still clear on the way out.
func (c *Crontab) fire(j *job) {
	defer c.wg.Done()
	if !c.overlap {
		defer j.running.Store(false)
	}
	defer func() {
		if r := recover(); r != nil {
			// onPanic is user code. If it panics too, swallow that: a panicking job must
			// never crash the process, and neither must a panicking handler.
			defer func() { _ = recover() }() //nolint:errcheck // deliberately swallow a handler panic
			c.onPanic(j.spec, r)
		}
	}()
	j.fn()
}

// Shutdown stops the scheduler and waits, bounded by ctx, for in-flight jobs to finish. It
// returns nil once they have all drained, or ctx.Err() if the deadline hits first. On timeout
// the still-running jobs are orphaned: they run to completion and are reaped at process exit,
// and the returned error is the signal that some were still going.
//
// Shutdown is idempotent and does not restart the scheduler. After it has been called, AddJob
// returns ErrClosed.
func (c *Crontab) Shutdown(ctx context.Context) error {
	c.once.Do(func() {
		// Flip stopped under the lock runDue reads it under, so any in-flight dispatch loop
		// finishes its wg.Add calls before the wait below starts. See runDue's drain note.
		c.mu.Lock()
		c.stopped.Store(true)
		c.mu.Unlock()
		close(c.stop)
	})
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// defaultPanicHandler writes a recovered job panic to stderr. It is the fallback when no
// WithPanicHandler is set, because a panicking cron job should never vanish silently.
func defaultPanicHandler(spec string, recovered any) {
	fmt.Fprintln(os.Stderr, "crontab: recovered panic in job", spec, recovered)
}
