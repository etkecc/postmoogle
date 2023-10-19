package bot

import (
	"maunium.net/go/mautrix/id"
)

func (b *Bot) lock(roomID id.RoomID, optionalEventID ...id.EventID) {
	b.mu.Lock(roomID.String())

	if len(optionalEventID) == 0 {
		return
	}
	evtID := optionalEventID[0]
	if _, err := b.lp.GetClient().SendReaction(roomID, evtID, "📨"); err != nil {
		b.log.Error().Err(err).Str("roomID", roomID.String()).Str("eventID", evtID.String()).Msg("cannot send reaction on lock")
	}
}

func (b *Bot) unlock(roomID id.RoomID, optionalEventID ...id.EventID) {
	b.mu.Unlock(roomID.String())

	if len(optionalEventID) == 0 {
		return
	}
	evtID := optionalEventID[0]
	if _, err := b.lp.GetClient().SendReaction(roomID, evtID, "✅"); err != nil {
		b.log.Error().Err(err).Str("roomID", roomID.String()).Str("eventID", evtID.String()).Msg("cannot send reaction on unlock")
	}
}
