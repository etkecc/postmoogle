package email

import (
	"net/url"
	"strings"

	"github.com/etkecc/postmoogle/internal/utils"
)

// image source kinds
const (
	ImageInline = "inline" // cid: link to an inline attachment
	ImageData   = "data"   // data: URI embedded into the HTML
	ImageRemote = "remote" // http(s) URL
)

// minPhotoSize is the shortest longer side of a photo in pixels, smaller images are logos, icons, and signatures
const minPhotoSize = 400

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

// ImageKind returns the kind of an image source, or an empty string if it is not supported
func ImageKind(src string) string {
	switch {
	case hasPrefixFold(src, "cid:"):
		return ImageInline
	case hasPrefixFold(src, "data:image/"):
		return ImageData
	case hasPrefixFold(src, "https://"), hasPrefixFold(src, "http://"), strings.HasPrefix(src, "//"):
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
	renderHTML(e.HTML, func(src string) *Image {
		if !seen[src] && ImageKind(src) != "" {
			seen[src] = true
			sources = append(sources, src)
		}
		return nil
	}, false)
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

// InlinesToSend returns inline attachments not shown inside the bodies, and photos, to open them in full size
func (e *Email) InlinesToSend(bodies ...string) []*utils.File {
	files := make([]*utils.File, 0, len(e.InlineFiles))
	for _, file := range e.InlineFiles {
		img := e.UploadedImage(file)
		if img == nil || img.IsPhoto() || !shownIn(img, bodies) {
			files = append(files, file)
		}
	}
	return files
}

func shownIn(img *Image, bodies []string) bool {
	for _, body := range bodies {
		if strings.Contains(body, img.URI) {
			return true
		}
	}
	return false
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

func hasPrefixFold(text, prefix string) bool {
	return len(text) >= len(prefix) && strings.EqualFold(text[:len(prefix)], prefix)
}
