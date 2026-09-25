# go-kit

[![Go Reference](https://pkg.go.dev/badge/github.com/etkecc/go-kit.svg)](https://pkg.go.dev/github.com/etkecc/go-kit)

The pile of small things every Go service ends up rewriting: dedup a slice, chunk a slice, hash a string, collect a fistful of errors and report them all at once, lock per-key instead of one big mutex over everything. We wrote them once, here, so nobody on the team writes `Chunk` for the hundredth time at 3am. Standard library only in the root, zero external dependencies, and it stays that way on purpose.

Used by pretty much every etke.cc Go backend. If you're about to reach for a third-party helper or hand-roll a utility, look here first, it's probably already in the box.

## Heads up: this is three modules, not one

Most of go-kit lives in one dependency-free root module. Two subpackages don't, because they need a dependency and we refuse to drag it into everyone else's `go.sum`. So there are **three** separate Go modules under this repo, and they install on three separate lines:

| Module | `go get` | Deps | Go floor |
|---|---|---|---|
| root (kit + crontab, crypter, httpclient, migrater, retry, template, workpool) | `go get github.com/etkecc/go-kit` | none (stdlib) | 1.26 |
| `format` | `go get github.com/etkecc/go-kit/format` | goldmark | 1.22 |
| `crypter/yaml` | `go get github.com/etkecc/go-kit/crypter/yaml` | yaml.v3 | 1.25 |

The one that catches people: `go get github.com/etkecc/go-kit` pulls the root and its in-module subpackages with **zero** transitive deps. The moment you `go get .../format` or `.../crypter/yaml`, you're pulling goldmark or yaml.v3, on their own Go floor. That's the deal, and it's why the root can honestly promise zero dependencies. Don't `go mod tidy` across the boundary expecting them to merge, they won't, and they shouldn't.

`format`'s Go floor is deliberately *below* the root's (1.22 vs 1.26), so a service stuck on old Go can still render Markdown. Leave it there.

## The top-level `kit` package

```go
import "github.com/etkecc/go-kit"
```

The grab-bag. Generics where they help, boring where they don't.

**Collect every error, not just the first.** `AggregateError` gathers errors from concurrent work or a pile of validations and reports them together, and it plays nice with `errors.Is`/`errors.As` across the whole bag. `Join` and the `New` constructor return `nil` when there's nothing to report, so the idiom is honest:

```go
if err := kit.NewAggregateError(validateName(u), validateEmail(u), validateAge(u)); err != nil {
    return err // "name is required; email is invalid", all of it at once
}
```

**Lock a key, not the world.** `Mutex` is a keyed mutex: two different keys never wait on each other, only same-key contenders block. The internal map is reference-counted, so it shrinks back to empty when nobody's holding anything, no slow leak of every key you've ever seen. Reach for it when you want to serialize work per user, per room, per whatever-ID, without one global lock strangling every unrelated caller.

```go
locks := kit.NewMutex()
locks.Lock(userID)
defer locks.Unlock(userID)
// user A and user B run in parallel; two goroutines on user A take turns
```

**Answer "what version am I?" honestly.** `Version` and `UserAgent` read the running binary's build info: a stamped release if there is one, otherwise the git revision (with `-dirty`), otherwise your fallback. A library reports its own version, never the host binary's git sha, because lying about whose code is running is how you chase a ghost for an afternoon.

```go
ua := kit.UserAgent("myservice", "") // "myservice/v1.4.2" released, "myservice/a1b2c3d-dirty" from your laptop
```

The rest, by shape:

- **slices**: `Uniq`, `Chunk`, `Reverse`, `MergeSlices`, `RemoveFromSlice`
- **maps**: `MapFromSlice`, `MapKeys`, `MergeMapKeys`
- **strings**: `Truncate` (rune-aware, only appends `...` when it actually cuts), `Hash` (SHA-256 hex), `Eq` (constant-time compare, use it for secrets), `Unquote`, `StringToInt`, `StringToSlice`, `SliceToString`
- **StringsBuilder**: a chainable wrapper over `strings.Builder` when you're sick of `WriteString` on every line
- **List**: a concurrency-safe ordered unique set (a generic set that hands you a sorted slice on demand)
- **errors**: `ErrorResponse` (JSON `{"error": "..."}`, HTTP 400 by default), `MatrixError` (Matrix `M_FORBIDDEN`-shape), `IsContextError` (canceled *or* deadline-exceeded in one call)
- **ip**: `AnonymizeIP`, `IsValidIP`
- **WaitGroup**: a `sync.WaitGroup` wrapper with a variadic `Do` that launches without blocking (you still call `Wait` yourself)
- **IsNil**: the reflect-y nil check for the typed-nil-in-interface trap

Full signatures and the fine print live in [godoc](https://pkg.go.dev/github.com/etkecc/go-kit). Every exported symbol carries a doc-comment; the weird ones carry the footgun too.

## Subpackages

Each has its own README with the "why you care" and the part that bites.

| Package | What it's for |
|---|---|
| [`crontab`](./crontab) | In-process five-field cron. Point it at `0 3 * * *`, hand it a func, done. |
| [`crypter`](./crypter) | Transparent AES-GCM encrypt/decrypt for string values (`ENCv1[...]`). |
| [`crypter/yaml`](./crypter/yaml) | Encrypt secret values *inside* a YAML file, in place, comments intact. Separate module. |
| [`format`](./format) | Markdown → HTML via goldmark, tuned for Matrix. Separate module. |
| [`httpclient`](./httpclient) | A tuned `*http.Client` with safe retries and an optional SSRF guard: one host, one fixed backend, or a wide crawl. |
| [`migrater`](./migrater) | Numbered `.sql` files → your database, forward-only, idempotent. |
| [`retry`](./retry) | Linear backoff with jitter, honors a server's `Retry-After`. |
| [`template`](./template) | Thin `html/template` wrapper (HTML-escaped, so XSS-safe by default). |
| [`workpool`](./workpool) | Bounded goroutine pool. Fixed workers, one job at a time each. |

## House rules for anything in here

- **Zero values are not usable.** Always go through the `New*` constructor. Every struct doc-comment says so; it's not a suggestion.
- **Pointer receivers throughout.** `*Mutex`, `*Retry`, `*Crypter`, `*List`, all of them.
- **No `panic()` in library code, ever.** Misuse (like handing a retry option to the transport constructor) is caught at compile time by a type split, not at runtime by a guard that fires in prod.
- **It has to stay boring and dependency-free** in the root. If a helper needs a dep, it becomes its own island module, same as `format` and `crypter/yaml`.

## License

GNU LGPL-3.0. See [LICENSE](./LICENSE).
