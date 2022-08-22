package bot

import (
	"context"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix/id"
)

const settingskey = "cc.etke.postmoogle.settings"

var migrations = []string{}

// settings of a room
type settings struct {
	Mailbox string
	Owner   id.UserID
}

// Allowed checks if change is allowed
func (s *settings) Allowed(noowner bool, userID id.UserID) bool {
	if noowner {
		return true
	}

	if s.Owner == "" {
		return true
	}

	return s.Owner == userID
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

func (b *Bot) getSettings(ctx context.Context, roomID id.RoomID) (*settings, error) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("getSettings"))
	defer span.Finish()

	var config settings
	err := b.lp.GetClient().GetRoomAccountData(roomID, settingskey, &config)

	return &config, err
}

func (b *Bot) setSettings(ctx context.Context, roomID id.RoomID, cfg *settings) error {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("setSettings"))
	defer span.Finish()

	return b.lp.GetClient().SetRoomAccountData(roomID, settingskey, cfg)
}
