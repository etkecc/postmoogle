# Postmoogle [![Matrix](https://img.shields.io/matrix/postmoogle:etke.cc?logo=matrix&style=for-the-badge&server_fqdn=matrix.org)](https://matrix.to/#/#postmoogle:etke.cc)

> [more about that name](https://finalfantasy.fandom.com/wiki/The_Little_Postmoogle_That_Could)

An Email to Matrix bridge. 1 room = 1 mailbox.

Postmoogle is an actual SMTP server that allows you to send and receive emails on your matrix server.
It can't be used with arbitrary email providers, because it acts as an actual email provider itself,
so you can use it to send emails from your apps and scripts as well.

## Roadmap

### Receive

- [x] SMTP server (plaintext and SSL)
- [x] live reload of SSL certs
- [x] Matrix bot
- [x] Configuration in room's account data
- [x] Receive emails to matrix rooms
- [x] Receive attachments
- [x] Subaddressing support
- [x] Mailbox aliases support
- [x] Catch-all mailbox
- [x] Strip forwarding, signatures, and other noise from emails if configured
- [x] Map email threads to matrix threads
- [x] Multi-domain support
- [x] SMTP verification
- [x] DKIM verification
- [x] SPF verification
- [x] RBL verification
- [x] MX verification
- [x] Spamlist of emails (wildcards supported)
- [x] Spamlist of hosts (per server only)
- [x] Greylisting (per server only)

### Send

- [x] SMTP client
- [x] SMTP server (you can use Postmoogle as general purpose SMTP server to send emails from your scripts or apps)
- [x] SMTP Relaying (postmoogle can send emails via relay host), global and per-mailbox
- [x] Send a message to matrix room with special format to send a new email, even to multiple email addresses at once
- [x] Reply to matrix thread sends reply into email thread
- [x] Email signatures
- [x] Email autoreply / autoresponder for new email threads

## Configuration

### 1. Bot (mandatory)

env vars

* **POSTMOOGLE_HOMESERVER** - homeserver url, eg: `https://matrix.example.com`
* **POSTMOOGLE_LOGIN** - user login, localpart when logging in with password (e.g., `moogle`), OR full MXID when using shared secret (e.g., `@moogle:example.com`)
* **POSTMOOGLE_PASSWORD** - user password, alternatively you may use shared secret
* **POSTMOOGLE_SHAREDSECRET** - alternative to password, shared secret ([details](https://github.com/devture/matrix-synapse-shared-secret-auth))
* **POSTMOOGLE_DOMAINS** - space separated list of SMTP domains to listen for new emails. The first domain acts as the default domain, all other as aliases

<details>
<summary>other optional config parameters</summary>

* **POSTMOOGLE_PORT** - SMTP port to listen for new emails
* **POSTMOOGLE_PROXIES** - space separated list of IP addresses considered as trusted proxies, thus never banned
* **POSTMOOGLE_TLS_PORT** - secure SMTP port to listen for new emails. Requires valid cert and key as well
* **POSTMOOGLE_TLS_CERT** - space separated list of paths to the SSL certificates (chain) of your domains, note that position in the cert list must match the position of the cert's key in the key list
* **POSTMOOGLE_TLS_KEY** - space separated list of paths to the SSL certificates' private keys of your domains, note that position on the key list must match the position of cert in the cert list
* **POSTMOOGLE_TLS_REQUIRED** - require TLS connection, **even** on the non-TLS port (`POSTMOOGLE_PORT`). TLS connections are always required on the TLS port (`POSTMOOGLE_TLS_PORT`) regardless of this setting.
* **POSTMOOGLE_DATA_SECRET** - secure key (password) to encrypt account data, must be 16, 24, or 32 bytes long
* **POSTMOOGLE_STATUSMSG** - presence status message
* **POSTMOOGLE_MONITORING_SENTRY_DSN** - sentry DSN
* **POSTMOOGLE_MONITORING_SENTRY_RATE** - sentry sample rate, from 0 to 100 (default: 20)
* **POSTMOOGLE_MONITORING_HEALTHCHECKS_URL** - healthchecks.io url, default: `https://hc-ping.com`
* **POSTMOOGLE_MONITORING_HEALTHCHECKS_UUID** - healthchecks.io UUID
* **POSTMOOGLE_MONITORING_HEALTHCHECKS_DURATION** - heathchecks.io duration between pings in secods (default: 5)
* **POSTMOOGLE_LOGLEVEL** - log level
* **POSTMOOGLE_DB_DSN** - database connection string
* **POSTMOOGLE_DB_DIALECT** - database dialect (postgres, sqlite3)
* **POSTMOOGLE_MAILBOXES_RESERVED** - space separated list of reserved mailboxes, [docs/mailboxes.md](docs/mailboxes.md)
* **POSTMOOGLE_MAILBOXES_FORWARDED** - space separated list of forwarded from emails that should be ignored when sending replies
* **POSTMOOGLE_MAILBOXES_ACTIVATION** - activation flow for new mailboxes, [docs/mailboxes.md](docs/mailboxes.md)
* **POSTMOOGLE_MAXSIZE** - max email size (including attachments) in megabytes
* **POSTMOOGLE_ADMINS** - a space-separated list of admin users. See `POSTMOOGLE_USERS` for syntax examples
* **POSTMOOGLE_RELAY_HOST** - (global) SMTP hostname of relay host (e.g. Sendgrid)
* **POSTMOOGLE_RELAY_PORT** - (global) SMTP port of relay host
* **POSTMOOGLE_RELAY_USERNAME** - (global) Username of relay host
* **POSTMOOGLE_RELAY_PASSWORD** - (global) Password of relay host

You can find default values in [internal/config/defaults.go](internal/config/defaults.go)

</details>

### 2. DNS (highly recommended)

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

> The following section is visible to all allowed users

* **`!pm help`** - Show this help message
* **`!pm stop`** - Disable bridge for the room and clear all configuration
* **`!pm send`** - Send email

---

#### mailbox ownership

> The following section is visible to the mailbox owners only

* **`!pm mailbox`** - Get or set mailbox of the room
* **`!pm aliases`** - Get or set comma-separated aliases of the room
* **`!pm domain`** - Get or set default domain of the room
* **`!pm owner`** - Get or set owner of the room
* **`!pm password`** - Get or set SMTP password of the room's mailbox
* **`!pm relay`** - Get or set SMTP relay of that mailbox. Format: `smtp://user:password@host:port`, e.g. `smtp://54b7bfb9-b95f-44b8-9879-9b560baf4e3a:8528a3a9-bea8-4583-9912-d4357ba565eb@example.com:587`
---

#### mailbox options

> The following section is visible to the mailbox owners only

* **`!pm autoreply`** - Get or set autoreply of the room (markdown supported) that will be sent on any new incoming email thread
* **`!pm signature`** - Get or set signature of the room (markdown supported)
* **`!pm threadify`** - Get or set `threadify` of the room (`true` - send incoming email body in thread; `false` - send incoming email body as part of the message)
* **`!pm stripify`** - Get or set `threadify` of the room (`true` - strip incoming email's reply quotes and signatures; `false` - send incoming email as-is)
* **`!pm nosend`** - Get or set `nosend` of the room (`true` - disable email sending; `false` - enable email sending)
* **`!pm noreplies`** - Get or set `noreplies` of the room (`true` - ignore matrix replies; `false` - parse matrix replies)
* **`!pm nosender`** - Get or set `nosender` of the room (`true` - hide email sender; `false` - show email sender)
* **`!pm norecipient`** - Get or set `norecipient` of the room (`true` - hide recipient; `false` - show recipient)
* **`!pm nocc`** - Get or set `nocc` of the room (`true` - hide CC; `false` - show CC)
* **`!pm nosubject`** - Get or set `nosubject` of the room (`true` - hide email subject; `false` - show email subject)
* **`!pm nohtml`** - Get or set `nohtml` of the room (`true` - ignore HTML in email; `false` - parse HTML in emails)
* **`!pm nothreads`** - Get or set `nothreads` of the room (`true` - ignore email threads; `false` - convert email threads into matrix threads)
* **`!pm nofiles`** - Get or set `nofiles` of the room (`true` - ignore email attachments; `false` - upload email attachments)
* **`!pm noinlines`** - Get or set `noinlines` of the room (`true` - ignore inline attachments; `false` - upload inline attachments)

---

#### mailbox security checks

> The following section is visible to the mailbox owners only

* **`!pm spamcheck:mx`** - only accept email from servers which seem prepared to receive it (those having valid MX records) (`true` - enable, `false` - disable)
* **`!pm spamcheck:spf`** - only accept email from senders which authorized to send it (those matching SPF records) (`true` - enable, `false` - disable)
* **`!pm spamcheck:rbl`** - reject incoming emails from hosts listed in DNS blocklists (`true` - enable, `false` - disable)
* **`!pm spamcheck:dkim`** - only accept correctly authorized emails (without DKIM signature at all or with valid DKIM signature) (`true` - enable, `false` - disable)
* **`!pm spamcheck:smtp`** - only accept email from servers which seem prepared to receive it (those listening on an SMTP port) (`true` - enable, `false` - disable)

---

#### mailbox anti-spam

> The following section is visible to the mailbox owners only

* **`!pm spam:list`** - Show comma-separated spamlist of the room, eg: `spammer@example.com,*@spammer.org,spam@*`
* **`!pm spam:add`** - Mark an email address (or pattern) as spam (or you can react to the email with emoji: ⛔️,🛑, or 🚫)
* **`!pm spam:remove`** - Unmark an email address (or pattern) as spam
* **`!pm spam:reset`** - Reset spamlist

---

#### server options

> The following section is visible to the bridge admins only

* **`!pm adminroom`** - Get or set admin room
* **`!pm users`** - Get or set allowed users
* **`!pm dkim`** - Get DKIM signature
* **`!pm catch-all`** - Get or set catch-all mailbox
* **`!pm queue:batch`** - max amount of emails to process on each queue check
* **`!pm queue:retries`** - max amount of tries per email in queue before removal
* **`!pm mailboxes`** - Show the list of all mailboxes
* **`!pm delete`** - Delete specific mailbox

---

#### server antispam

> The following section is visible to the bridge admins only

* **`!pm greylist`** - Set automatic greylisting duration in minutes (0 - disabled)
* **`!pm banlist`** - Enable/disable banlist and show current values
* **`!pm banlist:auth`** - Enable/disable automatic banning for invalid auth credentials
* **`!pm banlist:auto`** - Enable/disable automatic banning for invalid emails
* **`!pm banlist:totals`** - List banlist totals only
* **`!pm banlist:add`** - Ban an IP
* **`!pm banlist:remove`** - Unban an IP
* **`!pm banlist:reset`** - Reset banlist

</details>
