package linkpearl

import (
	"strconv"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// RespThreads is response of https://spec.matrix.org/v1.8/client-server-api/#get_matrixclientv1roomsroomidthreads
type RespThreads struct {
	Chunk     []*event.Event `json:"chunk"`
	NextBatch string         `json:"next_batch"`
}

// Threads endpoint, ref: https://spec.matrix.org/v1.8/client-server-api/#get_matrixclientv1roomsroomidthreads
func (l *Linkpearl) Threads(roomID id.RoomID, fromToken ...string) (*RespThreads, error) {
	var from string
	if len(fromToken) > 0 {
		from = fromToken[0]
	}

	query := map[string]string{
		"from":  from,
		"limit": strconv.Itoa(l.eventsLimit),
	}

	var resp *RespThreads
	urlPath := l.GetClient().BuildURLWithQuery(mautrix.ClientURLPath{"v1", "rooms", roomID, "threads"}, query)
	_, err := l.GetClient().MakeRequest("GET", urlPath, nil, &resp)
	return resp, UnwrapError(err)
}

// FindEventBy tries to find event by field and value
func (l *Linkpearl) FindEventBy(roomID id.RoomID, field, value string, fromToken ...string) *event.Event {
	var from string
	if len(fromToken) > 0 {
		from = fromToken[0]
	}

	resp, err := l.GetClient().Messages(roomID, from, "", mautrix.DirectionBackward, nil, l.eventsLimit)
	err = UnwrapError(err)
	if err != nil {
		l.log.Warn().Err(err).Str("roomID", roomID.String()).Msg("cannot get room events")
		return nil
	}

	for _, msg := range resp.Chunk {
		evt, contains := l.eventContains(msg, field, value)
		if contains {
			return evt
		}
	}

	if resp.End == "" { // nothing more
		return nil
	}

	return l.FindEventBy(roomID, field, value, resp.End)
}

func (l *Linkpearl) eventContains(evt *event.Event, field, value string) (*event.Event, bool) {
	if evt.Type == event.EventEncrypted {
		ParseContent(evt, &l.log)
		decrypted, err := l.GetClient().Crypto.Decrypt(evt)
		if err == nil {
			evt = decrypted
		}
	}

	return evt, EventContains(evt, field, value)
}
