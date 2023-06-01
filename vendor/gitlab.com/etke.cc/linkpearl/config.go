package linkpearl

import (
	"database/sql"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

// Config represents matrix config
type Config struct {
	// Homeserver url
	Homeserver string
	// Login is a localpart (honoroit - OK, @honoroit:example.com - wrong)
	Login string
	// Password for login/password auth only
	Password string

	// JoinPermit is a callback function that tells
	// if linkpearl should respond to the given "invite" event
	// and join the room
	JoinPermit func(*event.Event) bool

	// AutoLeave if true, linkpearl will automatically leave empty rooms
	AutoLeave bool

	// AccountDataCache size
	AccountDataCache int

	// AccountDataSecret (Password) for encryption
	AccountDataSecret string

	// MaxRetries for operations like auto join
	MaxRetries int

	// Logger
	Logger zerolog.Logger

	// DB object
	DB *sql.DB
	// Dialect of the DB: postgres, sqlite3
	Dialect string
}

// LoginAs for cryptohelper
func (cfg *Config) LoginAs() *mautrix.ReqLogin {
	return &mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: cfg.Login,
		},
		Password:           cfg.Password,
		StoreCredentials:   true,
		StoreHomeserverURL: true,
	}
}
