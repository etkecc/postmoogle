package bot

import (
	"context"
	"regexp"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

func parseMXIDpatterns(patterns []string, defaultPattern string) ([]*regexp.Regexp, error) {
	if len(patterns) == 0 && defaultPattern != "" {
		patterns = []string{defaultPattern}
	}

	return utils.WildcardMXIDsToRegexes(patterns)
}

func (b *Bot) allowUsers(actorID id.UserID) bool {
	if len(b.allowedUsers) != 0 {
		if !utils.Match(actorID.String(), b.allowedUsers) {
			return false
		}
	}

	return true
}

func (b *Bot) allowAnyone(actorID id.UserID, targetRoomID id.RoomID) bool {
	return true
}

func (b *Bot) allowOwner(actorID id.UserID, targetRoomID id.RoomID) bool {
	if !b.allowUsers(actorID) {
		return false
	}
	cfg, err := b.getRoomSettings(targetRoomID)
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

func (b *Bot) allowSend(actorID id.UserID, targetRoomID id.RoomID) bool {
	if !b.allowUsers(actorID) {
		return false
	}

	cfg, err := b.getRoomSettings(targetRoomID)
	if err != nil {
		b.Error(context.Background(), targetRoomID, "failed to retrieve settings: %v", err)
		return false
	}

	return !cfg.NoSend()
}
