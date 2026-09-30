package email

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/jhillyerd/enmime/v2"

	"github.com/etkecc/postmoogle/internal/utils"
)

func TestFilesToSend_Attachments(t *testing.T) {
	logo := utils.NewFile("logo.png", testPNG(t, 4, 4))
	logo.ContentID = "logo@example.com"
	photo := utils.NewFile("photo.png", testPNG(t, 8, 8))
	photo.ContentID = "photo@example.com"
	folded := utils.NewFile("signature.png", testPNG(t, 4, 4))
	folded.ContentID = "signature@example.com"
	report := utils.NewFile("report.pdf", []byte("%PDF-1.4 report"))
	eml := testEmail("", `<p><img src="cid:logo@example.com"> <img src="cid:photo@example.com"></p>`)
	eml.Files = []*utils.File{logo, photo, folded, report}
	eml.SetImage("cid:logo@example.com", &Image{URI: "mxc://example.com/logo", Width: 120, Height: 40, File: logo})
	eml.SetImage("cid:photo@example.com", &Image{URI: "mxc://example.com/photo", Width: 1600, Height: 1200, File: photo})
	eml.SetImage("cid:signature@example.com", &Image{URI: "mxc://example.com/signature", Width: 120, Height: 40, File: folded})

	msg := parsed(t, eml.Content("", testOptions()))
	toSend := eml.FilesToSend(eml.Files, msg.FormattedBody)

	if len(toSend) != 3 || toSend[0] != photo || toSend[1] != folded || toSend[2] != report {
		names := make([]string, 0, len(toSend))
		for _, file := range toSend {
			names = append(names, file.Name)
		}
		t.Errorf("only the logo shown inside the message should be skipped, got %v", names)
	}
}

func TestFromEnvelope_SkipsNonImageParts(t *testing.T) {
	logo := base64.StdEncoding.EncodeToString(testPNG(t, 4, 4))
	part := func(contentType, contentID, content string) string {
		return "--b1\r\nContent-Type: " + contentType + "\r\nContent-ID: <" + contentID + ">\r\n" +
			"Content-Transfer-Encoding: base64\r\n\r\n" + content + "\r\n"
	}
	raw := "From: jane@example.com\r\nTo: inbox@example.org\r\nSubject: Meeting\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/related; boundary=\"b1\"\r\n\r\n" +
		"--b1\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p>See you <img src=\"cid:logo@example.com\"></p>\r\n" +
		part("image/png", "logo@example.com", logo) +
		part("text/calendar", "invite@example.com", base64.StdEncoding.EncodeToString([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR"))) +
		part("font/woff2", "font@example.com", base64.StdEncoding.EncodeToString([]byte("wOF2 font data"))) +
		part("image/png", "fake@example.com", base64.StdEncoding.EncodeToString([]byte("<html>not a png</html>"))) +
		"--b1--\r\n"
	envelope, err := enmime.ReadEnvelope(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}

	eml := FromEnvelope("inbox@example.org", envelope)

	if len(eml.InlineFiles) != 1 || eml.InlineFiles[0].ContentID != "logo@example.com" {
		ids := make([]string, 0, len(eml.InlineFiles))
		for _, file := range eml.InlineFiles {
			ids = append(ids, file.ContentID)
		}
		t.Errorf("only the image should be kept from the parts without a disposition, got %v", ids)
	}
}
