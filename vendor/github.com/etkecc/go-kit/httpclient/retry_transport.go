package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"time"

	"github.com/etkecc/go-kit/retry"
)

// retryTransport is the retrying http.RoundTripper. The retrier owns the retry decision
// and the sleep; this layer converts a retryable HTTP status into an error the retrier can
// classify, manages per-attempt deadlines and body lifetime, and returns the final
// response live.
type retryTransport struct {
	base          http.RoundTripper
	retrier       *retry.Retry
	perAttempt    time.Duration
	nonIdem       bool
	maxRetryAfter time.Duration
	budget        RetryBudget
	onAttempt     func(AttemptInfo)
}

// retryableStatusError carries a retryable HTTP status out of an attempt so the retrier
// classifies it, and its parsed Retry-After (0 if absent) so the retrier honors it.
type retryableStatusError struct {
	status int
	delay  time.Duration
}

// attemptLoop holds the mutable state threaded across attempts of one RoundTrip: the held
// response to drain, the pending cancel, the attempt counter, and the terminal response.
type attemptLoop struct {
	rt        *retryTransport
	req       *http.Request
	retryable bool

	resp       *http.Response
	prev       *http.Response
	prevCancel context.CancelFunc
	attempt    int
}

// cancelOnClose ties an attempt context's cancel to the response body's Close. It closes
// the body first so a fully-read body returns its connection to the pool, then cancels.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

// Value receivers, so both a value and a pointer satisfy DelayHinter and errors.As matches
// however the error is wrapped (the trap the DelayHinter doc in retry.go warns about).
func (e retryableStatusError) Error() string {
	return "httpclient: retryable status " + strconv.Itoa(e.status)
}

func (e retryableStatusError) SuggestedRetryDelay() time.Duration { return e.delay }

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// RoundTrip runs the request through the retrier. The method-and-body gate is checked once
// up front, then each attempt runs via attemptLoop.
func (rt *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	_, idempotent := idempotentMethods[req.Method]
	retryable := rt.nonIdem || idempotent
	// A retryable request with a body it can't rewind can't be replayed; refuse loud
	// rather than send a consumed reader on attempt 2. http.NoBody is empty and replays
	// for free, so it is not a real body here.
	if retryable && req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		return nil, ErrNonReplayableBody
	}

	loop := &attemptLoop{rt: rt, req: req, retryable: retryable}
	if !retryable {
		// A non-idempotent request gets exactly one attempt regardless of how it fails.
		// The retrier and its classifier are method-blind (a bare func(error) bool), so a
		// timeout or a reset would otherwise retry a POST; the method gate has to live here.
		return loop.result(loop.once())
	}
	return loop.result(rt.retrier.DoCtx(req.Context(), loop.once))
}

// once runs a single attempt: drain the prior response, gate on budget, then send.
func (l *attemptLoop) once() error {
	l.attempt++
	l.drainPrev()
	if l.attempt > 1 {
		if !l.rt.budget.Allow() {
			l.rt.budget.Record(false)
			return errBudgetExhausted
		}
		l.rt.budget.Record(true)
	}
	return l.send()
}

// drainPrev drains and closes the response held from the previous attempt, freeing its
// connection. The current attempt's response is never drained here, so the terminal one
// survives to be returned live.
func (l *attemptLoop) drainPrev() {
	if l.prev == nil {
		return
	}
	drainClose(l.prev, l.prevCancel)
	l.prev, l.prevCancel = nil, nil
}

// send issues one attempt under a fresh per-attempt context, rewinding the body on retries.
func (l *attemptLoop) send() error {
	attemptCtx, cancel := l.rt.attemptContext(l.req.Context())
	var reused bool
	traced := httptrace.WithClientTrace(attemptCtx, &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused },
	})
	areq := l.req.WithContext(traced)
	if l.attempt > 1 && l.req.GetBody != nil {
		body, err := l.req.GetBody()
		if err != nil {
			cancel()
			return err
		}
		areq.Body = body
	}

	start := time.Now()
	r, rerr := l.rt.base.RoundTrip(areq) //nolint:bodyclose // returned to the caller (wrapped) or drained on retry
	elapsed := time.Since(start)

	if rerr != nil {
		cancel()
		normErr := l.rt.normalizeErr(l.req.Context(), attemptCtx, rerr)
		retrying := l.retryable && !isCallerCtxErr(normErr)
		l.rt.emit(l.req, l.attempt, 0, normErr, elapsed, retrying, reused)
		return normErr
	}
	return l.classify(r, cancel, elapsed, reused)
}

