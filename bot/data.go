package bot

import (
	"context"
	"strings"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix/id"
)

// account data keys
const (
	messagekey  = "cc.etke.postmoogle.message"
	settingskey = "cc.etke.postmoogle.settings"
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
		mailbox := cfg.Mailbox()
		if mailbox != "" {
			b.rooms[mailbox] = roomID
		}
	}

	return nil
}

func (b *Bot) getThreadID(ctx context.Context, roomID id.RoomID, messageID string) id.EventID {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("getThreadID"))
	defer span.Finish()

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

func (b *Bot) setThreadID(ctx context.Context, roomID id.RoomID, messageID string, eventID id.EventID) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("setThreadID"))
	defer span.Finish()

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
