package bot

import (
	"context"
	"regexp"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/raja/argon2pw"
	"gitlab.com/etke.cc/go/mxidwc"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

func parseMXIDpatterns(patterns []string, defaultPattern string) ([]*regexp.Regexp, error) {
	if len(patterns) == 0 && defaultPattern != "" {
		patterns = []string{defaultPattern}
	}

	return mxidwc.ParsePatterns(patterns)
}

func (b *Bot) allowUsers(actorID id.UserID) bool {
	if len(b.allowedUsers) != 0 {
		if !mxidwc.Match(actorID.String(), b.allowedUsers) {
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
		b.Error(sentry.SetHubOnContext(context.Background(), sentry.CurrentHub()), targetRoomID, "failed to retrieve settings: %v", err)
		return false
	}

	owner := cfg.Owner()
	if owner == "" {
		return true
	}

	return owner == actorID.String()
}

func (b *Bot) allowAdmin(actorID id.UserID, targetRoomID id.RoomID) bool {
	return mxidwc.Match(actorID.String(), b.allowedAdmins)
}

func (b *Bot) allowSend(actorID id.UserID, targetRoomID id.RoomID) bool {
	if !b.allowUsers(actorID) {
		return false
	}

	cfg, err := b.getRoomSettings(targetRoomID)
	if err != nil {
		b.Error(sentry.SetHubOnContext(context.Background(), sentry.CurrentHub()), targetRoomID, "failed to retrieve settings: %v", err)
		return false
	}

	return !cfg.NoSend()
}

// AllowAuth check if SMTP login (email) and password are valid
func (b *Bot) AllowAuth(email, password string) bool {
	if !strings.HasSuffix(email, "@"+b.domain) {
		return false
	}

	roomID, ok := b.getMapping(utils.Mailbox(email))
	if !ok {
		return false
	}
	cfg, err := b.getRoomSettings(roomID)
	if err != nil {
		b.log.Error("failed to retrieve settings: %v", err)
		return false
	}

	allow, err := argon2pw.CompareHashWithPassword(cfg.Password(), password)
	if err != nil {
		b.log.Warn("Password for %s is not valid: %v", email, err)
	}
	return allow
}
