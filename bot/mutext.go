package bot

import (
	"sync"

	"maunium.net/go/mautrix/id"
)

func (b *Bot) lock(roomID id.RoomID) {
	_, ok := b.mu[roomID]
	if !ok {
		b.mu[roomID] = &sync.Mutex{}
	}

	b.mu[roomID].Lock()
}

func (b *Bot) unlock(roomID id.RoomID) {
	_, ok := b.mu[roomID]
	if !ok {
		return
	}

	b.mu[roomID].Unlock()
	delete(b.mu, roomID)
}
