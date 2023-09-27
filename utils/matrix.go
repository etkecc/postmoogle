package utils

import (
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// RelatesTo returns relation object of a matrix event (either threads or reply-to)
func RelatesTo(threads bool, parentID id.EventID) *event.RelatesTo {
	if parentID == "" {
		return nil
	}

	if threads {
		return &event.RelatesTo{
			Type:    event.RelThread,
			EventID: parentID,
		}
	}

	return &event.RelatesTo{
		InReplyTo: &event.InReplyTo{
			EventID: parentID,
		},
	}
}
