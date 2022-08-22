package utils

import (
	"bytes"

	"maunium.net/go/mautrix"
)

type File struct {
	Name    string
	Type    string
	Length  int
	Content []byte
}

func NewFile(name, contentType string, content []byte) *File {
	file := &File{
		Name:    name,
		Type:    contentType,
		Content: content,
	}
	file.Length = len(content)

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
