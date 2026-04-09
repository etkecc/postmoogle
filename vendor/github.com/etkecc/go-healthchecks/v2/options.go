package healthchecks

import (
	"net/http"
	"time"
)

// Option for healthchecks client
type Option func(*Client)

// WithHTTPClient sets the http client
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.http = httpClient
	}
}

// WithBaseURL sets the base url
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithUserAgent sets the user agent
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		c.userAgent = userAgent
	}
}

// WithErrLog sets the error log
func WithErrLog(errLog ErrLog) Option {
	return func(c *Client) {
		c.log = errLog
	}
}

// WithCheckUUID sets the check UUID
func WithCheckUUID(uuid string) Option {
	return func(c *Client) {
		c.uuid = uuid
	}
}

// WithAutoProvision enables auto provision
func WithAutoProvision() Option {
	return func(c *Client) {
		c.create = true
	}
}

// WithGlobal sets this client as the global client
func WithGlobal() Option {
	return func(c *Client) {
		global = c
	}
}

// WithDone sets the done channel
func WithDone(done chan bool) Option {
	return func(c *Client) {
		c.done = done
	}
}

// DefaultHTTPClient returns a default http client, optimized for single-host usage with keep-alive connections, and with reasonable timeouts to prevent hanging requests.
func DefaultHTTPClient() *http.Client {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{Timeout: 10 * time.Second}
	}
	transport := defaultTransport.Clone()
	transport.MaxIdleConns = 100                       // Maximum number of idle (keep-alive) connections across ALL hosts. Zero means no limit.
	transport.MaxIdleConnsPerHost = 100                // Maximum number of idle (keep-alive) connections to keep PER-host. Zero means no limit.
	transport.MaxConnsPerHost = 100                    // Maximum number of connections PER-host, including connections in the dialing, active, and idle states. Zero means no limit.
	transport.IdleConnTimeout = 90 * time.Second       // Maximum amount of time an idle (keep-alive) connection will remain idle before closing itself. Zero means no limit.
	transport.ForceAttemptHTTP2 = true                 // If true, the Transport will attempt to use HTTP/2.0 if the server supports it, even if the Request's URL is not HTTPS.
	transport.TLSHandshakeTimeout = 10 * time.Second   // Maximum amount of time waiting to wait for a TLS handshake. Zero means no timeout.
	transport.ResponseHeaderTimeout = 10 * time.Second // Maximum amount of time to wait for a server's response headers after fully writing the request (including its body, if any). Zero means no timeout.
	transport.ExpectContinueTimeout = 1 * time.Second  // Maximum amount of time to wait for a server's first response headers after fully writing the request headers if the request has an "Expect: 100-continue" header. Zero means no timeout.

	return &http.Client{Transport: transport}
}
