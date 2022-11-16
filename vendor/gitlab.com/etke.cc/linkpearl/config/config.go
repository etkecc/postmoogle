// Package config was added to store cross-package structs and interfaces.
package config

import (
	"database/sql"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
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

	// AccountDataLogReplace contains map of field name => value
	// that will be used to replace mentioned account data fields with provided values
	// when printing in logs (DEBUG, TRACE)
	AccountDataLogReplace map[string]string

	// MaxRetries for operations like auto join
	MaxRetries int

	// NoEncryption disabled encryption support
	NoEncryption bool

	// LPLogger used for linkpearl's glue code
	LPLogger Logger
	// APILogger used for matrix CS API calls
	APILogger Logger
	// StoreLogger used for persistent store
	StoreLogger Logger
	// CryptoLogger used for OLM machine
	CryptoLogger Logger

	// DB object
	DB *sql.DB
	// Dialect of the DB: postgres, sqlite3
	Dialect string
}

// Logger implementation of crypto.Logger and mautrix.Logger
type Logger interface {
	crypto.Logger
	mautrix.WarnLogger

	Info(message string, args ...interface{})
}
