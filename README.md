# Postmoogle

An Email to Matrix bridge

## Features / Roadmap / TODO

- [x] SMTP server
- [ ] SMTP client
- [x] Matrix bot
- [x] Configuration in room's account data
- [ ] Receive emails to matrix rooms
- [ ] Map email threads to matrix threads
- [ ] Reply to matrix thread sends reply into email thread
- [ ] Send a message to matrix room with special format to send a new email

## Configuration

env vars

### mandatory

* **POSTMOOGLE_HOMESERVER** - homeserver url, eg: `https://matrix.example.com`
* **POSTMOOGLE_LOGIN** - user login/localpart, eg: `scheduler`
* **POSTMOOGLE_PASSWORD** - user password
* **POSTMOOGLE_DOMAIN** - SMTP domain to listen for new emails
* **POSTMOOGLE_PORT** - SMTP port to listen for new emails

### optional

* **POSTMOOGLE_SENTRY_DSN** - sentry DSN
* **POSTMOOGLE_SENTRY_RATE** - sentry sample rate, from 0 to 100 (default: 20)
* **POSTMOOGLE_LOGLEVEL** - log level
* **POSTMOOGLE_DB_DSN** - database connection string
* **POSTMOOGLE_DB_DIALECT** - database dialect (postgres, sqlite3)

You can find default values in [config/defaults.go](config/defaults.go)
