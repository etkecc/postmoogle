package bot

import (
	"strconv"
	"strings"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// account data key
const acRoomSettingsKey = "cc.etke.postmoogle.settings"

// option keys
const (
	optionOwner     = "owner"
	optionMailbox   = "mailbox"
	optionNoSender  = "nosender"
	optionNoSubject = "nosubject"
	optionNoHTML    = "nohtml"
	optionNoThreads = "nothreads"
	optionNoFiles   = "nofiles"
)

type roomsettings map[string]string

// settingsOld of a room
type settingsOld struct {
	Mailbox  string
	Owner    id.UserID
	NoSender bool
}

// Get option
func (s roomsettings) Get(key string) string {
	return s[strings.ToLower(strings.TrimSpace(key))]
}

// Set option
func (s roomsettings) Set(key, value string) {
	s[strings.ToLower(strings.TrimSpace(key))] = value
}

func (s roomsettings) Mailbox() string {
	return s.Get(optionMailbox)
}

func (s roomsettings) Owner() string {
	return s.Get(optionOwner)
}

func (s roomsettings) NoSender() bool {
	return utils.Bool(s.Get(optionNoSender))
}

func (s roomsettings) NoSubject() bool {
	return utils.Bool(s.Get(optionNoSubject))
}

func (s roomsettings) NoHTML() bool {
	return utils.Bool(s.Get(optionNoHTML))
}

func (s roomsettings) NoThreads() bool {
	return utils.Bool(s.Get(optionNoThreads))
}

func (s roomsettings) NoFiles() bool {
	return utils.Bool(s.Get(optionNoFiles))
}

// TODO: remove after migration
func (b *Bot) migrateSettings(roomID id.RoomID) {
	var config settingsOld
	err := b.lp.GetClient().GetRoomAccountData(roomID, acRoomSettingsKey, &config)
	if err != nil {
		// any error = no need to migrate
		return
	}

	if config.Mailbox == "" {
		return
	}
	cfg := roomsettings{}
	cfg.Set(optionMailbox, config.Mailbox)
	cfg.Set(optionOwner, config.Owner.String())
	cfg.Set(optionNoSender, strconv.FormatBool(config.NoSender))

	err = b.setRoomSettings(roomID, cfg)
	if err != nil {
		b.log.Error("cannot migrate settings: %v", err)
	}
}

func (b *Bot) getRoomSettings(roomID id.RoomID) (roomsettings, error) {
	cfg := b.cfg.Get(roomID.String())
	if cfg != nil {
		return cfg, nil
	}

	config := roomsettings{}
	err := b.lp.GetClient().GetRoomAccountData(roomID, acRoomSettingsKey, &config)
	if err != nil {
		if strings.Contains(err.Error(), "M_NOT_FOUND") {
			// Suppress `M_NOT_FOUND (HTTP 404): Room account data not found` errors.
			// Until some settings are explicitly set, we don't store any.
			// In such cases, just return a default (empty) settings object.
			err = nil
		}
	} else {
		b.cfg.Set(roomID.String(), config)
	}

	return config, utils.UnwrapError(err)
}

func (b *Bot) setRoomSettings(roomID id.RoomID, cfg roomsettings) error {
	b.cfg.Set(roomID.String(), cfg)
	return utils.UnwrapError(b.lp.GetClient().SetRoomAccountData(roomID, acRoomSettingsKey, cfg))
}
