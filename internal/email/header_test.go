package email

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestContent_HugeHeader(t *testing.T) {
	addresses := make([]string, 0, 5000)
	for i := range 5000 {
		addresses = append(addresses, fmt.Sprintf("person%d@example.com", i))
	}
	list := strings.Join(addresses, ",")
	references := strings.TrimSpace(strings.Repeat("<old@example.com> ", 10000) + "<latest@example.com>")
	eml := New("<"+strings.Repeat("i", 100000)+"@example.com>", "<"+strings.Repeat("r", 100000)+"@example.com>",
		references, strings.Repeat("Hello world ", 10000), "jane@example.com", list, "inbox@example.org", list,
		"", "<p>Body</p>", nil, nil)
	eml.FromName = strings.Repeat("Jane ", 20000)

	content := eml.Content("", testOptions())
	msg := parsed(t, content)
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) > maxContentSize {
		t.Errorf("the event is too large: %d bytes", len(data))
	}
	if eml.Truncated() || !strings.HasSuffix(msg.FormattedBody, "<hr><p>Body</p>") {
		t.Errorf("the body should still fit next to the shortened header: %s", msg.FormattedBody[len(msg.FormattedBody)-100:])
	}
	if !strings.Contains(msg.FormattedBody, "Hello world Hello world") || !strings.Contains(msg.FormattedBody, "…</h3>") {
		t.Error("the subject should be shown shortened")
	}
	if !strings.Contains(msg.Body, " more\nCc: person0@example.com, ") {
		t.Error("the To line should tell how many recipients are not shown")
	}

	for key, limit := range map[string]int{"subject": maxSubjectLen, "messageID": maxIDLen, "inReplyTo": maxIDLen} {
		if value := fmt.Sprint(content.Raw[key]); utf8.RuneCountInString(value) > limit+1 {
			t.Errorf("%s is not shortened: %d characters", key, utf8.RuneCountInString(value))
		}
	}
	for _, key := range []string{"to", "cc", "references"} {
		if value := fmt.Sprint(content.Raw[key]); len(value) > maxListLen || value == "" {
			t.Errorf("%s has %d bytes", key, len(value))
		}
	}
	if refs := fmt.Sprint(content.Raw["references"]); !strings.HasSuffix(refs, " <latest@example.com>") {
		t.Errorf("the latest references should be kept: %s", refs)
	}
	if to := fmt.Sprint(content.Raw["to"]); !strings.HasPrefix(to, "person0@example.com,person1@example.com,") {
		t.Errorf("the first recipients should be kept whole: %.60s", to)
	}
}

func TestContent_SmallHeaderIsKept(t *testing.T) {
	eml := testEmail("", "<p>Body</p>")
	eml.References = "<a@example.com> <b@example.com>"

	raw := eml.Content("", testOptions()).Raw

	if raw["references"] != eml.References || raw["to"] != eml.To || raw["subject"] != eml.Subject {
		t.Errorf("small header values should be stored as they are: %v", raw)
	}
}
