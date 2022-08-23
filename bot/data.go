package bot

import (
	"context"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix/id"
)

const settingskey = "cc.etke.postmoogle.settings"

var migrations = []string{}

type settings map[string]string

// settingsStruct of a room
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

	owner := s.Get("owner")
	if owner == "" {
		return true
	}

	return owner == userID.String()
}

// Get option
func (s settings) Get(key string) string {
	return s[strings.ToLower(strings.TrimSpace(key))]
}

// Set option
func (s settings) Set(key, value string) {
	s[strings.ToLower(strings.TrimSpace(key))] = value
}

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

func (b *Bot) syncRooms(ctx context.Context) error {
	b.roomsmu.Lock()
	defer b.roomsmu.Unlock()
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("syncRooms"))
	defer span.Finish()

	resp, err := b.lp.GetClient().JoinedRooms()
	if err != nil {
		return err
	}
	b.rooms = make(map[string]id.RoomID, len(resp.JoinedRooms))
	for _, roomID := range resp.JoinedRooms {
		b.migrateSettings(span.Context(), roomID)
		cfg, serr := b.getSettings(span.Context(), roomID)
		if serr != nil {
			b.log.Warn("cannot get %s settings: %v", roomID, err)
			continue
		}
		mailbox := cfg.Get("mailbox")
		if mailbox != "" {
			b.rooms[mailbox] = roomID
		}
	}

	return nil
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
	cfg.Set("mailbox", config.Mailbox)
	cfg.Set("owner", config.Owner.String())
	cfg.Set("nosender", strconv.FormatBool(config.NoSender))

	err = b.setSettings(ctx, roomID, cfg)
	if err != nil {
		b.log.Error("cannot migrate settings: %v", err)
	}
}

func (b *Bot) getSettings(ctx context.Context, roomID id.RoomID) (settings, error) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("getSettings"))
	defer span.Finish()

	var config settings
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
