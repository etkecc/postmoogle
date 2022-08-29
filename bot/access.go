package bot

import (
	"fmt"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

type accessCheckerFunc func(id.UserID, id.RoomID) (bool, error)

func (b *Bot) allowAnyone(actorID id.UserID, targetRoomID id.RoomID) (bool, error) {
	return true, nil
}

func (b *Bot) allowOwner(actorID id.UserID, targetRoomID id.RoomID) (bool, error) {
	if !utils.Match(actorID.String(), b.allowedUsers) {
		return false, nil
	}

	if b.noowner {
		return true, nil
	}

	cfg, err := b.getSettings(targetRoomID)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve settings: %v", err)
	}

	owner := cfg.Owner()
	if owner == "" {
		return true, nil
	}

	return owner == actorID.String(), nil
}

func (b *Bot) allowAdmin(actorID id.UserID, targetRoomID id.RoomID) (bool, error) {
	return utils.Match(actorID.String(), b.allowedAdmins), nil
}
