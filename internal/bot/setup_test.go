package bot

import (
	"strings"
	"testing"
)

func TestParseMailboxSetup(t *testing.T) {
	domains := []string{"example.org", "example.com"}
	list, err := parseMailboxSetup(`[
		{"address": "Inbox@Example.org", "owner": "@john:example.org", "relay": "smtp://inbox%40example.org:secret@smtp.example.org:587"},
		{"address": "news@example.com", "owner": "@john:example.org", "name": "Newsletters"}
	]`, domains)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].mailbox != "inbox" || list[0].domain != "example.org" || list[1].Name != "Newsletters" {
		t.Errorf("unexpected mailboxes: %+v %+v", list[0], list[1])
	}

	if list, err := parseMailboxSetup(" ", domains); err != nil || list != nil {
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
			_, err := parseMailboxSetup(value, domains)
			if err == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("the error shows the relay password: %v", err)
			}
		})
	}
}
