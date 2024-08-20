package utils

import (
	"strings"

	"github.com/mcnijman/go-emailaddress"
)

// Mailbox returns mailbox part from email address
func Mailbox(email string) string {
	mailbox, _, _ := EmailParts(email)
	return mailbox
}

// Subaddress returns sub address part form email address
func Subaddress(email string) string {
	_, sub, _ := EmailParts(email)
	return sub
}

// Hostname returns hostname part from email address
func Hostname(email string) string {
	_, _, hostname := EmailParts(email)
	return hostname
}

// EmailParts parses email address into mailbox, subaddress, and hostname
func EmailParts(email string) (mailbox, sub, hostname string) {
	address, err := emailaddress.Parse(email)
	if err == nil {
		mailbox = address.LocalPart
		hostname = address.Domain
	} else {
		mailbox = email
		hostname = email
		mIdx := strings.Index(email, "@")
		hIdx := strings.LastIndex(email, "@")
		if mIdx != -1 {
			mailbox = email[:mIdx]
		}
		if hIdx != -1 {
			hostname = email[hIdx+1:]
		}
	}

	idx := strings.Index(mailbox, "+")
	if idx != -1 {
		sub = strings.ReplaceAll(mailbox[idx:], "+", "")
		mailbox = strings.ReplaceAll(mailbox[:idx], "+", "")
	}
	return mailbox, sub, hostname
}

// EmailsList returns human-readable list of mailbox's emails for all available domains
func EmailsList(mailbox, domain string) string {
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
