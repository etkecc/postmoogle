package bot

import "maunium.net/go/mautrix/id"

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
	adminRoom := b.getBotSettings().AdminRoom()
	if adminRoom != "" {
		b.adminRooms = append(b.adminRooms, adminRoom)
	}

	resp, err := b.lp.GetClient().JoinedRooms()
	if err != nil {
		return err
	}
	for _, roomID := range resp.JoinedRooms {
		b.migrateRoomSettings(roomID)
		cfg, serr := b.getRoomSettings(roomID)
		if serr != nil {
			continue
		}
		mailbox := cfg.Mailbox()
		active := cfg.Active()
		if mailbox != "" && active {
			b.rooms.Store(mailbox, roomID)
		}

		if cfg.Owner() != "" && b.allowAdmin(id.UserID(cfg.Owner()), "") {
			b.adminRooms = append(b.adminRooms, roomID)
		}
	}

	return nil
}

func (b *Bot) syncBanlist() {
	b.lock("banlist")
	defer b.unlock("banlist")

	if !b.getBotSettings().BanlistEnabled() {
		b.banlist = make(bglist, 0)
		return
	}
	b.banlist = b.getBanlist()
}
