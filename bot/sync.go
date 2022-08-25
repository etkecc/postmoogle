package bot

import (
	"context"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

func (b *Bot) initSync() {
	b.lp.OnEventType(
		event.StateMember,
		func(_ mautrix.EventSource, evt *event.Event) {
			go b.onMembership(evt)
		},
	)
	b.lp.OnEventType(
		event.EventMessage,
		func(_ mautrix.EventSource, evt *event.Event) {
			go b.onMessage(evt)
		})
	b.lp.OnEventType(
		event.EventEncrypted,
		func(_ mautrix.EventSource, evt *event.Event) {
			go b.onEncryptedMessage(evt)
		})
}

func (b *Bot) onMembership(evt *event.Event) {
	hub := sentry.CurrentHub().Clone()

	if evt.Content.AsMember().Membership == event.MembershipJoin && evt.Sender == b.lp.GetClient().UserID {
		b.onBotJoin(evt, hub)
		return
	}

	// Potentially handle other membership events in the future
}

func (b *Bot) onMessage(evt *event.Event) {
	// ignore own messages
	if evt.Sender == b.lp.GetClient().UserID {
		return
	}

	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{ID: evt.Sender.String()})
		scope.SetContext("event", map[string]string{
			"id":     evt.ID.String(),
			"room":   evt.RoomID.String(),
			"sender": evt.Sender.String(),
		})
	})
	ctx := sentry.SetHubOnContext(context.Background(), hub)
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("onMessage"))
	defer span.Finish()

	b.handle(span.Context(), evt)
}

func (b *Bot) onEncryptedMessage(evt *event.Event) {
	// ignore own messages
	if evt.Sender == b.lp.GetClient().UserID {
		return
	}

	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{ID: evt.Sender.String()})
		scope.SetContext("event", map[string]string{
			"id":     evt.ID.String(),
			"room":   evt.RoomID.String(),
			"sender": evt.Sender.String(),
		})
	})
	ctx := sentry.SetHubOnContext(context.Background(), hub)
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("onMessage"))
	defer span.Finish()

	decrypted, err := b.lp.GetMachine().DecryptMegolmEvent(evt)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot decrypt a message: %v", err)
		return
	}

	b.handle(span.Context(), decrypted)
}

// onBotJoin handles the "bot joined the room" event
func (b *Bot) onBotJoin(evt *event.Event, hub *sentry.Hub) {
	// Workaround for membership=join events which are delivered to us twice,
	// as described in this bug report: https://github.com/matrix-org/synapse/issues/9768
	_, ok := b.handledJoinEvents.LoadOrStore(evt.ID, true)
	if ok {
		b.log.Info("Suppressing already handled event %s", evt.ID)
		return
	}

	ctx := sentry.SetHubOnContext(context.Background(), hub)

	b.sendIntroduction(ctx, evt.RoomID)
	b.sendHelp(ctx, evt.RoomID)
}
