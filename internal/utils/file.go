package utils

import (
	"bytes"
	"image"
	_ "image/gif"  // register GIF decoder for ImageSize
	_ "image/jpeg" // register JPEG decoder for ImageSize
	_ "image/png"  // register PNG decoder for ImageSize
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

// webImageTypes are image types that Matrix clients can display inline
var webImageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

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
		file.Name = defaultName(file.MsgType) + mtype.Extension()
	}

	return file
}

// IsWebImage reports whether the file is an image that Matrix clients can display inline
func (f *File) IsWebImage() bool {
	return webImageTypes[strings.SplitN(f.Type, ";", 2)[0]]
}

// ImageSize returns the image dimensions in pixels, or zeros if they cannot be detected
func (f *File) ImageSize() (width, height int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(f.Content))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
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

func defaultName(msgtype event.MessageType) string {
	switch msgtype {
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
