package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// WildcardUserPatternsToRegexPatterns converts a list of wildcard patterns to a list of regular expressions
func WildcardUserPatternsToRegexPatterns(wildCardPatterns []string) (*[]*regexp.Regexp, error) {
	regexPatterns := make([]*regexp.Regexp, len(wildCardPatterns))

	for idx, wildCardPattern := range wildCardPatterns {
		regex, err := parseAllowedUserRule(wildCardPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to parse allowed user rule `%s`: %s", wildCardPattern, err)
		}
		regexPatterns[idx] = regex
	}

	return &regexPatterns, nil
}

// MatchUserWithAllowedRegexes tells if the given user id is allowed to use the bot, according to the given whitelist
// An empty whitelist means "everyone is allowed"
func MatchUserWithAllowedRegexes(userID string, allowed []*regexp.Regexp) (bool, error) {
	// No whitelisted users means everyone is whitelisted
	if len(allowed) == 0 {
		return true, nil
	}

	for _, regex := range allowed {
		if regex.MatchString(userID) {
			return true, nil
		}
	}

	return false, nil
}

// parseAllowedUserRule parses a user whitelisting rule and returns a regular expression which corresponds to it
// Example conversion: `@bot.*.something:*.example.com` -> `^bot\.([^:@]*)\.something:([^:@]*)\.example.com$`
// Example of recognized wildcard patterns: `@someone:example.com`, `@*:example.com`, `@bot.*:example.com`, `@someone:*`, `@someone:*.example.com`
func parseAllowedUserRule(wildCardRule string) (*regexp.Regexp, error) {
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

	getRegexPatternForPart := func(part string) (string, error) {
		if part == "" {
			return "", fmt.Errorf("rejecting empty part")
		}

		var pattern strings.Builder
		for _, rune := range part {
			if rune == '*' {
				// We match everything except for `:` and `@`, because that would be an invalid MXID anyway
				pattern.WriteString("([^:@]*)")
				continue
			}

			pattern.WriteString(regexp.QuoteMeta(string(rune)))
		}

		return pattern.String(), nil
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
