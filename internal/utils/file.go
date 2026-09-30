package utils

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // register GIF decoder for ImageSize
	_ "image/jpeg" // register JPEG decoder for ImageSize
	_ "image/png"  // register PNG decoder for ImageSize
	"strings"
	"sync"

	"github.com/gabriel-vasile/mimetype"
	_ "golang.org/x/image/webp" // register WebP decoder for ImageSize
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

const (
	// maxImageSide is the largest width or height of an image shown inside a message, in pixels
	maxImageSide = 8192
	// maxImagePixels limits the memory needed to decode an image, 16 megapixels take up to 128 MiB
	maxImagePixels = 4096 * 4096
)

var (
	// webImageTypes are image types that Matrix clients can display inline
	webImageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

	// ErrImageTooLarge is returned for images with too many pixels to decode safely
	ErrImageTooLarge = errors.New("image dimensions are too large")

	// decodeMu decodes one image at a time, so that several large images cannot use up the memory together
	decodeMu sync.Mutex
)

type File struct {
	Name      string
	Type      string
	MsgType   event.MessageType
	Length    int
	Content   []byte
	ContentID string // Content-ID of an inline MIME part, referenced by cid: links in the email HTML
}

func NewFile(name string, content []byte) *File {
	file := &File{
		Name:    name,
		Content: content,
	}
	file.Length = len(content)

	mtype := mimetype.Detect(content)
	file.Type = mtype.String()
	file.MsgType = mimeMsgType(file.Type)
	if file.Name == "" {
		file.Name = file.defaultName() + mtype.Extension()
	}

	return file
}

// IsWebImage reports whether the file content starts like an image that Matrix clients can display inline
func (f *File) IsWebImage() bool {
	return webImageTypes[strings.SplitN(f.Type, ";", 2)[0]]
}

// ImageSize decodes the whole image and returns its size; crafted files with a valid header only are refused
func (f *File) ImageSize() (width, height int, err error) {
	if !f.IsWebImage() {
		return 0, 0, ErrNotImage
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(f.Content))
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ErrNotImage, err)
	}
	if "image/"+format != f.Type {
		return 0, 0, fmt.Errorf("%w: %s content in a %s file", ErrNotImage, format, f.Type)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxImageSide || cfg.Height > maxImageSide ||
		cfg.Width*cfg.Height > maxImagePixels {
		return 0, 0, fmt.Errorf("%w: %dx%d", ErrImageTooLarge, cfg.Width, cfg.Height)
	}

	decodeMu.Lock()
	defer decodeMu.Unlock()
	img, _, err := image.Decode(bytes.NewReader(f.Content))
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ErrNotImage, err)
	}
	return img.Bounds().Dx(), img.Bounds().Dy(), nil
}

func (f *File) Convert() *mautrix.ReqUploadMedia {
	return &mautrix.ReqUploadMedia{
		ContentBytes:  f.Content,
		Content:       bytes.NewReader(f.Content),
		ContentLength: int64(f.Length),
		ContentType:   f.Type,
		FileName:      f.Name,
	}
}

// defaultName names files that come without a name by their kind, e.g. image for image.png
func (f *File) defaultName() string {
	switch f.MsgType {
	case event.MsgImage:
		return "image"
	case event.MsgVideo:
		return "video"
	case event.MsgAudio:
		return "audio"
	default:
		return "file"
	}
}

func mimeMsgType(mime string) event.MessageType {
	if mime == "" {
		return event.MsgFile
	}
	if !strings.Contains(mime, "/") {
		return event.MsgFile
	}
	msection := strings.Split(mime, "/")[0]
	switch msection {
	case "image":
		return event.MsgImage
	case "video":
		return event.MsgVideo
	case "audio":
		return event.MsgAudio
	default:
		return event.MsgFile
	}
}
