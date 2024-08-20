package config

import (
	"net"
	"sort"
	"time"

	"github.com/etkecc/postmoogle/internal/utils"
)

// account data keys
const (
	acBanlistKey  = "cc.etke.postmoogle.banlist"
	acGreylistKey = "cc.etke.postmoogle.greylist"
)

// List config
type List map[string]string

// Slice returns slice of ban- or greylist items
func (l List) Slice() []string {
	slice := make([]string, 0, len(l))
	for item := range l {
		slice = append(slice, item)
	}
	sort.Strings(slice)

	return slice
}

// Has addr in ban- or greylist
func (l List) Has(addr net.Addr) bool {
	_, ok := l[utils.AddrIP(addr)]
	return ok
}

// Get when addr was added in ban- or greylist
func (l List) Get(addr net.Addr) (time.Time, bool) {
	from := l[utils.AddrIP(addr)]
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
func (l List) Add(addr net.Addr) {
	key := utils.AddrIP(addr)
	if _, ok := l[key]; ok {
		return
	}

	l[key] = time.Now().UTC().Format(time.RFC1123Z)
}

// Remove an addr from ban- or greylist
func (l List) Remove(addr net.Addr) {
	key := utils.AddrIP(addr)
	if _, ok := l[key]; !ok {
		return
	}

	delete(l, key)
}
