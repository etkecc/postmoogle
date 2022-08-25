package bot

import (
	"context"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
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

// Allowed checks if change is allowed
func (s settings) Allowed(noowner bool, userID id.UserID) bool {
	if noowner {
		return true
	}

	owner := s.Owner()
	if owner == "" {
		return true
	}

	return owner == userID.String()
}

// Get option
func (s settings) Get(key string) string {
	value := s[strings.ToLower(strings.TrimSpace(key))]

	sanitizer, ok := sanitizers[key]
	if ok {
		return sanitizer(value)
	}
	return value
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

// Set option
func (s settings) Set(key, value string) {
	s[strings.ToLower(strings.TrimSpace(key))] = value
}

// TODO: remove after migration
func (b *Bot) migrateSettings(ctx context.Context, roomID id.RoomID) {
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

	err = b.setSettings(ctx, roomID, cfg)
	if err != nil {
		b.log.Error("cannot migrate settings: %v", err)
	}
}

func (b *Bot) getSettings(ctx context.Context, roomID id.RoomID) (settings, error) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("getSettings"))
	defer span.Finish()

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

	return config, err
}

func (b *Bot) setSettings(ctx context.Context, roomID id.RoomID, cfg settings) error {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("setSettings"))
	defer span.Finish()

	return b.lp.GetClient().SetRoomAccountData(roomID, settingskey, cfg)
}
