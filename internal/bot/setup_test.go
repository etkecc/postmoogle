package bot

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/etkecc/postmoogle/internal/bot/config"
)

func TestParseMailboxSetup(t *testing.T) {
	b := &Bot{domains: []string{"example.org", "example.com"}}
	list, err := b.parseMailboxSetup(`[
		{"address": "Inbox@Example.org", "owner": "@john:example.org", "relay": "smtp://inbox%40example.org:secret@smtp.example.org:587"},
		{"address": "news@example.com", "owner": "@john:example.org", "name": "Newsletters"}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].mailbox != "inbox" || list[0].domain != "example.org" || list[1].Name != "Newsletters" {
		t.Errorf("unexpected mailboxes: %+v %+v", list[0], list[1])
	}

	if list, err := b.parseMailboxSetup(" "); err != nil || list != nil {
		t.Errorf("nothing to set up should be no error: %v %v", list, err)
	}

	for name, value := range map[string]string{
		"not json":       `{"address": "inbox@example.org"}`,
		"unknown domain": `[{"address": "inbox@example.net", "owner": "@john:example.org"}]`,
		"no address":     `[{"address": "example.org", "owner": "@john:example.org"}]`,
		"bad owner":      `[{"address": "inbox@example.org", "owner": "john"}]`,
		"bad relay":      `[{"address": "inbox@example.org", "owner": "@john:example.org", "relay": "secret-password"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := b.parseMailboxSetup(value)
			if err == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("the error shows the relay password: %v", err)
			}
		})
	}
}

const testRelay = "smtp://inbox:secret@smtp.example.org:587"

func testMailbox(t *testing.T, b *Bot, setup string) *mailboxSetup {
	t.Helper()
	list, err := b.parseMailboxSetup(setup)
	if err != nil || len(list) != 1 {
		t.Fatalf("cannot parse the mailbox: %v", err)
	}
	return list[0]
}

func TestCreateMailboxRoom(t *testing.T) {
	hs := newFakeHomeserver(t)
	b := newTestBot(t, hs)
	mb := testMailbox(t, b, `[{"address": "inbox@example.org", "owner": "@john:example.org", "relay": "`+testRelay+`", "name": "Inbox"}]`)
	ctx := context.Background()

	roomID, err := b.createMailboxRoom(ctx, mb)
	if err != nil || roomID != testRoomID {
		t.Fatalf("the room was not created: %q %v", roomID, err)
	}

	created := hs.find(http.MethodPost, "/createRoom")
	if len(created) != 1 {
		t.Fatalf("expected one room, got %d", len(created))
	}
	req := created[0].decode(t)
	if req["name"] != "Inbox" || req["topic"] != "Email for inbox@example.org" || req["preset"] != "private_chat" ||
		fmt.Sprint(req["invite"]) != "[@john:example.org]" {
		t.Errorf("unexpected room: %v", req)
	}
	levels := hs.find(http.MethodPut, "/state/m.room.power_levels")
	if len(levels) != 1 {
		t.Fatalf("expected the power levels to be set once, got %d", len(levels))
	}
	users, _ := levels[0].decode(t)["users"].(map[string]any)
	if users["@john:example.org"] != float64(ownerPowerLevel) || users["@bot:example.org"] != float64(100) {
		t.Errorf("the owner should be an admin next to the bot: %v", users)
	}
	cfg, err := b.cfg.GetRoom(ctx, roomID)
	if err != nil || cfg.Mailbox() != "inbox" || cfg.Domain() != "example.org" || cfg.Owner() != "@john:example.org" ||
		cfg.Get(config.RoomRelay) != testRelay || !cfg.Active() {
		t.Errorf("unexpected mailbox settings: %v %v", cfg, err)
	}
	if len(hs.find(http.MethodPut, "/account_data/cc.etke.postmoogle.settings")) != 1 {
		t.Error("the mailbox settings were not saved on the homeserver")
	}
	if mapped, ok := b.getMapping("inbox"); !ok || mapped != roomID {
		t.Errorf("the mailbox does not lead to the room: %q", mapped)
	}
	notices := hs.find(http.MethodPut, "/send/m.room.message/")
	if len(notices) != 1 || !strings.Contains(fmt.Sprint(notices[0].decode(t)["body"]), "`inbox@example.org`") {
		t.Errorf("expected a notice about the mailbox, got %d", len(notices))
	}
}

func TestCreateMailboxRoom_Fails(t *testing.T) {
	tests := map[string]struct {
		failing string
		mapped  bool
	}{
		"the room cannot be created":    {"/createRoom", false},
		"the settings cannot be saved":  {"/account_data/", false},
		"the owner cannot become admin": {"/state/m.room.power_levels", true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			hs := newFakeHomeserver(t)
			b := newTestBot(t, hs)
			hs.fail = []string{test.failing}
			mb := testMailbox(t, b, `[{"address": "inbox@example.org", "owner": "@john:example.org"}]`)

			_, err := b.createMailboxRoom(context.Background(), mb)

			if (err == nil) != test.mapped {
				t.Errorf("unexpected error: %v", err)
			}
			if _, ok := b.getMapping("inbox"); ok != test.mapped {
				t.Errorf("the mailbox is mapped: %v, expected %v", ok, test.mapped)
			}
		})
	}
}

func TestUpdateMailboxRoom(t *testing.T) {
	existing := config.Room{config.RoomMailbox: "inbox", config.RoomDomain: "example.com", config.RoomRelay: testRelay}
	tests := map[string]struct {
		setup    string
		existing config.Room
		written  bool
		domain   string
		relay    string
	}{
		"new domain and relay": {
			`[{"address": "inbox@example.org", "owner": "@john:example.org", "relay": "smtp://inbox:new@smtp.example.org:587"}]`,
			existing, true, "example.org", "smtp://inbox:new@smtp.example.org:587",
		},
		"nothing changed": {
			`[{"address": "inbox@example.com", "owner": "@john:example.org", "relay": "` + testRelay + `"}]`,
			existing, false, "example.com", testRelay,
		},
		"a relay set in the room is kept": {
			`[{"address": "inbox@example.com", "owner": "@john:example.org"}]`,
			existing, false, "example.com", testRelay,
		},
		"an alias room is not touched": {
			`[{"address": "inbox@example.org", "owner": "@john:example.org"}]`,
			config.Room{config.RoomMailbox: "sales", config.RoomDomain: "example.com"},
			false, "example.com", "",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			hs := newFakeHomeserver(t)
			b := newTestBot(t, hs)
			ctx := context.Background()
			// a copy, because the Matrix client caches the settings map itself and the update changes it
			if err := b.cfg.SetRoom(ctx, testRoomID, maps.Clone(test.existing)); err != nil {
				t.Fatal(err)
			}
			hs.reset()

			if err := b.updateMailboxRoom(ctx, testRoomID, testMailbox(t, b, test.setup)); err != nil {
				t.Fatal(err)
			}

			if written := len(hs.find(http.MethodPut, "/account_data/")) > 0; written != test.written {
				t.Errorf("the settings were saved: %v, expected %v", written, test.written)
			}
			cfg, err := b.cfg.GetRoom(ctx, testRoomID)
			if err != nil || cfg.Domain() != test.domain || cfg.Get(config.RoomRelay) != test.relay {
				t.Errorf("unexpected settings: %v %v", cfg, err)
			}
		})
	}
}
