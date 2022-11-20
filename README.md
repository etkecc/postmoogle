# Postmoogle [![Matrix](https://img.shields.io/matrix/postmoogle:etke.cc?logo=matrix&style=for-the-badge&server_fqdn=matrix.org)](https://matrix.to/#/#postmoogle:etke.cc)[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/etkecc) [![coverage report](https://gitlab.com/etke.cc/postmoogle/badges/main/coverage.svg)](https://gitlab.com/etke.cc/postmoogle/-/commits/main) [![Go Report Card](https://goreportcard.com/badge/gitlab.com/etke.cc/postmoogle)](https://goreportcard.com/report/gitlab.com/etke.cc/postmoogle) [![Go Reference](https://pkg.go.dev/badge/gitlab.com/etke.cc/postmoogle.svg)](https://pkg.go.dev/gitlab.com/etke.cc/postmoogle)

> [more about that name](https://finalfantasy.fandom.com/wiki/The_Little_Postmoogle_That_Could)

An Email to Matrix bridge. 1 room = 1 mailbox.

Postmoogle is an actual SMTP server that allows you to send and receive emails on your matrix server.
It can't be used with arbitrary email providers, because it acts as an actual email provider itself,
so you can use it to send emails from your apps and scripts as well.

## Roadmap

### Receive

- [x] SMTP server (plaintext and SSL)
- [x] Matrix bot
- [x] Configuration in room's account data
- [x] Receive emails to matrix rooms
- [x] Receive attachments
- [x] Catch-all mailbox
- [x] Map email threads to matrix threads
- [x] Multi-domain support
- [x] automatic banlist
- [x] automatic greylisting

#### deep dive

> features in that section considered as "nice to have", but not a priority

- [ ] DKIM verification
- [ ] SPF verification
- [ ] DMARC verification

### Send

- [x] SMTP client
- [x] SMTP server (you can use Postmoogle as general purpose SMTP server to send emails from your scripts or apps)
- [x] Send a message to matrix room with special format to send a new email, even to multiple email addresses at once
- [x] Reply to matrix thread sends reply into email thread

## Configuration

### 1. Bot (mandatory)

env vars

* **POSTMOOGLE_HOMESERVER** - homeserver url, eg: `https://matrix.example.com`
* **POSTMOOGLE_LOGIN** - user login/localpart, eg: `moogle`
* **POSTMOOGLE_PASSWORD** - user password
* **POSTMOOGLE_DOMAINS** - space separated list of SMTP domains to listen for new emails. The first domain acts as the default domain, all other as aliases

<details>
<summary>other optional config parameters</summary>

* **POSTMOOGLE_PORT** - SMTP port to listen for new emails
* **POSTMOOGLE_TLS_PORT** - secure SMTP port to listen for new emails. Requires valid cert and key as well
* **POSTMOOGLE_TLS_CERT** - space separated list of paths to the SSL certificates (chain) of your domains, note that position in the cert list must match the position of the cert's key in the key list
* **POSTMOOGLE_TLS_KEY** - space separated list of paths to the SSL certificates' private keys of your domains, note that position on the key list must match the position of cert in the cert list
* **POSTMOOGLE_TLS_REQUIRED** - require TLS connection, **even** on the non-TLS port (`POSTMOOGLE_PORT`). TLS connections are always required on the TLS port (`POSTMOOGLE_TLS_PORT`) regardless of this setting.
* **POSTMOOGLE_DATA_SECRET** - secure key (password) to encrypt account data, must be 16, 24, or 32 bytes long
* **POSTMOOGLE_NOENCRYPTION** - disable matrix encryption (libolm) support
* **POSTMOOGLE_STATUSMSG** - presence status message
* **POSTMOOGLE_SENTRY_DSN** - sentry DSN
* **POSTMOOGLE_LOGLEVEL** - log level
* **POSTMOOGLE_DB_DSN** - database connection string
* **POSTMOOGLE_DB_DIALECT** - database dialect (postgres, sqlite3)
* **POSTMOOGLE_MAILBOXES_RESERVED** - space separated list of reserved mailboxes (e.g.: `postmaster admin root`), nobody can create them
* **POSTMOOGLE_MAXSIZE** - max email size (including attachments) in megabytes
* **POSTMOOGLE_ADMINS** - a space-separated list of admin users. See `POSTMOOGLE_USERS` for syntax examples

You can find default values in [config/defaults.go](config/defaults.go)

</details>

### 2. DNS (optional)

Follow the [docs/dns](docs/dns.md)

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
* **!pm domain** - Get or set default domain of the room
* **!pm owner** - Get or set owner of the room
* **!pm password** - Get or set SMTP password of the room's mailbox

---

* **!pm nosender** - Get or set `nosender` of the room (`true` - hide email sender; `false` - show email sender)
* **!pm norecipient** - Get or set `norecipient` of the room (`true` - hide recipient; `false` - show recipient)
* **!pm nocc** - Get or set `nocc` of the room (`true` - hide CC; `false` - show CC)
* **!pm nosubject** - Get or set `nosubject` of the room (`true` - hide email subject; `false` - show email subject)
* **!pm nohtml** - Get or set `nohtml` of the room (`true` - ignore HTML in email; `false` - parse HTML in emails)
* **!pm nothreads** - Get or set `nothreads` of the room (`true` - ignore email threads; `false` - convert email threads into matrix threads)
* **!pm nofiles** - Get or set `nofiles` of the room (`true` - ignore email attachments; `false` - upload email attachments)

---

* **!pm spamcheck:mx** - only accept email from servers which seem prepared to receive it (those having valid MX records) (`true` - enable, `false` - disable)
* **!pm spamcheck:smtp** - only accept email from servers which seem prepared to receive it (those listening on an SMTP port) (`true` - enable, `false` - disable)
* **!pm spamlist** - Get or set `spamlist` of the room (comma-separated list), eg: `spammer@example.com,*@spammer.org,noreply@*`

---

* **!pm dkim** - Get DKIM signature
* **!pm catch-all** - Configure catch-all mailbox
* **!pm queue:batch** - max amount of emails to process on each queue check
* **!pm queue:retries** - max amount of tries per email in queue before removal
* **!pm users** - Get or set allowed users patterns
* **!pm mailboxes** - Show the list of all mailboxes
* **!pm delete** &lt;mailbox&gt; - Delete specific mailbox

---

* **!pm greylist** - Set automatic greylisting duration in minutes (0 - disabled)
* **!pm banlist** - Enable/disable banlist and show current values
* **!pm banlist:add** - Ban an IP
* **!pm banlist:remove** - Unban an IP
* **!pm banlist:reset** - Reset banlist

</details>


## Where to get

[docker registry](https://gitlab.com/etke.cc/postmoogle/container_registry), [etke.cc](https://etke.cc)
