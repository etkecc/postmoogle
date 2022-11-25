package config

import (
	"net"
	"sort"
	"time"
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

func (l List) getKey(addr net.Addr) string {
	key := addr.String()
	host, _, _ := net.SplitHostPort(key) //nolint:errcheck // either way it's ok
	if host != "" {
		key = host
	}
	return key
}

// Has addr in ban- or greylist
func (l List) Has(addr net.Addr) bool {
	_, ok := l[l.getKey(addr)]
	return ok
}

// Get when addr was added in ban- or greylist
func (l List) Get(addr net.Addr) (time.Time, bool) {
	from := l[l.getKey(addr)]
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
	key := l.getKey(addr)
	if _, ok := l[key]; ok {
		return
	}

	l[key] = time.Now().UTC().Format(time.RFC1123Z)
}

// Remove an addr from ban- or greylist
func (l List) Remove(addr net.Addr) {
	key := l.getKey(addr)
	if _, ok := l[key]; !ok {
		return
	}

	delete(l, key)
}
