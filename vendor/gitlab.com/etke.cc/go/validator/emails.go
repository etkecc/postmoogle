package validator

import (
	"fmt"
	"net"
	"net/mail"
	"strings"
	"time"

	"blitiri.com.ar/go/spf"
	"gitlab.com/etke.cc/go/trysmtp"
	"golang.org/x/net/context"
)

// Email checks if email is valid
func (v *V) Email(email string, optionalSenderIP ...net.IP) bool {

	// edge case: email may be optional
	if email == "" {
		return !v.enforce.Email
	}

	address, err := mail.ParseAddress(email)
	if err != nil {
		v.log.Info("email %s invalid, reason: %v", email, err)
		return false
	}
	email = address.Address
	return v.emailChecks(email, optionalSenderIP...)
}

func (v *V) emailChecks(email string, optionalSenderIP ...net.IP) bool {
	maxChecks := 4
	var senderIP net.IP
	if len(optionalSenderIP) > 0 {
		senderIP = optionalSenderIP[0]
	}
	errchan := make(chan error, maxChecks)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go v.emailSpamlist(ctx, email, errchan)
	go v.emailNoMX(ctx, email, errchan)
	go v.emailNoSPF(ctx, email, senderIP, errchan)
	go v.emailNoSMTP(ctx, email, errchan)

	var checks int
	for {
		checks++
		err := <-errchan
		if err != nil {
			v.log.Info("email %q is invalid, reason: %v", email, err)
			return false
		}
		if checks >= maxChecks {
			return true
		}
	}
}

func (v *V) emailSpamlist(ctx context.Context, email string, errchan chan error) {
	select {
	case <-ctx.Done():
		return
	default:
		emailb := []byte(email)
		for _, spamregex := range v.spamlist {
			if spamregex.Match(emailb) {
				errchan <- fmt.Errorf("spamlist")
				return
			}
		}
		errchan <- nil
	}
}

func (v *V) emailNoMX(ctx context.Context, email string, errchan chan error) {
	select {
	case <-ctx.Done():
		return
	default:
		if !v.enforce.MX {
			errchan <- nil
			return
		}

		at := strings.LastIndex(email, "@")
		domain := email[at+1:]
		if !v.MX(domain) {
			v.log.Info("email %s domain %s invalid, reason: no MX", email, domain)
			errchan <- fmt.Errorf("no MX")
			return
		}
		errchan <- nil
	}
}

func (v *V) emailNoSMTP(ctx context.Context, email string, errchan chan error) {
	select {
	case <-ctx.Done():
		return
	default:
		if !v.enforce.SMTP {
			errchan <- nil
			return
		}

		client, err := trysmtp.Connect(v.from, email)
		if err != nil {
			if strings.HasPrefix(err.Error(), "45") {
				v.log.Info("email %s may be invalid, reason: SMTP check (%v)", email, err)
				errchan <- nil
				return
			}

			v.log.Info("email %s invalid, reason: SMTP check (%v)", email, err)
			errchan <- fmt.Errorf("SMTP")
			return
		}
		client.Close()
		errchan <- nil
	}
}

func (v *V) emailNoSPF(ctx context.Context, email string, senderIP net.IP, errchan chan error) {
	select {
	case <-ctx.Done():
		return
	default:
		if !v.enforce.SPF {
			errchan <- nil
			return
		}

		result, _ := spf.CheckHostWithSender(senderIP, "", email, spf.WithTraceFunc(v.log.Info)) //nolint:errcheck // not a error
		if result == spf.Fail {
			errchan <- fmt.Errorf("SPF")
			return
		}
		errchan <- nil
	}
}