// classify decides an attempt's response: a retryable status within the Retry-After cap is
// held and handed back as an error for the retrier; anything else (success, non-retryable
// status, or a Retry-After past the cap) is the terminal response, returned live.
func (l *attemptLoop) classify(r *http.Response, cancel context.CancelFunc, elapsed time.Duration, reused bool) error {
	if _, ok := retryableStatuses[r.StatusCode]; l.retryable && ok {
		if delay, withinCap := l.rt.retryAfterDelay(r); withinCap {
			l.prev, l.prevCancel = r, cancel
			l.rt.emit(l.req, l.attempt, r.StatusCode, nil, elapsed, true, reused)
			return retryableStatusError{status: r.StatusCode, delay: delay}
		}
	}
	// Terminal response. l.prev is nil here: once drains it before every send, so there is
	// nothing held to clean up, and adding a drain would double-close.
	wrapBodyCancel(r, cancel)
	l.resp = r
	l.rt.emit(l.req, l.attempt, r.StatusCode, nil, elapsed, false, reused)
	return nil
}

// result maps the retrier's outcome to the RoundTrip return. A terminal response wins; a
// retry-exhausted retryable status returns the held response live (sentinel stripped, since
// http.Client discards the response on any error); a real error discards any held response.
func (l *attemptLoop) result(err error) (*http.Response, error) {
	if l.resp != nil {
		return l.resp, nil
	}
	var rse retryableStatusError
	if errors.As(err, &rse) && l.prev != nil {
		wrapBodyCancel(l.prev, l.prevCancel)
		return l.prev, nil
	}
	if l.prev != nil {
		drainClose(l.prev, l.prevCancel)
	}
	return nil, err
}

// attemptContext derives the per-attempt context: perAttempt, or the time left on the
// caller's deadline when that is sooner, recomputed each attempt so elapsed time counts.
func (rt *retryTransport) attemptContext(parent context.Context) (context.Context, context.CancelFunc) {
	if rt.perAttempt <= 0 {
		return context.WithCancel(parent)
	}
	d := rt.perAttempt
	if dl, ok := parent.Deadline(); ok {
		if remaining := time.Until(dl); remaining < d {
			d = remaining
		}
	}
	return context.WithTimeout(parent, d)
}

// normalizeErr disambiguates a failed attempt: the caller's own context death is terminal,
// a per-attempt timeout with the caller still alive is retryable, everything else is raw
// for the classifier. Both contexts surface the same DeadlineExceeded, so the classifier
// alone can't tell them apart; this is where per-attempt timeouts become retryable.
func (rt *retryTransport) normalizeErr(parent, attemptCtx context.Context, rerr error) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	if attemptCtx.Err() != nil {
		return errAttemptTimeout
	}
	return rerr
}

// retryAfterDelay parses a Retry-After from a 429 or 503 (delta-seconds or HTTP-date) and
// reports whether it is within maxRetryAfter. A value beyond the cap returns withinCap
// false, the signal to return the response live rather than wait.
func (rt *retryTransport) retryAfterDelay(r *http.Response) (delay time.Duration, withinCap bool) {
	delay = rt.parseRetryAfter(r)
	if delay > rt.maxRetryAfter {
		return 0, false
	}
	return delay, true
}

// parseRetryAfter reads a Retry-After from a 429 or 503 in delta-seconds or HTTP-date form.
// A delta-seconds large enough to overflow int64 nanoseconds is clamped just past the cap
// (so retryAfterDelay reports it out of range) rather than wrapping to a negative duration.
func (rt *retryTransport) parseRetryAfter(r *http.Response) time.Duration {
	if r.StatusCode != http.StatusTooManyRequests && r.StatusCode != http.StatusServiceUnavailable {
		return 0
	}
	v := r.Header.Get("Retry-After")
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		if secs > int(rt.maxRetryAfter/time.Second)+1 {
			return rt.maxRetryAfter + time.Second
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func (rt *retryTransport) emit(req *http.Request, attempt, status int, err error, elapsed time.Duration, retrying, reused bool) {
	if rt.onAttempt == nil {
		return
	}
	rt.onAttempt(AttemptInfo{
		Method:   req.Method,
		Host:     req.URL.Host,
		Attempt:  attempt,
		Status:   status,
		Err:      err,
		Elapsed:  elapsed,
		Retrying: retrying,
		Reused:   reused,
	})
}

// wrapBodyCancel replaces the response body with one that cancels the attempt context on
// Close, so the body stays readable after RoundTrip returns.
func wrapBodyCancel(r *http.Response, cancel context.CancelFunc) {
	r.Body = &cancelOnClose{ReadCloser: r.Body, cancel: cancel}
}

// drainClose drains and closes a response body best-effort, then cancels its attempt
// context. Draining lets the connection return to the pool instead of being torn down.
func drainClose(r *http.Response, cancel context.CancelFunc) {
	_, _ = io.Copy(io.Discard, r.Body) //nolint:errcheck // best-effort drain; a failure just costs the connection
	_ = r.Body.Close()
	cancel()
}
