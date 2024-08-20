package bot

import (
	"context"

	"maunium.net/go/mautrix/event"
)

func (b *Bot) initSync() {
	b.lp.SetJoinPermit(b.joinPermit)

	b.lp.OnEventType(
		event.StateMember,
		func(ctx context.Context, evt *event.Event) {
			go b.onMembership(ctx, evt)
		},
	)
	b.lp.OnEventType(
		event.EventMessage,
		func(ctx context.Context, evt *event.Event) {
			go b.onMessage(ctx, evt)
		},
	)
	b.lp.OnEventType(
		event.EventReaction,
		func(ctx context.Context, evt *event.Event) {
			go b.onReaction(ctx, evt)
		},
	)
}

// joinPermit is called by linkpearl when processing "invite" events and deciding if rooms should be auto-joined or not
func (b *Bot) joinPermit(ctx context.Context, evt *event.Event) bool {
	if !b.allowUsers(ctx, evt.Sender, evt.RoomID) {
		b.log.Debug().Str("userID", evt.Sender.String()).Msg("Rejecting room invitation from unallowed user")
		return false
	}

	return true
}

func (b *Bot) onMembership(ctx context.Context, evt *event.Event) {
	// mautrix 0.15.x migration
	if b.ignoreBefore >= evt.Timestamp {
		return
	}

	ctx = newContext(ctx, evt)

	evtType := evt.Content.AsMember().Membership
	if evtType == event.MembershipJoin && evt.Sender == b.lp.GetClient().UserID {
		b.onBotJoin(ctx)
		return
	}

	if evtType == event.MembershipBan || evtType == event.MembershipLeave && evt.Sender != b.lp.GetClient().UserID {
		b.onLeave(ctx)
	}

	// Potentially handle other membership events in the future
}

func (b *Bot) onMessage(ctx context.Context, evt *event.Event) {
	// ignore own messages
	if evt.Sender == b.lp.GetClient().UserID {
		return
	}
	// mautrix 0.15.x migration
	if b.ignoreBefore >= evt.Timestamp {
		return
	}

	ctx = newContext(ctx, evt)
	b.handle(ctx)
}

func (b *Bot) onReaction(ctx context.Context, evt *event.Event) {
	// ignore own messages
	if evt.Sender == b.lp.GetClient().UserID {
		return
	}
	// mautrix 0.15.x migration
	if b.ignoreBefore >= evt.Timestamp {
		return
	}

	ctx = newContext(ctx, evt)
	b.handleReaction(ctx)
}

// onBotJoin handles the "bot joined the room" event
func (b *Bot) onBotJoin(ctx context.Context) {
	evt := eventFromContext(ctx)
	// Workaround for membership=join events which are delivered to us twice,
	// as described in this bug report: https://github.com/matrix-org/synapse/issues/9768
	_, ok := b.handledMembershipEvents.LoadOrStore(evt.ID, true)
	if ok {
		b.log.Info().Str("eventID", evt.ID.String()).Msg("Suppressing already handled event")
		return
	}

	b.sendIntroduction(ctx, evt.RoomID)
	b.sendHelp(ctx)
}

func (b *Bot) onLeave(ctx context.Context) {
	evt := eventFromContext(ctx)
	_, ok := b.handledMembershipEvents.LoadOrStore(evt.ID, true)
	if ok {
		b.log.Info().Str("eventID", evt.ID.String()).Msg("Suppressing already handled event")
		return
	}
	members, err := b.lp.GetClient().StateStore.GetRoomJoinedOrInvitedMembers(ctx, evt.RoomID)
	if err != nil {
		b.log.Error().Err(err).Str("roomID", evt.RoomID.String()).Msg("cannot get joined or invited members")
		return
	}

	count := len(members)
	if count == 1 && members[0] == b.lp.GetClient().UserID {
		b.log.Info().Str("roomID", evt.RoomID.String()).Msg("no more users left in the room")
		b.runStop(ctx)
		_, err := b.lp.GetClient().LeaveRoom(ctx, evt.RoomID)
		if err != nil {
			b.Error(ctx, "cannot leave empty room: %v", err)
		}
	}
}
