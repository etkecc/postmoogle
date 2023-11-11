package smtp

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strings"

	"github.com/rs/zerolog"
)

type MailSender interface {
	Send(from, to, data string) error
}

// SMTP client
type Client struct {
	config *RelayConfig
	log    *zerolog.Logger
}

var errNoSMTP = fmt.Errorf("cannot connect to any SMTP server")

func newClient(cfg *RelayConfig, log *zerolog.Logger) *Client {
	return &Client{
		config: cfg,
		log:    log,
	}
}

// Send email
func (c Client) Send(from, to, data string) error {
	log := c.log.With().Str("from", from).Str("to", to).Logger()
	log.Debug().Msg("sending email")

	var conn *smtp.Client
	var err error
	if c.config.Host != "" {
		log.Debug().Msg("creating relay client...")
		conn, err = c.createRelayClient(from, to)
	} else {
		log.Debug().Msg("trying direct SMTP connection...")
		conn, err = c.createDirectClient(from, to)
	}

	if conn == nil {
		log.Error().Err(err).Str("server_of", to).Msg("cannot connect to SMTP server")
		return err
	}
	if err != nil {
		log.Warn().Err(err).Str("server_of", to).Msg("connection to the SMTP server returned non-fatal error(-s)")
	}
	defer conn.Close()

	var w io.WriteCloser
	w, err = conn.Data()
	if err != nil {
		log.Error().Err(err).Msg("cannot send DATA command")
		return err
	}
	defer w.Close()
	log.Debug().Str("DATA", data).Msg("sending command")
	_, err = strings.NewReader(data).WriteTo(w)
	if err != nil {
		log.Error().Err(err).Msg("cannot write DATA")
		return err
	}

	log.Debug().Msg("email has been sent")
	return nil
}

// createRelayClientconnects directly to the provided smtp host
func (c *Client) createRelayClient(from, to string) (*smtp.Client, error) {
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
		config := &tls.Config{ServerName: c.config.Host} //nolint:gosec // it's smtp, even that is too strict sometimes
		conn.StartTLS(config)                            //nolint:errcheck // if it doesn't work - we can't do anything anyway
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

func (c *Client) createDirectClient(from, to string) (*smtp.Client, error) {
	localname := strings.SplitN(from, "@", 2)[1]
	hostname := strings.SplitN(to, "@", 2)[1]
	client, cerr := c.trySMTP(localname, hostname)
	if client == nil {
		c.log.Warn().Err(cerr).Str("from", from).Str("to", to).Msg("cannot create direct SMTP client")
		return nil, cerr
	}

	err := client.Mail(from)
	if err != nil {
		c.log.Warn().Err(err).Str("from", from).Str("to", to).Msg("cannot send MAIL command")
		client.Close()
		return nil, err
	}

	err = client.Rcpt(to)
	if err != nil {
		c.log.Warn().Err(err).Str("from", from).Str("to", to).Msg("cannot send RCPT command")
		client.Close()
		return nil, err
	}

	return client, cerr
}

func (c *Client) trySMTP(localname, hostname string) (*smtp.Client, error) {
	mxs, err := net.LookupMX(hostname)
	if err != nil {
		return nil, err
	}

	var client *smtp.Client
	for _, mx := range mxs {
		if mx.Host == "." {
			continue // no records case
		}
		client = c.connect(localname, hostname, strings.TrimSuffix(mx.Host, "."))
		if client != nil {
			return client, nil
		}
	}

	// If there are no MX records, according to https://datatracker.ietf.org/doc/html/rfc5321#section-5.1,
	// we're supposed to try talking directly to the host.
	client = c.connect(localname, hostname, hostname)
	if client != nil {
		return client, nil
	}

	return nil, errNoSMTP
}

func (c *Client) connect(localname, serverOf, mxhost string) *smtp.Client {
	target := mxhost + ":25"
	conn, err := smtp.Dial(target)
	if err != nil {
		c.log.Warn().Err(err).Str("target", serverOf).Str("host", mxhost).Msg("cannot dial SMTP server")
		return nil
	}
	err = conn.Hello(localname)
	if err != nil {
		c.log.Warn().Err(err).Str("target", serverOf).Str("host", mxhost).Msg("cannot hello SMTP server")
		return nil
	}
	if ok, _ := conn.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: mxhost} //nolint:gosec // it's smtp, even that is too strict sometimes
		conn.StartTLS(config)                     //nolint:errcheck // if it doesn't work - we can't do anything anyway
	}

	return conn
}
