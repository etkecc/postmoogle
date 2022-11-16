package store

import (
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// NOTE: functions in that file are for mautrix.Storer implementation
// ref: https://pkg.go.dev/maunium.net/go/mautrix#Storer

// SaveFilterID to DB
func (s *Store) SaveFilterID(userID id.UserID, filterID string) {
	s.log.Debug("saving filter ID %q for %q", filterID, userID)
	tx, err := s.db.Begin()
	if err != nil {
		s.log.Error("cannot begin transaction: %v", err)
		return
	}

	var insert string
	switch s.dialect {
	case "sqlite3":
		insert = "INSERT OR IGNORE INTO user_filter_ids VALUES (?, ?)"
	case "postgres":
		insert = "INSERT INTO user_filter_ids VALUES ($1, $2) ON CONFLICT DO NOTHING"
	}
	update := "UPDATE user_filter_ids SET filter_id = $1 WHERE user_id = $2"

	_, updateErr := tx.Exec(update, filterID, userID)
	if updateErr != nil {
		s.log.Error("cannot update filter ID: %v", updateErr)
		// nolint // no need to check error here
		tx.Rollback()
		return
	}

	_, insertErr := tx.Exec(insert, userID, filterID)
	if insertErr != nil {
		s.log.Error("cannot create filter ID: %v", insertErr)
		// nolint // no need to check error here
		tx.Rollback()
		return
	}

	commitErr := tx.Commit()
	if commitErr != nil {
		s.log.Error("cannot upsert filter ID: %v", commitErr)
		// nolint // no need to check error here
		tx.Rollback()
	}
}

// LoadFilterID from DB
func (s *Store) LoadFilterID(userID id.UserID) string {
	s.log.Debug("loading filter ID for %q", userID)
	query := "SELECT filter_id FROM user_filter_ids WHERE user_id = $1"
	row := s.db.QueryRow(query, userID)
	var filterID string
	if err := row.Scan(&filterID); err != nil {
		s.log.Error("cannot load filter ID: %q", err)
		return ""
	}
	return filterID
}

// SaveNextBatch to DB
func (s *Store) SaveNextBatch(userID id.UserID, nextBatchToken string) {
	s.log.Debug("saving next batch token for %q", userID)
	tx, err := s.db.Begin()
	if err != nil {
		s.log.Error("cannot begin transaction: %v", err)
		return
	}

	var insert string
	switch s.dialect {
	case "sqlite3":
		insert = "INSERT OR IGNORE INTO user_batch_tokens VALUES (?, ?)"
	case "postgres":
		insert = "INSERT INTO user_batch_tokens VALUES ($1, $2) ON CONFLICT DO NOTHING"
	}
	update := "UPDATE user_batch_tokens SET next_batch_token = $1 WHERE user_id = $2"

	if _, err := tx.Exec(update, nextBatchToken, userID); err != nil {
		s.log.Error("cannot update next batch token: %v", err)
		// nolint // no need to check error here
		tx.Rollback()
		return
	}

	if _, err := tx.Exec(insert, userID, nextBatchToken); err != nil {
		s.log.Error("cannot insert next batch token: %v", err)
		// nolint // no need to check error here
		tx.Rollback()
		return
	}

	commitErr := tx.Commit()
	if commitErr != nil {
		s.log.Error("cannot commit transaction: %v", commitErr)
	}
}

// LoadNextBatch from DB
func (s *Store) LoadNextBatch(userID id.UserID) string {
	s.log.Debug("loading next batch token for %q", userID)
	query := "SELECT next_batch_token FROM user_batch_tokens WHERE user_id = $1"
	row := s.db.QueryRow(query, userID)
	var batchToken string
	if err := row.Scan(&batchToken); err != nil {
		s.log.Error("cannot load next batch token: %v", err)
		return ""
	}
	return batchToken
}

// SaveRoom to DB, not implemented
func (s *Store) SaveRoom(room *mautrix.Room) {
	s.log.Debug("saving room %q (stub, not implemented)", room.ID)
}

// LoadRoom from DB, not implemented
func (s *Store) LoadRoom(roomID id.RoomID) *mautrix.Room {
	s.log.Debug("loading room %q (stub, not implemented)", roomID)
	return mautrix.NewRoom(roomID)
}
