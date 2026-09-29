package email

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/jhillyerd/enmime/v2"
	"maunium.net/go/mautrix/event"

	"github.com/etkecc/postmoogle/internal/utils"
)

func testOptions() *ContentOptions {
	return &ContentOptions{
		CC:            true,
		Sender:        true,
		Recipient:     true,
		Subject:       true,
		HTML:          true,
		Threads:       true,
		InlineImages:  true,
		RemoteImages:  true,
		MessageIDKey:  "messageID",
		InReplyToKey:  "inReplyTo",
		ReferencesKey: "references",
		SubjectKey:    "subject",
		FromKey:       "from",
		ToKey:         "to",
		CcKey:         "cc",
		RcptToKey:     "rcptTo",
	}
}

func testEmail(text, html string) *Email {
	eml := New("<id@example.com>", "", "", "Hello there", "jane@example.com", "inbox+news@example.org", "inbox@example.org", "a@example.com,b@example.com", text, html, nil, nil)
	eml.FromName = "Jane Doe"
	return eml
}

func parsed(t *testing.T, content *event.Content) *event.MessageEventContent {
	t.Helper()
	msg, ok := content.Parsed.(*event.MessageEventContent)
	if !ok {
		t.Fatalf("unexpected content type %T", content.Parsed)
	}
	return msg
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestContent_Header(t *testing.T) {
	eml := testEmail("", "<p>Body</p>")

	msg := parsed(t, eml.Content("", testOptions()))

	expectedHTML := "<h3>Hello there</h3>" +
		"<p>From: <strong>Jane Doe</strong> &lt;jane@example.com&gt;<br>To: inbox@example.org (news)<br>Cc: a@example.com, b@example.com</p>" +
		"<hr><p>Body</p>"
	if msg.FormattedBody != expectedHTML {
		t.Errorf("\nexpected: %s\n  output: %s", expectedHTML, msg.FormattedBody)
	}
	expectedText := "Hello there\nFrom: Jane Doe <jane@example.com>\nTo: inbox@example.org (news)\nCc: a@example.com, b@example.com\n\nBody"
	if msg.Body != expectedText {
		t.Errorf("\nexpected: %q\n  output: %q", expectedText, msg.Body)
	}
	if msg.Format != event.FormatHTML || msg.Mentions == nil {
		t.Errorf("unexpected format %q or mentions %v", msg.Format, msg.Mentions)
	}
}

func TestContent_HeaderOptions(t *testing.T) {
	eml := testEmail("", "<p>Body</p>")
	eml.FromName = ""
	options := testOptions()
	options.Recipient = false
	options.CC = false

	msg := parsed(t, eml.Content("$thread", options))

	expected := "<p>From: <strong>jane@example.com</strong></p><hr><p>Body</p>"
	if msg.FormattedBody != expected {
		t.Errorf("\nexpected: %s\n  output: %s", expected, msg.FormattedBody)
	}
	if msg.RelatesTo == nil || msg.RelatesTo.EventID != "$thread" {
		t.Errorf("thread relation is missing: %+v", msg.RelatesTo)
	}
}

func TestContent_NoHeader(t *testing.T) {
	eml := testEmail("", "<p>Body</p>")
	options := testOptions()
	options.Subject = false
	options.Sender = false
	options.Recipient = false
	options.CC = false

	msg := parsed(t, eml.Content("", options))

	if msg.FormattedBody != "<p>Body</p>" || msg.Body != "Body" {
		t.Errorf("unexpected content: %q / %q", msg.FormattedBody, msg.Body)
	}
}

func TestContent_Metadata(t *testing.T) {
	eml := testEmail("", "<p>Body</p>")

	content := eml.Content("", testOptions())

	for key, expected := range map[string]string{
		"messageID": "<id@example.com>",
		"subject":   "Hello there",
		"from":      "jane@example.com",
		"to":        "inbox+news@example.org",
		"rcptTo":    "inbox@example.org",
		"cc":        "a@example.com, b@example.com",
	} {
		if content.Raw[key] != expected {
			t.Errorf("%s: expected %q, got %q", key, expected, content.Raw[key])
		}
	}
}

func TestContent_Threadify(t *testing.T) {
	eml := testEmail("", "<p>Body</p>")
	options := testOptions()
	options.Threadify = true

	root := parsed(t, eml.Content("", options))
	body := parsed(t, eml.ContentBody("$root", options))

	if strings.Contains(root.FormattedBody, "Body") || !strings.Contains(root.FormattedBody, "<h3>Hello there</h3>") {
		t.Errorf("thread root should contain the header only: %s", root.FormattedBody)
	}
	if body.FormattedBody != "<p>Body</p>" || body.RelatesTo == nil || body.RelatesTo.EventID != "$root" {
		t.Errorf("unexpected thread body: %s %+v", body.FormattedBody, body.RelatesTo)
	}
	if eml.ContentBody("$root", testOptions()) != nil {
		t.Error("ContentBody should return nil when threadify is disabled")
	}
}

func TestContent_Stripify(t *testing.T) {
	html := `<div dir="ltr">Thanks, <b>sounds good</b>! 2*3 &lt;script&gt;` +
		`<img src="https://example.com/smile.png" alt="smile" width="16" height="16"></div><br>` +
		`<div class="gmail_quote"><div dir="ltr" class="gmail_attr">On Tue, Sep 29, 2026 at 8:56 AM John &lt;john@example.com&gt; wrote:<br></div>` +
		`<blockquote class="gmail_quote">previous message<br>line 2</blockquote></div>`
	eml := testEmail("", html)
	eml.SetImage("https://example.com/smile.png", &Image{URI: "mxc://example.com/smile"})
	options := testOptions()
	options.Stripify = true

	stripped := parsed(t, eml.Content("$thread", options))
	root := parsed(t, eml.Content("", options))

	expected := `<hr>Thanks, <strong>sounds good</strong>! 2*3 &lt;script&gt;<img src="mxc://example.com/smile" alt="smile" width="16" height="16">`
	if !strings.HasSuffix(stripped.FormattedBody, expected) {
		t.Errorf("\nexpected suffix: %s\n          output: %s", expected, stripped.FormattedBody)
	}
	if !strings.Contains(root.FormattedBody, "previous message") {
		t.Errorf("new threads should not be stripped: %s", root.FormattedBody)
	}
}

func TestContent_PlainText(t *testing.T) {
	eml := testEmail("Hello *world*\n<b>not bold</b>\n\nSecond paragraph", "")
	options := testOptions()
	options.Subject = false
	options.Sender = false
	options.Recipient = false
	options.CC = false

	msg := parsed(t, eml.Content("", options))

	expected := "<p>Hello <em>world</em><br>\n&lt;b&gt;not bold&lt;/b&gt;</p>\n<p>Second paragraph</p>"
	if msg.FormattedBody != expected {
		t.Errorf("\nexpected: %q\n  output: %q", expected, msg.FormattedBody)
	}
}

func TestContent_HTMLFallbackToText(t *testing.T) {
	tooDeep := strings.Repeat("<div>", 1000) + "deep" + strings.Repeat("</div>", 1000)
	tests := map[string]*Email{
		"html disabled":    testEmail("text version", "<p>html version</p>"),
		"unparseable html": testEmail("text version", tooDeep),
		"empty html":       testEmail("text version", `<div style="display:none">hidden</div>`),
	}
	for name, eml := range tests {
		t.Run(name, func(t *testing.T) {
			options := testOptions()
			options.HTML = name != "html disabled"

			msg := parsed(t, eml.Content("", options))

			if !strings.HasSuffix(msg.FormattedBody, "<hr>text version") {
				t.Errorf("text part was not used: %s", msg.FormattedBody)
			}
		})
	}
}

func TestContent_Truncated(t *testing.T) {
	var html strings.Builder
	for i := range 3000 {
		fmt.Fprintf(&html, `<p>Paragraph %d with a <a href="https://example.com/long/tracking/url/%d?utm_source=news">link</a></p>`, i, i)
	}
	eml := testEmail("", html.String())

	content := eml.Content("", testOptions())
	msg := parsed(t, content)
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}

	if !eml.Truncated() {
		t.Error("the email should be truncated")
	}
	if len(data) > maxContentSize {
		t.Errorf("content is too large: %d bytes", len(data))
	}
	if !strings.Contains(msg.FormattedBody, "<p>Paragraph 0 with") || !strings.HasSuffix(msg.FormattedBody, "<p><em>"+truncatedNotice+"</em></p>") {
		t.Errorf("unexpected truncated content: %s", msg.FormattedBody)
	}
	if !strings.HasSuffix(msg.Body, truncatedNotice) {
		t.Errorf("text body has no notice: %s", msg.Body[len(msg.Body)-100:])
	}
	if file := eml.FullVersion(); file.Name != "email.html" || file.Length != len(eml.HTML) {
		t.Errorf("unexpected full version: %s %d", file.Name, file.Length)
	}

	eml.Content("", testOptions()) // the flag is reset by every render
	short := testEmail("", "<p>short</p>")
	short.Content("", testOptions())
	if short.Truncated() {
		t.Error("short email should not be truncated")
	}
}

