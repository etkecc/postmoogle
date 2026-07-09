// Package httpclient builds an *http.Client tuned for talking to one backend a lot:
// a right-sized connection pool, a deadline on each attempt, and transparent retries
// that only replay requests it is safe to replay.
//
// New is the general constructor; NewSingleHost presets the pool for a single backend;
// Wrap adds the retry layer to a caller-supplied client. All three return a plain
// *http.Client with no error to check.
//
// Zero external dependencies: stdlib plus the go-kit retry sibling.
package httpclient

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/etkecc/go-kit/retry"
)

var (
	// ErrNonReplayableBody guards against silent body corruption on retry: it is returned by
	// RoundTrip when a request is eligible for retry (idempotent method, or opted in via
	// WithRetryNonIdempotent) but its body has no GetBody to rewind. Retrying would replay an
	// already-consumed reader, so it fails loud instead of sending a truncated body on the
	// second attempt.
	ErrNonReplayableBody = errors.New("httpclient: retryable request has a body but no GetBody to rewind it")

	errAttemptTimeout  = errors.New("httpclient: per-attempt timeout")
	errBudgetExhausted = errors.New("httpclient: retry budget exhausted")
)

// AttemptInfo is passed to the WithOnAttempt hook after every attempt, so a caller can
// observe a retry storm as it happens.
type AttemptInfo struct {
	Method   string
	Host     string
	Attempt  int           // 1-based
	Status   int           // 0 when the attempt errored before any response
	Err      error         // nil on a response, even a retryable one
	Elapsed  time.Duration // wall time for this attempt alone
	Retrying bool          // the attempt was retry-eligible and handed back to the retrier, which may still stop on exhausted attempts or budget
	Reused   bool          // the connection was reused (via httptrace, when a conn was got)
}

// RetryBudget gates retries across requests to bound retry amplification during an
// outage. Allow is consulted before each retry, never before the first attempt. Record
// reports whether a retry was performed (true) or denied by the budget (false), so a
// token-bucket implementation can account for spend; it is not a success/failure signal.
// The default implementation always allows; supply a token bucket to cap. Implementations
// must be safe for concurrent use: a shared client calls this from every RoundTrip.
type RetryBudget interface {
	Allow() bool
	Record(retried bool)
}

// noopBudget is the default RetryBudget: it allows every retry.
type noopBudget struct{}

func (noopBudget) Allow() bool   { return true }
func (noopBudget) Record(_ bool) {}

// config is the mutable target of the functional options. defaultConfig seeds every
// field; options overwrite; build turns it into a wired *http.Client.
type config struct {
	maxIdleConns        int
	maxIdleConnsPerHost int
	maxConnsPerHost     int

	idleConnTimeout       time.Duration
	tlsHandshakeTimeout   time.Duration
	responseHeaderTimeout time.Duration
	expectContinueTimeout time.Duration
	perAttemptTimeout     time.Duration

	protocols     *http.Protocols
	http2         *http.HTTP2Config
	tlsMinVersion uint16
	dialContext   func(context.Context, string, string) (net.Conn, error)
	dialControl   func(string, string, syscall.RawConn) error

	retrier            *retry.Retry
	retryIf            func(error) bool
	maxRetries         int
	delayStep          time.Duration
	retryNonIdempotent bool
	maxRetryAfter      time.Duration
	budget             RetryBudget

	onAttempt func(AttemptInfo)
}

// New builds a general-purpose retrying client, suitable for many hosts: per-host
// connections stay uncapped and the idle pool is sized for throughput.
func New(opts ...Option) *http.Client {
	return defaultConfig().apply(opts).build()
}

// NewSingleHost presets the pool for one backend: all three pool dimensions sized to
// DefaultPoolSize (per-host connections bounded, not merely idle-capped) plus HTTP/2
// keepalive pings to notice a dead persistent connection.
func NewSingleHost(opts ...Option) *http.Client {
	return defaultConfig().apply(append([]Option{singleHostPreset()}, opts...)).build()
}

// NewMultiHost presets the pool for a wide, shallow crawl: thousands of hosts each seen once and
// never called back, so per-host idle drops low while per-host concurrency stays unbounded.
func NewMultiHost(opts ...Option) *http.Client {
	return defaultConfig().apply(append([]Option{multiHostPreset()}, opts...)).build()
}

