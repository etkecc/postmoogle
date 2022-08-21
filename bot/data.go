package bot

import (
	"context"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix/id"
)

const settingskey = "cc.etke.postmoogle.settings"

var migrations = []string{
	`
		CREATE TABLE IF NOT EXISTS settings (
			room_id VARCHAR(255),
			mailbox VARCHAR(255)
		)
		`,
}

// settings of a room
type settings struct {
	Mailbox string
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
	query := "SELECT * FROM settings WHERE room_id = "
	switch b.lp.GetStore().GetDialect() {
	case "postgres":
		query += "$1"
	case "sqlite3":
		query += "?"
	}
	row := b.lp.GetDB().QueryRow(query, roomID)
	err := row.Scan(&config.Mailbox)
	if err == nil {
		return &config, nil
	}
	b.log.Error("cannot find settings in database: %v", err)

	err = b.lp.GetClient().GetRoomAccountData(roomID, settingskey, &config)

	return &config, err
}

func (b *Bot) setSettings(ctx context.Context, roomID id.RoomID, cfg *settings) error {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("setSettings"))
	defer span.Finish()

	tx, err := b.lp.GetDB().Begin()
	if err == nil {
		var insert string
		switch b.lp.GetStore().GetDialect() {
		case "postgres":
			insert = "INSERT INTO settings VALUES ($1) ON CONFLICT (room_id) DO UPDATE SET mailbox = $1"
		case "sqlite3":
			insert = "INSERT INTO settings VALUES (?) ON CONFLICT (room_id) DO UPDATE SET mailbox = ?"
		}
		_, err = tx.Exec(insert, cfg.Mailbox)
		if err != nil {
			b.log.Error("cannot insert settigs: %v", err)
			// nolint // no need to check error here
			tx.Rollback()
		}

		if err != nil {
			err = tx.Commit()
			if err != nil {
				b.log.Error("cannot commit transaction: %v", err)
			}
		}
	}

	return b.lp.GetClient().SetRoomAccountData(roomID, settingskey, cfg)
}

// FindRoomID by mailbox
func (b *Bot) FindRoomID(mailbox string) (id.RoomID, error) {
	query := "SELECT room_id FROM settings WHERE mailbox = "
	switch b.lp.GetStore().GetDialect() {
	case "postgres":
		query += "$1"
	case "sqlite3":
		query += "?"
	}

	var roomID string
	row := b.lp.GetDB().QueryRow(query, mailbox)
	err := row.Scan(&roomID)

	return id.RoomID(roomID), err
}
