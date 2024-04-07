package healthchecks

import (
	"fmt"

	"github.com/google/uuid"
)

// DefaultAPI base url for checks
const DefaultAPI = "https://hc-ping.com"

// ErrLog used to log errors occurred during an operation
type ErrLog func(operation string, err error)

// DefaultErrLog if you don't provide one yourself
var DefaultErrLog = func(operation string, err error) {
	fmt.Printf("healtchecks operation %q failed: %v\n", operation, err)
}

// New healthchecks client
func New(options ...Option) *Client {
	rid, _ := uuid.NewRandom()
	c := &Client{
		rid: rid.String(),
	}
	c.init(options...)

	return c
}
