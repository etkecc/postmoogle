package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// WildcardMXIDsToRegexes converts a list of wildcard patterns to a list of regular expressions
func WildcardMXIDsToRegexes(wildCardPatterns []string) ([]*regexp.Regexp, error) {
	regexPatterns := make([]*regexp.Regexp, len(wildCardPatterns))

	for idx, wildCardPattern := range wildCardPatterns {
		regex, err := parseMXIDWildcard(wildCardPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to parse allowed user rule `%s`: %s", wildCardPattern, err)
		}
		regexPatterns[idx] = regex
	}

	return regexPatterns, nil
}

// Match tells if the given user id is allowed to use the bot, according to the given whitelist
func Match(userID string, allowed []*regexp.Regexp) bool {
	for _, regex := range allowed {
		if regex.MatchString(userID) {
			return true
		}
	}

	return false
}

// parseMXIDWildcard parses a user whitelisting wildcard rule and returns a regular expression which corresponds to it
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
func parseMXIDWildcard(wildCardRule string) (*regexp.Regexp, error) {
	if !strings.HasPrefix(wildCardRule, "@") {
		return nil, fmt.Errorf("rules need to be fully-qualified, starting with a @")
	}

	remainingRule := wildCardRule[1:]
	if strings.Contains(remainingRule, "@") {
		return nil, fmt.Errorf("rules cannot contain more than one @")
	}

	parts := strings.Split(remainingRule, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected exactly 2 parts in the rule, separated by `:`")
	}

	localPart := parts[0]
	localPartPattern, err := getRegexPatternForPart(localPart)
	if err != nil {
		return nil, fmt.Errorf("failed to convert local part `%s` to regex: %s", localPart, err)
	}

	domainPart := parts[1]
	domainPartPattern, err := getRegexPatternForPart(domainPart)
	if err != nil {
		return nil, fmt.Errorf("failed to convert domain part `%s` to regex: %s", domainPart, err)
	}

	finalPattern := fmt.Sprintf("^@%s:%s$", localPartPattern, domainPartPattern)

	regex, err := regexp.Compile(finalPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex `%s`: %s", finalPattern, err)
	}

	return regex, nil
}

func getRegexPatternForPart(part string) (string, error) {
	if part == "" {
		return "", fmt.Errorf("rejecting empty part")
	}

	var pattern strings.Builder
	for _, rune := range part {
		if rune == '*' {
			// We match everything except for `:` and `@`, because that would be an invalid MXID anyway.
			//
			// If the whole part is `*` (only) instead of merely containing `*` within it,
			// we may also consider replacing it with `([^:@]+)` (+, instead of *).
			// See parseMXIDWildcard for notes about this.
			pattern.WriteString("([^:@]*)")
			continue
		}

		pattern.WriteString(regexp.QuoteMeta(string(rune)))
	}

	return pattern.String(), nil
}
