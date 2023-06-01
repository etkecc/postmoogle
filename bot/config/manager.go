package config

import (
	"github.com/rs/zerolog"
	"gitlab.com/etke.cc/linkpearl"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// Manager of configs
type Manager struct {
	mu  utils.Mutex
	log *zerolog.Logger
	lp  *linkpearl.Linkpearl
}

// New config manager
func New(lp *linkpearl.Linkpearl, log *zerolog.Logger) *Manager {
	m := &Manager{
		mu:  utils.NewMutex(),
		lp:  lp,
		log: log,
	}

	return m
}

// GetBot config
func (m *Manager) GetBot() Bot {
	var err error
	var config Bot
	config, err = m.lp.GetAccountData(acBotKey)
	if err != nil {
		m.log.Error().Err(utils.UnwrapError(err)).Msg("cannot get bot settings")
	}
	if config == nil {
		config = make(Bot, 0)
		return config
	}

	return config
}

// SetBot config
func (m *Manager) SetBot(cfg Bot) error {
	return utils.UnwrapError(m.lp.SetAccountData(acBotKey, cfg))
}

// GetRoom config
func (m *Manager) GetRoom(roomID id.RoomID) (Room, error) {
	config, err := m.lp.GetRoomAccountData(roomID, acRoomKey)
	if config == nil {
		config = make(Room, 0)
	}

	return config, utils.UnwrapError(err)
}

// SetRoom config
func (m *Manager) SetRoom(roomID id.RoomID, cfg Room) error {
	return utils.UnwrapError(m.lp.SetRoomAccountData(roomID, acRoomKey, cfg))
}

// GetBanlist config
func (m *Manager) GetBanlist() List {
	if !m.GetBot().BanlistEnabled() {
		return make(List, 0)
	}

	m.mu.Lock("banlist")
	defer m.mu.Unlock("banlist")
	config, err := m.lp.GetAccountData(acBanlistKey)
	if err != nil {
		m.log.Error().Err(utils.UnwrapError(err)).Msg("cannot get banlist")
	}
	if config == nil {
		config = make(List, 0)
		return config
	}
	return config
}

// SetBanlist config
func (m *Manager) SetBanlist(cfg List) error {
	if !m.GetBot().BanlistEnabled() {
		return nil
	}

	m.mu.Lock("banlist")
	defer m.mu.Unlock("banlist")
	if cfg == nil {
		cfg = make(List, 0)
	}

	return utils.UnwrapError(m.lp.SetAccountData(acBanlistKey, cfg))
}

// GetGreylist config
func (m *Manager) GetGreylist() List {
	config, err := m.lp.GetAccountData(acGreylistKey)
	if err != nil {
		m.log.Error().Err(utils.UnwrapError(err)).Msg("cannot get banlist")
	}
	if config == nil {
		config = make(List, 0)
		return config
	}

	return config
}

// SetGreylist config
func (m *Manager) SetGreylist(cfg List) error {
	return utils.UnwrapError(m.lp.SetAccountData(acGreylistKey, cfg))
}
