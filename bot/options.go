package bot

import (
	"context"

	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/postmoogle/utils"
	"maunium.net/go/mautrix/event"
)

type sanitizerFunc func(string) string

// sanitizers is map of option name => sanitizer function
var sanitizers = map[string]sanitizerFunc{
	"mailbox":  utils.Mailbox,
	"nosender": utils.SanitizeBoolString,
}

func (b *Bot) handleOption(ctx context.Context, evt *event.Event, command []string) {
	if len(command) == 1 {
		b.getOption(ctx, evt, command[0])
		return
	}
	b.setOption(ctx, evt, command[0], command[1])
}

func (b *Bot) getOption(ctx context.Context, evt *event.Event, name string) {
	msg := "`%s` of this room is %s"
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("getOption"))
	defer span.Finish()

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	value := cfg.Get(name)
	if value == "" {
		b.Notice(span.Context(), evt.RoomID, "`%s` is not set", name)
		return
	}

	if name == "mailbox" {
		msg = msg + "@" + b.domain
	}

	b.Notice(span.Context(), evt.RoomID, msg, name, value)
}

func (b *Bot) setOption(ctx context.Context, evt *event.Event, name, value string) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("setOption"))
	defer span.Finish()
	msg := "`%s` of this room set to %s"

	sanitizer, ok := sanitizers[name]
	if ok {
		value = sanitizer(value)
	}

	if name == "mailbox" {
		existingID, ok := b.GetMapping(ctx, value)
		if ok && existingID != "" && existingID != evt.RoomID {
			b.Notice(span.Context(), evt.RoomID, "Mailbox %s@%s already taken", value, b.domain)
			return
		}
	}

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	if !cfg.Allowed(b.noowner, evt.Sender) {
		b.Notice(span.Context(), evt.RoomID, "you don't have permission to do that")
		return
	}

	cfg.Set(name, value)
	if name == "mailbox" {
		msg = msg + "@" + b.domain
		cfg.Set("owner", evt.Sender.String())
		b.roomsmu.Lock()
		b.rooms[value] = evt.RoomID
		b.roomsmu.Unlock()
	}

	err = b.setSettings(span.Context(), evt.RoomID, cfg)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot update settings: %v", err)
		return
	}

	b.Notice(span.Context(), evt.RoomID, msg, name, value)
}
