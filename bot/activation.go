package bot

import (
	"context"
	"fmt"

	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

type activationFlow func(context.Context, id.UserID, id.RoomID, string) bool

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
func (b *Bot) ActivateMailbox(ctx context.Context, ownerID id.UserID, roomID id.RoomID, mailbox string) bool {
	flow := b.getActivationFlow()
	return flow(ctx, ownerID, roomID, mailbox)
}

func (b *Bot) activateNone(_ context.Context, ownerID id.UserID, roomID id.RoomID, mailbox string) bool {
	b.log.Debug().Str("mailbox", mailbox).Str("roomID", roomID.String()).Str("ownerID", ownerID.String()).Msg("activating mailbox through the flow 'none'")
	b.rooms.Store(mailbox, roomID)

	return true
}

func (b *Bot) activateNotify(ctx context.Context, ownerID id.UserID, roomID id.RoomID, mailbox string) bool {
	b.log.Debug().Str("mailbox", mailbox).Str("roomID", roomID.String()).Str("ownerID", ownerID.String()).Msg("activating mailbox through the flow 'notify'")
	b.rooms.Store(mailbox, roomID)
	if len(b.adminRooms) == 0 {
		return true
	}

	msg := fmt.Sprintf("Mailbox %q has been registered by %q for the room %q", mailbox, ownerID, roomID)
	for _, adminRoom := range b.adminRooms {
		content := format.RenderMarkdown(msg, true, true)
		_, err := b.lp.Send(ctx, adminRoom, &content)
		if err != nil {
			b.log.Info().Str("adminRoom", adminRoom.String()).Msg("cannot send mailbox activation notification to the admin room")
			continue
		}
		break
	}
	return true
}
