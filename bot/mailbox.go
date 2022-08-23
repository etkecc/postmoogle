package bot

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/postmoogle/utils"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

func (b *Bot) syncRooms(ctx context.Context) error {
	b.roomsmu.Lock()
	defer b.roomsmu.Unlock()
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("syncRooms"))
	defer span.Finish()

	resp, err := b.lp.GetClient().JoinedRooms()
	if err != nil {
		return err
	}
	b.rooms = make(map[string]id.RoomID, len(resp.JoinedRooms))
	for _, roomID := range resp.JoinedRooms {
		cfg, serr := b.getSettings(span.Context(), roomID)
		if serr != nil {
			b.log.Warn("cannot get %s settings: %v", roomID, err)
			continue
		}
		if cfg.Mailbox != "" {
			b.rooms[cfg.Mailbox] = roomID
		}
	}

	return nil
}

func (b *Bot) handleMailbox(ctx context.Context, evt *event.Event, command []string) {
	if len(command) == 1 {
		b.getMailbox(ctx, evt)
		return
	}
	b.setMailbox(ctx, evt, command[1])
}

func (b *Bot) getMailbox(ctx context.Context, evt *event.Event) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("getMailbox"))
	defer span.Finish()

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve setting: %v", err)
		return
	}

	if cfg.Mailbox == "" {
		b.Notice(span.Context(), evt.RoomID, "mailbox name is not set")
		return
	}

	content := format.RenderMarkdown("Mailbox of this room is **"+cfg.Mailbox+"@"+b.domain+"**", true, true)
	content.MsgType = event.MsgNotice
	_, err = b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
	}
}

func (b *Bot) setMailbox(ctx context.Context, evt *event.Event, mailbox string) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("setMailbox"))
	defer span.Finish()

	mailbox = utils.Mailbox(mailbox)
	existingID, ok := b.GetMapping(ctx, mailbox)
	if ok && existingID != "" && existingID != evt.RoomID {
		content := format.RenderMarkdown("Mailbox "+mailbox+"@"+b.domain+" already taken", true, true)
		content.MsgType = event.MsgNotice
		_, err := b.lp.Send(evt.RoomID, content)
		if err != nil {
			b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
		}
	}
	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve setting: %v", err)
		return
	}

	if !cfg.Allowed(b.noowner, evt.Sender) {
		b.Notice(span.Context(), evt.RoomID, "you don't have permission to do that")
		return
	}

	cfg.Owner = evt.Sender
	cfg.Mailbox = mailbox
	err = b.setSettings(span.Context(), evt.RoomID, cfg)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot update settings: %v", err)
		return
	}

	b.roomsmu.Lock()
	b.rooms[mailbox] = evt.RoomID
	b.roomsmu.Unlock()

	content := format.RenderMarkdown("Mailbox of this room set to **"+cfg.Mailbox+"@"+b.domain+"**", true, true)
	content.MsgType = event.MsgNotice
	_, err = b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
	}
}

func (b *Bot) handleHideSenderAddress(ctx context.Context, evt *event.Event, command []string) {
	getter := func(entity settings) bool {
		return entity.HideSenderAddress
	}

	setter := func(entity *settings, value bool) error {
		entity.HideSenderAddress = value
		return nil
	}

	b.handleBooleanConfigurationKey(ctx, evt, command, "hide-sender-address", getter, setter)
}

func (b *Bot) handleBooleanConfigurationKey(
	ctx context.Context,
	evt *event.Event,
	command []string,
	configKey string,
	getter func(entity settings) bool,
	setter func(entity *settings, value bool) error,
) {
	if len(command) == 1 {
		b.getBooleanConfigurationKey(ctx, evt, configKey, getter, setter)
		return
	}

	b.setBooleanConfigurationKey(ctx, evt, command[1], configKey, getter, setter)
}

func (b *Bot) getBooleanConfigurationKey(
	ctx context.Context,
	evt *event.Event,
	configKey string,
	getter func(entity settings) bool,
	setter func(entity *settings, value bool) error,
) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName(fmt.Sprintf("getBooleanConfigurationKey.%s", configKey)))
	defer span.Finish()

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.log.Warn("cannot get %s settings: %v", evt.RoomID, err)
		return
	}

	value := getter(cfg)

	content := format.RenderMarkdown(fmt.Sprintf("`%s` configuration setting for this room is currently set to `%v`", configKey, value), true, true)
	content.MsgType = event.MsgNotice
	_, err = b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
	}
}

func (b *Bot) setBooleanConfigurationKey(
	ctx context.Context,
	evt *event.Event,
	value string,
	configKey string,
	getter func(entity settings) bool,
	setter func(entity *settings, value bool) error,
) {
	var actualValue bool
	if value == "true" {
		actualValue = true
	} else if value == "false" {
		actualValue = false
	} else {
		b.Notice(ctx, evt.RoomID, "you are supposed to send a true or false value")
		return
	}

	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName(fmt.Sprintf("setBooleanConfigurationKey.%s", configKey)))
	defer span.Finish()

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve setting: %v", err)
		return
	}

	if !cfg.Allowed(b.noowner, evt.Sender) {
		b.Notice(span.Context(), evt.RoomID, "you don't have permission to do that")
		return
	}

	setter(&cfg, actualValue)

	err = b.setSettings(span.Context(), evt.RoomID, cfg)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot update settings: %v", err)
		return
	}

	content := format.RenderMarkdown(fmt.Sprintf("`%s` configuration setting for this room has been set to `%v`", configKey, actualValue), true, true)
	content.MsgType = event.MsgNotice
	_, err = b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
	}
}
