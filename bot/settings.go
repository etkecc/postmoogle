package bot

import (
	"strconv"
	"strings"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// settings of a room
type settings map[string]string

// settingsOld of a room
type settingsOld struct {
	Mailbox  string
	Owner    id.UserID
	NoSender bool
}

// Get option
func (s settings) Get(key string) string {
	return s[strings.ToLower(strings.TrimSpace(key))]
}

func (s settings) Mailbox() string {
	return s.Get(optionMailbox)
}

func (s settings) Owner() string {
	return s.Get(optionOwner)
}

func (s settings) NoSender() bool {
	return utils.Bool(s.Get(optionNoSender))
}

func (s settings) NoSubject() bool {
	return utils.Bool(s.Get(optionNoSubject))
}

func (s settings) NoHTML() bool {
	return utils.Bool(s.Get(optionNoHTML))
}

func (s settings) NoThreads() bool {
	return utils.Bool(s.Get(optionNoThreads))
}

func (s settings) NoFiles() bool {
	return utils.Bool(s.Get(optionNoFiles))
}

// Set option
func (s settings) Set(key, value string) {
	s[strings.ToLower(strings.TrimSpace(key))] = value
}

// TODO: remove after migration
func (b *Bot) migrateSettings(roomID id.RoomID) {
	var config settingsOld
	err := b.lp.GetClient().GetRoomAccountData(roomID, settingskey, &config)
	if err != nil {
		// any error = no need to migrate
		return
	}

	if config.Mailbox == "" {
		return
	}
	cfg := settings{}
	cfg.Set(optionMailbox, config.Mailbox)
	cfg.Set(optionOwner, config.Owner.String())
	cfg.Set(optionNoSender, strconv.FormatBool(config.NoSender))

	err = b.setSettings(roomID, cfg)
	if err != nil {
		b.log.Error("cannot migrate settings: %v", err)
	}
}

func (b *Bot) getSettings(roomID id.RoomID) (settings, error) {
	config := settings{}
	err := b.lp.GetClient().GetRoomAccountData(roomID, settingskey, &config)
	if err != nil {
		if strings.Contains(err.Error(), "M_NOT_FOUND") {
			// Suppress `M_NOT_FOUND (HTTP 404): Room account data not found` errors.
			// Until some settings are explicitly set, we don't store any.
			// In such cases, just return a default (empty) settings object.
			err = nil
		}
	}

	return config, utils.UnwrapError(err)
}

func (b *Bot) setSettings(roomID id.RoomID, cfg settings) error {
	return utils.UnwrapError(b.lp.GetClient().SetRoomAccountData(roomID, settingskey, cfg))
}
