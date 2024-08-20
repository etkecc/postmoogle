package validator

import (
	"fmt"
	"regexp"
	"strings"
)

// V is a validator implementation
type V struct {
	cfg *Config
}

var defaultLogfunc = func(msg string, args ...any) {
	fmt.Printf(msg+"\n", args...)
}

// New validator
func New(cfg *Config) *V {
	if cfg.Log == nil {
		cfg.Log = defaultLogfunc
	}
	spamregexes, err := parseSpamlist(cfg.Email.Spamlist)
	if err != nil {
		cfg.Log("cannot parse spamlist: %v", err)
	}
	cfg.Email.spamlist = spamregexes
	return &V{cfg}
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
