package healthchecks

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Client for healthchecks
type Client struct {
	http    *http.Client
	log     func(string, error)
	baseURL string
	uuid    string
	rid     string
	create  bool
	done    chan bool
}

// init client
func (c *Client) init(options ...Option) {
	for _, option := range options {
		option(c)
	}
	if c.log == nil {
		c.log = DefaultErrLog
	}
	if c.baseURL == "" {
		c.baseURL = DefaultAPI
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: 10 * time.Second}
	}
	if c.done == nil {
		c.done = make(chan bool, 1)
	}
	if c.uuid == "" {
		randomUUID, _ := uuid.NewRandom()
		c.uuid = randomUUID.String()
		c.create = true
		c.log("uuid", fmt.Errorf("check UUID is not provided, using random %q with auto provision", c.uuid))
	}
}

func (c *Client) call(operation, endpoint string, body ...io.Reader) {
	var err error
	var resp *http.Response
	targetURL := fmt.Sprintf("%s/%s%s?rid=%s", c.baseURL, c.uuid, endpoint, c.rid)
	if c.create {
		targetURL += "&create=1"
	}
	if len(body) > 0 {
		resp, err = c.http.Post(targetURL, "text/plain; charset=utf-8", body[0])
	} else {
		resp, err = c.http.Head(targetURL)
	}
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
		rerr = fmt.Errorf(string(respb))
		c.log(operation+":response", rerr)
		return
	}
}

// Start signal means the job started
func (c *Client) Start(optionalBody ...io.Reader) {
	c.call("start", "/start", optionalBody...)
}

// Success signal means the job has completed successfully (or, a continuously running process is still running and healthy).
func (c *Client) Success(optionalBody ...io.Reader) {
	c.call("success", "", optionalBody...)
}

// Fail signal means the job failed
func (c *Client) Fail(optionalBody ...io.Reader) {
	c.call("fail", "/fail", optionalBody...)
}

// Log signal just adds an event to the job log, without changing job status
func (c *Client) Log(optionalBody ...io.Reader) {
	c.call("log", "/log", optionalBody...)
}

// ExitStatus signal sends job's exit code (0-255)
func (c *Client) ExitStatus(exitCode int, optionalBody ...io.Reader) {
	c.call("exit status", "/"+strconv.Itoa(exitCode), optionalBody...)
}
