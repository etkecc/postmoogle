package utils

import (
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// RelatesTo block of matrix event content
func RelatesTo(noThreads bool, parentID id.EventID) *event.RelatesTo {
	if parentID == "" {
		return nil
	}

	if noThreads {
		return &event.RelatesTo{
			InReplyTo: &event.InReplyTo{
				EventID: parentID,
			},
		}
	}

	return &event.RelatesTo{
		Type:    event.RelThread,
		EventID: parentID,
	}
}
