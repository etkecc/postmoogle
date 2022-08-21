package smtp

import (
	"log"
	"os"
	"time"

	"github.com/emersion/go-smtp"
	"maunium.net/go/mautrix/id"
)

type backend struct {
	domain string
	rooms  map[string]id.RoomID
}

func (b *backend) newSession() *session {
	return &session{
		domain: b.domain,
		rooms:  b.rooms,
	}
}

func (b *backend) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	return nil, smtp.ErrAuthUnsupported
}

func (b *backend) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	return b.newSession(), nil
}

func NewServer(domain string, mapping map[string]id.RoomID, port string) *smtp.Server {
	be := &backend{
		domain: domain,
		rooms:  mapping,
	}
	s := smtp.NewServer(be)
	s.Addr = ":" + port
	s.Domain = domain
	s.AuthDisabled = true
	s.ReadTimeout = 360 * time.Second
	s.WriteTimeout = 360 * time.Second
	s.MaxMessageBytes = 63 * 1024
	s.MaxRecipients = 50
	s.Debug = os.Stdout

	log.Println("Starting SMTP server")
	log.Fatal(s.ListenAndServe())
	return s
}