// NewTransport returns the tuned base transport (safe pool defaults, no retry layer) for a caller
// building their own client. Read-only by intent: rebuild it into an H2-coalescing transport and
// you hand back the cross-host cert confusion the pool defaults exist to dodge. Your problem then.
func NewTransport(opts ...TransportOption) *http.Transport {
	c := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt.apply(c)
		}
	}
	return c.newTransport()
}

// Wrap adds the retry layer to a caller-supplied client, leaving its transport tuning
// alone. It takes only RetryOption, so a transport knob is a compile error, not a runtime
// conflict. A nil client is treated as empty. A non-zero client.Timeout is kept and bounds
// the whole retry sequence, not one attempt. Do not Wrap a client that already carries this
// package's retry middleware: the layers stack and attempts multiply.
func Wrap(client *http.Client, opts ...RetryOption) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	c := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt.apply(c)
		}
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	return &http.Client{
		Transport:     c.newRetryTransport(base),
		CheckRedirect: client.CheckRedirect,
		Jar:           client.Jar,
		Timeout:       client.Timeout,
	}
}

// apply runs the options in order, skipping nils, and returns the config for chaining.
func (c *config) apply(opts []Option) *config {
	for _, opt := range opts {
		if opt != nil {
			opt.apply(c)
		}
	}
	return c
}

// build wires the config into a *http.Client.
func (c *config) build() *http.Client {
	return &http.Client{Transport: c.newRetryTransport(c.newTransport())}
}

// newRetryTransport wraps base in the retrying RoundTripper from the current config.
func (c *config) newRetryTransport(base http.RoundTripper) *retryTransport {
	return &retryTransport{
		base:          base,
		retrier:       c.buildRetrier(),
		perAttempt:    c.perAttemptTimeout,
		nonIdem:       c.retryNonIdempotent,
		maxRetryAfter: c.maxRetryAfter,
		budget:        c.budget,
		onAttempt:     c.onAttempt,
	}
}

// buildRetrier returns the caller's retrier via WithRetry, or a default one with the
// classifier baked in once (static, not rebuilt per request).
func (c *config) buildRetrier() *retry.Retry {
	if c.retrier != nil {
		return c.retrier
	}
	classify := c.retryIf
	if classify == nil {
		classify = defaultRetryIf
	}
	return retry.New(
		retry.WithMaxRetries(c.maxRetries),
		retry.WithDelayStep(c.delayStep),
		retry.WithJitter(true),
		retry.WithRetryIf(classify),
	)
}

// newTransport builds the base *http.Transport from the pool and protocol config. The
// client Timeout stays 0: retryTransport owns per-attempt deadlines, and a client Timeout
// would cap the whole retry sequence instead, killing a legitimate second try.
func (c *config) newTransport() *http.Transport {
	t := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          c.maxIdleConns,
		MaxIdleConnsPerHost:   c.maxIdleConnsPerHost,
		MaxConnsPerHost:       c.maxConnsPerHost,
		IdleConnTimeout:       c.idleConnTimeout,
		TLSHandshakeTimeout:   c.tlsHandshakeTimeout,
		ResponseHeaderTimeout: c.responseHeaderTimeout,
		ExpectContinueTimeout: c.expectContinueTimeout,
		Protocols:             c.protocols,
		HTTP2:                 c.http2,
		TLSClientConfig:       &tls.Config{MinVersion: c.tlsMinVersion},
	}
	if c.dialContext != nil {
		// caller owns the dial and its own Control; WithDialGuard does not reach here.
		t.DialContext = c.dialContext
	} else {
		// built-in dialer: a 30s dial timeout + keepalive, carrying the guard (nil unless WithDialGuard)
		// and honoring a WithDialIP pin by rewriting addr so Control fires on the pinned IP, not the name.
		dialer := &net.Dialer{Timeout: defaultDialTimeout, KeepAlive: defaultDialKeepAlive, Control: c.dialControl}
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if pinned := dialIPFromContext(ctx); pinned != "" {
				_, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				addr = net.JoinHostPort(pinned, port)
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}
	if c.dialControl != nil {
		// a guard and an egress proxy are mutually exclusive: Control would only see the proxy's IP.
		t.Proxy = nil
	}
	return t
}
