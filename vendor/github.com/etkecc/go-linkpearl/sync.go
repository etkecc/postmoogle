package linkpearl

import (
	"context"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// OnEventType allows callers to be notified when there are new events for the given event type.
// There are no duplicate checks.
func (l *Linkpearl) OnEventType(eventType event.Type, callback mautrix.EventHandler) {
	l.api.Syncer.(mautrix.ExtensibleSyncer).OnEventType(eventType, callback) //nolint:forcetypeassert,errcheck // we know it's an ExtensibleSyncer
}

// OnSync shortcut to mautrix.DefaultSyncer.OnSync
func (l *Linkpearl) OnSync(callback mautrix.SyncHandler) {
	l.api.Syncer.(mautrix.ExtensibleSyncer).OnSync(callback) //nolint:forcetypeassert,errcheck // we know it's an ExtensibleSyncer
}

// OnEvent shortcut to mautrix.DefaultSyncer.OnEvent
func (l *Linkpearl) OnEvent(callback mautrix.EventHandler) {
	l.api.Syncer.(mautrix.ExtensibleSyncer).OnEvent(callback) //nolint:forcetypeassert,errcheck // we know it's an ExtensibleSyncer
}

func (l *Linkpearl) initSync() {
	l.api.Syncer.(mautrix.ExtensibleSyncer).OnEventType( //nolint:forcetypeassert,errcheck // we know it's an ExtensibleSyncer
		event.StateEncryption,
		func(ctx context.Context, evt *event.Event) {
			go l.onEncryption(ctx, evt)
		},
	)
	l.api.Syncer.(mautrix.ExtensibleSyncer).OnEventType( //nolint:forcetypeassert,errcheck // we know it's an ExtensibleSyncer
		event.StateMember,
		func(ctx context.Context, evt *event.Event) {
			go l.onMembership(ctx, evt)
		},
	)
}

func (l *Linkpearl) onMembership(ctx context.Context, evt *event.Event) {
	l.ch.Machine().HandleMemberEvent(ctx, evt)
	if err := l.api.StateStore.SetMembership(ctx, evt.RoomID, id.UserID(evt.GetStateKey()), evt.Content.AsMember().Membership); err != nil {
		l.log.Error().Err(err).Str("roomID", evt.RoomID.String()).Str("userID", evt.GetStateKey()).Msg("cannot set membership")
	}

	// potentially autoaccept invites
	l.onInvite(ctx, evt)

	// autoleave empty rooms
	l.onEmpty(ctx, evt)
}

func (l *Linkpearl) onInvite(ctx context.Context, evt *event.Event) {
	userID := l.api.UserID.String()
	invite := evt.Content.AsMember().Membership == event.MembershipInvite
	if !invite || evt.GetStateKey() != userID {
		return
	}

	if l.joinPermit(ctx, evt) {
		l.tryJoin(ctx, evt.RoomID, 0)
		return
	}

	l.tryLeave(ctx, evt.RoomID, 0)
}

func (l *Linkpearl) tryJoin(ctx context.Context, roomID id.RoomID, retry int) {
	if retry >= l.maxretries {
		return
	}

	_, err := l.api.JoinRoom(ctx, roomID.String(), "", nil)
	err = UnwrapError(err)
	if err != nil {
		l.log.Error().Err(err).Str("roomID", roomID.String()).Msg("cannot join room")
		if strings.HasPrefix(err.Error(), "403") || strings.HasPrefix(err.Error(), "M_FORBIDDEN") { // no permission to join, no need to retry
			return
		}
		time.Sleep(5 * time.Second)
		l.log.Error().Err(err).Str("roomID", roomID.String()).Int("retry", retry+1).Msg("trying to join again")
		l.tryJoin(ctx, roomID, retry+1)
	}
}

func (l *Linkpearl) tryLeave(ctx context.Context, roomID id.RoomID, retry int) {
	if retry >= l.maxretries {
		return
	}

	_, err := l.api.LeaveRoom(ctx, roomID)
	err = UnwrapError(err)
	if err != nil {
		l.log.Error().Err(err).Str("roomID", roomID.String()).Msg("cannot leave room")
		time.Sleep(5 * time.Second)
		l.log.Error().Err(err).Str("roomID", roomID.String()).Int("retry", retry+1).Msg("trying to leave again")
		l.tryLeave(ctx, roomID, retry+1)
	}
}

func (l *Linkpearl) onEmpty(ctx context.Context, evt *event.Event) {
	if !l.autoleave {
		return
	}

	members, err := l.api.StateStore.GetRoomJoinedOrInvitedMembers(ctx, evt.RoomID)
	err = UnwrapError(err)
	if err != nil {
		l.log.Error().Err(err).Str("roomID", evt.RoomID.String()).Msg("cannot get joined or invited members")
		return
	}

	if len(members) >= 1 && members[0] != l.api.UserID {
		return
	}

	l.tryLeave(ctx, evt.RoomID, 0)
}

func (l *Linkpearl) onEncryption(ctx context.Context, evt *event.Event) {
	if err := l.api.StateStore.SetEncryptionEvent(ctx, evt.RoomID, evt.Content.AsEncryption()); err != nil {
		l.log.Error().Err(err).Str("roomID", evt.RoomID.String()).Msg("cannot set encryption event")
	}
}