func TestContent_TruncatedText(t *testing.T) {
	eml := testEmail(strings.Repeat("A line of a plain text email that goes on and on\n", 3000), "")

	content := eml.Content("", testOptions())
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}

	if !eml.Truncated() || len(data) > maxContentSize {
		t.Errorf("text email was not truncated to fit: %v %d", eml.Truncated(), len(data))
	}
	if file := eml.FullVersion(); file.Name != "email.txt" {
		t.Errorf("unexpected full version: %s", file.Name)
	}
}

func TestImageSources(t *testing.T) {
	html := `<img src="https://example.com/logo.png"><img src="cid:image001.png@01D9">` +
		`<img src="https://example.com/logo.png"><img src="data:image/png;base64,iVBORw0KGgo=">` +
		`<img src="//cdn.example.com/icon.png"><img src="ftp://example.com/nope.png"><img src="relative.png">` +
		`<img src="https://example.com/open.gif" width="1" height="1"><div style="display:none"><img src="https://example.com/hidden.png"></div>`
	eml := testEmail("", html)

	sources := eml.ImageSources()

	expected := []string{"https://example.com/logo.png", "cid:image001.png@01D9", "data:image/png;base64,iVBORw0KGgo=", "//cdn.example.com/icon.png"}
	if strings.Join(sources, " ") != strings.Join(expected, " ") {
		t.Errorf("\nexpected: %v\n  output: %v", expected, sources)
	}
	if len(testEmail("text", "").ImageSources()) != 0 {
		t.Error("text emails have no images")
	}
}

