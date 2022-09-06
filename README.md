# Postmoogle [![Matrix](https://img.shields.io/matrix/postmoogle:etke.cc?logo=matrix&style=for-the-badge&server_fqdn=matrix.org)](https://matrix.to/#/#postmoogle:etke.cc)[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/etkecc) [![coverage report](https://gitlab.com/etke.cc/postmoogle/badges/main/coverage.svg)](https://gitlab.com/etke.cc/postmoogle/-/commits/main) [![Go Report Card](https://goreportcard.com/badge/gitlab.com/etke.cc/postmoogle)](https://goreportcard.com/report/gitlab.com/etke.cc/postmoogle) [![Go Reference](https://pkg.go.dev/badge/gitlab.com/etke.cc/postmoogle.svg)](https://pkg.go.dev/gitlab.com/etke.cc/postmoogle)

> [more about that name](https://finalfantasy.fandom.com/wiki/The_Little_Postmoogle_That_Could)

An Email to Matrix bridge. 1 room = 1 mailbox.

Postmoogle is an actual SMTP server that allows you to receive emails on your matrix server.
It can't be used with arbitrary email providers, but setup your own provider "with matrix interface" instead.

## Roadmap

### Receive

- [x] SMTP server
- [x] Matrix bot
- [x] Configuration in room's account data
- [x] Receive emails to matrix rooms
- [x] Receive attachments
- [x] Map email threads to matrix threads

#### deep dive

> features in that section considered as "nice to have", but not a priority

- [ ] DKIM verification
- [ ] SPF verification
- [ ] DMARC verification
- [ ] Blocklists 

### Send

- [x] SMTP client
- [x] Send a message to matrix room with special format to send a new email
- [ ] Reply to matrix thread sends reply into email thread

## Configuration

### 1. Bot (mandatory)

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
* **POSTMOOGLE_ADMINS** - a space-separated list of admin users. See `POSTMOOGLE_USERS` for syntax examples

You can find default values in [config/defaults.go](config/defaults.go)

</details>

### 2. DNS (optional)

The following configuration is needed only if you want to send outgoing emails via Postmoogle (it's not necessary if you only want to receive emails).

**First**, add a new DMARC DNS record of the `TXT` type for subdomain `_dmarc` with a proper policy. The simplest policy you can use is: `v=DMARC1; p=quarantine;`.

<details>
<summary>Example</summary>

```bash
$ dig txt _dmarc.example.com

; <<>> DiG 9.18.6 <<>> txt _dmarc.example.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 57306
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 1232
;; QUESTION SECTION:
;_dmarc.example.com.			IN	TXT

;; ANSWER SECTION:
_dmarc.example.com.		1799	IN	TXT	"v=DMARC1; p=quarantine;"

;; Query time: 46 msec
;; SERVER: 1.1.1.1#53(1.1.1.1) (UDP)
;; WHEN: Sun Sep 04 21:31:30 EEST 2022
;; MSG SIZE  rcvd: 79
```

</details>

**Second**, add a new SPF DNS record of the `TXT` type for your domain that will be used with Postmoogle, with format: `v=spf1 ip4:SERVER_IP -all` (replace `SERVER_IP` with your server's IP address)

<details>
<summary>Example</summary>

```bash
$ dig txt example.com

; <<>> DiG 9.18.6 <<>> txt example.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 24796
;; flags: qr rd ra; QUERY: 1, ANSWER: 4, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 1232
;; QUESTION SECTION:
;example.com.			IN	TXT

;; ANSWER SECTION:
example.com.		1799	IN	TXT	"v=spf1 ip4:111.111.111.111 -all"

;; Query time: 36 msec
;; SERVER: 1.1.1.1#53(1.1.1.1) (UDP)
;; WHEN: Sun Sep 04 21:35:04 EEST 2022
;; MSG SIZE  rcvd: 255
```

</details>

**Third**, add a new MX DNS record of the `MX` type for your domain that will be used with postmoogle. It should point to the same (sub-)domain.
Looks odd, but some mail servers will refuse to interact with your mail server (and Postmoogle is already a mail server) without MX records.

<details>
<summary>Example</summary>

```bash
dig MX example.com

; <<>> DiG 9.18.6 <<>> MX example.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12688
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 1232
;; QUESTION SECTION:
;example.com.			IN	MX

;; ANSWER SECTION:
example.com.		1799	IN	MX	10 example.com.

;; Query time: 40 msec
;; SERVER: 1.1.1.1#53(1.1.1.1) (UDP)
;; WHEN: Tue Sep 06 16:44:47 EEST 2022
;; MSG SIZE  rcvd: 59
```

</details>

**Fourth** (and the last one), add new DKIM DNS record of `TXT` type for subdomain `postmoogle._domainkey` that will be used with postmoogle.

You can get that signature using the `!pm dkim` command:

<details>
<summary>!pm dkim</summary>

DKIM signature is: `v=DKIM1; k=ed25519; p=OcVzOwAONDfgbJX/5vwzlXOs9gUDO0YKlXHaDnBJtXw=`.
You need to add it to your DNS records (if not already):
Add new DNS record with type = `TXT`, key (subdomain/from): `postmoogle._domainkey` and value (to):

```
v=DKIM1; k=ed25519; p=OcVzOwAONDfgbJX/5vwzlXOs9gUDO0YKlXHaDnBJtXw=
```

Without that record other email servers may reject your emails as spam, kupo.

</details>

<details>
<summary>Example</summary>

```bash
$ dig TXT postmoogle._domainkey.example.com

; <<>> DiG 9.18.6 <<>> TXT postmoogle._domainkey.example.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 59014
;; flags: qr rd ra; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 1232
;; QUESTION SECTION:
;postmoogle._domainkey.example.com.	IN	TXT

;; ANSWER SECTION:
postmoogle._domainkey.example.com. 600	IN TXT  "v=DKIM1; k=ed25519; p=OcVzOwAONDfgbJX/5vwzlXOs9gUDO0YKlXHaDnBJtXw="

;; Query time: 90 msec
;; SERVER: 1.1.1.1#53(1.1.1.1) (UDP)
;; WHEN: Mon Sep 05 16:16:21 EEST 2022
;; MSG SIZE  rcvd: 525
```

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

* **!pm dkim** - Get DKIM signature
* **!pm users** - Get or set allowed users patterns
* **!pm mailboxes** - Show the list of all mailboxes
* **!pm delete** &lt;mailbox&gt; - Delete specific mailbox

</details>


## Where to get

[docker registry](https://gitlab.com/etke.cc/postmoogle/container_registry), [etke.cc](https://etke.cc)
