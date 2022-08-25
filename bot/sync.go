package bot

import (
	"context"

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
	ctx := newContext(evt)

	if evt.Content.AsMember().Membership == event.MembershipJoin && evt.Sender == b.lp.GetClient().UserID {
		b.onBotJoin(ctx)
		return
	}

	// Potentially handle other membership events in the future
}

func (b *Bot) onMessage(evt *event.Event) {
	// ignore own messages
	if evt.Sender == b.lp.GetClient().UserID {
		return
	}
	ctx := newContext(evt)
	b.handle(ctx)
}

func (b *Bot) onEncryptedMessage(evt *event.Event) {
	// ignore own messages
	if evt.Sender == b.lp.GetClient().UserID {
		return
	}
	ctx := newContext(evt)

	decrypted, err := b.lp.GetMachine().DecryptMegolmEvent(evt)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot decrypt a message: %v", err)
		return
	}
	ctx = eventToContext(ctx, decrypted)

	b.handle(ctx)
}

// onBotJoin handles the "bot joined the room" event
func (b *Bot) onBotJoin(ctx context.Context) {
	evt := eventFromContext(ctx)
	// Workaround for membership=join events which are delivered to us twice,
	// as described in this bug report: https://github.com/matrix-org/synapse/issues/9768
	_, ok := b.handledJoinEvents.LoadOrStore(evt.ID, true)
	if ok {
		b.log.Info("Suppressing already handled event %s", evt.ID)
		return
	}

	b.sendIntroduction(ctx, evt.RoomID)
	b.sendHelp(ctx, evt.RoomID)
}
