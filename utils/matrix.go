package utils

import (
	"maunium.net/go/mautrix"
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

// EventParent returns parent event - either thread ID or reply-to ID
func EventParent(currentID id.EventID, content *event.MessageEventContent) id.EventID {
	if content == nil {
		return currentID
	}

	if content.GetRelatesTo() == nil {
		return currentID
	}

	threadParent := content.RelatesTo.GetThreadParent()
	if threadParent != "" {
		return threadParent
	}

	replyParent := content.RelatesTo.GetReplyTo()
	if replyParent != "" {
		return replyParent
	}

	return currentID
}

// EventField returns field value from raw event content
func EventField[T comparable](content *event.Content, field string) T {
	var zero T
	raw := content.Raw[field]
	if raw == nil {
		return zero
	}

	v, ok := raw.(T)
	if !ok {
		return zero
	}

	return v
}

// UnwrapError tries to unwrap a error into something meaningful, like mautrix.HTTPError or mautrix.RespError
func UnwrapError(err error) error {
	switch err.(type) {
	case nil:
		return nil
	case mautrix.HTTPError:
		return unwrapHTTPError(err)
	default:
		return err
	}
}

func unwrapHTTPError(err error) error {
	httperr, ok := err.(mautrix.HTTPError)
	if !ok {
		return err
	}

	uwerr := httperr.Unwrap()
	if uwerr != nil {
		return uwerr
	}

	return httperr
}
