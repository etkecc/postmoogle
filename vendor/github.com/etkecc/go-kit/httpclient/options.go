package httpclient

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/etkecc/go-kit/retry"
)

// Exported defaults, safe to reference when overriding one knob relative to another.
const (
	// DefaultPoolSize is the connection-pool size NewSingleHost applies to all three pool
	// dimensions, overriding the stdlib default of 2 idle connections per host.
	DefaultPoolSize = 256
	// DefaultIdleConnTimeout is how long an idle connection is kept before closing.
	DefaultIdleConnTimeout = 90 * time.Second
	// DefaultPerAttemptTimeout is the deadline applied to each individual attempt.
	DefaultPerAttemptTimeout = 10 * time.Second
	// DefaultMaxRetryAfter is the ceiling on an honored Retry-After: past it, the response
	// is returned live instead of waiting.
	DefaultMaxRetryAfter = 30 * time.Second
	// DefaultMultiHostIdleConnsPerHost caps idle connections kept per host under NewMultiHost,
	// low because a many-host crawler rarely revisits a host before the idle timeout reclaims it.
	DefaultMultiHostIdleConnsPerHost = 2
	// DefaultMultiHostIdleConnTimeout is how long NewMultiHost keeps an idle connection, short so
	// a finished host's FDs come back fast during a wide fan-out.
	DefaultMultiHostIdleConnTimeout = 30 * time.Second
)

const (
	defaultMaxRetries            = 3
	defaultDelayStep             = 200 * time.Millisecond
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultResponseHeaderTimeout = 10 * time.Second
	defaultExpectContinueTimeout = 1 * time.Second
	defaultH2PingInterval        = 15 * time.Second
	defaultH2PingTimeout         = 15 * time.Second
	defaultDialTimeout           = 30 * time.Second
	defaultDialKeepAlive         = 30 * time.Second
)

// dialIPKey carries a WithDialIP pin on the request context; a struct key can't collide with a
// string key from another package.
type dialIPKey struct{}

// Option configures a client during New, NewSingleHost, or NewMultiHost, which take either kind
// below. Each kind also satisfies a narrower interface, so the specialized constructors reject the
// wrong knob at compile time instead of swallowing it at runtime.
type Option interface{ apply(*config) }

// TransportOption tunes the transport, so it is valid on NewTransport (a bare transport, no retry)
// as well as the full-client constructors. A retry knob is not one, and won't compile on NewTransport.
type TransportOption interface {
	Option
	isTransport()
}

// RetryOption tunes the retry layer, so it is valid on Wrap (retry over a BYO client) as well as
// the full-client constructors. A transport knob is not one, and won't compile on Wrap.
type RetryOption interface {
	Option
	isRetry()
}

// optionFunc adapts a plain config mutator into a TransportOption.
type optionFunc func(*config)

func (f optionFunc) apply(c *config) { f(c) }
func (optionFunc) isTransport()      {}

// retryOptionFunc adapts a config mutator into a RetryOption: it satisfies apply (from
// Option) and the isRetry marker, so it is usable everywhere an Option is.
type retryOptionFunc func(*config)

func (f retryOptionFunc) apply(c *config) { f(c) }
func (retryOptionFunc) isRetry()          {}

// defaultConfig seeds the shared defaults both constructors start from.
func defaultConfig() *config {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	return &config{
		maxIdleConns:          DefaultPoolSize,
		maxIdleConnsPerHost:   DefaultPoolSize,
		maxConnsPerHost:       0,
		idleConnTimeout:       DefaultIdleConnTimeout,
		tlsHandshakeTimeout:   defaultTLSHandshakeTimeout,
		responseHeaderTimeout: defaultResponseHeaderTimeout,
		expectContinueTimeout: defaultExpectContinueTimeout,
		perAttemptTimeout:     DefaultPerAttemptTimeout,
		maxRetries:            defaultMaxRetries,
		delayStep:             defaultDelayStep,
		protocols:             protocols,
		tlsMinVersion:         tls.VersionTLS12,
		maxRetryAfter:         DefaultMaxRetryAfter,
		budget:                noopBudget{},
	}
}

