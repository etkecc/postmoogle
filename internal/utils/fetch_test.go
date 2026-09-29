package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPublicAddr(t *testing.T) {
	tests := map[string]bool{
		"8.8.8.8":              true,
		"1.1.1.1":              true,
		"2606:4700:4700::1111": true,
		"127.0.0.1":            false,
		"10.1.2.3":             false,
		"172.16.0.1":           false,
		"192.168.1.1":          false,
		"169.254.169.254":      false,
		"100.64.0.1":           false,
		"0.0.0.0":              false,
		"255.255.255.255":      false,
		"224.0.0.1":            false,
		"198.18.0.1":           false,
		"::":                   false,
		"::1":                  false,
		"fe80::1":              false,
		"fc00::1":              false,
		"ff02::1":              false,
		"::ffff:127.0.0.1":     false,
		"::ffff:10.0.0.1":      false,
		"::127.0.0.1":          false,
		"64:ff9b::a00:1":       false,
		"2002:a00:1::1":        false,
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			if output := PublicAddr(netip.MustParseAddr(input)); output != expected {
				t.Errorf("expected %v, got %v", expected, output)
			}
		})
	}
}

func TestImageFetcher_RefusesLocalAddresses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(testPNG(t, 2, 2))
	}))
	defer server.Close()

	_, err := NewImageFetcher(1024).Fetch(context.Background(), server.URL+"/image.png")

	if !errors.Is(err, ErrForbiddenAddress) {
		t.Errorf("expected ErrForbiddenAddress, got %v", err)
	}
}

func TestImageFetcher_Fetch(t *testing.T) {
	picture := testPNG(t, 3, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("/logo.png", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain") // the real type is detected from the content
		w.Write(picture)
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/logo.png", http.StatusFound)
	})
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	})
	mux.HandleFunc("/text", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("<html>not an image</html>"))
	})
	mux.HandleFunc("/large.png", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(append(picture, make([]byte, 2048)...))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	fetcher := newImageFetcher(1024, func(netip.Addr) bool { return true })

	file, err := fetcher.Fetch(context.Background(), server.URL+"/redirect")
	if err != nil {
		t.Fatal(err)
	}
	if width, height := file.ImageSize(); file.Name != "logo.png" || file.Type != "image/png" || width != 3 || height != 2 {
		t.Errorf("unexpected file %s %s %dx%d", file.Name, file.Type, width, height)
	}

	for path, expected := range map[string]error{
		"/text":      ErrNotImage,
		"/large.png": ErrTooLarge,
		"/missing":   ErrDownloadFailed,
		"/loop":      ErrUnsupportedURL,
	} {
		if _, err := fetcher.Fetch(context.Background(), server.URL+path); !errors.Is(err, expected) {
			t.Errorf("%s: expected %v, got %v", path, expected, err)
		}
	}
	for _, target := range []string{"ftp://example.com/logo.png", "file:///etc/passwd", "https:///logo.png", "logo.png"} {
		if _, err := fetcher.Fetch(context.Background(), target); !errors.Is(err, ErrUnsupportedURL) {
			t.Errorf("%s: expected ErrUnsupportedURL, got %v", target, err)
		}
	}
}

func TestFileFromDataURI(t *testing.T) {
	picture := testPNG(t, 2, 2)
	encoded := base64.StdEncoding.EncodeToString(picture)

	file, err := FileFromDataURI("data:image/png;base64,"+encoded[:10]+"\n "+encoded[10:], 1024)
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "image.png" || !bytes.Equal(file.Content, picture) || !file.IsWebImage() {
		t.Errorf("unexpected file %s %s", file.Name, file.Type)
	}

	tests := map[string]struct {
		input    string
		limit    int
		expected error
	}{
		"too large":     {"data:image/png;base64," + encoded, 10, ErrTooLarge},
		"not an image":  {"data:text/plain;base64,aGVsbG8=", 1024, ErrNotImage},
		"svg":           {"data:image/svg+xml,<svg></svg>", 1024, ErrNotImage},
		"not data":      {"https://example.com/image.png", 1024, ErrUnsupportedURL},
		"no data":       {"data:image/png;base64", 1024, ErrUnsupportedURL},
		"invalid data":  {"data:image/png;base64,!!!", 1024, nil},
		"invalid chars": {"data:image/png,%zz", 1024, nil},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := FileFromDataURI(test.input, test.limit)
			if err == nil || (test.expected != nil && !errors.Is(err, test.expected)) {
				t.Errorf("expected %v, got %v", test.expected, err)
			}
		})
	}
}

func TestNewFile_DefaultName(t *testing.T) {
	if name := NewFile("", testPNG(t, 1, 1)).Name; name != "image.png" {
		t.Errorf("unexpected image name %q", name)
	}
	if name := NewFile("", []byte("hello")).Name; name != "file.txt" {
		t.Errorf("unexpected file name %q", name)
	}
	if name := NewFile("report.pdf", []byte("hello")).Name; name != "report.pdf" {
		t.Errorf("the given name was not kept: %q", name)
	}
}
