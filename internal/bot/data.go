package bot

import (
	"context"
	"strconv"
	"strings"
	"time"

	"maunium.net/go/mautrix/id"

	"github.com/etkecc/postmoogle/internal/bot/config"
)

func (b *Bot) addRoom(roomID id.RoomID, cfg config.Room) {
	mailbox := strings.ToLower(strings.TrimSpace(cfg.Mailbox()))
	b.rooms.Store(mailbox, roomID)
	aliases := cfg.Aliases()
	if len(aliases) > 0 {
		for _, alias := range aliases {
			alias = strings.ToLower(strings.TrimSpace(alias))
			b.rooms.Store(alias, roomID)
		}
	}
}

func (b *Bot) syncRooms(ctx context.Context) error {
	adminRooms := []id.RoomID{}

	adminRoom := b.cfg.GetBot(ctx).AdminRoom()
	if adminRoom != "" {
		adminRooms = append(adminRooms, adminRoom)
	}

	resp, err := b.lp.GetClient().JoinedRooms(ctx)
	if err != nil {
		return err
	}
	for _, roomID := range resp.JoinedRooms {
		b.migrateRoomSettings(ctx, roomID)
		cfg, serr := b.cfg.GetRoom(ctx, roomID)
		if serr != nil {
			continue
		}
		if cfg.Mailbox() != "" && cfg.Active() {
			b.addRoom(roomID, cfg)
		}

		if cfg.Owner() != "" && b.allowAdmin(ctx, id.UserID(cfg.Owner()), "") {
			adminRooms = append(adminRooms, roomID)
		}
	}
	b.adminRooms = adminRooms

	return nil
}

func (b *Bot) migrateRoomSettings(ctx context.Context, roomID id.RoomID) {
	cfg, err := b.cfg.GetRoom(ctx, roomID)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot retrieve room settings")
		return
	}
	if _, ok := cfg[config.RoomActive]; !ok {
		cfg.Set(config.RoomActive, "true")
	}

	if cfg["spamlist:emails"] == "" && cfg["spamlist:localparts"] == "" && cfg["spamlist:hosts"] == "" {
		return
	}
	cfg.MigrateSpamlistSettings()
	err = b.cfg.SetRoom(ctx, roomID, cfg)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot migrate room settings")
	}
}

// migrateMautrix015 adds a special timestamp to bot's config
// to ignore any message events happened before that timestamp
// with migration to maturix 0.15.x the state store has been changed
// alongside with other database configs to simplify maintenance,
// but with that simplification there is no proper way to migrate
// existing sync token and session info. No data loss, tho.
func (b *Bot) migrateMautrix015(ctx context.Context) error {
	cfg := b.cfg.GetBot(ctx)
	ts := cfg.Mautrix015Migration()
	// already migrated
	if ts > 0 {
		b.ignoreBefore = ts
		return nil
	}

	ts = time.Now().UTC().UnixMilli()
	b.ignoreBefore = ts

	tss := strconv.FormatInt(ts, 10)
	cfg.Set(config.BotMautrix015Migration, tss)
	return b.cfg.SetBot(ctx, cfg)
}

func (b *Bot) initBotUsers(ctx context.Context) ([]string, error) {
	cfg := b.cfg.GetBot(ctx)
	cfgUsers := cfg.Users()
	if len(cfgUsers) > 0 {
		return cfgUsers, nil
	}

	_, homeserver, err := b.lp.GetClient().UserID.Parse()
	if err != nil {
		return nil, err
	}
	cfg.Set(config.BotUsers, "@*:"+homeserver)
	return cfg.Users(), b.cfg.SetBot(ctx, cfg)
}

// SyncRooms and mailboxes
func (b *Bot) SyncRooms() {
	b.syncRooms(context.Background()) //nolint:errcheck // nothing can be done here
}
