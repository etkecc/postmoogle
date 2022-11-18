package bot

import (
	"context"
	"net"
	"regexp"
	"strings"
	"time"

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

// IsGreylisted checks if host is in greylist
func (b *Bot) IsGreylisted(addr net.Addr) bool {
	if b.getBotSettings().Greylist() == 0 {
		return false
	}

	greylist := b.getGreylist()
	greylistedAt, ok := greylist.Get(addr)
	if !ok {
		b.log.Debug("greylisting %s", addr.String())
		greylist.Add(addr)
		err := b.setGreylist(greylist)
		if err != nil {
			b.log.Error("cannot update greylist with %s: %v", addr.String(), err)
		}
		return true
	}
	duration := time.Duration(b.getBotSettings().Greylist()) * time.Minute

	return greylistedAt.Add(duration).After(time.Now().UTC())
}

// IsBanned checks if address is banned
func (b *Bot) IsBanned(addr net.Addr) bool {
	return b.banlist.Has(addr)
}

// Ban an address
func (b *Bot) Ban(addr net.Addr) {
	if !b.getBotSettings().BanlistEnabled() {
		return
	}

	b.log.Debug("banning %s", addr.String())
	banlist := b.getBanlist()
	banlist.Add(addr)
	err := b.setBanlist(banlist)
	if err != nil {
		b.log.Error("cannot update banlist with %s: %v", addr.String(), err)
	}
}

// AllowAuth check if SMTP login (email) and password are valid
func (b *Bot) AllowAuth(email, password string) bool {
	var suffix bool
	for _, domain := range b.domains {
		if strings.HasSuffix(email, "@"+domain) {
			suffix = true
			break
		}
	}
	if !suffix {
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
