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