// defaultHTTP2Config is the keepalive-ping config both presets layer on to catch a dead conn.
func defaultHTTP2Config() *http.HTTP2Config {
	return &http.HTTP2Config{
		SendPingTimeout: defaultH2PingInterval,
		PingTimeout:     defaultH2PingTimeout,
	}
}

// singleHostPreset caps per-host connections to the pool size and enables HTTP/2 keepalive
// pings, on top of the shared defaults. Applied before caller options, so callers still win.
func singleHostPreset() Option {
	return optionFunc(func(c *config) {
		WithMaxConnsPerHost(DefaultPoolSize).apply(c)
		WithHTTP2Config(defaultHTTP2Config()).apply(c)
	})
}

// multiHostPreset trims per-host idle conns and reclaims them fast: a wide crawl meets each host once, then ghosts it.
func multiHostPreset() Option {
	return optionFunc(func(c *config) {
		WithMaxIdleConnsPerHost(DefaultMultiHostIdleConnsPerHost).apply(c)
		WithIdleConnTimeout(DefaultMultiHostIdleConnTimeout).apply(c)
		WithHTTP2Config(defaultHTTP2Config()).apply(c)
	})
}

// WithMaxIdleConns sets the total idle-connection pool size across all hosts.
func WithMaxIdleConns(n int) TransportOption {
	return optionFunc(func(c *config) { c.maxIdleConns = n })
}

// WithMaxIdleConnsPerHost sets the idle-connection pool size per host.
func WithMaxIdleConnsPerHost(n int) TransportOption {
	return optionFunc(func(c *config) { c.maxIdleConnsPerHost = n })
}

// WithMaxConnsPerHost caps total (active plus idle) connections per host; 0 is unlimited.
func WithMaxConnsPerHost(n int) TransportOption {
	return optionFunc(func(c *config) { c.maxConnsPerHost = n })
}

// WithIdleConnTimeout sets how long an idle connection is kept before closing.
func WithIdleConnTimeout(d time.Duration) TransportOption {
	return optionFunc(func(c *config) { c.idleConnTimeout = d })
}

// WithTLSHandshakeTimeout sets the TLS handshake deadline.
func WithTLSHandshakeTimeout(d time.Duration) TransportOption {
	return optionFunc(func(c *config) { c.tlsHandshakeTimeout = d })
}

// WithResponseHeaderTimeout sets how long to wait for response headers after the request.
func WithResponseHeaderTimeout(d time.Duration) TransportOption {
	return optionFunc(func(c *config) { c.responseHeaderTimeout = d })
}

// WithExpectContinueTimeout sets the wait for a 100-Continue after Expect headers.
func WithExpectContinueTimeout(d time.Duration) TransportOption {
	return optionFunc(func(c *config) { c.expectContinueTimeout = d })
}

// WithProtocols sets the HTTP protocols the transport negotiates.
func WithProtocols(p *http.Protocols) TransportOption {
	return optionFunc(func(c *config) { c.protocols = p })
}

// WithHTTP2Config sets the transport's HTTP/2 configuration.
func WithHTTP2Config(h2 *http.HTTP2Config) TransportOption {
	return optionFunc(func(c *config) { c.http2 = h2 })
}

// WithTLSMinVersion sets the minimum accepted TLS version (e.g. tls.VersionTLS13).
func WithTLSMinVersion(v uint16) TransportOption {
	return optionFunc(func(c *config) { c.tlsMinVersion = v })
}

// WithDialContext redirects only the TCP dial to a caller-resolved IP; the URL host stays the pool
// key, Host header, and SNI, which is what keeps two SNI-routed backends on one IP from colliding on
// a shared TLS session. The dialer MUST dial that ctx IP and IGNORE addr: Go passes the URL host as
// addr, so dialing it re-resolves through DNS and hands your pinned request to whatever the resolver
// feels like, quietly undoing the pin. It runs concurrently across RoundTrips, so guard shared
// resolver state; it also ghosts HTTP(S)_PROXY (that rides in addr too), leaving the proxy feeling
// invited while every request strolls past it. Honor ctx cancellation and set a TCP timeout.
// You own the dial now, so WithDialGuard does not reach this: put your own Control on the dialer.
func WithDialContext(fn func(context.Context, string, string) (net.Conn, error)) TransportOption {
	return optionFunc(func(c *config) { c.dialContext = fn })
}

