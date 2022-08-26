# Postmoogle [![Matrix](https://img.shields.io/matrix/postmoogle:etke.cc?logo=matrix&style=for-the-badge)](https://matrix.to/#/#postmoogle:etke.cc)[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/etkecc) [![coverage report](https://gitlab.com/etke.cc/postmoogle/badges/main/coverage.svg)](https://gitlab.com/etke.cc/postmoogle/-/commits/main) [![Go Report Card](https://goreportcard.com/badge/gitlab.com/etke.cc/postmoogle)](https://goreportcard.com/report/gitlab.com/etke.cc/postmoogle) [![Go Reference](https://pkg.go.dev/badge/gitlab.com/etke.cc/postmoogle.svg)](https://pkg.go.dev/gitlab.com/etke.cc/postmoogle)

> [more about that name](https://finalfantasy.fandom.com/wiki/The_Little_Postmoogle_That_Could)

An Email to Matrix bridge

## Roadmap

### Receive

- [x] SMTP server
- [x] Matrix bot
- [x] Configuration in room's account data
- [x] Receive emails to matrix rooms
- [x] Receive attachments
- [x] Map email threads to matrix threads

### Send

- [ ] SMTP client
- [ ] Reply to matrix thread sends reply into email thread
- [ ] Send a message to matrix room with special format to send a new email

## Configuration

env vars

### mandatory

* **POSTMOOGLE_HOMESERVER** - homeserver url, eg: `https://matrix.example.com`
* **POSTMOOGLE_LOGIN** - user login/localpart, eg: `moogle`
* **POSTMOOGLE_PASSWORD** - user password
* **POSTMOOGLE_DOMAIN** - SMTP domain to listen for new emails
* **POSTMOOGLE_PORT** - SMTP port to listen for new emails

### optional

* **POSTMOOGLE_NOOWNER** - allow change room settings by any room partisipant
* **POSTMOOGLE_FEDERATION** - allow usage of Postmoogle by users from others homeservers
* **POSTMOOGLE_NOENCRYPTION** - disable encryption support
* **POSTMOOGLE_STATUSMSG** - presence status message
* **POSTMOOGLE_SENTRY_DSN** - sentry DSN
* **POSTMOOGLE_LOGLEVEL** - log level
* **POSTMOOGLE_DB_DSN** - database connection string
* **POSTMOOGLE_DB_DIALECT** - database dialect (postgres, sqlite3)
* **POSTMOOGLE_MAXSIZE** - max email size (including attachments) in megabytes
* **POSTMOOGLE_USERS** - a space-separated list of whitelisted users allowed to use the bridge. If not defined, everyone is allowed. Example rule: `@someone:example.com @another:example.com @bot.*:example.com @*:another.com`

You can find default values in [config/defaults.go](config/defaults.go)

## Where to get

[docker registry](https://gitlab.com/etke.cc/postmoogle/container_registry), [etke.cc](https://etke.cc)
