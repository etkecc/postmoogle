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
