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

// Image is an email image uploaded to the Matrix content repository
type Image struct {
	URI    string      // mxc:// URI of the uploaded image
	Width  int         // natural width in pixels, 0 if unknown
	Height int         // natural height in pixels, 0 if unknown
	File   *utils.File // inline attachment the image was made from, nil for other images
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

// InlineFile returns the inline attachment referenced by a cid: image source
func (e *Email) InlineFile(src string) *utils.File {
	if ImageKind(src) != ImageInline {
		return nil
	}
	cid := src[len("cid:"):]
	if unescaped, err := url.QueryUnescape(cid); err == nil {
		cid = unescaped
	}
	cid = strings.Trim(strings.TrimSpace(cid), "<>")
	for _, file := range e.InlineFiles {
		if file.ContentID == cid {
			return file
		}
	}
	for _, file := range e.InlineFiles {
		if file.ContentID != "" && strings.EqualFold(file.ContentID, cid) {
			return file
		}
	}
	return nil
}

// UnembeddedInlines returns inline attachments that are not shown inside any of the given formatted bodies
func (e *Email) UnembeddedInlines(bodies ...string) []*utils.File {
	files := make([]*utils.File, 0, len(e.InlineFiles))
	for _, file := range e.InlineFiles {
		if !e.embedded(file, bodies) {
			files = append(files, file)
		}
	}
	return files
}

func (e *Email) embedded(file *utils.File, bodies []string) bool {
	for _, img := range e.Images {
		if img == nil || img.File != file {
			continue
		}
		for _, body := range bodies {
			if strings.Contains(body, img.URI) {
				return true
			}
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
