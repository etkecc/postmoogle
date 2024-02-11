package linkpearl

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"database/sql"
	"encoding/hex"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

// Config represents matrix config
type Config struct {
	// Homeserver url
	Homeserver string
	// Login is a localpart for password auth or full mxid for shared secret auth (honoroit - for password, @honoroit:example.com - for shared secret)
	Login string
	// Password for login/password auth only
	Password string
	// Shared secret for login/sharedsecret auth only
	SharedSecret string

	// JoinPermit is a callback function that tells
	// if linkpearl should respond to the given "invite" event
	// and join the room
	JoinPermit func(context.Context, *event.Event) bool

	// AutoLeave if true, linkpearl will automatically leave empty rooms
	AutoLeave bool

	// AccountDataCache size
	AccountDataCache int

	// AccountDataSecret (Password) for encryption
	AccountDataSecret string

	// MaxRetries for operations like auto join
	MaxRetries int

	// EventsLimit for methods like lp.Threads() or lp.FindEventBy()
	EventsLimit int

	// Logger
	Logger zerolog.Logger

	// DB object
	DB *sql.DB
	// Dialect of the DB: postgres, sqlite3
	Dialect string
}

// LoginAs for cryptohelper
func (cfg *Config) LoginAs() *mautrix.ReqLogin {
	loginReq := mautrix.ReqLogin{
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: cfg.Login,
		},
		StoreCredentials:   true,
		StoreHomeserverURL: true,
	}

	if cfg.SharedSecret != "" {
		loginReq.Type = mautrix.AuthTypeDevtureSharedSecret
		mac := hmac.New(sha512.New, []byte(cfg.SharedSecret))
		mac.Write([]byte(cfg.Login))
		loginReq.Token = hex.EncodeToString(mac.Sum(nil))
	} else {
		loginReq.Type = mautrix.AuthTypePassword
		loginReq.Password = cfg.Password
	}

	return &loginReq
}
