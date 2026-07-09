package retry

import "time"

// DefaultMaxRetries is the default number of retries: 3 total attempts (1 initial + 2 retries).
// When used with Do, the function will be called at most 3 times.
const DefaultMaxRetries = 3

// DefaultDelayStep is the default delay step applied in the linear backoff calculation.
// Between attempt i (0-indexed) and the next, the delay is delayStep*(i+1) before jitter is applied.
// With DefaultDelayStep=1s, the delays between attempts are approximately 1s, 2s, 3s (before jitter).
const DefaultDelayStep = 1 * time.Second

// DefaultJitter is the default setting for adding randomness to delays.
// When true, full jitter (AWS canonical) is used: the actual delay is a random value in [0, base),
// where base = delayStep*(i+1). Mean delay is 0.5*base.
// When false, delays are deterministic: delayStep*1, delayStep*2, etc.
const DefaultJitter = true

// Option is a function that sets a configuration option for a Retry instance.
// It implements the functional options pattern: each Option is a closure that mutates a *Retry.
// Options are applied in order during New.
type Option func(*Retry)

// WithMaxRetries sets the maximum number of attempts (1 initial + retries).
// For example, WithMaxRetries(5) means the function will be called at most 5 times total.
// Values less than 1 are clamped to 1 by New, ensuring fn is invoked at least once.
func WithMaxRetries(maxRetries int) Option {
	return func(r *Retry) {
		r.maxRetries = maxRetries
	}
}

// WithDelayStep sets the base delay step for the linear backoff.
// Between attempt i (0-indexed) and the next, the delay is delayStep*(i+1) before jitter.
// A value of 0 disables the inter-attempt delay (no sleep between retries).
// Negative values are treated as 0 by time.Sleep.
func WithDelayStep(delayStep time.Duration) Option {
	return func(r *Retry) {
		r.delayStep = delayStep
	}
}

// WithJitter sets whether random jitter is added to each inter-attempt delay.
// When true, full jitter (AWS canonical) is used: the actual delay is a random value
// in [0, base), reducing thundering herd problems in distributed systems.
// When false, delays are deterministic: delayStep*(i+1) for attempt i (0-indexed).
func WithJitter(jitter bool) Option {
	return func(r *Retry) {
		r.jitter = jitter
	}
}

// WithRetryIf sets a predicate that decides whether an error is retryable.
// If the predicate returns false, Do/DoCtx return the error immediately
// without further attempts. Nil predicates are ignored (default behavior preserved).
//
// Example: don't retry on 4xx errors:
//
//	r := retry.New(retry.WithRetryIf(func(err error) bool {
//	    var apiErr *APIError
//	    if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
//	        return false
//	    }
//	    return true
//	}))
func WithRetryIf(predicate func(error) bool) Option {
	return func(r *Retry) {
		if predicate != nil {
			r.retryIf = predicate
		}
	}
}
