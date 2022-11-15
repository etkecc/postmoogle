package bot

import (
	"sync"
)

func (b *Bot) lock(key string) {
	_, ok := b.mu[key]
	if !ok {
		b.mu[key] = &sync.Mutex{}
	}

	b.mu[key].Lock()
}

func (b *Bot) unlock(key string) {
	_, ok := b.mu[key]
	if !ok {
		return
	}

	b.mu[key].Unlock()
	delete(b.mu, key)
}
