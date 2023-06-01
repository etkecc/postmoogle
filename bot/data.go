package bot

import (
	"strconv"
	"time"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/bot/config"
)

func (b *Bot) syncRooms() error {
	adminRooms := []id.RoomID{}

	adminRoom := b.cfg.GetBot().AdminRoom()
	if adminRoom != "" {
		adminRooms = append(adminRooms, adminRoom)
	}

	resp, err := b.lp.GetClient().JoinedRooms()
	if err != nil {
		return err
	}
	for _, roomID := range resp.JoinedRooms {
		b.migrateRoomSettings(roomID)
		cfg, serr := b.cfg.GetRoom(roomID)
		if serr != nil {
			continue
		}
		mailbox := cfg.Mailbox()
		active := cfg.Active()
		if mailbox != "" && active {
			b.rooms.Store(mailbox, roomID)
		}

		if cfg.Owner() != "" && b.allowAdmin(id.UserID(cfg.Owner()), "") {
			adminRooms = append(adminRooms, roomID)
		}
	}
	b.adminRooms = adminRooms

	return nil
}

func (b *Bot) migrateRoomSettings(roomID id.RoomID) {
	cfg, err := b.cfg.GetRoom(roomID)
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
	err = b.cfg.SetRoom(roomID, cfg)
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
func (b *Bot) migrateMautrix015() error {
	cfg := b.cfg.GetBot()
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
	return b.cfg.SetBot(cfg)
}

func (b *Bot) initBotUsers() ([]string, error) {
	cfg := b.cfg.GetBot()
	cfgUsers := cfg.Users()
	if len(cfgUsers) > 0 {
		return cfgUsers, nil
	}

	_, homeserver, err := b.lp.GetClient().UserID.Parse()
	if err != nil {
		return nil, err
	}
	cfg.Set(config.BotUsers, "@*:"+homeserver)
	return cfg.Users(), b.cfg.SetBot(cfg)
}

// SyncRooms and mailboxes
func (b *Bot) SyncRooms() {
	b.syncRooms() //nolint:errcheck // nothing can be done here
}
