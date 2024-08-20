package bot

import (
	"context"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type ctxkey int

const (
	ctxEvent    ctxkey = iota
	ctxThreadID ctxkey = iota
)

func newContext(ctx context.Context, evt *event.Event) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
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
		scope.SetContext("event", map[string]any{
			"id":     evt.ID,
			"room":   evt.RoomID,
			"sender": evt.Sender,
		})
	})

	return ctx
}

func threadIDToContext(ctx context.Context, threadID id.EventID) context.Context {
	return context.WithValue(ctx, ctxThreadID, threadID)
}

func threadIDFromContext(ctx context.Context) id.EventID {
	v := ctx.Value(ctxThreadID)
	if v == nil {
		return ""
	}

	threadID, ok := v.(id.EventID)
	if !ok {
		return ""
	}

	return threadID
}
