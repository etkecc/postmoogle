package utils

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"testing"
)

// testWebP is a 1x1 lossless WebP image
const testWebP = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="

func TestFile_ImageSize(t *testing.T) {
	var jpegImage, gifImage bytes.Buffer
	if err := jpeg.Encode(&jpegImage, image.NewRGBA(image.Rect(0, 0, 5, 4)), nil); err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(&gifImage, image.NewPaletted(image.Rect(0, 0, 6, 3), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	webp, err := base64.StdEncoding.DecodeString(testWebP)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		content       []byte
		width, height int
	}{
		"png":  {testPNG(t, 3, 2), 3, 2},
		"jpeg": {jpegImage.Bytes(), 5, 4},
		"gif":  {gifImage.Bytes(), 6, 3},
		"webp": {webp, 1, 1},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			width, height, err := NewFile("", test.content).ImageSize()
			if err != nil || width != test.width || height != test.height {
				t.Errorf("expected %dx%d, got %dx%d, %v", test.width, test.height, width, height, err)
			}
		})
	}
}

func TestFile_ImageSizeRefusesFakes(t *testing.T) {
	tests := map[string]struct {
		content  []byte
		expected error
	}{
		// "<s" and "cr" of "<script>" are read as 29500x29283 pixels by a GIF header check
		"html polyglot with a huge size":  {[]byte("GIF89a<script>alert(1)</script>"), ErrImageTooLarge},
		"html polyglot with a small size": {append([]byte("GIF89a\x0a\x00\x0a\x00\x00\x00\x00"), "<html>hi</html>"...), ErrNotImage},
		"png header only":                 {testPNG(t, 3, 2)[:40], ErrNotImage},
		"text":                            {[]byte("hello"), ErrNotImage},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := NewFile("", test.content).ImageSize(); !errors.Is(err, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, err)
			}
		})
	}
}

func TestFile_ImageSizeRefusesTooManyPixels(t *testing.T) {
	var picture bytes.Buffer
	if err := gif.Encode(&picture, image.NewPaletted(image.Rect(0, 0, 8193, 1), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}

	if _, _, err := NewFile("", picture.Bytes()).ImageSize(); !errors.Is(err, ErrImageTooLarge) {
		t.Errorf("expected ErrImageTooLarge, got %v", err)
	}
}
