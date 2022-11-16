# logger

Simple go logger, based on [log](https://pkg.go.dev/log) with following features:

* implements mautrix-go [Logger](https://pkg.go.dev/maunium.net/go/mautrix#Logger), [WarnLogger](https://pkg.go.dev/maunium.net/go/mautrix#WarnLogger), [crypto Logger](https://pkg.go.dev/maunium.net/go/mautrix/crypto#Logger)
* integrated with [sentry](https://sentry.io) - automatically add breadcrumbs for any log entry
