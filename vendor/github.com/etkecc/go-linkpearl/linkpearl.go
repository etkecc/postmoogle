// Package linkpearl represents the library itself
package linkpearl

import (
	"context"
	"database/sql"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"
	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const (
	// DefaultMaxRetries for operations like autojoin
	DefaultMaxRetries = 10
	// DefaultAccountDataCache size
	DefaultAccountDataCache = 1000
	// DefaultEventsLimit for methods like lp.Threads() and lp.FindEventBy()
	DefaultEventsLimit = 1000
	// DefaultTypingTimeout in seconds for typing notifications
	DefaultTypingTimeout = 60
	// DefaultUserAgent for HTTP requests
	DefaultUserAgent = "Linkpearl (library; +https://gitlab.com/etke.cc/linkpearl)"
)

// Linkpearl object
type Linkpearl struct {
	db  *sql.DB
	ch  *cryptohelper.CryptoHelper
	acc *lru.Cache[string, map[string]string]
	acr *Crypter
	log zerolog.Logger
	api *mautrix.Client

	joinPermit  func(ctx context.Context, evt *event.Event) bool
	autoleave   bool
	maxretries  int
	eventsLimit int
}

type ReqPresence struct {
	Presence  event.Presence `json:"presence"`
	StatusMsg string         `json:"status_msg,omitempty"`
}

func setDefaults(cfg *Config) {
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = DefaultMaxRetries
	}
	if cfg.AccountDataCache == 0 {
		cfg.AccountDataCache = DefaultAccountDataCache
	}
	if cfg.EventsLimit == 0 {
		cfg.EventsLimit = DefaultEventsLimit
	}
	if cfg.JoinPermit == nil {
		// By default, we approve all join requests
		cfg.JoinPermit = func(_ context.Context, _ *event.Event) bool { return true }
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
}

func initCrypter(secret string) (*Crypter, error) {
	if secret == "" {
		return nil, nil
	}

	return NewCrypter(secret)
}

// New linkpearl
func New(cfg *Config) (*Linkpearl, error) {
	setDefaults(cfg)
	mautrix.DefaultUserAgent = cfg.UserAgent
	api, err := mautrix.NewClient(cfg.Homeserver, "", "")
	if err != nil {
		return nil, err
	}
	api.Log = cfg.Logger

	acc, _ := lru.New[string, map[string]string](cfg.AccountDataCache) //nolint:errcheck // addressed in setDefaults()
	acr, err := initCrypter(cfg.AccountDataSecret)
	if err != nil {
		return nil, err
	}

	lp := &Linkpearl{
		db:          cfg.DB,
		acc:         acc,
		acr:         acr,
		api:         api,
		log:         cfg.Logger,
		joinPermit:  cfg.JoinPermit,
		autoleave:   cfg.AutoLeave,
		maxretries:  cfg.MaxRetries,
		eventsLimit: cfg.EventsLimit,
	}

	db, err := dbutil.NewWithDB(cfg.DB, cfg.Dialect)
	if err != nil {
		return nil, err
	}
	db.Log = dbutil.ZeroLogger(cfg.Logger)
	localpart, _, mxidErr := id.UserID(cfg.Login).Parse()
	if mxidErr != nil {
		localpart = cfg.Login
	}
	lp.ch, err = cryptohelper.NewCryptoHelper(lp.api, []byte(localpart), db)
	if err != nil {
		return nil, err
	}
	lp.ch.LoginAs = cfg.LoginAs()
	if err := lp.ch.Init(context.Background()); err != nil {
		return nil, err
	}
	lp.api.Crypto = lp.ch
	return lp, nil
}

// GetClient returns underlying API client
func (l *Linkpearl) GetClient() *mautrix.Client {
	return l.api
}

// GetDB returns underlying DB object
func (l *Linkpearl) GetDB() *sql.DB {
	return l.db
}

// GetMachine returns underlying OLM machine
func (l *Linkpearl) GetMachine() *crypto.OlmMachine {
	return l.ch.Machine()
}

// GetAccountDataCrypter returns crypter used for account data (if any)
func (l *Linkpearl) GetAccountDataCrypter() *Crypter {
	return l.acr
}

// SetJoinPermit sets the join permit callback function
func (l *Linkpearl) SetJoinPermit(value func(context.Context, *event.Event) bool) {
	l.joinPermit = value
}

// Start performs matrix /sync
func (l *Linkpearl) Start(ctx context.Context, optionalStatusMsg ...string) error {
	l.initSync()
	var statusMsg string
	if len(optionalStatusMsg) > 0 {
		statusMsg = optionalStatusMsg[0]
	}

	err := l.api.SetPresence(ctx, mautrix.ReqPresence{Presence: event.PresenceOnline, StatusMsg: statusMsg})
	if err != nil {
		l.log.Error().Err(err).Msg("cannot set presence")
	}
	defer l.Stop(ctx)

	l.log.Info().Msg("client has been started")
	return l.api.SyncWithContext(ctx)
}

// Stop the client
func (l *Linkpearl) Stop(ctx context.Context) {
	l.log.Debug().Msg("stopping the client")
	if err := l.api.SetPresence(ctx, mautrix.ReqPresence{Presence: event.PresenceOffline}); err != nil {
		l.log.Error().Err(err).Msg("cannot set presence")
	}
	l.api.StopSync()
	if err := l.ch.Close(); err != nil {
		l.log.Error().Err(err).Msg("cannot close crypto helper")
	}
	l.log.Info().Msg("client has been stopped")
}
