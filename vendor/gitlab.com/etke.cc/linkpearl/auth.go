package linkpearl

import (
	"errors"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

func (l *Linkpearl) login(username string, password string) error {
	if err := l.restoreSession(); err == nil {
		l.log.Debug("session restored successfully")
		return nil
	}

	l.log.Debug("auth using login and password...")
	_, err := l.api.Login(&mautrix.ReqLogin{
		Type: "m.login.password",
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: username,
		},
		Password:         password,
		StoreCredentials: true,
	})
	if err != nil {
		l.log.Error("cannot authorize using login and password: %v", err)
		return err
	}
	l.store.SaveSession(l.api.UserID, l.api.DeviceID, l.api.AccessToken)

	return nil
}

// restoreSession tries to load previous active session token from db (if any)
func (l *Linkpearl) restoreSession() error {
	l.log.Debug("restoring previous session...")

	userID, deviceID, token := l.store.LoadSession()
	if userID == "" || deviceID == "" || token == "" {
		return errors.New("cannot restore session from db")
	}
	if !l.validateSession(userID, deviceID, token) {
		return errors.New("restored session is invalid")
	}

	l.api.AccessToken = token
	l.api.UserID = userID
	l.api.DeviceID = deviceID
	return nil
}

func (l *Linkpearl) validateSession(userID id.UserID, deviceID id.DeviceID, token string) bool {
	valid := true
	// preserve current values
	currentToken := l.api.AccessToken
	currentUserID := l.api.UserID
	currentDeviceID := l.api.DeviceID
	// set new values
	l.api.AccessToken = token
	l.api.UserID = userID
	l.api.DeviceID = deviceID

	if _, err := l.api.GetOwnPresence(); err != nil {
		l.log.Debug("previous session token was not found or invalid: %v", err)
		valid = false
	}

	// restore original values
	l.api.AccessToken = currentToken
	l.api.UserID = currentUserID
	l.api.DeviceID = currentDeviceID
	return valid
}
