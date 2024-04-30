# healthchecks

A fully async [healthchecks.io](https://github.com/healthchecks/healthchecks) golang client, with lots of features, some highlights:

* Highly configurable: `WithHTTPClient()`, `WithBaseURL()`, `WithUserAgent()`, `WithErrLog()`, `WithCheckUUID()`, `WithAutoProvision()`, etc.
* Automatic determination of HTTP method (`POST`, `HEAD`) based on body existence
* Auto mode: just call `client.Auto(time.Duration)` and client will send `Success()` request automatically with specified frequency
* Global mode: init client once with `healthchecks.New()`, and access it from anywhere by calling `healthchecks.Global()`

Check [godoc](https://pkg.go.dev/gitlab.com/etke.cc/go/healthchecks/v2) for more details.

```go
package main

import (
    "time"

    "gitlab.com/etke.cc/go/healthchecks/v2"
)

var hc *healthchecks.Client

func main() {
    hc = healthchecks.New(
        healthchecks.WithCheckUUID("CHECK_UUID")
    )
    defer hc.Shutdown()
    // send basic success request
    hc.Success()

    // or use auto mode, that will send success request with the specified frequency
    go hc.Auto(1*time.Minute)

    // need to call the client from another place in your project?
    // just call healthchecks.Global() and you will get the same client
}
```
