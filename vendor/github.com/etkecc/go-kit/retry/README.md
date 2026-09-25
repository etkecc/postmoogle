# retry

[![Go Reference](https://pkg.go.dev/badge/github.com/etkecc/go-kit.svg)](https://pkg.go.dev/github.com/etkecc/go-kit/retry)

Run a function, and if it errors, run it again with a growing, jittered delay between tries. That's it. Linear backoff, full jitter so a fleet doesn't retry in lockstep, and a way for a server to say "wait exactly this long" and be obeyed.

In the root module, no dependencies.

```go
go get github.com/etkecc/go-kit
```

```go
import "github.com/etkecc/go-kit/retry"

err := retry.New().Do(func() error {
    return callFlakyThing()
})

// or with knobs:
r := retry.New(
    retry.WithMaxRetries(5),
    retry.WithDelayStep(100*time.Millisecond),
    retry.WithRetryIf(func(err error) bool { return !errors.Is(err, ErrNotFound) }),
)
err = r.DoCtx(ctx, callFlakyThing) // DoCtx bails the moment ctx is done
```

Defaults: 3 total attempts (1 try plus 2 retries), a 1s step, jitter on, retry on any non-nil error. Delay for retry `i` is a random duration in `[0, step*(i+1))`, the AWS "full jitter" recipe. No sleep after the final attempt, because there's nothing left to wait for.

## The footgun: `DelayHinter` wants a VALUE receiver

Here's the nice part: if an error in the chain can suggest its own retry delay (a 429 carrying `Retry-After`, say), retry will honor it instead of the linear step. An error satisfies this by implementing `DelayHinter`:

```go
type DelayHinter interface {
    SuggestedRetryDelay() time.Duration
}
```

And here's the part that eats a whole debugging session. retry finds the hint with `errors.As`, and **`errors.As` matches against the value's method set.** Implement `SuggestedRetryDelay` on a **pointer** receiver and wrap the error by value, and the method isn't in the value's method set, so `errors.As` walks right past it. The hint silently drops, backoff falls back to the linear default, and every test that doesn't specifically assert the delay still passes, because the retry still *works*, it just ignores the server's instruction and wakes your whole fleet in lockstep during the exact outage `Retry-After` existed to smooth out.

So: **implement `SuggestedRetryDelay` on a value receiver.** If you just need a bare hint-carrying error, `retry.After(d)` hands you one that's already correct. A zero or negative hint is ignored and you get linear backoff, no surprise there.

`DoCtx`, `retry.After`, and the full option list are in [godoc](https://pkg.go.dev/github.com/etkecc/go-kit/retry).

## License

GNU LGPL-3.0. See [../LICENSE](../LICENSE).
