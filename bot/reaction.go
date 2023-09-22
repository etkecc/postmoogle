package bot

import (
	"context"

	"maunium.net/go/mautrix/event"

	"gitlab.com/etke.cc/postmoogle/utils"
)

var supportedReactions = map[string]string{
	"⛔️":   commandSpamlistAdd,
	"🛑":    commandSpamlistAdd,
	"🚫":    commandSpamlistAdd,
	"spam": commandSpamlistAdd,
}

func (b *Bot) handleReaction(ctx context.Context) {
	evt := eventFromContext(ctx)
	content := evt.Content.AsReaction()
	action, ok := supportedReactions[content.GetRelatesTo().Key]
	if !ok { // cannot do anything with it
		return
	}

	srcID := content.GetRelatesTo().EventID
	srcEvt, err := b.lp.GetClient().GetEvent(evt.RoomID, srcID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot find event %s: %v", srcID, err)
		return
	}
	utils.ParseContent(evt, event.EventMessage)

	switch action {
	case commandSpamlistAdd:
		sender := utils.EventField[string](&srcEvt.Content, eventFromKey)
		if sender == "" {
			b.Error(ctx, evt.RoomID, "cannot get sender of the email")
			return
		}
		b.runSpamlistAdd(ctx, []string{commandSpamlistAdd, utils.EventField[string](&srcEvt.Content, eventFromKey)})
	}
}
