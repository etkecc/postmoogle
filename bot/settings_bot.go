package bot

import (
	"strings"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// account data key
const acBotSettingsKey = "cc.etke.postmoogle.config"

// bot options keys
const (
	botOptionUsers          = "users"
	botOptionDKIMSignature  = "dkim.pub"
	botOptionDKIMPrivateKey = "dkim.pem"
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
	if value == "" {
		return []string{}
	}

	if strings.Contains(value, " ") {
		return strings.Split(value, " ")
	}

	return []string{value}
}

// DKIMSignature (DNS TXT record)
func (s botSettings) DKIMSignature() string {
	return s.Get(botOptionDKIMSignature)
}

// DKIMPrivateKey keep it secret
func (s botSettings) DKIMPrivateKey() string {
	return s.Get(botOptionDKIMPrivateKey)
}

func (b *Bot) initBotUsers() ([]string, error) {
	config := b.getBotSettings()
	cfgUsers := config.Users()
	if len(cfgUsers) > 0 {
		return cfgUsers, nil
	}

	_, homeserver, err := b.lp.GetClient().UserID.Parse()
	if err != nil {
		return nil, err
	}
	config.Set(botOptionUsers, "@*:"+homeserver)
	return config.Users(), b.setBotSettings(config)
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
