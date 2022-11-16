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
	botOptionCatchAll       = "catch-all"
	botOptionDKIMSignature  = "dkim.pub"
	botOptionDKIMPrivateKey = "dkim.pem"
	botOptionQueueBatch     = "queue:batch"
	botOptionQueueRetries   = "queue:retries"
	botOptionBanlistEnabled = "banlist:enabled"
	botOptionGreylist       = "greylist"
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

// CatchAll option
func (s botSettings) CatchAll() string {
	return s.Get(botOptionCatchAll)
}

// BanlistEnabled option
func (s botSettings) BanlistEnabled() bool {
	return utils.Bool(s.Get(botOptionBanlistEnabled))
}

// Greylist option (duration in minutes)
func (s botSettings) Greylist() int {
	return utils.Int(s.Get(botOptionGreylist))
}

// DKIMSignature (DNS TXT record)
func (s botSettings) DKIMSignature() string {
	return s.Get(botOptionDKIMSignature)
}

// DKIMPrivateKey keep it secret
func (s botSettings) DKIMPrivateKey() string {
	return s.Get(botOptionDKIMPrivateKey)
}

// QueueBatch option
func (s botSettings) QueueBatch() int {
	return utils.Int(s.Get(botOptionQueueBatch))
}

// QueueRetries option
func (s botSettings) QueueRetries() int {
	return utils.Int(s.Get(botOptionQueueRetries))
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
	config, err := b.lp.GetAccountData(acBotSettingsKey)
	if err != nil {
		b.log.Error("cannot get bot settings: %v", utils.UnwrapError(err))
	}
	if config == nil {
		config = map[string]string{}
	}

	return config
}

func (b *Bot) setBotSettings(cfg botSettings) error {
	return utils.UnwrapError(b.lp.SetAccountData(acBotSettingsKey, cfg))
}
