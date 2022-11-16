// Package linkpearl represents the library itself
package linkpearl

import (
	"database/sql"

	lru "github.com/hashicorp/golang-lru/v2"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/event"

	"gitlab.com/etke.cc/linkpearl/config"
	"gitlab.com/etke.cc/linkpearl/store"
)

const (
	// DefaultMaxRetries for operations like autojoin
	DefaultMaxRetries = 10
	// DefaultAccountDataCache size
	DefaultAccountDataCache = 1000
)

// Linkpearl object
type Linkpearl struct {
	db    *sql.DB
	acc   *lru.Cache[string, map[string]string]
	acr   *Crypter
	aclr  map[string]string
	log   config.Logger
	api   *mautrix.Client
	olm   *crypto.OlmMachine
	store *store.Store

	joinPermit func(*event.Event) bool
	autoleave  bool
	maxretries int
}

type ReqPresence struct {
	Presence  event.Presence `json:"presence"`
	StatusMsg string         `json:"status_msg,omitempty"`
}

func setDefaults(cfg *config.Config) {
	if cfg.AccountDataLogReplace == nil {
		cfg.AccountDataLogReplace = make(map[string]string)
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = DefaultMaxRetries
	}
	if cfg.AccountDataCache == 0 {
		cfg.AccountDataCache = DefaultAccountDataCache
	}
	if cfg.JoinPermit == nil {
		// By default, we approve all join requests
		cfg.JoinPermit = func(*event.Event) bool { return true }
	}
}

func initCrypter(secret string) (*Crypter, error) {
	if secret == "" {
		return nil, nil
	}

	return NewCrypter(secret)
}

// New linkpearl
func New(cfg *config.Config) (*Linkpearl, error) {
	setDefaults(cfg)
	api, err := mautrix.NewClient(cfg.Homeserver, "", "")
	if err != nil {
		return nil, err
	}
	api.Logger = cfg.APILogger

	acc, _ := lru.New[string, map[string]string](cfg.AccountDataCache) //nolint:errcheck // addressed in setDefaults()
	acr, err := initCrypter(cfg.AccountDataSecret)
	if err != nil {
		return nil, err
	}

	lp := &Linkpearl{
		db:         cfg.DB,
		acc:        acc,
		acr:        acr,
		aclr:       cfg.AccountDataLogReplace,
		api:        api,
		log:        cfg.LPLogger,
		joinPermit: cfg.JoinPermit,
		autoleave:  cfg.AutoLeave,
		maxretries: cfg.MaxRetries,
	}

	storer := store.New(cfg.DB, cfg.Dialect, cfg.StoreLogger)
	if err = storer.CreateTables(); err != nil {
		return nil, err
	}
	lp.store = storer
	lp.api.Store = storer

	if err = lp.login(cfg.Login, cfg.Password); err != nil {
		return nil, err
	}

	if !cfg.NoEncryption {
		if err = lp.store.WithCrypto(lp.api.UserID, lp.api.DeviceID, cfg.StoreLogger); err != nil {
			return nil, err
		}
		lp.olm = crypto.NewOlmMachine(lp.api, cfg.CryptoLogger, lp.store, lp.store)
		if err = lp.olm.Load(); err != nil {
			return nil, err
		}
	}

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

// GetStore returns underlying persistent store object, compatible with crypto.Store, crypto.StateStore and mautrix.Storer
func (l *Linkpearl) GetStore() *store.Store {
	return l.store
}

// GetMachine returns underlying OLM machine
func (l *Linkpearl) GetMachine() *crypto.OlmMachine {
	return l.olm
}

// GetAccountDataCrypter returns crypter used for account data (if any)
func (l *Linkpearl) GetAccountDataCrypter() *Crypter {
	return l.acr
}

// SetPresence (own). See https://spec.matrix.org/v1.3/client-server-api/#put_matrixclientv3presenceuseridstatus
func (l *Linkpearl) SetPresence(presence event.Presence, message string) error {
	req := ReqPresence{Presence: presence, StatusMsg: message}
	u := l.GetClient().BuildClientURL("v3", "presence", l.GetClient().UserID, "status")
	_, err := l.GetClient().MakeRequest("PUT", u, req, nil)

	return err
}

// SetJoinPermit sets the the join permit callback function
func (l *Linkpearl) SetJoinPermit(value func(*event.Event) bool) {
	l.joinPermit = value
}

// Start performs matrix /sync
func (l *Linkpearl) Start(optionalStatusMsg ...string) error {
	l.initSync()
	var statusMsg string
	if len(optionalStatusMsg) > 0 {
		statusMsg = optionalStatusMsg[0]
	}

	err := l.SetPresence(event.PresenceOnline, statusMsg)
	if err != nil {
		l.log.Error("cannot set presence: %v", err)
	}
	defer l.Stop()

	l.log.Info("client has been started")
	return l.api.Sync()
}

// Stop the client
func (l *Linkpearl) Stop() {
	l.log.Debug("stopping the client")
	err := l.api.SetPresence(event.PresenceOffline)
	if err != nil {
		l.log.Error("cannot set presence: %v", err)
	}
	l.api.StopSync()
	l.log.Info("client has been stopped")
}
