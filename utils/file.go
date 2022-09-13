package utils

import (
	"bytes"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

type File struct {
	Name    string
	Type    string
	MsgType event.MessageType
	Length  int
	Content []byte
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

	return file
}

func (f *File) Convert() mautrix.ReqUploadMedia {
	return mautrix.ReqUploadMedia{
		ContentBytes:  f.Content,
		Content:       bytes.NewReader(f.Content),
		ContentLength: int64(f.Length),
		ContentType:   f.Type,
		FileName:      f.Name,
	}
}

func mimeMsgType(mime string) event.MessageType {
	if mime == "" {
		return event.MsgFile
	}
	if !strings.Contains(mime, "/") {
		return event.MsgFile
	}
	msection := strings.SplitN(mime, "/", 1)[0]
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
