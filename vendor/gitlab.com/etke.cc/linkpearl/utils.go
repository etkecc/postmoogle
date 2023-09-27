package linkpearl

import (
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// EventParent returns parent event ID (either from thread or from reply-to relation)
func EventParent(currentID id.EventID, content *event.MessageEventContent) id.EventID {
	if content == nil {
		return currentID
	}

	relation := content.OptionalGetRelatesTo()
	if relation == nil {
		return currentID
	}

	threadParent := relation.GetThreadParent()
	if threadParent != "" {
		return threadParent
	}

	replyParent := relation.GetReplyTo()
	if replyParent != "" {
		return replyParent
	}

	return currentID
}

// EventField returns field value from raw event content
func EventField[T any](content *event.Content, field string) T {
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

func ParseContent(evt *event.Event, eventType event.Type, log *zerolog.Logger) {
	if evt.Content.Parsed != nil {
		return
	}
	perr := evt.Content.ParseRaw(eventType)
	if perr != nil {
		log.Error().Err(perr).Msg("cannot parse event content")
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
