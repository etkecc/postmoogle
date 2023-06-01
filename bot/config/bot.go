package config

import (
	"strings"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// account data key
const acBotKey = "cc.etke.postmoogle.config"

// bot options keys
const (
	BotAdminRoom           = "adminroom"
	BotUsers               = "users"
	BotCatchAll            = "catch-all"
	BotDKIMSignature       = "dkim.pub"
	BotDKIMPrivateKey      = "dkim.pem"
	BotQueueBatch          = "queue:batch"
	BotQueueRetries        = "queue:retries"
	BotBanlistEnabled      = "banlist:enabled"
	BotGreylist            = "greylist"
	BotMautrix015Migration = "mautrix015migration"
)

// Bot map
type Bot map[string]string

// Get option
func (s Bot) Get(key string) string {
	return s[strings.ToLower(strings.TrimSpace(key))]
}

// Set option
func (s Bot) Set(key, value string) {
	s[strings.ToLower(strings.TrimSpace(key))] = value
}

// Mautrix015Migration option (timestamp)
func (s Bot) Mautrix015Migration() int64 {
	return utils.Int64(s.Get(BotMautrix015Migration))
}

// Users option
func (s Bot) Users() []string {
	value := s.Get(BotUsers)
	if value == "" {
		return []string{}
	}

	if strings.Contains(value, " ") {
		return strings.Split(value, " ")
	}

	return []string{value}
}

// CatchAll option
func (s Bot) CatchAll() string {
	return s.Get(BotCatchAll)
}

// AdminRoom option
func (s Bot) AdminRoom() id.RoomID {
	return id.RoomID(s.Get(BotAdminRoom))
}

// BanlistEnabled option
func (s Bot) BanlistEnabled() bool {
	return utils.Bool(s.Get(BotBanlistEnabled))
}

// Greylist option (duration in minutes)
func (s Bot) Greylist() int {
	return utils.Int(s.Get(BotGreylist))
}

// DKIMSignature (DNS TXT record)
func (s Bot) DKIMSignature() string {
	return s.Get(BotDKIMSignature)
}

// DKIMPrivateKey keep it secret
func (s Bot) DKIMPrivateKey() string {
	return s.Get(BotDKIMPrivateKey)
}

// QueueBatch option
func (s Bot) QueueBatch() int {
	return utils.Int(s.Get(BotQueueBatch))
}

// QueueRetries option
func (s Bot) QueueRetries() int {
	return utils.Int(s.Get(BotQueueRetries))
}
