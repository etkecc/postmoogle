package utils

import "testing"

func TestMailbox(t *testing.T) {
	tests := map[string]string{
		"mailbox@example.com":       "mailbox",
		"mail-box@example.com":      "mail-box",
		"mailbox":                   "mailbox",
		"mail@box@example.com":      "mail",
		"mailbox+@example.com":      "mailbox",
		"mailbox+sub@example.com":   "mailbox",
		"mailbox+++sub@example.com": "mailbox",
	}

	for in, expected := range tests {
		t.Run(in, func(t *testing.T) {
			output := Mailbox(in)
			if output != expected {
				t.Error(expected, "!=", output)
			}
		})
	}
}

func TestSubaddress(t *testing.T) {
	tests := map[string]string{
		"mailbox@example@example.com": "",
		"mail-box@example.com":        "",
		"mailbox+":                    "",
		"mailbox+sub@example.com":     "sub",
		"mailbox+++sub@example.com":   "sub",
	}

	for in, expected := range tests {
		t.Run(in, func(t *testing.T) {
			output := Subaddress(in)
			if output != expected {
				t.Error(expected, "!=", output)
			}
		})
	}
}

func TestHostname(t *testing.T) {
	tests := map[string]string{
		"mailbox@example.com":  "example.com",
		"mailbox":              "mailbox",
		"mail@box@example.com": "example.com",
	}

	for in, expected := range tests {
		t.Run(in, func(t *testing.T) {
			output := Hostname(in)
			if output != expected {
				t.Error(expected, "!=", output)
			}
		})
	}
}

func TestEmailList(t *testing.T) {
	domains = []string{"example.com", "example.org"}
	expected := "test@example.org, test@example.com"

	actual := EmailsList("test", "example.org")
	if actual != expected {
		t.Error(expected, "!=", actual)
	}
}
