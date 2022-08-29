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

* **POSTMOOGLE_HOMESERVER** - homeserver url, eg: `https://matrix.example.com`
* **POSTMOOGLE_LOGIN** - user login/localpart, eg: `moogle`
* **POSTMOOGLE_PASSWORD** - user password
* **POSTMOOGLE_DOMAIN** - SMTP domain to listen for new emails
* **POSTMOOGLE_PORT** - SMTP port to listen for new emails

<details>
<summary>other optional config parameters</summary>

* **POSTMOOGLE_NOENCRYPTION** - disable encryption support
* **POSTMOOGLE_STATUSMSG** - presence status message
* **POSTMOOGLE_SENTRY_DSN** - sentry DSN
* **POSTMOOGLE_LOGLEVEL** - log level
* **POSTMOOGLE_DB_DSN** - database connection string
* **POSTMOOGLE_DB_DIALECT** - database dialect (postgres, sqlite3)
* **POSTMOOGLE_MAXSIZE** - max email size (including attachments) in megabytes
* **POSTMOOGLE_USERS** - a space-separated list of whitelisted users allowed to use the bridge. If not defined, everyone is allowed. Example rule: `@someone:example.com @another:example.com @bot.*:example.com @*:another.com`
* **POSTMOOGLE_ADMINS** - a space-separated list of admin users. See `POSTMOOGLE_USERS` for syntax examples

You can find default values in [config/defaults.go](config/defaults.go)

</details>

## Usage

### How to start

1. Invite the bot into a room you want to use as mailbox
2. Read the bot's introduction
3. Set mailbox using `!pm mailbox NAME` where `NAME` is part of email (e.g. `NAME@example.com`)
4. Done. Mailbox owner and other options will be set automatically when you configure mailbox.
If you want to change them - check available options in the help message (`!pm help`)

<details>
<summary>Full list of available commands</summary>

* **!pm help** - Show help message
* **!pm stop** - Disable bridge for the room and clear all configuration

---

* **!pm mailbox** - Get or set mailbox of the room
* **!pm owner** - Get or set owner of the room

---

* **!pm nosender** - Get or set `nosender` of the room (`true` - hide email sender; `false` - show email sender)
* **!pm nosubject** - Get or set `nosubject` of the room (`true` - hide email subject; `false` - show email subject)
* **!pm nohtml** - Get or set `nohtml` of the room (`true` - ignore HTML in email; `false` - parse HTML in emails)
* **!pm nothreads** - Get or set `nothreads` of the room (`true` - ignore email threads; `false` - convert email threads into matrix threads)
* **!pm nofiles** - Get or set `nofiles` of the room (`true` - ignore email attachments; `false` - upload email attachments)

---

* **!pm mailboxes** - Show the list of all mailboxes
* **!pm delete** &lt;mailbox&gt; - Delete specific mailbox

</details>


## Where to get

[docker registry](https://gitlab.com/etke.cc/postmoogle/container_registry), [etke.cc](https://etke.cc)
