package healthchecks

import "net/http"

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
