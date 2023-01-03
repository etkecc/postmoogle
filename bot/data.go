package bot

import (
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/bot/config"
)

var migrations = []string{}

func (b *Bot) migrate() error {
	b.log.Debug("migrating database...")
	tx, beginErr := b.lp.GetDB().Begin()
	if beginErr != nil {
		b.log.Error("cannot begin transaction: %v", beginErr)
		return beginErr
	}

	for _, query := range migrations {
		_, execErr := tx.Exec(query)
		if execErr != nil {
			b.log.Error("cannot apply migration: %v", execErr)
			// nolint // we already have the execErr to return
			tx.Rollback()
			return execErr
		}
	}

	commitErr := tx.Commit()
	if commitErr != nil {
		b.log.Error("cannot commit transaction: %v", commitErr)
		// nolint // we already have the commitErr to return
		tx.Rollback()
		return commitErr
	}

	return nil
}

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
		b.log.Error("cannot retrieve room settings: %v", err)
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
		b.log.Error("cannot migrate room settings: %v", err)
	}
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
