package validator

import (
	"regexp"
	"strings"
)

// V is a validator implementation
type V struct {
	spamlist []*regexp.Regexp
	enforce  Enforce
	from     string
	log      Logger
}

// Enforce checks
type Enforce struct {
	// Email enforces email check and rejects empty emails
	Email bool
	// Domain enforces domain check and rejects empty domains
	Domain bool
	// SMTP enforces SMTP check (email actually exists on mail server) and rejects non-existing emails
	SMTP bool
	// MX enforces MX records check on email's mail server
	MX bool
}

type Logger interface {
	Info(string, ...interface{})
	Error(string, ...interface{})
}

// New Validator, accepts spamlist with wildcards
func New(spamlist []string, enforce Enforce, smtpFrom string, log Logger) *V {
	spamregexes, err := parseSpamlist(spamlist)
	if err != nil {
		log.Error("cannot parse spamlist: %v", err)
	}

	return &V{
		spamlist: spamregexes,
		enforce:  enforce,
		from:     smtpFrom,
		log:      log,
	}
}

func parseSpamlist(patterns []string) ([]*regexp.Regexp, error) {
	regexes := []*regexp.Regexp{}
	for _, pattern := range patterns {
		rule, err := regexp.Compile("^" + parsePattern(pattern) + "$")
		if err != nil {
			return regexes, err
		}

		regexes = append(regexes, rule)
	}

	return regexes, nil
}

func parsePattern(pattern string) string {
	var regexpattern strings.Builder
	for _, runeItem := range pattern {
		if runeItem == '*' {
			regexpattern.WriteString("(.*)")
			continue
		}
		regexpattern.WriteString(regexp.QuoteMeta(string(runeItem)))
	}

	return regexpattern.String()
}
