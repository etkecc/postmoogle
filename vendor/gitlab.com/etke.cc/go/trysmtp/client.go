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

func unwrapErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	tokens := strings.Repeat("%v; ", len(errs))
	// make it compatible with < 1.18
	ierrs := make([]interface{}, len(errs))
	for _, err := range errs {
		ierrs = append(ierrs, err)
	}

	return fmt.Errorf(tokens, ierrs...)
}

func initClient(localname, hostname string) (*smtp.Client, error) {
	mxs, err := net.LookupMX(hostname)
	if err != nil {
		return nil, err
	}

	cerrs := []error{}
	var client *smtp.Client
	for _, mx := range mxs {
		for _, addr := range SMTPAddrs {
			client, err = trySMTP(localname, strings.TrimSuffix(mx.Host, "."), addr)
			if err != nil {
				cerrs = append(cerrs, err)
			}
			if client != nil {
				return client, unwrapErrors(cerrs)
			}
		}
	}

	// If there are no MX records, according to https://datatracker.ietf.org/doc/html/rfc5321#section-5.1,
	// we're supposed to try talking directly to the host.
	if len(mxs) == 0 {
		for _, addr := range SMTPAddrs {
			client, err = trySMTP(localname, hostname, addr)
			if err != nil {
				cerrs = append(cerrs, err)
			}
			if client != nil {
				return client, unwrapErrors(cerrs)
			}
		}
	}

	return nil, unwrapErrors(cerrs)
}

func trySMTP(localname, mxhost, addr string) (*smtp.Client, error) {
	target := mxhost + addr
	conn, err := smtp.Dial(target)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", target, err)
	}
	err = conn.Hello(localname)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", target, err)
	}
	if ok, _ := conn.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: mxhost}
		conn.StartTLS(config) //nolint:errcheck // if it doesn't work - we can't do anything anyway
	}

	return conn, nil
}