func TestImageOptions(t *testing.T) {
	eml := testEmail("", `<p><img src="https://example.com/remote.png" alt="remote"> <img src="cid:inline@x" alt="inline"></p>`)
	eml.SetImage("https://example.com/remote.png", &Image{URI: "mxc://example.com/remote"})
	eml.SetImage("cid:inline@x", &Image{URI: "mxc://example.com/inline"})
	tests := map[string]struct {
		inline, remote bool
		expected       string
	}{
		"all":         {true, true, `<p><img src="mxc://example.com/remote" alt="remote"> <img src="mxc://example.com/inline" alt="inline"></p>`},
		"inline only": {true, false, `<p>remote <img src="mxc://example.com/inline" alt="inline"></p>`},
		"remote only": {false, true, `<p><img src="mxc://example.com/remote" alt="remote"> inline</p>`},
		"none":        {false, false, `<p>remote inline</p>`},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			options := testOptions()
			options.InlineImages, options.RemoteImages = test.inline, test.remote
			options.Subject, options.Sender, options.Recipient, options.CC = false, false, false, false

			msg := parsed(t, eml.Content("", options))

			if msg.FormattedBody != test.expected {
				t.Errorf("\nexpected: %s\n  output: %s", test.expected, msg.FormattedBody)
			}
		})
	}
}

