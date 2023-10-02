package bot

import (
	"context"

	"gitlab.com/etke.cc/linkpearl"
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
		b.Error(ctx, "cannot find event %s: %v", srcID, err)
		return
	}
	linkpearl.ParseContent(srcEvt, b.log)
	if b.lp.GetMachine().StateStore.IsEncrypted(evt.RoomID) {
		decrypted, derr := b.lp.GetClient().Crypto.Decrypt(srcEvt)
		if derr == nil {
			srcEvt = decrypted
		}
	}
	threadID := linkpearl.EventParent(srcID, srcEvt.Content.AsMessage())
	ctx = threadIDToContext(ctx, threadID)
	linkpearl.ParseContent(evt, b.log)

	if action == commandSpamlistAdd {
		sender := linkpearl.EventField[string](&srcEvt.Content, eventFromKey)
		if sender == "" {
			b.Error(ctx, "cannot get sender of the email")
			return
		}
		b.runSpamlistAdd(ctx, []string{commandSpamlistAdd, linkpearl.EventField[string](&srcEvt.Content, eventFromKey)})
	}
}
