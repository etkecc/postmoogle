package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strings"

	"gitlab.com/etke.cc/go/logger"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// Bot interface to send emails into matrix
type Bot interface {
	GetMapping(string) (id.RoomID, bool)
	Send2Matrix(ctx context.Context, email *utils.Email) error
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
	conn, err := m.connect(from, to)
	if err != nil {
		m.log.Error("cannot connect to SMTP server of %s: %v", to, err)
		return err
	}
	defer conn.Close()
	err = conn.Mail(from)
	if err != nil {
		m.log.Error("cannot call MAIL command: %v", err)
		return err
	}
	err = conn.Rcpt(to)
	if err != nil {
		m.log.Error("cannot send RCPT command: %v", err)
		return err
	}

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

func (m *mta) tryServer(localname, mxhost string) *smtp.Client {
	m.log.Debug("trying SMTP connection to %s", mxhost)
	conn, err := smtp.Dial(mxhost + ":smtp")
	if err != nil {
		m.log.Warn("cannot connect to the %s: %v", mxhost, err)
		return nil
	}
	err = conn.Hello(localname)
	if err != nil {
		m.log.Warn("cannot call HELLO command of the %s: %v", mxhost, err)
		return nil
	}
	if ok, _ := conn.Extension("STARTTLS"); ok {
		m.log.Debug("%s supports STARTTLS", mxhost)
		config := &tls.Config{ServerName: mxhost}
		err = conn.StartTLS(config)
		if err != nil {
			m.log.Warn("STARTTLS connection to the %s failed: %v", mxhost, err)
		}
	}

	return conn
}

func (m *mta) connect(from, to string) (*smtp.Client, error) {
	localname := strings.SplitN(from, "@", 2)[1]
	hostname := strings.SplitN(to, "@", 2)[1]

	m.log.Debug("performing MX lookup of %s", hostname)
	mxs, err := net.LookupMX(hostname)
	if err != nil {
		m.log.Error("cannot perform MX lookup: %v", err)
		return nil, err
	}

	for _, mx := range mxs {
		client := m.tryServer(localname, mx.Host)
		if client != nil {
			return client, nil
		}
	}

	return nil, fmt.Errorf("target SMTP server not found")
}
