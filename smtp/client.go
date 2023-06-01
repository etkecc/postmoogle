package smtp

import (
	"crypto/tls"
	"io"
	"net/smtp"
	"strings"

	"github.com/rs/zerolog"
	"gitlab.com/etke.cc/go/trysmtp"
)

type MailSender interface {
	Send(from string, to string, data string) error
}

// SMTP client
type Client struct {
	config *RelayConfig
	log    *zerolog.Logger
}

func newClient(cfg *RelayConfig, log *zerolog.Logger) *Client {
	return &Client{
		config: cfg,
		log:    log,
	}
}

// Send email
func (c Client) Send(from string, to string, data string) error {
	c.log.Debug().Str("from", from).Str("to", to).Msg("sending email")

	var conn *smtp.Client
	var err error
	if c.config.Host != "" {
		conn, err = c.createDirectClient(from, to)
	} else {
		conn, err = trysmtp.Connect(from, to)
	}

	if conn == nil {
		c.log.Error().Err(err).Str("server_of", to).Msg("cannot connect to SMTP server")
		return err
	}
	if err != nil {
		c.log.Warn().Err(err).Str("server_of", to).Msg("connection to the SMTP server returned non-fatal error(-s)")
	}
	defer conn.Close()

	var w io.WriteCloser
	w, err = conn.Data()
	if err != nil {
		c.log.Error().Err(err).Msg("cannot send DATA command")
		return err
	}
	defer w.Close()
	c.log.Debug().Str("DATA", data).Msg("sending command")
	_, err = strings.NewReader(data).WriteTo(w)
	if err != nil {
		c.log.Error().Err(err).Msg("cannot write DATA")
		return err
	}

	c.log.Debug().Msg("email has been sent")
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
