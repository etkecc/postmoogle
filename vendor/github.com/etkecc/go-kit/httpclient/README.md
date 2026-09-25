# httpclient

[![Go Reference](https://pkg.go.dev/badge/github.com/etkecc/go-kit.svg)](https://pkg.go.dev/github.com/etkecc/go-kit/httpclient)

`http.DefaultClient` has no timeout, an idle pool of 2 connections per host, and no retries. Fine for a script, a slow leak of trouble for a service. This builds you an `*http.Client` that's already right: a sized connection pool, a deadline on every attempt, and retries that only ever replay a request it's safe to replay. Pick the constructor that matches your traffic shape and go.

Part of the dependency-free root module (stdlib plus the [`retry`](../retry) sibling, nothing else).

```go
go get github.com/etkecc/go-kit
```

```go
import "github.com/etkecc/go-kit/httpclient"

client := httpclient.NewSingleHost() // pooled, retrying, TLS 1.2 floor, done
resp, err := client.Get("https://api.example.com/thing")
```

## Five constructors, pick by traffic shape

| Constructor | For |
|---|---|
| `New` | General purpose, many hosts. Per-host connections uncapped, idle pool sized for throughput. |
| `NewSingleHost` | You hammer one backend. All three pool dimensions sized up, plus HTTP/2 keepalive pings to notice a dead connection. |
| `NewMultiHost` | A wide, shallow crawl: thousands of hosts each seen once and ghosted. Per-host idle kept tiny, reclaimed fast. |
| `Wrap` | You already have an `*http.Client` you like. This bolts the retry layer on and leaves its transport tuning alone. |
| `NewTransport` | You want just the tuned `*http.Transport`, no retry, to build your own client around. |

None of them return an error. There's nothing to check, nothing to `log.Fatal` on at startup. Misuse is caught at compile time instead (see below), so construction has nothing left to fail on.

## The type-split, a compile-time bouncer

Three kinds of option, and the type system keeps each away from the constructor it would silently do nothing on:

- `TransportOption` (pool and TLS knobs): valid on the transport constructors and `NewTransport`.
- `RetryOption` (retry knobs): valid on the full-client constructors and `Wrap`.
- Both narrow `Option`, so the full-client constructors take either.

So `httpclient.Wrap(c, httpclient.WithMaxConnsPerHost(10))` **does not compile**, and neither does `httpclient.NewTransport(httpclient.WithMaxRetries(3))`. A transport knob handed to the retry layer, or a retry knob handed to a bare transport, has nothing to apply itself to. The danger was never that it errors, it's that it would sit there doing nothing while you assume it took, and a silently-ignored option is the worst way for a bug to behave. The split turns "quietly does nothing at runtime" into "your editor underlines it in red." And it's marker interfaces doing the rejecting, not a runtime guard, on purpose: a runtime check means a panic, and this library does not panic.

## The SSRF guard, for attacker-influenced URLs

```go
client := httpclient.New(httpclient.WithDialGuard()) // because hostnames lie
```

`WithDialGuard()` refuses any dial to a loopback, private, link-local, or cloud-metadata IP, checked at **dial time** on the resolved address, which is the last honest moment before the connection opens. A hostname will swear it's public right up until it resolves to `169.254.169.254`, so checking the string is theater; checking the resolved IP is the real thing. Use it anywhere the URL or a redirect target is influenced by user input. It covers the built-in dialer only, and it's mutually exclusive with an egress proxy (the guard wins and nulls the proxy, because through a proxy the guard would only ever see the proxy's own IP).

## Retries that don't shoot you in the foot

- **Only safe replays.** Idempotent methods retry by default; POST and PATCH do not, because a replayed POST can double-charge a card. Opt them in with `WithRetryNonIdempotent(true)` only when you know the endpoint is idempotent.
- **Loud on an unrewindable body.** If a retryable request has a body but no `GetBody` to rewind it, you get `ErrNonReplayableBody` instead of a silently truncated second attempt. Better a clear error than a corrupt request.
- **Honors `Retry-After`.** A 429 or 503 with a `Retry-After` gets that delay (jittered so the fleet doesn't stampede back all at once), capped by `WithMaxRetryAfter` so a server can't tell you to sleep for an hour.
- **Per-attempt deadline, not per-sequence.** Each attempt gets its own timeout; the client's own `Timeout` stays 0 on purpose, because a client-level timeout would cap the entire retry sequence and kill a legitimate second try.

Every knob, default, and the `WithOnAttempt` / `WithRetryBudget` observability hooks are in [godoc](https://pkg.go.dev/github.com/etkecc/go-kit/httpclient).

## License

GNU LGPL-3.0. See [../LICENSE](../LICENSE).
