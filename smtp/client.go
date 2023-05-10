package smtp

import (
	"crypto/tls"
	"io"
	"net/smtp"
	"strings"

	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/go/trysmtp"
)

type MailSender interface {
	Send(from string, to string, data string) error
}

// SMTP client
type Client struct {
	config *RelayConfig
	log    *logger.Logger
}

func newClient(cfg *RelayConfig, log *logger.Logger) *Client {
	return &Client{
		config: cfg,
		log:    log,
	}
}

// Send email
func (c Client) Send(from string, to string, data string) error {
	c.log.Debug("Sending email from %s to %s", from, to)

	var conn *smtp.Client
	var err error
	if c.config.Host != "" {
		conn, err = c.createDirectClient(from, to)
	} else {
		conn, err = trysmtp.Connect(from, to)
	}

	if conn == nil {
		c.log.Error("cannot connect to SMTP server of %s: %v", to, err)
		return err
	}
	if err != nil {
		c.log.Warn("connection to the SMTP server of %s returned the following non-fatal error(-s): %v", err)
	}
	defer conn.Close()

	var w io.WriteCloser
	w, err = conn.Data()
	if err != nil {
		c.log.Error("cannot send DATA command: %v", err)
		return err
	}
	defer w.Close()
	c.log.Debug("sending DATA:\n%s", data)
	_, err = strings.NewReader(data).WriteTo(w)
	if err != nil {
		c.log.Debug("cannot write DATA: %v", err)
		return err
	}

	c.log.Debug("email has been sent")
	return nil
}

// createDirectClient connects directly to the provided smtp host
func (c *Client) createDirectClient(from string, to string) (*smtp.Client, error) {
	localname := strings.SplitN(from, "@", 2)[1]
	target := c.config.Host + ":" + c.config.Port
	conn, err := smtp.Dial(target)
	if err != nil {
		return nil, err
	}

	err = conn.Hello(localname)
	if err != nil {
		return nil, err
	}

	if ok, _ := conn.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: c.config.Host}
		conn.StartTLS(config) //nolint:errcheck // if it doesn't work - we can't do anything anyway
	}

	if c.config.Usename != "" {
		err = conn.Auth(smtp.PlainAuth("", c.config.Usename, c.config.Password, c.config.Host))
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	err = conn.Mail(from)
	if err != nil {
		conn.Close()
		return nil, err
	}

	err = conn.Rcpt(to)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}
