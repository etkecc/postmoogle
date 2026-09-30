package email

import (
	"strings"
	"testing"
	"time"
)

func foldOptions() *ContentOptions {
	options := testOptions()
	options.Subject, options.Sender, options.Recipient, options.CC = false, false, false, false
	options.Collapse = true
	return options
}

func TestContent_Recipient(t *testing.T) {
	tests := map[string]struct {
		to, rcptTo string
		shown      bool
	}{
		"the mailbox itself":  {"inbox@example.org", "inbox@example.org", false},
		"with a subaddress":   {"inbox+news@example.org", "inbox+news@example.org", true},
		"someone else":        {"team@example.org", "inbox@example.org", true},
		"several recipients":  {"inbox@example.org,bob@example.com", "inbox@example.org", true},
		"no recipient header": {"", "inbox@example.org", false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			eml := New("<id@example.com>", "", "", "Hi", "jane@example.com", test.to, test.rcptTo, "", "", "<p>Body</p>", nil, nil)

			msg := parsed(t, eml.Content("", testOptions()))

			if shown := strings.Contains(msg.FormattedBody, muted("To:")); shown != test.shown {
				t.Errorf("To line shown: %v, expected %v: %s", shown, test.shown, msg.FormattedBody)
			}
		})
	}
}

func TestContent_SentDate(t *testing.T) {
	defer func(original func() time.Time) { now = original }(now)
	sent := time.Date(2024, 3, 11, 9, 15, 0, 0, time.Local)
	eml := testEmail("", "<p>Body</p>")
	eml.Sent = sent

	now = func() time.Time { return sent.Add(10 * time.Minute) }
	if msg := parsed(t, eml.Content("", testOptions())); strings.Contains(msg.Body, "Date:") {
		t.Errorf("an email that came right away should have no date: %s", msg.Body)
	}

	now = func() time.Time { return sent.Add(48 * time.Hour) }
	msg := parsed(t, eml.Content("", testOptions()))
	if !strings.Contains(msg.FormattedBody, muted("Date:")+" Mon, 11 Mar 2024 09:15</p>") ||
		!strings.Contains(msg.Body, "\nDate: Mon, 11 Mar 2024 09:15\n") {
		t.Errorf("an old email should show when it was sent: %s", msg.FormattedBody)
	}
}

func TestContent_FoldQuote(t *testing.T) {
	reply := `<div dir="ltr">Sounds good!</div><br><div class="gmail_quote"><div dir="ltr" class="gmail_attr">` +
		`On Tue, Sep 29, 2026 at 8:56 AM John &lt;john@example.com&gt; wrote:<br></div>` +
		`<blockquote class="gmail_quote">Saturday at 9?</blockquote></div>`
	tests := map[string]struct {
		subject, inReplyTo string
		collapse           bool
		folded             bool
	}{
		"reply":                {"Re: Plans", "<a@example.com>", true, true},
		"reply by subject":     {"AW: Plans", "", true, true},
		"not a reply":          {"Plans", "", true, false},
		"forward":              {"Fwd: Plans", "<a@example.com>", true, false},
		"folding switched off": {"Re: Plans", "<a@example.com>", false, false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			eml := New("<id@example.com>", test.inReplyTo, "", test.subject, "jane@example.com", "inbox@example.org", "inbox@example.org", "", "", reply, nil, nil)
			options := foldOptions()
			options.Collapse = test.collapse

			msg := parsed(t, eml.Content("", options))

			expected := `<p>Sounds good!</p><details><summary>Show quoted text</summary><p>On Tue, Sep 29, 2026 at 8:56 AM John ` +
				`&lt;john@example.com&gt; wrote:</p><blockquote><p>Saturday at 9?</p></blockquote></details>`
			if folded := msg.FormattedBody == expected; folded != test.folded {
				t.Errorf("folded: %v, expected %v: %s", folded, test.folded, msg.FormattedBody)
			}
			if !strings.Contains(msg.Body, "Saturday at 9?") {
				t.Errorf("the plain text should keep the quote: %s", msg.Body)
			}
		})
	}
}

