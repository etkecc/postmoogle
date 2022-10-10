package smtp

import (
	"context"
	"io"
	"strings"

	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/go/trysmtp"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// Bot interface to send emails into matrix
type Bot interface {
	AllowAuth(string, string) bool
	GetMapping(string) (id.RoomID, bool)
	GetIFOptions(id.RoomID) utils.IncomingFilteringOptions
	Send2Matrix(ctx context.Context, email *utils.Email, incoming bool) error
	SetMTA(mta utils.MTA)
}

// mta is Mail Transfer Agent
type mta struct {
	log *logger.Logger
}

func NewMTA(loglevel string) utils.MTA {
	return &mta{
		log: logger.New("smtp/mta.", loglevel),
	}
}

func (m *mta) Send(from, to, data string) error {
	m.log.Debug("Sending email from %s to %s", from, to)
	conn, err := trysmtp.Connect(from, to)
	if err != nil {
		m.log.Error("cannot connect to SMTP server of %s: %v", to, err)
		return err
	}
	defer conn.Close()

	var w io.WriteCloser
	w, err = conn.Data()
	if err != nil {
		m.log.Error("cannot send DATA command: %v", err)
		return err
	}
	defer w.Close()
	m.log.Debug("sending DATA:\n%s", data)
	_, err = strings.NewReader(data).WriteTo(w)
	if err != nil {
		m.log.Debug("cannot write DATA: %v", err)
		return err
	}

	m.log.Debug("email has been sent")
	return nil
}
