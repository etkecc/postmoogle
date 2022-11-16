package store

import (
	"encoding/json"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var acceptedMembershipTypes = []event.Membership{
	event.MembershipJoin,
	event.MembershipInvite,
	event.MembershipBan,
	event.MembershipLeave,
}

// IsEncrypted returns whether a room is encrypted.
func (s *Store) IsEncrypted(roomID id.RoomID) bool {
	if !s.encryption {
		return false
	}

	s.log.Debug("checking if room %s is encrypted", roomID)
	return s.GetEncryptionEvent(roomID) != nil
}

// SetEncryptionEvent creates or updates room's encryption event info
func (s *Store) SetEncryptionEvent(evt *event.Event) {
	if !s.encryption {
		return
	}
	if evt == nil {
		return
	}

	var encryptionEventJSON []byte
	encryptionEventJSON, err := json.Marshal(evt)
	if err != nil {
		s.log.Debug("cannot marshal encryption event: %v", err)
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		s.log.Error("cannot begin transaction: %v", err)
		return
	}

	var insert string
	switch s.dialect {
	case "sqlite3":
		insert = "INSERT OR IGNORE INTO rooms VALUES (?, ?)"
	case "postgres":
		insert = "INSERT INTO rooms VALUES ($1, $2) ON CONFLICT DO NOTHING"
	}
	update := "UPDATE rooms SET encryption_event = $1 WHERE room_id = $2"

	_, err = tx.Exec(update, encryptionEventJSON, evt.RoomID)
	if err != nil {
		s.log.Error("cannot update encryption event: %v", err)
		// nolint // we already have err to return
		tx.Rollback()
		return
	}

	_, err = tx.Exec(insert, evt.RoomID, encryptionEventJSON)
	if err != nil {
		s.log.Error("cannot insert encryption event: %v", err)
		// nolint // interface doesn't allow to return error
		tx.Rollback()
		return
	}

	err = tx.Commit()
	if err != nil {
		s.log.Error("cannot commit transaction: %v", err)
	}
}

// SetMembership saves room members
func (s *Store) SetMembership(evt *event.Event) {
	s.log.Debug("saving membership event for %s", evt.RoomID)
	tx, err := s.db.Begin()
	if err != nil {
		s.log.Error("cannot begin transaction: %v", err)
		return
	}

	var insert string
	switch s.dialect {
	case "sqlite3":
		insert = "INSERT OR IGNORE INTO room_members VALUES (?, ?)"
	case "postgres":
		insert = "INSERT INTO room_members VALUES ($1, $2) ON CONFLICT DO NOTHING"
	}
	del := "DELETE FROM room_members WHERE room_id = $1 AND user_id = $2"

	membership := evt.Content.AsMember().Membership
	if s.shouldIgnoreMembership(membership) {
		return
	}
	if membership.IsInviteOrJoin() {
		_, err := tx.Exec(insert, evt.RoomID, evt.GetStateKey())
		if err != nil {
			s.log.Error("cannot insert membership event: %v", err)
			// nolint // interface doesn't allow to return error
			tx.Rollback()
			return
		}
	} else {
		_, err := tx.Exec(del, evt.RoomID, evt.GetStateKey())
		if err != nil {
			s.log.Error("cannot delete membership event: %v", err)
			// nolint // interface doesn't allow to return error
			tx.Rollback()
			return
		}
	}

	commitErr := tx.Commit()
	if commitErr != nil {
		s.log.Error("cannot commit transaction: %v", commitErr)
		// nolint // interface doesn't allow to return error
		tx.Rollback()
	}
}

// GetRoomMembers ...
func (s *Store) GetRoomMembers(roomID id.RoomID) []id.UserID {
	s.log.Debug("loading room members of %s", roomID)
	query := "SELECT user_id FROM room_members WHERE room_id = $1"
	rows, err := s.db.Query(query, roomID)
	users := make([]id.UserID, 0)
	if err != nil {
		s.log.Error("cannot load room members: %v", err)
		return users
	}
	defer rows.Close()

	var userID id.UserID
	for rows.Next() {
		if err := rows.Scan(&userID); err == nil {
			users = append(users, userID)
		}
	}
	return users
}

// SaveSession to DB
func (s *Store) SaveSession(userID id.UserID, deviceID id.DeviceID, accessToken string) {
	s.log.Debug("saving session credentials of %s/%s", userID, deviceID)
	tx, err := s.db.Begin()
	if err != nil {
		s.log.Error("cannot begin transaction: %v", err)
		return
	}

	var insert string
	switch s.dialect {
	case "sqlite3":
		insert = "INSERT OR IGNORE INTO session VALUES (?, ?, ?)"
	case "postgres":
		insert = "INSERT INTO session VALUES ($1, $2, $3) ON CONFLICT DO NOTHING"
	}
	update := "UPDATE session SET access_token = $1, device_id = $2 WHERE user_id = $3"

	if _, err = tx.Exec(update, accessToken, deviceID, userID); err != nil {
		s.log.Error("cannot update session credentials: %v", err)
		// nolint // no need to check error here
		tx.Rollback()
		return
	}

	if _, err = tx.Exec(insert, userID, deviceID, accessToken); err != nil {
		s.log.Error("cannot insert session credentials: %v", err)
		// nolint // no need to check error here
		tx.Rollback()
		return
	}

	err = tx.Commit()
	if err != nil {
		s.log.Error("cannot commit transaction: %v", err)
	}
}

// LoadSession from DB (user ID, device ID, access token)
func (s *Store) LoadSession() (id.UserID, id.DeviceID, string) {
	s.log.Debug("loading session credentials...")
	row := s.db.QueryRow("SELECT * FROM session LIMIT 1")
	var userID id.UserID
	var deviceID id.DeviceID
	var accessToken string
	if err := row.Scan(&userID, &deviceID, &accessToken); err != nil {
		s.log.Error("cannot load session credentials: %v", err)
		return "", "", ""
	}
	return userID, deviceID, accessToken
}

func (s *Store) shouldIgnoreMembership(membership event.Membership) bool {
	for _, mtype := range acceptedMembershipTypes {
		if membership == mtype {
			return false
		}
	}

	return true
}
