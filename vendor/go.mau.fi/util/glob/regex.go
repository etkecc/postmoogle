package glob

import (
	"fmt"
	"regexp"
	"strings"
)

type RegexGlob struct {
	regex *regexp.Regexp
}

func (rg *RegexGlob) Match(s string) bool {
	return rg.regex.MatchString(s)
}

func CompileRegex(pattern string) (*RegexGlob, error) {
	var buf strings.Builder
	buf.WriteRune('^')
	for _, part := range SplitPattern(pattern) {
		if strings.ContainsRune(part, '*') || strings.ContainsRune(part, '?') {
			questions := strings.Count(part, "?")
			star := strings.ContainsRune(part, '*')
			if star {
				if questions > 0 {
					_, _ = fmt.Fprintf(&buf, ".{%d,}", questions)
				} else {
					buf.WriteString(".*")
				}
			} else if questions > 0 {
				_, _ = fmt.Fprintf(&buf, ".{%d}", questions)
			}
		} else {
			buf.WriteString(regexp.QuoteMeta(part))
		}
	}
	buf.WriteRune('$')
	regex, err := regexp.Compile(buf.String())
	if err != nil {
		return nil, err
	}
	return &RegexGlob{regex}, nil
}