// WithDialGuard refuses any dial to a private, loopback, link-local, or cloud-metadata IP, checked
// at dial time on the resolved (and WithDialIP-pinned) address. Use it wherever a URL or redirect
// target is attacker-influenced. It also nulls the proxy: a guard and an egress proxy are mutually
// exclusive (Control would only ever see the proxy's IP), so the guard wins. Covers the built-in
// dialer only, not a caller-supplied WithDialContext.
func WithDialGuard() TransportOption {
	return optionFunc(func(c *config) { c.dialControl = dialGuard })
}

// WithDialIP pins the TCP dial target for a delegated host whose resolved IP differs from its name.
// Empty ip is a no-op. Takes effect only on the built-in dialer, not a WithDialContext override.
func WithDialIP(ctx context.Context, ip string) context.Context {
	if ip == "" {
		return ctx
	}
	return context.WithValue(ctx, dialIPKey{}, ip)
}

func dialIPFromContext(ctx context.Context) string {
	ip, ok := ctx.Value(dialIPKey{}).(string)
	if !ok {
		return ""
	}
	return ip
}

// WithPerAttemptTimeout sets the deadline applied to each attempt; 0 disables it.
func WithPerAttemptTimeout(d time.Duration) RetryOption {
	return retryOptionFunc(func(c *config) { c.perAttemptTimeout = d })
}

// WithMaxRetryAfter sets the ceiling on an honored Retry-After header.
func WithMaxRetryAfter(d time.Duration) RetryOption {
	return retryOptionFunc(func(c *config) { c.maxRetryAfter = d })
}

// WithRetry replaces the default retrier. The caller then owns backoff, jitter, and the
// retry predicate; WithRetryIf, WithMaxRetries, and WithRetryDelayStep are ignored once this
// is set.
func WithRetry(r *retry.Retry) RetryOption {
	return retryOptionFunc(func(c *config) { c.retrier = r })
}

// WithRetryIf overrides the predicate deciding which errors are retryable. Ignored when
// WithRetry supplies a full retrier.
func WithRetryIf(predicate func(error) bool) RetryOption {
	return retryOptionFunc(func(c *config) { c.retryIf = predicate })
}

// WithMaxRetries sets the default retrier's total attempt count, first try included: n=1 is one
// shot, zero mercy. Same warty count as retry.WithMaxRetries; ignored when WithRetry brings its own.
func WithMaxRetries(n int) RetryOption {
	return retryOptionFunc(func(c *config) { c.maxRetries = n })
}

// WithRetryDelayStep sets the default retrier's linear backoff step: retry i waits step*(i+1),
// jittered. Same retrier, same classifier, just a longer fuse before each try. Ignored when
// WithRetry supplies its own.
func WithRetryDelayStep(d time.Duration) RetryOption {
	return retryOptionFunc(func(c *config) { c.delayStep = d })
}

// WithRetryNonIdempotent opts non-idempotent methods (POST, PATCH) into retry. Off by
// default: a replayed POST can double-apply a side effect.
func WithRetryNonIdempotent(on bool) RetryOption {
	return retryOptionFunc(func(c *config) { c.retryNonIdempotent = on })
}

// WithRetryBudget sets the cross-request retry budget.
func WithRetryBudget(b RetryBudget) RetryOption {
	return retryOptionFunc(func(c *config) {
		if b != nil {
			c.budget = b
		}
	})
}

// WithOnAttempt registers a hook called after every attempt with its AttemptInfo. The hook
// runs on every concurrent RoundTrip of a shared client, so it must be safe for concurrent use.
func WithOnAttempt(hook func(AttemptInfo)) RetryOption {
	return retryOptionFunc(func(c *config) { c.onAttempt = hook })
}
