package healthchecks

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Client for healthchecks
type Client struct {
	HTTP    *http.Client
	log     func(string, error)
	baseURL string
	uuid    string
	rid     string
	done    chan bool
}

func (c *Client) call(operation, endpoint string, body ...io.Reader) {
	var err error
	var resp *http.Response
	targetURL := fmt.Sprintf("%s/%s%s?rid=%s", c.baseURL, c.uuid, endpoint, c.rid)
	if len(body) > 0 {
		resp, err = c.HTTP.Post(targetURL, "text/plain; charset=utf-8", body[0])
	} else {
		resp, err = c.HTTP.Head(targetURL)
	}
	if err != nil {
		c.log(operation, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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
