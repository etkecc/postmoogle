package linkpearl

import (
	"context"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

const (
	// MaxRetries is the maximum number of retries for sending a message
	MaxRetries = 5
	// DefaultTypingTimeout is the step delay for sending message retries
	RetryDelay = 5 * time.Second
)

// SendMessageEvent sends a message event to the roomID and automatically retries if the server returns a 502 or 404 error
func (l *Linkpearl) SendMessageEvent(ctx context.Context, roomID id.RoomID, eventType event.Type, contentJSON any, currentAttempt ...int) (resp *mautrix.RespSendEvent, err error) {
	attempt := 1
	if len(currentAttempt) > 0 {
		attempt = currentAttempt[0]
	}

	resp, err = l.api.SendMessageEvent(ctx, roomID, eventType, contentJSON)
	if err == nil {
		return resp, nil
	}
	err = UnwrapError(err)
	if (strings.Contains(err.Error(), "HTTP 502") || strings.Contains(err.Error(), "HTTP 404")) && attempt <= MaxRetries {
		l.log.Warn().
			Err(err).
			Str("roomID", roomID.String()).
			Int("attempt", attempt).
			Int("of", MaxRetries).
			Msg("cannot send event, sleeping and retrying")
		time.Sleep(RetryDelay * time.Duration(attempt))
		return l.SendMessageEvent(ctx, roomID, eventType, contentJSON, attempt+1)
	}
	return nil, err
}

// Send a message to the roomID and automatically try to encrypt it, if the destination room is encrypted
//
//nolint:unparam // it's public interface
func (l *Linkpearl) Send(ctx context.Context, roomID id.RoomID, content any) (id.EventID, error) {
	l.log.Debug().Str("roomID", roomID.String()).Any("content", content).Msg("sending event")
	resp, err := l.SendMessageEvent(ctx, roomID, event.EventMessage, content)
	if err != nil {
		return "", UnwrapError(err)
	}
	return resp.EventID, nil
}

// SendTyping notification
func (l *Linkpearl) SendTyping(ctx context.Context, roomID id.RoomID, typing bool, timeout ...int) {
	ttl := DefaultTypingTimeout
	if len(timeout) > 0 {
		ttl = timeout[0]
	}
	_, err := l.api.UserTyping(ctx, roomID, typing, time.Duration(ttl)*time.Second)
	if err != nil {
		l.log.Warn().Err(err).Bool("typing", typing).Msg("cannot set typing")
	}
}

// SendNotice to a room with optional relations, markdown supported
func (l *Linkpearl) SendNotice(ctx context.Context, roomID id.RoomID, message string, relates ...*event.RelatesTo) {
	var withRelatesTo bool
	content := format.RenderMarkdown(message, true, true)
	content.MsgType = event.MsgNotice
	if len(relates) > 0 {
		withRelatesTo = true
		content.RelatesTo = relates[0]
	}

	_, err := l.Send(ctx, roomID, &content)
	if err != nil {
		l.log.Error().Err(UnwrapError(err)).Str("roomID", roomID.String()).Str("retries", "1/2").Msg("cannot send a notice into the room")
		if withRelatesTo {
			content.RelatesTo = nil
			_, err = l.Send(ctx, roomID, &content)
			if err != nil {
				l.log.Error().Err(UnwrapError(err)).Str("roomID", roomID.String()).Str("retries", "2/2").Msg("cannot send a notice into the room even without relations")
			}
		}
	}
}

// SendFile to a matrix room
func (l *Linkpearl) SendFile(ctx context.Context, roomID id.RoomID, req *mautrix.ReqUploadMedia, msgtype event.MessageType, relates ...*event.RelatesTo) error {
	var relation *event.RelatesTo
	if len(relates) > 0 {
		relation = relates[0]
	}

	resp, err := l.GetClient().UploadMedia(ctx, *req)
	if err != nil {
		err = UnwrapError(err)
		l.log.Error().Err(err).Str("file", req.FileName).Msg("cannot upload file")
		return err
	}
	content := &event.MessageEventContent{
		MsgType:   msgtype,
		Body:      req.FileName,
		URL:       resp.ContentURI.CUString(),
		RelatesTo: relation,
	}

	_, err = l.Send(ctx, roomID, content)
	err = UnwrapError(err)
	if err != nil {
		l.log.Error().Err(err).Str("roomID", roomID.String()).Str("retries", "1/2").Msg("cannot send file into the room")
		if relation != nil {
			content.RelatesTo = nil
			_, err = l.Send(ctx, roomID, &content)
			err = UnwrapError(err)
			if err != nil {
				l.log.Error().Err(UnwrapError(err)).Str("roomID", roomID.String()).Str("retries", "2/2").Msg("cannot send file into the room even without relations")
			}
		}
	}

	return err
}
