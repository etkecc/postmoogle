package bot

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/etkecc/postmoogle/internal/bot/config"
	"github.com/etkecc/postmoogle/internal/email"
	"github.com/etkecc/postmoogle/internal/utils"
)

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// testEmail returns an email with the given HTML and an inline logo
func testEmail(t *testing.T, html string) *email.Email {
	t.Helper()
	logo := utils.NewFile("logo.png", testPNG(t, 40, 20))
	logo.ContentID = "logo@example.com"
	eml := email.New("<id@example.com>", "", "", "Hello", "jane@example.com", "inbox@example.org", "inbox@example.org", "",
		"", html, nil, []*utils.File{logo})
	return eml
}

func TestImagesToLoad(t *testing.T) {
	var html strings.Builder
	for i := range 30 {
		fmt.Fprintf(&html, `<img src="https://example.com/%d.png" alt="%d">`, i, i)
	}
	html.WriteString(`<img src="cid:logo@example.com"><img src="data:image/png;base64,iVBORw0KGgo=">`)
	eml := testEmail(t, html.String())
	b := &Bot{}
	tests := map[string]struct {
		inline, remote bool
		expected       int
	}{
		"linked images need to be switched on": {true, false, 2},
		"linked images are limited":            {true, true, maxRemoteImages + 2},
		"inline images can be switched off":    {false, true, maxRemoteImages},
		"no images":                            {false, false, 0},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			options := &email.ContentOptions{HTML: true, InlineImages: test.inline, RemoteImages: test.remote}
			if sources := b.imagesToLoad(eml, options); len(sources) != test.expected {
				t.Errorf("expected %d images, got %d: %v", test.expected, len(sources), sources)
			}
		})
	}

	eml.SetImage("cid:logo@example.com", nil)
	if sources := b.imagesToLoad(eml, &email.ContentOptions{InlineImages: true}); len(sources) != 1 {
		t.Errorf("images that were already processed should be skipped: %v", sources)
	}
}

func TestImagesToLoad_TotalLimit(t *testing.T) {
	var html strings.Builder
	for i := range maxEmailImages + 10 {
		fmt.Fprintf(&html, `<img src="cid:image%d@example.com">`, i)
	}
	eml := testEmail(t, html.String())

	sources := (&Bot{}).imagesToLoad(eml, &email.ContentOptions{InlineImages: true})

	if len(sources) != maxEmailImages {
		t.Errorf("expected %d images, got %d", maxEmailImages, len(sources))
	}
}

func TestLoadImage(t *testing.T) {
	hs := newFakeHomeserver(t)
	b := newTestBot(t, hs)
	picture := testPNG(t, 4, 3)
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(picture) }))
	defer local.Close()
	fake := utils.NewFile("fake.gif", []byte("GIF89a\x0a\x00\x0a\x00\x00\x00\x00<html>not an image</html>"))
	fake.ContentID = "fake@example.com"
	eml := testEmail(t, "")
	eml.InlineFiles = append(eml.InlineFiles, fake)
	ctx := context.Background()

	logo := b.loadImage(ctx, eml, "cid:logo@example.com")
	if logo == nil || !strings.HasPrefix(logo.URI, "mxc://example.org/upload") || logo.Width != 40 || logo.Height != 20 ||
		logo.File != eml.InlineFiles[0] {
		t.Fatalf("the inline image was not uploaded: %+v", logo)
	}
	data := b.loadImage(ctx, eml, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(picture))
	if data == nil || data.Width != 4 || data.Height != 3 || data.File != nil {
		t.Fatalf("the data: image was not uploaded: %+v", data)
	}
	uploads := hs.find(http.MethodPost, "/media/v3/upload")
	if len(uploads) != 2 || !bytes.Equal(uploads[0].body, eml.InlineFiles[0].Content) || !bytes.Equal(uploads[1].body, picture) {
		t.Errorf("expected 2 uploads of the images, got %d", len(uploads))
	}

	for _, src := range []string{"cid:fake@example.com", "cid:missing@example.com", local.URL + "/image.png", "ftp://example.com/a.png"} {
		if img := b.loadImage(ctx, eml, src); img != nil {
			t.Errorf("%s should not be uploaded: %+v", src, img)
		}
	}
	if uploads := hs.find(http.MethodPost, "/media/v3/upload"); len(uploads) != 2 {
		t.Errorf("invalid images and local addresses should not be uploaded, got %d uploads", len(uploads))
	}
}

func TestEmbedImages(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(testPNG(t, 2, 2)) }))
	defer local.Close()
	html := `<p><img src="cid:logo@example.com" alt="Logo"> <img src="` + local.URL + `/tracker.png" alt="Remote"></p>`
	tests := map[string]struct {
		settings     map[string]string
		inline       bool // the inline logo is uploaded and shown
		remoteLoaded bool // loading the linked image was tried
	}{
		"default":               {map[string]string{}, true, false},
		"linked images on":      {map[string]string{config.RoomRemoteImages: "true"}, true, true},
		"no inline images":      {map[string]string{config.RoomNoInlines: "true"}, false, false},
		"no html":               {map[string]string{config.RoomNoHTML: "true", config.RoomRemoteImages: "true"}, false, false},
		"only linked images on": {map[string]string{config.RoomNoInlines: "true", config.RoomRemoteImages: "true"}, false, true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			hs := newFakeHomeserver(t)
			b := newTestBot(t, hs)
			eml := testEmail(t, html)
			cfg := config.Room(test.settings)

			b.embedImages(context.Background(), eml, cfg)

			if eml.HasImage(local.URL+"/tracker.png") != test.remoteLoaded {
				t.Errorf("the linked image was tried: %v, expected %v", !test.remoteLoaded, test.remoteLoaded)
			}
			if eml.Images[local.URL+"/tracker.png"] != nil {
				t.Error("an image on a local address must never be uploaded")
			}
			shown := strings.Contains(b.formattedBody(eml.Content("", cfg.ContentOptions())), `<img src="mxc://`)
			if shown != test.inline {
				t.Errorf("the inline logo shown: %v, expected %v", shown, test.inline)
			}
		})
	}
}
