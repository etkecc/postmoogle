package validator

import (
	"net/mail"
	"strings"

	"gitlab.com/etke.cc/go/trysmtp"
)

// Email checks if email is valid
func (v *V) Email(email string) bool {
	// edge case: email may be optional
	if email == "" {
		return !v.enforce.Email
	}

	length := len(email)
	// email cannot too short and too big
	if length < 3 || length > 254 {
		v.log.Info("email %s invalid, reason: length", email)
		return false
	}

	_, err := mail.ParseAddress(email)
	if err != nil {
		v.log.Info("email %s invalid, reason: %v", email, err)
		return false
	}

	emailb := []byte(email)
	for _, spamregex := range v.spamlist {
		if spamregex.Match(emailb) {
			v.log.Info("email %s invalid, reason: spamlist", email)
			return false
		}
	}

	if v.enforce.MX {
		if v.emailNoMX(email) {
			return false
		}
	}

	if v.enforce.SMTP {
		if v.emailNoSMTP(email) {
			return false
		}
	}

	return true
}

func (v *V) emailNoMX(email string) bool {
	at := strings.LastIndex(email, "@")
	domain := email[at+1:]

	nomx := !v.MX(domain)
	if nomx {
		v.log.Info("email %s domain %s invalid, reason: no MX", email, domain)
		return true
	}

	return false
}

func (v *V) emailNoSMTP(email string) bool {
	client, err := trysmtp.Connect(v.from, email)
	if err != nil {
		if strings.HasPrefix(err.Error(), "45") {
			v.log.Info("email %s may be invalid, reason: SMTP check (%v)", email, err)
			return false
		}

		v.log.Info("email %s invalid, reason: SMTP check (%v)", email, err)
		return true
	}
	defer client.Close()

	return false
}
