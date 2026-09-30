# Mailboxes configuration

## `POSTMOOGLE_MAILBOXES_RESERVED`

Space separated list of reserved mailboxes, example:

```bash
export POSTMOOGLE_MAILBOXES_RESERVED=admin root postmaster
```

Nobody can create a mailbox from that list

## `POSTMOOGLE_MAILBOXES_ACTIVATION`

Type of activation flow:

### `none` (default)

If `POSTMOOGLE_MAILBOXES_ACTIVATION=none` mailbox will be just created as is, without any additional checks.

### `notify`

If `POSTMOOGLE_MAILBOXES_ACTIVATION=notify`, mailbox will be created as in `none` case **and** notification will be sent to one of the mailboxes managed by a postmoogle admin.

To make it work, a postmoogle admin (or multiple admins) should either set `!pm adminroom` or create at least one mailbox.

## `POSTMOOGLE_MAILBOXES_SETUP`

A JSON list of mailboxes the bot sets up by itself, so nobody has to create rooms and send `!pm` commands:

```bash
export POSTMOOGLE_MAILBOXES_SETUP='[
  {"address": "inbox@example.com", "owner": "@you:example.com", "relay": "smtp://inbox%40example.com:password@smtp.example.com:587"},
  {"address": "news@example.com", "owner": "@you:example.com", "name": "Newsletters"}
]'
```

* `address` - the mailbox, its domain must be one of `POSTMOOGLE_DOMAINS`
* `owner` - the Matrix user who owns the mailbox; they are invited to its room and made its admin
* `relay` - optional, the SMTP relay for sending from this mailbox, like `!pm relay`
* `name` - optional, the room name, the address by default

On every start, the bot creates a room for each mailbox that has none, invites the owner, and sets the mailbox, domain, owner, and relay.
Rooms that already exist are kept, only their domain and relay are set again, so a changed password reaches them.
If the owner leaves the room, the mailbox is removed as usual, and a new room is created on the next start; take the mailbox out of the list to stop that.
