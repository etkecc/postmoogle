package bot

import (
	"context"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/etkecc/go-mxidwc"
	"github.com/raja/argon2pw"
	"maunium.net/go/mautrix/id"

	"github.com/etkecc/postmoogle/internal/utils"
)

func parseMXIDpatterns(patterns []string, defaultPattern string) ([]*regexp.Regexp, error) {
	if len(patterns) == 0 && defaultPattern != "" {
		patterns = []string{defaultPattern}
	}

	return mxidwc.ParsePatterns(patterns)
}

func (b *Bot) allowUsers(ctx context.Context, actorID id.UserID, targetRoomID id.RoomID) bool {
	// first, check if it's an allowed user
	if mxidwc.Match(actorID.String(), b.allowedUsers) {
		return true
	}

	// second, check if it's an admin (admin may not fit the allowed users pattern)
	if b.allowAdmin(ctx, actorID, targetRoomID) {
		return true
	}

	// then, check if it's the owner (same as above)
	cfg, err := b.cfg.GetRoom(ctx, targetRoomID)
	if err == nil && cfg.Owner() == actorID.String() {
		return true
	}

	return false
}

func (b *Bot) allowAnyone(_ context.Context, _ id.UserID, _ id.RoomID) bool {
	return true
}

func (b *Bot) allowOwner(ctx context.Context, actorID id.UserID, targetRoomID id.RoomID) bool {
	if !b.allowUsers(ctx, actorID, targetRoomID) {
		return false
	}
	cfg, err := b.cfg.GetRoom(ctx, targetRoomID)
	if err != nil {
		b.Error(context.Background(), "failed to retrieve settings: %v", err)
		return false
	}

	owner := cfg.Owner()
	if owner == "" {
		return true
	}

	return owner == actorID.String() || b.allowAdmin(ctx, actorID, targetRoomID)
}

func (b *Bot) allowAdmin(_ context.Context, actorID id.UserID, _ id.RoomID) bool {
	return mxidwc.Match(actorID.String(), b.allowedAdmins)
}

func (b *Bot) allowSend(ctx context.Context, actorID id.UserID, targetRoomID id.RoomID) bool {
	if !b.allowUsers(ctx, actorID, targetRoomID) {
		return false
	}

	cfg, err := b.cfg.GetRoom(ctx, targetRoomID)
	if err != nil {
		b.Error(context.Background(), "failed to retrieve settings: %v", err)
		return false
	}

	return !cfg.NoSend()
}

func (b *Bot) allowReply(ctx context.Context, actorID id.UserID, targetRoomID id.RoomID) bool {
	if !b.allowUsers(ctx, actorID, targetRoomID) {
		return false
	}

	cfg, err := b.cfg.GetRoom(ctx, targetRoomID)
	if err != nil {
		b.Error(ctx, "failed to retrieve settings: %v", err)
		return false
	}

	return !cfg.NoReplies()
}

func (b *Bot) isReserved(mailbox string) bool {
	for _, reserved := range b.mbxc.Reserved {
		if mailbox == reserved {
			return true
		}
	}
	return false
}

// IsGreylisted checks if host is in greylist
func (b *Bot) IsGreylisted(ctx context.Context, addr net.Addr) bool {
	if b.cfg.GetBot(ctx).Greylist() == 0 {
		return false
	}

	greylist := b.cfg.GetGreylist(ctx)
	greylistedAt, ok := greylist.Get(addr)
	if !ok {
		b.log.Debug().Str("addr", addr.String()).Msg("greylisting")
		greylist.Add(addr)
		err := b.cfg.SetGreylist(ctx, greylist)
		if err != nil {
			b.log.Error().Err(err).Str("addr", addr.String()).Msg("cannot update greylist")
		}
		return true
	}
	duration := time.Duration(b.cfg.GetBot(ctx).Greylist()) * time.Minute

	return greylistedAt.Add(duration).After(time.Now().UTC())
}

// IsBanned checks if address is banned
func (b *Bot) IsBanned(ctx context.Context, addr net.Addr) bool {
	return b.cfg.GetBanlist(ctx).Has(addr)
}

// IsTrusted checks if address is a trusted (proxy)
func (b *Bot) IsTrusted(addr net.Addr) bool {
	ip := utils.AddrIP(addr)
	for _, proxy := range b.proxies {
		if ip == proxy {
			b.log.Debug().Str("addr", ip).Msg("address is trusted")
			return true
		}
	}

	return false
}

// Ban an address automatically
func (b *Bot) BanAuto(ctx context.Context, addr net.Addr) {
	if !b.cfg.GetBot(ctx).BanlistEnabled() {
		return
	}

	if !b.cfg.GetBot(ctx).BanlistAuto() {
		return
	}

	if b.IsTrusted(addr) {
		return
	}
	b.log.Debug().Str("addr", addr.String()).Msg("attempting to automatically ban")
	banlist := b.cfg.GetBanlist(ctx)
	banlist.Add(addr)
	err := b.cfg.SetBanlist(ctx, banlist)
	if err != nil {
		b.log.Error().Err(err).Str("addr", addr.String()).Msg("cannot update banlist")
	}
}

// Ban an address for incorrect auth automatically
func (b *Bot) BanAuth(ctx context.Context, addr net.Addr) {
	if !b.cfg.GetBot(ctx).BanlistEnabled() {
		return
	}

	if !b.cfg.GetBot(ctx).BanlistAuth() {
		return
	}

	if b.IsTrusted(addr) {
		return
	}
	b.log.Debug().Str("addr", addr.String()).Msg("attempting to automatically ban")
	banlist := b.cfg.GetBanlist(ctx)
	banlist.Add(addr)
	err := b.cfg.SetBanlist(ctx, banlist)
	if err != nil {
		b.log.Error().Err(err).Str("addr", addr.String()).Msg("cannot update banlist")
	}
}

// Ban an address manually
func (b *Bot) BanManually(ctx context.Context, addr net.Addr) {
	if !b.cfg.GetBot(ctx).BanlistEnabled() {
		return
	}
	if b.IsTrusted(addr) {
		return
	}
	b.log.Debug().Str("addr", addr.String()).Msg("attempting to manually ban")
	banlist := b.cfg.GetBanlist(ctx)
	banlist.Add(addr)
	err := b.cfg.SetBanlist(ctx, banlist)
	if err != nil {
		b.log.Error().Err(err).Str("addr", addr.String()).Msg("cannot update banlist")
	}
}

// AllowAuth check if SMTP login (email) and password are valid
func (b *Bot) AllowAuth(ctx context.Context, email, password string) (id.RoomID, bool) {
	var suffix bool
	for _, domain := range b.domains {
		if strings.HasSuffix(email, "@"+domain) {
			suffix = true
			break
		}
	}
	if !suffix {
		return "", false
	}

	roomID, ok := b.getMapping(utils.Mailbox(email))
	if !ok {
		return "", false
	}
	cfg, err := b.cfg.GetRoom(ctx, roomID)
	if err != nil {
		b.log.Error().Err(err).Msg("failed to retrieve settings")
		return "", false
	}

	if cfg.NoSend() {
		b.log.Warn().Str("email", email).Str("roomID", roomID.String()).Msg("trying to send email, but room is receive-only")
		return "", false
	}

	allow, err := argon2pw.CompareHashWithPassword(cfg.Password(), password)
	if err != nil {
		b.log.Warn().Err(err).Str("email", email).Msg("Password is not valid")
	}
	return roomID, allow
}
