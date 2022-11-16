package trysmtp

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// SMTPAddrs priority list
var SMTPAddrs = []string{":25", ":587", ":465"}

// Connect to SMTP server and call MAIL and RCPT commands
func Connect(from, to string) (*smtp.Client, error) {
	localname := strings.SplitN(from, "@", 2)[1]
	hostname := strings.SplitN(to, "@", 2)[1]
	client, err := initClient(localname, hostname)
	if err != nil {
		return nil, err
	}

	err = client.Mail(from)
	if err != nil {
		client.Close()
		return nil, err
	}

	err = client.Rcpt(to)
	if err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}

func initClient(localname, hostname string) (*smtp.Client, error) {
	mxs, err := net.LookupMX(hostname)
	if err != nil {
		return nil, err
	}

	for _, mx := range mxs {
		for _, addr := range SMTPAddrs {
			client := trySMTP(localname, strings.TrimSuffix(mx.Host, "."), addr)
			if client != nil {
				return client, nil
			}
		}
	}

	// If there are no MX records, according to https://datatracker.ietf.org/doc/html/rfc5321#section-5.1,
	// we're supposed to try talking directly to the host.
	if len(mxs) == 0 {
		for _, addr := range SMTPAddrs {
			client := trySMTP(localname, hostname, addr)
			if client != nil {
				return client, nil
			}
		}
	}

	return nil, fmt.Errorf("target SMTP server not found")
}

func trySMTP(localname, mxhost, addr string) *smtp.Client {
	conn, err := smtp.Dial(mxhost + addr)
	if err != nil {
		return nil
	}
	err = conn.Hello(localname)
	if err != nil {
		return nil
	}
	if ok, _ := conn.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: mxhost}
		conn.StartTLS(config) //nolint:errcheck // if it doesn't work - we can't do anything anyway
	}

	return conn
}
