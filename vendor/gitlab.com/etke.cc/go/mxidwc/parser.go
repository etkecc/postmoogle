package mxidwc

import (
	"fmt"
	"regexp"
	"strings"
)

// Match tells if the given user id is allowed according to the given list of regexes
func Match(userID string, allowed []*regexp.Regexp) bool {
	for _, regex := range allowed {
		if regex.MatchString(userID) {
			return true
		}
	}

	return false
}

// ParsePatterns converts a list of wildcard patterns to a list of regular expressions
// See ParsePattern for details
func ParsePatterns(patterns []string) ([]*regexp.Regexp, error) {
	regexes := make([]*regexp.Regexp, 0, len(patterns))

	for _, pattern := range patterns {
		regex, err := ParsePattern(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pattern `%s`: %s", pattern, err)
		}
		regexes = append(regexes, regex)
	}

	return regexes, nil
}

// ParsePattern parses a user wildcard pattern and returns a regular expression which corresponds to it
//
// Example conversion: `@bot.*.something:*.example.com` -> `^bot\.([^:@]*)\.something:([^:@]*)\.example.com$`
// Example of recognized wildcard patterns: `@someone:example.com`, `@*:example.com`, `@bot.*:example.com`, `@someone:*`, `@someone:*.example.com`
//
// The `*` wildcard character is normally interpretted as "a number of literal characters or an empty string".
// Our implementation below matches this (yielding `([^:@])*`), which could provide a slightly suboptimal regex in these cases:
// - `@*:example.com` -> `^@([^:@])*:example\.com$`, although `^@([^:@])+:example\.com$` would be preferable
// - `@someone:*` -> `@someone:([^:@])*$`, although `@someone:([^:@])+$` would be preferable
// When it's a bare wildcard (`*`, instead of `*.example.com`) we likely prefer to yield a regex that matches **at least one character**.
// This probably doesn't matter because mxids that we'll match against are all valid and fully complete.
func ParsePattern(pattern string) (*regexp.Regexp, error) {
	if !strings.HasPrefix(pattern, "@") {
		return nil, fmt.Errorf("patterns need to be fully-qualified, starting with a @")
	}

	pattern = pattern[1:]
	if strings.Contains(pattern, "@") {
		return nil, fmt.Errorf("patterns cannot contain more than one @")
	}

	parts := strings.Split(pattern, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected exactly 2 parts in the pattern, separated by `:`")
	}

	localpart := parts[0]
	localpartPattern, err := getPattern(localpart)
	if err != nil {
		return nil, fmt.Errorf("failed to convert localpart `%s` to regex: %s", localpart, err)
	}

	domain := parts[1]
	domainPattern, err := getPattern(domain)
	if err != nil {
		return nil, fmt.Errorf("failed to convert domain `%s` to regex: %s", domain, err)
	}

	pattern = fmt.Sprintf("^@%s:%s$", localpartPattern, domainPattern)

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex `%s`: %s", pattern, err)
	}

	return regex, nil
}

func getPattern(part string) (string, error) {
	if part == "" {
		return "", fmt.Errorf("rejecting empty part")
	}

	var pattern strings.Builder
	for _, runeItem := range part {
		if runeItem == '*' {
			// We match everything except for `:` and `@`, because that would be an invalid MXID anyway.
			//
			// If the whole part is `*` (only) instead of merely containing `*` within it,
			// we may also consider replacing it with `([^:@]+)` (+, instead of *).
			// See ParsePattern for notes about this.
			pattern.WriteString("([^:@]*)")
			continue
		}

		pattern.WriteString(regexp.QuoteMeta(string(runeItem)))
	}

	return pattern.String(), nil
}
