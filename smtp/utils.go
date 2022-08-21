package smtp

import "strings"

func Domain(email string) string {
	return email[strings.LastIndex(email, "@")+1:]
}
