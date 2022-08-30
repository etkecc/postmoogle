package bot

import (
	"strings"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// account data key
const acBotSettingsKey = "cc.etke.postmoogle.config"

// bot options keys
const (
	botOptionUsers = "users"
)

type botSettings map[string]string

// Get option
func (s botSettings) Get(key string) string {
	return s[strings.ToLower(strings.TrimSpace(key))]
}

// Set option
func (s botSettings) Set(key, value string) {
	s[strings.ToLower(strings.TrimSpace(key))] = value
}

// Users option
func (s botSettings) Users() []string {
	value := s.Get(botOptionUsers)
	if strings.Contains(value, " ") {
		return strings.Split(value, " ")
	}
	return []string{}
}

// TODO: remove after migration
func (b *Bot) migrateBotSettings(users []string) error {
	config := b.getBotSettings()
	cfgUsers := config.Users()
	if len(users) > 0 && len(cfgUsers) == 0 {
		_, err := parseMXIDpatterns(users, "")
		if err != nil {
			return err
		}
		config.Set(botOptionUsers, strings.Join(users, " "))
		return b.setBotSettings(config)
	}

	return nil
}

func (b *Bot) getBotSettings() botSettings {
	cfg := b.botcfg.Get(acBotSettingsKey)
	if cfg != nil {
		return cfg
	}

	config := botSettings{}
	err := b.lp.GetClient().GetAccountData(acBotSettingsKey, &config)
	if err != nil {
		if strings.Contains(err.Error(), "M_NOT_FOUND") {
			err = nil
		} else {
			b.log.Error("cannot get bot settings: %v", utils.UnwrapError(err))
		}
	}

	if err == nil {
		b.botcfg.Set(acBotSettingsKey, config)
	}

	return config
}

func (b *Bot) setBotSettings(cfg botSettings) error {
	b.botcfg.Set(acBotSettingsKey, cfg)
	return utils.UnwrapError(b.lp.GetClient().SetAccountData(acBotSettingsKey, cfg))
}
