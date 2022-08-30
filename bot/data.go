package bot

import (
	"strings"

	"maunium.net/go/mautrix/id"
)

// account data keys
const (
	messagekey = "cc.etke.postmoogle.message"
)

// event keys
const (
	eventMessageIDkey = "cc.etke.postmoogle.messageID"
	eventInReplyToKey = "cc.etke.postmoogle.inReplyTo"
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
	resp, err := b.lp.GetClient().JoinedRooms()
	if err != nil {
		return err
	}
	for _, roomID := range resp.JoinedRooms {
		b.migrateSettings(roomID)
		cfg, serr := b.getRoomSettings(roomID)
		if serr != nil {
			b.log.Warn("cannot get %s settings: %v", roomID, err)
			continue
		}
		mailbox := cfg.Mailbox()
		if mailbox != "" {
			b.rooms.Store(mailbox, roomID)
		}
	}

	return nil
}

func (b *Bot) getThreadID(roomID id.RoomID, messageID string) id.EventID {
	key := messagekey + "." + messageID
	data := map[string]id.EventID{}
	err := b.lp.GetClient().GetRoomAccountData(roomID, key, &data)
	if err != nil {
		if !strings.Contains(err.Error(), "M_NOT_FOUND") {
			b.log.Error("cannot retrieve account data %s: %v", key, err)
			return ""
		}
	}

	return data["eventID"]
}

func (b *Bot) setThreadID(roomID id.RoomID, messageID string, eventID id.EventID) {
	key := messagekey + "." + messageID
	data := map[string]id.EventID{
		"eventID": eventID,
	}

	err := b.lp.GetClient().SetRoomAccountData(roomID, key, data)
	if err != nil {
		if !strings.Contains(err.Error(), "M_NOT_FOUND") {
			b.log.Error("cannot save account data %s: %v", key, err)
		}
	}
}
