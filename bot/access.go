package bot

import (
	"context"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

func (b *Bot) allowAnyone(actorID id.UserID, targetRoomID id.RoomID) bool {
	return true
}

func (b *Bot) allowOwner(actorID id.UserID, targetRoomID id.RoomID) bool {
	if !utils.Match(actorID.String(), b.allowedUsers) {
		return false
	}

	if b.noowner {
		return true
	}

	cfg, err := b.getSettings(targetRoomID)
	if err != nil {
		b.Error(context.Background(), targetRoomID, "failed to retrieve settings: %v", err)
		return false
	}

	owner := cfg.Owner()
	if owner == "" {
		return true
	}

	return owner == actorID.String()
}

func (b *Bot) allowAdmin(actorID id.UserID, targetRoomID id.RoomID) bool {
	return utils.Match(actorID.String(), b.allowedAdmins)
}
