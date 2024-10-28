package healthchecks

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

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
}

// init client
func (c *Client) init(options ...Option) {
	c.enabled = true
	c.log = DefaultErrLog
	c.baseURL = DefaultAPI
	c.userAgent = DefaultUserAgent
	c.http = &http.Client{Timeout: 10 * time.Second}
	c.done = make(chan bool, 1)
	c.uuid = ""

	if len(options) == 0 {
		c.enabled = false
	}

	for _, option := range options {
		option(c)
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
	if !c.enabled {
		return
	}

	c.wg.Add(1)
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
		req, err = http.NewRequest(http.MethodPost, targetURL, body[0])
	} else {
		req, err = http.NewRequest(http.MethodHead, targetURL, http.NoBody)
	}
	if err != nil {
		c.log(operation, err)
		return
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := c.http.Do(req)
	if err != nil {
		c.log(operation, err)
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
	c.enabled = enabled
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

// Shutdown the client
func (c *Client) Shutdown() {
	c.done <- true
	c.wg.Wait()
}
