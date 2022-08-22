package utils

import "strings"

func Mailbox(email string) string {
	index := strings.LastIndex(email, "@")
	if index == -1 {
		return email
	}
	return email[:strings.LastIndex(email, "@")]
}

func Hostname(email string) string {
	return email[strings.LastIndex(email, "@")+1:]
}
