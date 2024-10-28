package email

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"maunium.net/go/mautrix/id"
)

var styleRegex = regexp.MustCompile("<style((.|\n|\r)*?)</style>")

// AddressValid checks if email address is valid
func AddressValid(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// MessageID generates email Message-Id from matrix event ID
func MessageID(eventID id.EventID, domain string) string {
	return fmt.Sprintf("<%s@%s>", eventID, domain)
}

// Address gets email address from a valid email address notation (eg: "Jane Doe" <jane@example.com> -> jane@example.com)
func Address(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, _ := mail.ParseAddress(email) //nolint:errcheck // if it fails here, nothing will help
	if addr == nil {
		list := AddressList(email)
		if len(list) > 0 {
			return strings.Join(list, ",")
		}
		return email
	}

	return addr.Address
}

// Address gets email address from a valid email address notation (eg: "Jane Doe" <jane@example.com>, john.doe@example.com -> jane@example.com, john.doe@example.com)
func AddressList(emailList string) []string {
	if emailList == "" {
		return []string{}
	}
	emailList = strings.ToLower(strings.TrimSpace(emailList))
	list, _ := mail.ParseAddressList(emailList) //nolint:errcheck // if it fails here, nothing will help
	if len(list) == 0 {
		return []string{}
	}

	addrs := make([]string, 0, len(list))
	for _, addr := range list {
		addrs = append(addrs, addr.Address)
	}

	return addrs
}

// dateNow returns Date in RFC1123 with numeric timezone
func dateNow(original ...time.Time) string {
	now := time.Now().UTC()
	if len(original) > 0 && !original[0].IsZero() {
		now = original[0]
	}

	return now.Format(time.RFC1123Z)
}
