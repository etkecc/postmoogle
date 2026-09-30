package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/etkecc/postmoogle/internal/bot/config"
	"github.com/etkecc/postmoogle/internal/utils"
)

// ownerPowerLevel lets the owner of a mailbox room rename it and change its settings
const ownerPowerLevel = 100

// mailboxSetup is a mailbox the bot sets up by itself, from POSTMOOGLE_MAILBOXES_SETUP
type mailboxSetup struct {
	Address string `json:"address"` // mailbox@domain
	Owner   string `json:"owner"`   // Matrix user who owns the mailbox and is invited to its room
	Relay   string `json:"relay"`   // optional SMTP relay to send from this mailbox, smtp://user:pass@host:port
	Name    string `json:"name"`    // optional room name, the address by default

	mailbox string
	domain  string
}

// parseMailboxSetup reads and checks the JSON list of mailboxes to set up, which must use the bot domains
func (b *Bot) parseMailboxSetup(value string) ([]*mailboxSetup, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var list []*mailboxSetup
	if err := json.Unmarshal([]byte(value), &list); err != nil {
		return nil, fmt.Errorf("cannot parse the mailboxes to set up: %w", err)
	}
	for _, mb := range list {
		mb.mailbox, _, mb.domain = utils.EmailParts(mb.Address)
		if !strings.Contains(mb.Address, "@") || mb.mailbox == "" || !slices.Contains(b.domains, mb.domain) {
			return nil, fmt.Errorf("mailbox %q needs an address in one of the domains %s", mb.Address, strings.Join(b.domains, ", "))
		}
		if _, _, err := id.UserID(mb.Owner).Parse(); err != nil {
			return nil, fmt.Errorf("owner %q of mailbox %q is not a Matrix user ID: %w", mb.Owner, mb.Address, err)
		}
		// the relay holds a password, so it is never part of an error
		if relay, err := url.Parse(mb.Relay); mb.Relay != "" && (err != nil || relay.Host == "") {
			return nil, fmt.Errorf("relay of mailbox %q is not a URL like smtp://user:pass@host:port", mb.Address)
		}
	}
	return list, nil
}

// setupMailboxes creates a room for every mailbox to set up that has none, and keeps the settings of the others
func (b *Bot) setupMailboxes(ctx context.Context) {
	for _, mb := range b.setup {
		log := b.log.With().Str("mailbox", mb.Address).Logger()
		if roomID, ok := b.getMapping(mb.mailbox); ok {
			if err := b.updateMailboxRoom(ctx, roomID, mb); err != nil {
				log.Error().Err(err).Str("room_id", roomID.String()).Msg("cannot update the mailbox settings")
			}
			continue
		}
		roomID, err := b.createMailboxRoom(ctx, mb)
		if err != nil {
			log.Error().Err(err).Msg("cannot set up the mailbox")
			continue
		}
		log.Info().Str("room_id", roomID.String()).Str("owner", mb.Owner).Msg("mailbox has been set up")
	}
}

// createMailboxRoom creates a room for the mailbox, invites its owner, and configures the mailbox
func (b *Bot) createMailboxRoom(ctx context.Context, mb *mailboxSetup) (id.RoomID, error) {
	owner := id.UserID(mb.Owner)
	name := mb.Name
	if name == "" {
		name = mb.Address
	}
	resp, err := b.lp.GetClient().CreateRoom(ctx, &mautrix.ReqCreateRoom{
		Name:   name,
		Topic:  "Email for " + mb.Address,
		Preset: "private_chat",
		Invite: []id.UserID{owner},
	})
	if err != nil {
		return "", err
	}
	roomID := resp.RoomID
	b.setPowerLevel(ctx, roomID, owner, ownerPowerLevel)

	cfg := config.Room{}
	cfg.Set(config.RoomMailbox, mb.mailbox)
	cfg.Set(config.RoomDomain, mb.domain)
	cfg.Set(config.RoomOwner, owner.String())
	if mb.Relay != "" {
		cfg.Set(config.RoomRelay, mb.Relay)
	}
	cfg.Set(config.RoomActive, strconv.FormatBool(b.ActivateMailbox(ctx, owner, roomID, mb.mailbox)))
	if err := b.cfg.SetRoom(ctx, roomID, cfg); err != nil {
		b.rooms.Delete(mb.mailbox)
		return roomID, err
	}
	b.lp.SendNotice(ctx, roomID, fmt.Sprintf("This room gets the emails sent to `%s`. "+
		"To answer an email, reply in its thread. Send `%s help` to see the settings.", mb.Address, b.prefix))
	return roomID, nil
}

// updateMailboxRoom keeps the domain and relay of an existing mailbox room as they are set up
func (b *Bot) updateMailboxRoom(ctx context.Context, roomID id.RoomID, mb *mailboxSetup) error {
	cfg, err := b.cfg.GetRoom(ctx, roomID)
	if err != nil {
		return err
	}
	if cfg.Mailbox() != mb.mailbox { // the name is an alias of another mailbox, which is not touched
		return nil
	}
	var changed bool
	if cfg.Domain() != mb.domain {
		cfg.Set(config.RoomDomain, mb.domain)
		changed = true
	}
	if mb.Relay != "" && cfg.Get(config.RoomRelay) != mb.Relay {
		cfg.Set(config.RoomRelay, mb.Relay)
		changed = true
	}
	if !changed {
		return nil
	}
	b.log.Info().Str("mailbox", mb.Address).Str("room_id", roomID.String()).Msg("updating the mailbox settings")
	return b.cfg.SetRoom(ctx, roomID, cfg)
}

// setPowerLevel gives a user a power level in a room the bot created, where only the bot has power at first
func (b *Bot) setPowerLevel(ctx context.Context, roomID id.RoomID, userID id.UserID, level int) {
	client := b.lp.GetClient()
	var levels event.PowerLevelsEventContent
	if err := client.StateEvent(ctx, roomID, event.StatePowerLevels, "", &levels); err != nil {
		b.log.Warn().Err(err).Str("room_id", roomID.String()).Msg("cannot get power levels")
		return
	}
	levels.SetUserLevel(userID, level)
	if _, err := client.SendStateEvent(ctx, roomID, event.StatePowerLevels, "", &levels); err != nil {
		b.log.Warn().Err(err).Str("room_id", roomID.String()).Msg("cannot set power levels")
	}
}
