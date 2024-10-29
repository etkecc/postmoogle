package config

import (
	"context"

	"github.com/etkecc/go-linkpearl"
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/id"

	"github.com/etkecc/postmoogle/internal/utils"
)

// Manager of configs
type Manager struct {
	mu            utils.Mutex
	dkimPrivKey   string
	dkimSignature string
	log           *zerolog.Logger
	lp            *linkpearl.Linkpearl
}

// New config manager
func New(lp *linkpearl.Linkpearl, log *zerolog.Logger, dkimPrivKey, dkimSignature string) *Manager {
	m := &Manager{
		mu:            utils.NewMutex(),
		lp:            lp,
		log:           log,
		dkimPrivKey:   dkimPrivKey,
		dkimSignature: dkimSignature,
	}

	return m
}

// GetBot config
func (m *Manager) GetBot(ctx context.Context) Bot {
	var err error
	var config Bot
	config, err = m.lp.GetAccountData(ctx, acBotKey)
	if err != nil {
		m.log.Error().Err(err).Msg("cannot get bot settings")
	}
	if config == nil {
		config = make(Bot, 0)
	}

	if config.DKIMPrivateKey() == "" {
		config.Set(BotDKIMPrivateKey, m.dkimPrivKey)
		config.Set(BotDKIMSignature, m.dkimSignature)
	}

	return config
}

// SetBot config
func (m *Manager) SetBot(ctx context.Context, cfg Bot) error {
	return m.lp.SetAccountData(ctx, acBotKey, cfg)
}

// GetRoom config
func (m *Manager) GetRoom(ctx context.Context, roomID id.RoomID) (Room, error) {
	config, err := m.lp.GetRoomAccountData(ctx, roomID, acRoomKey)
	if err != nil {
		m.log.Warn().Err(err).Str("room_id", roomID.String()).Msg("cannot get room settings")
	}
	if config == nil {
		config = make(Room, 0)
	}

	return config, err
}

// SetRoom config
func (m *Manager) SetRoom(ctx context.Context, roomID id.RoomID, cfg Room) error {
	return m.lp.SetRoomAccountData(ctx, roomID, acRoomKey, cfg)
}

// GetBanlist config
func (m *Manager) GetBanlist(ctx context.Context) List {
	if !m.GetBot(ctx).BanlistEnabled() {
		return make(List, 0)
	}

	m.mu.Lock("banlist")
	defer m.mu.Unlock("banlist")
	config, err := m.lp.GetAccountData(ctx, acBanlistKey)
	if err != nil {
		m.log.Error().Err(err).Msg("cannot get banlist")
	}
	if config == nil {
		config = make(List, 0)
		return config
	}
	return config
}

// SetBanlist config
func (m *Manager) SetBanlist(ctx context.Context, cfg List) error {
	if !m.GetBot(ctx).BanlistEnabled() {
		return nil
	}

	m.mu.Lock("banlist")
	defer m.mu.Unlock("banlist")
	if cfg == nil {
		cfg = make(List, 0)
	}

	return m.lp.SetAccountData(ctx, acBanlistKey, cfg)
}

// GetGreylist config
func (m *Manager) GetGreylist(ctx context.Context) List {
	config, err := m.lp.GetAccountData(ctx, acGreylistKey)
	if err != nil {
		m.log.Error().Err(err).Msg("cannot get banlist")
	}
	if config == nil {
		config = make(List, 0)
		return config
	}

	return config
}

// SetGreylist config
func (m *Manager) SetGreylist(ctx context.Context, cfg List) error {
	return m.lp.SetAccountData(ctx, acGreylistKey, cfg)
}
