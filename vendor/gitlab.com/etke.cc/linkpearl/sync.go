package linkpearl

import (
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// OnEventType allows callers to be notified when there are new events for the given event type.
// There are no duplicate checks.
func (l *Linkpearl) OnEventType(eventType event.Type, callback mautrix.EventHandler) {
	l.api.Syncer.(mautrix.ExtensibleSyncer).OnEventType(eventType, callback)
}

// OnSync shortcut to mautrix.DefaultSyncer.OnSync
func (l *Linkpearl) OnSync(callback mautrix.SyncHandler) {
	l.api.Syncer.(mautrix.ExtensibleSyncer).OnSync(callback)
}

// OnEvent shortcut to mautrix.DefaultSyncer.OnEvent
func (l *Linkpearl) OnEvent(callback mautrix.EventHandler) {
	l.api.Syncer.(mautrix.ExtensibleSyncer).OnEvent(callback)
}

func (l *Linkpearl) initSync() {
	l.api.Syncer.(mautrix.ExtensibleSyncer).OnEventType(
		event.StateEncryption,
		func(source mautrix.EventSource, evt *event.Event) {
			go l.onEncryption(source, evt)
		},
	)
	l.api.Syncer.(mautrix.ExtensibleSyncer).OnEventType(
		event.StateMember,
		func(source mautrix.EventSource, evt *event.Event) {
			go l.onMembership(source, evt)
		},
	)
}

func (l *Linkpearl) onMembership(src mautrix.EventSource, evt *event.Event) {
	l.ch.Machine().HandleMemberEvent(src, evt)
	l.api.StateStore.SetMembership(evt.RoomID, id.UserID(evt.GetStateKey()), evt.Content.AsMember().Membership)

	// potentially autoaccept invites
	l.onInvite(evt)

	// autoleave empty rooms
	l.onEmpty(evt)
}

func (l *Linkpearl) onInvite(evt *event.Event) {
	userID := l.api.UserID.String()
	invite := evt.Content.AsMember().Membership == event.MembershipInvite
	if !invite || evt.GetStateKey() != userID {
		return
	}

	if l.joinPermit(evt) {
		l.tryJoin(evt.RoomID, 0)
		return
	}

	l.tryLeave(evt.RoomID, 0)
}

// TODO: https://spec.matrix.org/v1.8/client-server-api/#post_matrixclientv3joinroomidoralias
// endpoint supports server_name param and tells "The servers to attempt to join the room through. One of the servers must be participating in the room.",
// meaning you can specify more than 1 server. It is not clear, what format should be used "example.com,example.org", or "example.com example.org", or whatever else.
// Moreover, it is not clear if the following values can be used together with that field: l.api.UserID.Homeserver() and evt.Sender.Homeserver()
func (l *Linkpearl) tryJoin(roomID id.RoomID, retry int) {
	if retry >= l.maxretries {
		return
	}

	_, err := l.api.JoinRoom(roomID.String(), "", nil)
	err = UnwrapError(err)
	if err != nil {
		l.log.Error().Err(err).Str("roomID", roomID.String()).Msg("cannot join room")
		if strings.HasPrefix(err.Error(), "403") || strings.HasPrefix(err.Error(), "M_FORBIDDEN") { // no permission to join, no need to retry
			return
		}
		time.Sleep(5 * time.Second)
		l.log.Error().Err(err).Str("roomID", roomID.String()).Int("retry", retry+1).Msg("trying to join again")
		l.tryJoin(roomID, retry+1)
	}
}

func (l *Linkpearl) tryLeave(roomID id.RoomID, retry int) {
	if retry >= l.maxretries {
		return
	}

	_, err := l.api.LeaveRoom(roomID)
	err = UnwrapError(err)
	if err != nil {
		l.log.Error().Err(err).Str("roomID", roomID.String()).Msg("cannot leave room")
		time.Sleep(5 * time.Second)
		l.log.Error().Err(err).Str("roomID", roomID.String()).Int("retry", retry+1).Msg("trying to leave again")
		l.tryLeave(roomID, retry+1)
	}
}

func (l *Linkpearl) onEmpty(evt *event.Event) {
	if !l.autoleave {
		return
	}

	members, err := l.api.StateStore.GetRoomJoinedOrInvitedMembers(evt.RoomID)
	err = UnwrapError(err)
	if err != nil {
		l.log.Error().Err(err).Str("roomID", evt.RoomID.String()).Msg("cannot get joined or invited members")
		return
	}

	if len(members) >= 1 && members[0] != l.api.UserID {
		return
	}

	l.tryLeave(evt.RoomID, 0)
}

func (l *Linkpearl) onEncryption(_ mautrix.EventSource, evt *event.Event) {
	l.api.StateStore.SetEncryptionEvent(evt.RoomID, evt.Content.AsEncryption())
}
