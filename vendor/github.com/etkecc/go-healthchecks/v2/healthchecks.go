package healthchecks

import (
	"fmt"

	"github.com/google/uuid"
)

const (
	// DefaultAPI base url for checks
	DefaultAPI = "https://hc-ping.com"
	// DefaultUserAgent for the client
	DefaultUserAgent = "Go-Healthchecks (lib; +https://gitlab.com/etke.cc/go/healthchecks)"
)

// ErrLog used to log errors occurred during an operation
type ErrLog func(operation string, err error)

// global client
var global *Client

// DefaultErrLog if you don't provide one yourself
var DefaultErrLog = func(operation string, err error) {
	fmt.Printf("healtchecks operation %q failed: %v\n", operation, err)
}

// New healthchecks client
func New(options ...Option) *Client {
	rid, _ := uuid.NewRandom() //nolint:errcheck // ignore error
	c := &Client{
		rid: rid.String(),
	}
	c.init(options...)

	if global == nil {
		global = c
	}

	return c
}

// Global healthchecks client
func Global() *Client {
	if global == nil {
		global = New()
	}

	return global
}
