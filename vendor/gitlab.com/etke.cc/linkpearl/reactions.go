package linkpearl

import (
	"context"

	"maunium.net/go/mautrix/id"
)

// reactionPrefix is the prefix for all reaction in account data
const reactionPrefix = "cc.etke.linkpearl.reaction."

// SendReaction sends a reaction to a message
func (l *Linkpearl) SendReaction(ctx context.Context, roomID id.RoomID, eventID id.EventID, reaction string) error {
	// Check if the reaction already exists
	if l.getReactionAD(ctx, roomID, eventID, reaction) != "" {
		return nil
	}

	resp, err := l.GetClient().SendReaction(ctx, roomID, eventID, reaction)
	if err != nil {
		return err
	}
	return l.updateReactionsAD(ctx, roomID, eventID, reaction, resp.EventID)
}

// RedactReaction redacts a reaction from a message
func (l *Linkpearl) RedactReaction(ctx context.Context, roomID id.RoomID, eventID id.EventID, reaction string) error {
	existingID := l.getReactionAD(ctx, roomID, eventID, reaction)
	// Check if the reaction already exists
	if existingID == "" {
		return nil
	}
	if _, err := l.GetClient().RedactEvent(ctx, roomID, id.EventID(existingID)); err != nil {
		return err
	}

	return l.updateReactionsAD(ctx, roomID, eventID, reaction, "")
}

// ReplaceReaction replaces a reaction with another
func (l *Linkpearl) ReplaceReaction(ctx context.Context, roomID id.RoomID, eventID id.EventID, oldReaction, newReaction string) error {
	if err := l.RedactReaction(ctx, roomID, eventID, oldReaction); err != nil {
		return err
	}
	return l.SendReaction(ctx, roomID, eventID, newReaction)
}

func (l *Linkpearl) getReactionAD(ctx context.Context, roomID id.RoomID, eventID id.EventID, reaction string) string {
	adID := reactionPrefix + eventID.String()
	existing, err := l.GetRoomAccountData(ctx, roomID, adID)
	if err != nil {
		l.log.Error().Err(err).Msg("failed to get existing reactions")
		return ""
	}
	return existing[reaction]
}

func (l *Linkpearl) updateReactionsAD(ctx context.Context, roomID id.RoomID, eventID id.EventID, reaction string, reactionID id.EventID) error {
	adID := reactionPrefix + eventID.String()
	existing, err := l.GetRoomAccountData(ctx, roomID, adID)
	if err != nil {
		return err
	}

	if reactionID == "" {
		delete(existing, reaction)
	} else {
		existing[reaction] = reactionID.String()
	}
	return l.SetRoomAccountData(ctx, roomID, adID, existing)
}
