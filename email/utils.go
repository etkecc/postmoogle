package email

import (
	"fmt"
	"net/mail"
	"regexp"
	"time"

	"maunium.net/go/mautrix/id"
)

var styleRegex = regexp.MustCompile("<style((.|\n|\r)*?)<\\/style>")

// AddressValid checks if email address is valid
func AddressValid(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// MessageID generates email Message-Id from matrix event ID
func MessageID(eventID id.EventID, domain string) string {
	return fmt.Sprintf("<%s@%s>", eventID, domain)
}

// dateNow returns Date in RFC1123 with numeric timezone
func dateNow(original ...time.Time) string {
	now := time.Now().UTC()
	if len(original) > 0 && !original[0].IsZero() {
		now = original[0]
	}

	return now.Format(time.RFC1123Z)
}
