package linkpearl

import (
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// OnEventType allows callers to be notified when there are new events for the given event type.
// There are no duplicate checks.
func (l *Linkpearl) OnEventType(eventType event.Type, callback mautrix.EventHandler) {
	l.api.Syncer.(*mautrix.DefaultSyncer).OnEventType(eventType, callback)
}

// OnSync shortcut to mautrix.DefaultSyncer.OnSync
func (l *Linkpearl) OnSync(callback mautrix.SyncHandler) {
	l.api.Syncer.(*mautrix.DefaultSyncer).OnSync(callback)
}

// OnEvent shortcut to mautrix.DefaultSyncer.OnEvent
func (l *Linkpearl) OnEvent(callback mautrix.EventHandler) {
	l.api.Syncer.(*mautrix.DefaultSyncer).OnEvent(callback)
}

func (l *Linkpearl) initSync() {
	if l.olm != nil {
		l.api.Syncer.(*mautrix.DefaultSyncer).OnSync(l.olm.ProcessSyncResponse)
		l.api.Syncer.(*mautrix.DefaultSyncer).OnEventType(
			event.StateEncryption,
			func(source mautrix.EventSource, evt *event.Event) {
				go l.onEncryption(source, evt)
			},
		)
	}

	l.api.Syncer.(*mautrix.DefaultSyncer).OnEventType(
		event.StateMember,
		func(source mautrix.EventSource, evt *event.Event) {
			go l.onMembership(source, evt)
		},
	)
}

func (l *Linkpearl) onMembership(_ mautrix.EventSource, evt *event.Event) {
	if l.olm != nil {
		l.olm.HandleMemberEvent(evt)
	}
	l.store.SetMembership(evt)

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

func (l *Linkpearl) tryJoin(roomID id.RoomID, retry int) {
	if retry >= l.maxretries {
		return
	}

	_, err := l.api.JoinRoomByID(roomID)
	if err != nil {
		l.log.Error("cannot join the room %q: %v", roomID, err)
		time.Sleep(5 * time.Second)
		l.log.Debug("trying to join again (%d/%d)", retry+1, l.maxretries)
		l.tryJoin(roomID, retry+1)
	}
}

func (l *Linkpearl) tryLeave(roomID id.RoomID, retry int) {
	if retry >= l.maxretries {
		return
	}

	_, err := l.api.LeaveRoom(roomID)
	if err != nil {
		l.log.Error("cannot leave room: %v", err)
		time.Sleep(5 * time.Second)
		l.log.Debug("trying to leave again (%d/%d)", retry+1, l.maxretries)
		l.tryLeave(roomID, retry+1)
	}
}

func (l *Linkpearl) onEmpty(evt *event.Event) {
	if !l.autoleave {
		return
	}

	members := l.store.GetRoomMembers(evt.RoomID)
	if len(members) >= 1 && members[0] != l.api.UserID {
		return
	}

	l.tryLeave(evt.RoomID, 0)
}

func (l *Linkpearl) onEncryption(_ mautrix.EventSource, evt *event.Event) {
	l.store.SetEncryptionEvent(evt)
}