func TestContent_FoldPlainTextQuote(t *testing.T) {
	tests := map[string]struct {
		text     string
		expected string
	}{
		"quote at the end": {
			text: "Thursday works.\n\nCan\n\nOn Mon, 28 Sep 2026 at 10:12, Emin Demir <emin@example.com>\nwrote:\n> Does Thursday work?\n>\n> Emin",
			expected: "<p>Thursday works.</p>\n<p>Can</p><details><summary>Show quoted text</summary>" +
				"<p>On Mon, 28 Sep 2026 at 10:12, Emin Demir <a href=\"mailto:emin@example.com\">emin@example.com</a><br>\nwrote:</p>\n" +
				"<blockquote>\n<p>Does Thursday work?</p>\n<p>Emin</p>\n</blockquote></details>",
		},
		"answers between quotes are not folded": {
			text:     "> 1. Ready?\nYes.\n> 2. Approved?\nNot yet.",
			expected: "<blockquote>\n<ol>\n<li>Ready?</li>\n</ol>\n</blockquote>\n<p>Yes.</p>\n<blockquote>\n<ol start=\"2\">\n<li>Approved?</li>\n</ol>\n</blockquote>\n<p>Not yet.</p>",
		},
		"only a quote": {
			text:     "> forwarded words",
			expected: "<blockquote>\n<p>forwarded words</p>\n</blockquote>",
		},
		"one line": {
			text:     "Just one line",
			expected: "<p>Just one line</p>",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			eml := New("<id@example.com>", "<a@example.com>", "", "Re: Meeting", "can@example.com", "inbox@example.org", "inbox@example.org", "", test.text, "", nil, nil)

			msg := parsed(t, eml.Content("", foldOptions()))

			if msg.FormattedBody != test.expected {
				t.Errorf("\nexpected: %q\n  output: %q", test.expected, msg.FormattedBody)
			}
		})
	}
}

func TestContent_FoldLongEmail(t *testing.T) {
	var html strings.Builder
	for i := range 20 {
		html.WriteString("<p>" + strings.Repeat("word ", 60) + "</p>")
		if i == 0 {
			html.WriteString(`<h2>Details</h2>`)
		}
	}
	eml := testEmail("", html.String())

	msg := parsed(t, eml.Content("", foldOptions()))

	before, folded, ok := strings.Cut(msg.FormattedBody, "<details><summary>Show the rest of the email</summary>")
	if !ok || !strings.HasSuffix(folded, "</details>") {
		t.Fatalf("the end of a long email should be folded: %s", msg.FormattedBody)
	}
	if shown := len([]rune(blockText(before))); shown < previewTextLen || shown > previewTextLen+400 {
		t.Errorf("unexpected preview length %d", shown)
	}
	if strings.Count(msg.Body, "word") != 20*60 {
		t.Error("the plain text should keep the whole email")
	}
	if short := parsed(t, testEmail("", "<p>short</p>").Content("", foldOptions())); strings.Contains(short.FormattedBody, "<details>") {
		t.Errorf("short emails should not be folded: %s", short.FormattedBody)
	}
}

func TestContent_FoldTruncated(t *testing.T) {
	var html strings.Builder
	for i := range 3000 {
		html.WriteString("<p>Paragraph " + strings.Repeat("x", i%50) + "</p>")
	}
	eml := testEmail("", html.String())

	msg := parsed(t, eml.Content("", foldOptions()))

	if !eml.Truncated() || !strings.HasSuffix(msg.FormattedBody, "</details><p><em>"+truncatedNotice+"</em></p>") {
		t.Errorf("the truncation notice should follow the fold: %s", msg.FormattedBody[len(msg.FormattedBody)-200:])
	}
}
