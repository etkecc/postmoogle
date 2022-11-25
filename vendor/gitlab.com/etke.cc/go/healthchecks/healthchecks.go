package healthchecks

import (
	"fmt"
	"net/http"
	"time"

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
func New(hcUUID string, errlog ...ErrLog) *Client {
	rid, _ := uuid.NewRandom()
	c := &Client{
		baseURL: DefaultAPI,
		uuid:    hcUUID,
		rid:     rid.String(),
		done:    make(chan bool, 1),
	}
	c.HTTP = &http.Client{
		Timeout: 10 * time.Second,
	}
	c.log = DefaultErrLog
	if len(errlog) > 0 {
		c.log = errlog[0]
	}

	return c
}
