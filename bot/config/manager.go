package config

import (
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/linkpearl"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// Manager of configs
type Manager struct {
	bl  List
	ble bool
	mu  utils.Mutex
	log *logger.Logger
	lp  *linkpearl.Linkpearl
}

// New config manager
func New(lp *linkpearl.Linkpearl, log *logger.Logger) *Manager {
	m := &Manager{
		mu:  utils.NewMutex(),
		bl:  make(List, 0),
		lp:  lp,
		log: log,
	}
	m.ble = m.GetBot().BanlistEnabled()

	return m
}

// BanlistEnalbed or not
func (m *Manager) BanlistEnalbed() bool {
	return m.ble
}

// GetBot config
func (m *Manager) GetBot() Bot {
	var err error
	var config Bot
	config, err = m.lp.GetAccountData(acBotKey)
	if err != nil {
		m.log.Error("cannot get bot settings: %v", utils.UnwrapError(err))
	}
	if config == nil {
		config = make(Bot, 0)
		return config
	}
	m.ble = config.BanlistEnabled()

	return config
}

// SetBot config
func (m *Manager) SetBot(cfg Bot) error {
	m.ble = cfg.BanlistEnabled()
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
	if len(m.bl) > 0 || !m.ble {
		return m.bl
	}

	m.mu.Lock("banlist")
	defer m.mu.Unlock("banlist")
	config, err := m.lp.GetAccountData(acBanlistKey)
	if err != nil {
		m.log.Error("cannot get banlist: %v", utils.UnwrapError(err))
	}
	if config == nil {
		config = make(List, 0)
		return config
	}
	m.bl = config
	return config
}

// SetBanlist config
func (m *Manager) SetBanlist(cfg List) error {
	if !m.ble {
		return nil
	}

	m.mu.Lock("banlist")
	if cfg == nil {
		cfg = make(List, 0)
	}
	m.bl = cfg
	defer m.mu.Unlock("banlist")

	return utils.UnwrapError(m.lp.SetAccountData(acBanlistKey, cfg))
}

// GetGreylist config
func (m *Manager) GetGreylist() List {
	config, err := m.lp.GetAccountData(acGreylistKey)
	if err != nil {
		m.log.Error("cannot get banlist: %v", utils.UnwrapError(err))
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
