package utils

import "strings"

// Mailbox returns mailbox part from email address
func Mailbox(email string) string {
	index := strings.LastIndex(email, "@")
	if index == -1 {
		return email
	}
	return email[:index]
}

// EmailsList returns human-readable list of mailbox's emails for all available domains
func EmailsList(mailbox string, domain string) string {
	var msg strings.Builder
	domain = SanitizeDomain(domain)
	msg.WriteString(mailbox)
	msg.WriteString("@")
	msg.WriteString(domain)

	count := len(domains) - 1
	for i, aliasDomain := range domains {
		if i < count {
			msg.WriteString(", ")
		}
		if aliasDomain == domain {
			continue
		}
		msg.WriteString(mailbox)
		msg.WriteString("@")
		msg.WriteString(aliasDomain)
	}

	return msg.String()
}

// Hostname returns hostname part from email address
func Hostname(email string) string {
	return email[strings.LastIndex(email, "@")+1:]
}
