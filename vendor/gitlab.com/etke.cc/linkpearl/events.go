package linkpearl

import (
	"context"
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

// RespRelations is response of https://spec.matrix.org/v1.8/client-server-api/#get_matrixclientv1roomsroomidrelationseventidreltype
type RespRelations struct {
	Chunk     []*event.Event `json:"chunk"`
	NextBatch string         `json:"next_batch"`
}

// Threads endpoint, ref: https://spec.matrix.org/v1.8/client-server-api/#get_matrixclientv1roomsroomidthreads
func (l *Linkpearl) Threads(ctx context.Context, roomID id.RoomID, fromToken ...string) (*RespThreads, error) {
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
	_, err := l.GetClient().MakeRequest(ctx, "GET", urlPath, nil, &resp)
	return resp, UnwrapError(err)
}

// Relations returns all relations of the given type for the given event
func (l *Linkpearl) Relations(ctx context.Context, roomID id.RoomID, eventID id.EventID, relType string, fromToken ...string) (*RespRelations, error) {
	var from string
	if len(fromToken) > 0 {
		from = fromToken[0]
	}

	query := map[string]string{
		"from":  from,
		"limit": "100",
	}

	var resp *RespRelations
	urlPath := l.GetClient().BuildURLWithQuery(mautrix.ClientURLPath{"v1", "rooms", roomID, "relations", eventID, relType}, query)
	_, err := l.GetClient().MakeRequest(ctx, "GET", urlPath, nil, &resp)
	return resp, UnwrapError(err)
}

// FindThreadBy tries to find thread message event by field and value
func (l *Linkpearl) FindThreadBy(ctx context.Context, roomID id.RoomID, fieldValue map[string]string, fromToken ...string) *event.Event {
	var from string
	if len(fromToken) > 0 {
		from = fromToken[0]
	}

	resp, err := l.Threads(ctx, roomID, from)
	err = UnwrapError(err)
	if err != nil {
		l.log.Warn().Err(err).Str("roomID", roomID.String()).Msg("cannot get room threads")
		return nil
	}

	for _, msg := range resp.Chunk {
		for field, value := range fieldValue {
			evt, contains := l.eventContains(ctx, msg, field, value)
			if contains {
				return evt
			}
		}
	}

	if resp.NextBatch == "" { // nothing more
		return nil
	}

	return l.FindThreadBy(ctx, roomID, fieldValue, resp.NextBatch)
}

// FindEventBy tries to find message event by field and value
func (l *Linkpearl) FindEventBy(ctx context.Context, roomID id.RoomID, fieldValue map[string]string, fromToken ...string) *event.Event {
	var from string
	if len(fromToken) > 0 {
		from = fromToken[0]
	}

	resp, err := l.GetClient().Messages(ctx, roomID, from, "", mautrix.DirectionBackward, nil, l.eventsLimit)
	err = UnwrapError(err)
	if err != nil {
		l.log.Warn().Err(err).Str("roomID", roomID.String()).Msg("cannot get room events")
		return nil
	}

	for _, msg := range resp.Chunk {
		for field, value := range fieldValue {
			evt, contains := l.eventContains(ctx, msg, field, value)
			if contains {
				return evt
			}
		}
	}

	if resp.End == "" { // nothing more
		return nil
	}

	return l.FindEventBy(ctx, roomID, fieldValue, resp.End)
}

func (l *Linkpearl) eventContains(ctx context.Context, evt *event.Event, field, value string) (*event.Event, bool) {
	if evt.Type == event.EventEncrypted {
		ParseContent(evt, &l.log)
		decrypted, err := l.GetClient().Crypto.Decrypt(ctx, evt)
		if err == nil {
			evt = decrypted
		}
	}

	return evt, EventContains(evt, field, value)
}
