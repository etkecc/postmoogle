package healthchecks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/etkecc/go-kit/httpclient"
	"github.com/google/uuid"
)

// Client for healthchecks
// if client initialized without any options, it will be disabled by default,
// but you can override it by calling SetEnabled(true).
type Client struct {
	wg        sync.WaitGroup
	enabled   bool
	http      *http.Client
	log       func(string, error)
	userAgent string
	baseURL   string
	uuid      string
	rid       string
	create    bool
	done      chan bool
	ctx       context.Context //nolint:containedctx // lifecycle ctx for a background pinger with no inbound request scope; Shutdown cancels it to bound in-flight retries
	cancel    context.CancelFunc
	mu        sync.Mutex // guards closed + wg.Add against Shutdown's Wait
	closed    bool
}

// init client
func (c *Client) init(options ...Option) {
	c.enabled = true
	c.log = DefaultErrLog
	c.baseURL = DefaultAPI
	c.userAgent = DefaultUserAgent
	c.done = make(chan bool, 1)
	c.uuid = ""
	c.ctx, c.cancel = context.WithCancel(context.Background())

	if len(options) == 0 {
		c.enabled = false
	}

	for _, option := range options {
		option(c)
	}

	// A WithHTTPClient option set c.http (BYO): wrap it in retry, keeping its transport.
	// Otherwise build a fresh single-host retrying client. Both constructors are infallible.
	if c.http != nil {
		c.http = httpclient.Wrap(c.http, httpclient.WithRetryNonIdempotent(true), httpclient.WithOnAttempt(c.onAttempt))
	} else {
		c.http = httpclient.NewSingleHost(httpclient.WithRetryNonIdempotent(true), httpclient.WithOnAttempt(c.onAttempt))
	}

	if c.uuid == "" {
		randomUUID, _ := uuid.NewRandom() //nolint:errcheck // ignore error
		c.uuid = randomUUID.String()
		c.create = true
		c.log("uuid", fmt.Errorf("check UUID is not provided, using random %q with auto provision", c.uuid))
	}
}

// Call API
func (c *Client) Call(operation, endpoint string, body ...io.Reader) {
	// enabled/closed check and Add share the lock: an Add racing wg.Wait at zero panics.
	c.mu.Lock()
	if !c.enabled || c.closed {
		c.mu.Unlock()
		return
	}
	c.wg.Add(1)
	c.mu.Unlock()

	go c.call(operation, endpoint, body...)
}

// call API, extracted from c.Call to reduce cyclomatic complexity
func (c *Client) call(operation, endpoint string, body ...io.Reader) {
	defer c.wg.Done()

	targetURL := fmt.Sprintf("%s/%s%s?rid=%s", c.baseURL, c.uuid, endpoint, c.rid)
	if c.create {
		targetURL += "&create=1"
	}

	var req *http.Request
	var err error
	if len(body) > 0 {
		req, err = http.NewRequestWithContext(c.ctx, http.MethodPost, targetURL, body[0])
	} else {
		req, err = http.NewRequestWithContext(c.ctx, http.MethodHead, targetURL, http.NoBody)
	}
	if err != nil {
		c.log(operation, err)
		return
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := c.http.Do(req)
	if err != nil {
		// A canceled-by-Shutdown ping is expected, not a failure worth logging.
		if !errors.Is(err, context.Canceled) {
			c.log(operation, err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respb, rerr := io.ReadAll(resp.Body)
		if rerr != nil {
			c.log(operation+":response", rerr)
			return
		}
		rerr = errors.New(string(respb))
		c.log(operation+":response", rerr)
		return
	}
}

// SetEnabled sets the enabled flag, ignoring the options
// if client initialized without any options, it will be disabled by default,
// but you can override it by calling SetEnabled(true).
func (c *Client) SetEnabled(enabled bool) {
	c.mu.Lock()
	c.enabled = enabled
	c.mu.Unlock()
}

// Start signal means the job started
func (c *Client) Start(optionalBody ...io.Reader) {
	c.Call("start", "/start", optionalBody...)
}

// Success signal means the job has completed successfully (or, a continuously running process is still running and healthy).
func (c *Client) Success(optionalBody ...io.Reader) {
	c.Call("success", "", optionalBody...)
}

// Fail signal means the job failed
func (c *Client) Fail(optionalBody ...io.Reader) {
	c.Call("fail", "/fail", optionalBody...)
}

// Log signal just adds an event to the job log, without changing job status
func (c *Client) Log(optionalBody ...io.Reader) {
	c.Call("log", "/log", optionalBody...)
}

// ExitStatus signal sends job's exit code (0-255)
func (c *Client) ExitStatus(exitCode int, optionalBody ...io.Reader) {
	c.Call("exit status", "/"+strconv.Itoa(exitCode), optionalBody...)
}

// onAttempt logs intermediate transport-error retries only; a status retry (503->200) has a nil Err and stays silent.
func (c *Client) onAttempt(info httpclient.AttemptInfo) { //nolint:gocritic // by value to satisfy httpclient.WithOnAttempt's func(AttemptInfo) signature
	if info.Retrying && info.Err != nil {
		c.log("retry", fmt.Errorf("%s %s attempt %d: %w", info.Method, info.Host, info.Attempt, info.Err))
	}
}

// Shutdown rejects new calls, cancels in-flight retries, then drains. Idempotent; cancel precedes Wait to bound the drain.
func (c *Client) Shutdown() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()

	c.cancel()
	c.done <- true
	c.wg.Wait()
}