func TestInlineFiles(t *testing.T) {
	logo := utils.NewFile("logo.png", testPNG(t, 4, 4))
	logo.ContentID = "logo+1@example.com"
	photo := utils.NewFile("photo.png", testPNG(t, 8, 8))
	photo.ContentID = "Photo@Example.com"
	other := utils.NewFile("other.png", testPNG(t, 2, 2))
	eml := testEmail("", `<img src="cid:logo%2B1@example.com"><img src="cid:photo@example.com">`)
	eml.InlineFiles = []*utils.File{logo, photo, other}

	if eml.InlineFile("cid:logo%2B1@example.com") != logo || eml.InlineFile("cid:photo@example.com") != photo {
		t.Error("inline files were not found by their content ID")
	}
	if eml.InlineFile("cid:unknown@example.com") != nil || eml.InlineFile("https://example.com/logo.png") != nil {
		t.Error("unexpected inline file")
	}

	eml.SetImage("cid:logo%2B1@example.com", &Image{URI: "mxc://example.com/logo", File: logo})
	eml.SetImage("cid:photo@example.com", nil)
	if !eml.HasImage("cid:photo@example.com") || eml.HasImage("cid:unknown@example.com") {
		t.Error("processed images are not tracked")
	}
	msg := parsed(t, eml.Content("", testOptions()))

	unembedded := eml.UnembeddedInlines(msg.FormattedBody)
	if len(unembedded) != 2 || unembedded[0] != photo || unembedded[1] != other {
		t.Errorf("only the embedded logo should be skipped, got %d files", len(unembedded))
	}
}

func TestFromEnvelope(t *testing.T) {
	logo := base64.StdEncoding.EncodeToString(testPNG(t, 4, 4))
	raw := "From: =?UTF-8?Q?Jan=C3=A9_Doe?= <Jane@Example.com>\r\n" +
		"To: inbox@example.org\r\n" +
		"Subject: Related\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/related; boundary=\"b1\"\r\n\r\n" +
		"--b1\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" +
		"<html><head><style>p{color:red}</style></head><body><p>Hi <img src=\"cid:logo@example.com\"></p></body></html>\r\n" +
		"--b1\r\nContent-Type: image/png\r\nContent-Transfer-Encoding: base64\r\nContent-ID: <logo@example.com>\r\n\r\n" +
		logo + "\r\n--b1--\r\n"
	envelope, err := enmime.ReadEnvelope(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}

	eml := FromEnvelope("inbox@example.org", envelope)

	if eml.From != "jane@example.com" || eml.FromName != "Jané Doe" {
		t.Errorf("unexpected sender: %q %q", eml.From, eml.FromName)
	}
	if len(eml.InlineFiles) != 1 || eml.InlineFiles[0].ContentID != "logo@example.com" || eml.InlineFiles[0].Name != "image.png" {
		t.Fatalf("inline image without disposition was not found: %+v", eml.InlineFiles)
	}
	if eml.InlineFile("cid:logo@example.com") != eml.InlineFiles[0] {
		t.Error("inline image cannot be found by its content ID")
	}
	if strings.Contains(eml.HTML, "<style>") || !strings.Contains(eml.FullVersion().Name, "email.html") ||
		!bytes.Contains(eml.FullVersion().Content, []byte("<style>")) {
		t.Error("the full version should keep the original HTML")
	}
}

func TestFromEnvelope_UptimeRobot(t *testing.T) {
	file, err := os.Open("../../e2e/uptimerobot.eml")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	envelope, err := enmime.ReadEnvelope(file)
	if err != nil {
		t.Fatal(err)
	}
	eml := FromEnvelope("test@localhost", envelope)
	sources := eml.ImageSources()
	if len(sources) != 1 {
		t.Fatalf("expected one image, got %v", sources)
	}
	eml.SetImage(sources[0], &Image{URI: "mxc://example.com/logo", Width: 549, Height: 79})

	msg := parsed(t, eml.Content("", testOptions()))

	for _, expected := range []string{
		"<h3>Monitor is UP: Buscarron</h3><p>From: <strong>UptimeRobot</strong> &lt;alert@uptimerobot.com&gt;<br>To: test@localhost</p><hr>",
		`<img src="mxc://example.com/logo" alt="UptimeRobot" width="180" height="25"></a> <a href=`,
		"<h3>Buscarron is up.</h3><p>Hello etke.cc,</p>",
		"<p>Monitor name</p><h4>Buscarron</h4><hr><p>Checked URL</p>",
		`<strong>View incident details</strong></a></p>`,
	} {
		if !strings.Contains(msg.FormattedBody, expected) {
			t.Errorf("\nexpected to contain: %s\n              output: %s", expected, msg.FormattedBody)
		}
	}
	if strings.Contains(msg.FormattedBody, "Monitor is  UP") {
		t.Error("the HTML title should not be shown")
	}
}
