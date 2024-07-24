package validator

import "regexp"

// Config of the validator
type Config struct {
	Email  Email                         // Email config
	Domain Domain                        // Domain config
	Log    func(msg string, args ...any) // Log func
}

// Email checks configuration
type Email struct {
	// Enforce enforces email check and rejects empty emails
	Enforce bool
	// SMTP enforces SMTP check (email actually exists on mail server) and rejects non-existing emails
	SMTP bool
	// SPF enforces SPF record check (sender allowed to use that email and send emails) and rejects unathorized emails
	SPF bool
	// MX enforces MX records check on email's mail server
	MX bool
	// From is a valid email address that will be used for SMTP checks
	From string
	// Spamlist is a list of spam emails with wildcards
	Spamlist []string
	spamlist []*regexp.Regexp
}

// Domain checks configuration
type Domain struct {
	// Enforce enforces domain check and rejects empty domains
	Enforce bool
	// PrivateSuffixes considers subdomains with the following suffixes as domains
	PrivateSuffixes []string
}
