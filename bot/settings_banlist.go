package bot

import (
	"net"
	"time"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// account data key
const acBanlistKey = "cc.etke.postmoogle.banlist"

type banList map[string]string

// Slice returns slice of banlist items
func (b banList) Slice() []string {
	slice := make([]string, 0, len(b))
	for item := range b {
		slice = append(slice, item)
	}

	return slice
}

func (b banList) getKey(addr net.Addr) string {
	key := addr.String()
	host, _, _ := net.SplitHostPort(key) //nolint:errcheck // either way it's ok
	if host != "" {
		key = host
	}
	return key
}

// Has addr in banlist
func (b banList) Has(addr net.Addr) bool {
	_, ok := b[b.getKey(addr)]
	return ok
}

// Add an addr to banlist
func (b banList) Add(addr net.Addr) {
	key := b.getKey(addr)
	if _, ok := b[key]; ok {
		return
	}

	b[key] = time.Now().UTC().Format(time.RFC1123Z)
}

// Remove an addr from banlist
func (b banList) Remove(addr net.Addr) {
	key := b.getKey(addr)
	if _, ok := b[key]; !ok {
		return
	}

	delete(b, key)
}

func (b *Bot) getBanlist() banList {
	config, err := b.lp.GetAccountData(acBanlistKey)
	if err != nil {
		b.log.Error("cannot get banlist: %v", utils.UnwrapError(err))
	}
	if config == nil {
		config = map[string]string{}
	}

	return config
}

func (b *Bot) setBanlist(cfg banList) error {
	return utils.UnwrapError(b.lp.SetAccountData(acBanlistKey, cfg))
}
