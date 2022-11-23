
# blitiri.com.ar/go/spf

[![GoDoc](https://godoc.org/blitiri.com.ar/go/spf?status.svg)](https://pkg.go.dev/blitiri.com.ar/go/spf)
[![Build Status](https://gitlab.com/albertito/spf/badges/master/pipeline.svg)](https://gitlab.com/albertito/spf/-/pipelines)
[![Go Report Card](https://goreportcard.com/badge/github.com/albertito/spf)](https://goreportcard.com/report/github.com/albertito/spf)
[![Coverage Status](https://coveralls.io/repos/github/albertito/spf/badge.svg?branch=next)](https://coveralls.io/github/albertito/spf)

[spf](https://godoc.org/blitiri.com.ar/go/spf) is an open source
implementation of the [Sender Policy Framework
(SPF)](https://en.wikipedia.org/wiki/Sender_Policy_Framework) in Go.

It is used by the [chasquid](https://blitiri.com.ar/p/chasquid/) and
[maddy](https://maddy.email) SMTP servers.


## Example

```go
// Check if `sender` is authorized to send from the given `ip`. The `domain`
// is used if the sender doesn't have one.
result, err := spf.CheckHostWithSender(ip, domain, sender)
if result == spf.Fail {
	// Not authorized to send.
}
```

See the [package documentation](https://pkg.go.dev/blitiri.com.ar/go/spf) for
more details.


## Status

All SPF mechanisms, modifiers, and macros are supported.

The API should be considered stable. Major version changes will be announced
to the mailing list (details below).


## Contact

If you have any questions, comments or patches please send them to the mailing
list, `chasquid@googlegroups.com`.

To subscribe, send an email to `chasquid+subscribe@googlegroups.com`.

You can also browse the
[archives](https://groups.google.com/forum/#!forum/chasquid).

