package bot

import (
	"context"

	"maunium.net/go/mautrix/id"
)

const (
	reactionLock   = "📨"
	reactionUnlock = "✅"
)

func (b *Bot) lock(ctx context.Context, roomID id.RoomID, eventID id.EventID) {
	b.mu.Lock(roomID.String())

	if err := b.lp.SendReaction(ctx, roomID, eventID, reactionLock); err != nil {
		b.log.Error().Err(err).Str("roomID", roomID.String()).Str("eventID", eventID.String()).Msg("cannot send reaction on lock")
	}
}

func (b *Bot) unlock(ctx context.Context, roomID id.RoomID, eventID id.EventID) {
	b.mu.Unlock(roomID.String())
	if err := b.lp.ReplaceReaction(ctx, roomID, eventID, reactionLock, reactionUnlock); err != nil {
		b.log.Error().Err(err).Str("roomID", roomID.String()).Str("eventID", eventID.String()).Msg("cannot send reaction on unlock")
	}
}
