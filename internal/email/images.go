package email

import (
	"html"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/etkecc/postmoogle/internal/utils"
)

// image source kinds
const (
	ImageInline = "inline" // cid: link to an inline attachment
	ImageData   = "data"   // data: URI embedded into the HTML
	ImageRemote = "remote" // http(s) URL
)

const (
	// minPhotoSize is the shortest longer side of a photo in pixels, smaller images are logos, icons, and signatures
	minPhotoSize = 400
	// maxImageWidth is the widest an embedded image is displayed, in pixels
	maxImageWidth = 600
	// maxDimension caps image dimensions parsed from HTML, in pixels
	maxDimension = 10000
)

// Image is an email image uploaded to the Matrix content repository
type Image struct {
	URI    string      // mxc:// URI of the uploaded image
	Width  int         // natural width in pixels, 0 if unknown
	Height int         // natural height in pixels, 0 if unknown
	File   *utils.File // attachment the image was made from, nil for other images
}

// IsPhoto reports whether the image is big enough to be a photo rather than a logo or an icon
func (img *Image) IsPhoto() bool {
	return max(img.Width, img.Height) >= minPhotoSize
}

// tag returns the HTML tag showing the image with the given size; a size below 1 is left to the client
func (img *Image) tag(alt, title string, width, height int) string {
	var tag strings.Builder
	tag.WriteString(`<img src="` + html.EscapeString(img.URI) + `"`)
	if alt != "" {
		tag.WriteString(` alt="` + html.EscapeString(alt) + `"`)
	}
	if title != "" {
		tag.WriteString(` title="` + html.EscapeString(title) + `"`)
	}
	if width > 0 {
		tag.WriteString(` width="` + strconv.Itoa(width) + `"`)
	}
	if height > 0 {
		tag.WriteString(` height="` + strconv.Itoa(height) + `"`)
	}
	tag.WriteString(">")
	return tag.String()
}

// displaySize calculates the size the image is shown at, keeping its aspect ratio; -1 means unknown
func (img *Image) displaySize(width, height int) (displayWidth, displayHeight int) {
	if img.Width > 0 && img.Height > 0 {
		switch {
		case width < 0 && height < 0:
			width, height = img.scale(img.Width, 1, 1), img.scale(img.Height, 1, 1)
		case height < 0:
			height = img.scale(width, img.Height, img.Width)
		case width < 0:
			width = img.scale(height, img.Width, img.Height)
		}
	}
	if width > maxImageWidth {
		if height > 0 {
			height = img.scale(height, maxImageWidth, width)
		}
		width = maxImageWidth
	}
	return width, height
}

// scale returns value * numerator / denominator, limited to maxDimension to stay safe with huge images
func (img *Image) scale(value, numerator, denominator int) int {
	return int(math.Min(float64(value)*float64(numerator)/float64(denominator), maxDimension))
}

// shownIn reports whether the image is shown inside any of the formatted bodies
func (img *Image) shownIn(bodies []string) bool {
	for _, body := range bodies {
		if strings.Contains(body, img.URI) {
			return true
		}
	}
	return false
}

// ImageKind returns the kind of an image source, or an empty string if it is not supported
func ImageKind(src string) string {
	prefixed := func(prefix string) bool {
		return len(src) >= len(prefix) && strings.EqualFold(src[:len(prefix)], prefix)
	}
	switch {
	case prefixed("cid:"):
		return ImageInline
	case prefixed("data:image/"):
		return ImageData
	case prefixed("https://"), prefixed("http://"), strings.HasPrefix(src, "//"):
		return ImageRemote
	default:
		return ""
	}
}

// ImageSources returns unique supported sources of the images shown in the email HTML, in document order
func (e *Email) ImageSources() []string {
	if e.HTML == "" {
		return nil
	}
	seen := map[string]bool{}
	sources := []string{}
	newRenderer(func(src string) *Image {
		if !seen[src] && ImageKind(src) != "" {
			seen[src] = true
			sources = append(sources, src)
		}
		return nil
	}, false).render(e.HTML)
	return sources
}

// SetImage stores the uploaded image for the source; nil marks a source that cannot be shown
func (e *Email) SetImage(src string, img *Image) {
	if e.Images == nil {
		e.Images = map[string]*Image{}
	}
	e.Images[src] = img
}

// HasImage reports whether the image source was already processed
func (e *Email) HasImage(src string) bool {
	_, ok := e.Images[src]
	return ok
}

// CIDFile returns the attachment referenced by a cid: image source, inline or not
func (e *Email) CIDFile(src string) *utils.File {
	if ImageKind(src) != ImageInline {
		return nil
	}
	cid := src[len("cid:"):]
	if unescaped, err := url.QueryUnescape(cid); err == nil {
		cid = unescaped
	}
	cid = strings.Trim(strings.TrimSpace(cid), "<>")
	lists := [][]*utils.File{e.InlineFiles, e.Files}
	for _, files := range lists {
		for _, file := range files {
			if file.ContentID == cid {
				return file
			}
		}
	}
	for _, files := range lists {
		for _, file := range files {
			if file.ContentID != "" && strings.EqualFold(file.ContentID, cid) {
				return file
			}
		}
	}
	return nil
}

// UploadedImage returns the image uploaded from the attachment to be shown inside the message, nil if there is none
func (e *Email) UploadedImage(file *utils.File) *Image {
	for _, img := range e.Images {
		if img != nil && img.File == file {
			return img
		}
	}
	return nil
}

// FilesToSend drops the files already shown as images inside the bodies, except photos, to open them in full size
func (e *Email) FilesToSend(files []*utils.File, bodies ...string) []*utils.File {
	toSend := make([]*utils.File, 0, len(files))
	for _, file := range files {
		img := e.UploadedImage(file)
		if img == nil || img.IsPhoto() || !img.shownIn(bodies) {
			toSend = append(toSend, file)
		}
	}
	return toSend
}

// imageResolver returns images allowed by the options, to be used for rendering
func (e *Email) imageResolver(options *ContentOptions) imageResolver {
	return func(src string) *Image {
		switch ImageKind(src) {
		case ImageRemote:
			if !options.RemoteImages {
				return nil
			}
		case ImageInline, ImageData:
			if !options.InlineImages {
				return nil
			}
		default:
			return nil
		}
		return e.Images[src]
	}
}
