package bot

import (
	"context"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix/event"
)

type ctxkey int

const (
	ctxEvent ctxkey = iota
)

func newContext(evt *event.Event) context.Context {
	ctx := context.Background()
	hub := sentry.CurrentHub().Clone()
	ctx = sentry.SetHubOnContext(ctx, hub)
	ctx = eventToContext(ctx, evt)

	return ctx
}

func eventFromContext(ctx context.Context) *event.Event {
	v := ctx.Value(ctxEvent)
	if v == nil {
		return nil
	}

	evt, ok := v.(*event.Event)
	if !ok {
		return nil
	}

	return evt
}

func eventToContext(ctx context.Context, evt *event.Event) context.Context {
	ctx = context.WithValue(ctx, ctxEvent, evt)
	sentry.GetHubFromContext(ctx).ConfigureScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{ID: evt.Sender.String()})
		scope.SetContext("event", map[string]string{
			"id":     evt.ID.String(),
			"room":   evt.RoomID.String(),
			"sender": evt.Sender.String(),
		})
	})

	return ctx
}
