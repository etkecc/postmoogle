package smtp

import (
	"io"
	"log"

	"github.com/emersion/go-smtp"
	"maunium.net/go/mautrix/id"
)

type session struct {
	domain string
	rooms  map[string]id.RoomID
}

func (s *session) Mail(from string, opts smtp.MailOptions) error {
	log.Println("mail from", from)
	return nil
}

func (s *session) Rcpt(to string) error {
	_, ok := s.rooms[to]
	if !ok {
		return smtp.ErrAuthRequired
	}

	if Domain(to) != s.domain {
		return smtp.ErrAuthRequired
	}
	log.Println("rcpt to", to)
	return nil
}

func (s *session) Data(r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	log.Println("Data", string(b))
	return nil
}

func (s *session) Reset() {}

func (s *session) Logout() error {
	return nil
}
