package bot

import (
	"strings"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// account data keys
const (
	messagekey   = "cc.etke.postmoogle.message"
	settingskey  = "cc.etke.postmoogle.settings"
	botconfigkey = "cc.etke.postmoogle.config"
)

// event keys
const (
	eventMessageIDkey = "cc.etke.postmoogle.messageID"
	eventInReplyToKey = "cc.etke.postmoogle.inReplyTo"
)

// option keys
const (
	optionOwner     = "owner"
	optionMailbox   = "mailbox"
	optionNoSender  = "nosender"
	optionNoSubject = "nosubject"
	optionNoHTML    = "nohtml"
	optionNoThreads = "nothreads"
	optionNoFiles   = "nofiles"

	botOptionUsers = "users"
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
		cfg, serr := b.getSettings(roomID)
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

// TODO: remove after migration
func (b *Bot) migrateBotSettings(users []string) error {
	config := b.getBotSettings()
	cfgUsers := config.Users()
	if len(users) > 0 && len(cfgUsers) == 0 {
		_, err := parseMXIDpatterns(users, "")
		if err != nil {
			return err
		}
		config.Set(botOptionUsers, strings.Join(users, " "))
		return b.setBotSettings(config)
	}

	return nil
}

func (b *Bot) getBotSettings() settings {
	cfg := b.cfg.Get(botconfigkey)
	if cfg != nil {
		return cfg
	}

	config := settings{}
	err := b.lp.GetClient().GetAccountData(botconfigkey, &config)
	if err != nil {
		if strings.Contains(err.Error(), "M_NOT_FOUND") {
			err = nil
		}
		b.log.Error("cannot get bot settings: %v", utils.UnwrapError(err))
	} else {
		b.cfg.Set(botconfigkey, config)
	}

	return config
}

func (b *Bot) setBotSettings(cfg settings) error {
	b.cfg.Set(botconfigkey, cfg)
	return utils.UnwrapError(b.lp.GetClient().SetAccountData(botconfigkey, cfg))
}
