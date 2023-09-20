package config

import "time"

// Config of Postmoogle
type Config struct {
	// Homeserver url
	Homeserver string
	// Login is a localpart if logging in with password (postmoogle) OR full MXID if logging in with shared secret (@postmoogle:example.com)
	Login string
	// Password for login/password auth only
	Password string
	// SharedSecret for login/sharedsecret auth only
	SharedSecret string
	// Domains for SMTP
	Domains []string
	// Port for SMTP
	Port string
	// Proxies is list of trusted SMTP proxies
	Proxies []string
	// RoomID of the admin room
	LogLevel string
	// DataSecret is account data secret key (password) to encrypt all account data values
	DataSecret string
	// NoEncryption disabled encryption support
	NoEncryption bool
	// Prefix for commands
	Prefix string
	// MaxSize of an email (including attachments)
	MaxSize int
	// StatusMsg of the bot
	StatusMsg string
	// Mailboxes config
	Mailboxes Mailboxes
	// Admins holds list of admin users (wildcards supported), e.g.: @*:example.com, @bot.*:example.com, @admin:*. Empty = no admins
	Admins []string

	// DB config
	DB DB

	// TLS config
	TLS TLS

	// Monitoring config
	Monitoring Monitoring

	Relay Relay
}

// DB config
type DB struct {
	// DSN is a database connection string
	DSN string
	// Dialect of database, one of sqlite3, postgres
	Dialect string
}

// TLS config
type TLS struct {
	Certs    []string
	Keys     []string
	Port     string
	Required bool
}

// Monitoring config
type Monitoring struct {
	SentryDSN          string
	SentrySampleRate   int
	HealchecksUUID     string
	HealthechsDuration time.Duration
}

// Mailboxes config
type Mailboxes struct {
	Reserved   []string
	Forwarded  []string
	Activation string
}

// Relay config
type Relay struct {
	Host     string
	Port     string
	Username string
	Password string
}
