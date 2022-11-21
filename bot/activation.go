package bot

import (
	"fmt"

	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

type activationFlow func(id.UserID, id.RoomID, string) bool

func (b *Bot) getActivationFlow() activationFlow {
	switch b.mbxc.Activation {
	case "none":
		return b.activateNone
	case "notify":
		return b.activateNotify
	default:
		return b.activateNone
	}
}

// ActivateMailbox using the configured flow
func (b *Bot) ActivateMailbox(ownerID id.UserID, roomID id.RoomID, mailbox string) bool {
	flow := b.getActivationFlow()
	return flow(ownerID, roomID, mailbox)
}

func (b *Bot) activateNone(ownerID id.UserID, roomID id.RoomID, mailbox string) bool {
	b.log.Debug("activating mailbox %q (%q) of %q through flow 'none'", mailbox, roomID, ownerID)
	b.rooms.Store(mailbox, roomID)

	return true
}

func (b *Bot) activateNotify(ownerID id.UserID, roomID id.RoomID, mailbox string) bool {
	b.log.Debug("activating mailbox %q (%q) of %q through flow 'notify'", mailbox, roomID, ownerID)
	b.rooms.Store(mailbox, roomID)
	if len(b.adminRooms) == 0 {
		return true
	}

	msg := fmt.Sprintf("Mailbox %q has been registered by %q for the room %q", mailbox, ownerID, roomID)
	for _, adminRoom := range b.adminRooms {
		content := format.RenderMarkdown(msg, true, true)
		_, err := b.lp.Send(adminRoom, &content)
		if err != nil {
			b.log.Info("cannot send mailbox activation notification to the admin room %q", adminRoom)
			continue
		}
		break
	}
	return true
}
