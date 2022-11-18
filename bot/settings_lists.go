package bot

import (
	"net"
	"sort"
	"time"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// account data keys
const (
	acBanlistKey  = "cc.etke.postmoogle.banlist"
	acGreylistKey = "cc.etke.postmoogle.greylist"
)

type bglist map[string]string

// Slice returns slice of ban- or greylist items
func (b bglist) Slice() []string {
	slice := make([]string, 0, len(b))
	for item := range b {
		slice = append(slice, item)
	}
	sort.Strings(slice)

	return slice
}

func (b bglist) getKey(addr net.Addr) string {
	key := addr.String()
	host, _, _ := net.SplitHostPort(key) //nolint:errcheck // either way it's ok
	if host != "" {
		key = host
	}
	return key
}

// Has addr in ban- or greylist
func (b bglist) Has(addr net.Addr) bool {
	_, ok := b[b.getKey(addr)]
	return ok
}

// Get when addr was added in ban- or greylist
func (b bglist) Get(addr net.Addr) (time.Time, bool) {
	from := b[b.getKey(addr)]
	if from == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC1123Z, from)
	if err != nil {
		return time.Time{}, false
	}

	return t, true
}

// Add an addr to ban- or greylist
func (b bglist) Add(addr net.Addr) {
	key := b.getKey(addr)
	if _, ok := b[key]; ok {
		return
	}

	b[key] = time.Now().UTC().Format(time.RFC1123Z)
}

// Remove an addr from ban- or greylist
func (b bglist) Remove(addr net.Addr) {
	key := b.getKey(addr)
	if _, ok := b[key]; !ok {
		return
	}

	delete(b, key)
}

func (b *Bot) getBanlist() bglist {
	config, err := b.lp.GetAccountData(acBanlistKey)
	if err != nil {
		b.log.Error("cannot get banlist: %v", utils.UnwrapError(err))
	}
	if config == nil {
		config = make(bglist, 0)
	}

	return config
}

func (b *Bot) setBanlist(cfg bglist) error {
	b.lock("banlist")
	if cfg == nil {
		cfg = make(bglist, 0)
	}
	b.banlist = cfg
	defer b.unlock("banlist")

	return utils.UnwrapError(b.lp.SetAccountData(acBanlistKey, cfg))
}

func (b *Bot) getGreylist() bglist {
	config, err := b.lp.GetAccountData(acGreylistKey)
	if err != nil {
		b.log.Error("cannot get banlist: %v", utils.UnwrapError(err))
	}
	if config == nil {
		config = make(bglist, 0)
	}

	return config
}

func (b *Bot) setGreylist(cfg bglist) error {
	return utils.UnwrapError(b.lp.SetAccountData(acGreylistKey, cfg))
}
