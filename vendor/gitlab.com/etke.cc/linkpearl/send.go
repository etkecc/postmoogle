package linkpearl

import (
	"fmt"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

// Send a message to the roomID and automatically try to encrypt it, if the destination room is encrypted
//
//nolint:unparam // it's public interface
func (l *Linkpearl) Send(roomID id.RoomID, content interface{}) (id.EventID, error) {
	l.log.Debug().Str("roomID", roomID.String()).Any("content", content).Msg("sending event")
	resp, err := l.api.SendMessageEvent(roomID, event.EventMessage, content)
	if err != nil {
		return "", err
	}
	return resp.EventID, nil
}

// SendNotice to a room with optional thread relation
func (l *Linkpearl) SendNotice(roomID id.RoomID, threadID id.EventID, message string, args ...interface{}) {
	content := format.RenderMarkdown(fmt.Sprintf(message, args...), true, true)
	if threadID != "" {
		content.RelatesTo = &event.RelatesTo{
			Type:    event.RelThread,
			EventID: threadID,
		}
	}

	_, err := l.Send(roomID, &content)
	if err != nil {
		l.log.Error().Err(err).Str("roomID", roomID.String()).Msg("cannot send a notice int the room")
	}
}

// SendFile to a matrix room
func (l *Linkpearl) SendFile(roomID id.RoomID, req *mautrix.ReqUploadMedia, msgtype event.MessageType, relation *event.RelatesTo) error {
	resp, err := l.GetClient().UploadMedia(*req)
	if err != nil {
		l.log.Error().Err(err).Str("file", req.FileName).Msg("cannot upload file")
		return err
	}
	_, err = l.Send(roomID, &event.Content{
		Parsed: &event.MessageEventContent{
			MsgType:   msgtype,
			Body:      req.FileName,
			URL:       resp.ContentURI.CUString(),
			RelatesTo: relation,
		},
	})
	if err != nil {
		l.log.Error().Err(err).Str("file", req.FileName).Msg("cannot send uploaded file")
	}

	return err
}
