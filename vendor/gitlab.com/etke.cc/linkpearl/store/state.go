package store

import (
	"database/sql"
	"encoding/json"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// NOTE: functions in that file are for crypto.StateStore implementation
// ref: https://pkg.go.dev/maunium.net/go/mautrix/crypto#StateStore

// GetEncryptionEvent returns the encryption event's content for an encrypted room.
func (s *Store) GetEncryptionEvent(roomID id.RoomID) *event.EncryptionEventContent {
	if !s.encryption {
		return nil
	}
	s.log.Debug("finding encryption event of %q", roomID)
	query := "SELECT encryption_event FROM rooms WHERE room_id = $1"
	row := s.db.QueryRow(query, roomID)

	var encryptionEventJSON []byte
	err := row.Scan(&encryptionEventJSON)
	if err != nil && err != sql.ErrNoRows {
		s.log.Error("cannot find encryption event: %v", err)
		return nil
	}
	var encryptionEvent event.EncryptionEventContent
	if err := json.Unmarshal(encryptionEventJSON, &encryptionEvent); err != nil {
		s.log.Debug("cannot unmarshal encryption event: %q", err)
		return nil
	}

	return &encryptionEvent
}

// FindSharedRooms returns the encrypted rooms that another user is also in for a user ID.
func (s *Store) FindSharedRooms(userID id.UserID) []id.RoomID {
	if !s.encryption {
		return nil
	}
	s.log.Debug("loading shared rooms for %q", userID)
	query := "SELECT room_id FROM room_members WHERE user_id = $1"
	rows, queryErr := s.db.Query(query, userID)
	rooms := make([]id.RoomID, 0)
	if queryErr != nil {
		s.log.Error("cannot load room members: %q", queryErr)
		return rooms
	}
	defer rows.Close()

	var roomID id.RoomID
	for rows.Next() {
		scanErr := rows.Scan(&roomID)
		if scanErr != nil {
			continue
		}
		rooms = append(rooms, roomID)
	}

	return rooms
}
