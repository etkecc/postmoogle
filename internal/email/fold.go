package email

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// foldTextLen is how much text a body needs to have its end folded, shorter bodies are shown in full
	foldTextLen = 4000
	// previewTextLen is how much text of a folded body is shown before the fold
	previewTextLen = 1500
	quoteSummary   = "Show quoted text"
	moreSummary    = "Show the rest of the email"
)

var (
	// subject prefixes of replies and forwards in common languages
	replySubjectRegex   = regexp.MustCompile(`(?i)^\s*(?:re|aw|sv|vs|ynt|odp|antw|antwort|rif)\s*(?:\[\d+\])?\s*:`)
	forwardSubjectRegex = regexp.MustCompile(`(?i)^\s*(?:fwd?|wg|tr|rv|enc|doorst|[iİ]lt)\s*(?:\[\d+\])?\s*:`)
)

// fold hides the quoted earlier messages of a reply and the end of a very long body until the reader opens them
func (e *Email) fold(b *body, quoteAt int, options *ContentOptions) *body {
	if !options.Collapse {
		return b
	}
	end := len(b.parts)
	if quoteAt > 0 && e.isReply() && !e.isForward() {
		b.quoteAt = quoteAt
		end = quoteAt
	}
	b.moreAt = b.previewEnd(end)
	return b
}

func (e *Email) isReply() bool {
	return e.InReplyTo != "" || e.References != "" || replySubjectRegex.MatchString(e.Subject)
}

func (e *Email) isForward() bool {
	return forwardSubjectRegex.MatchString(e.Subject)
}

// separateQuotes ends a quote where the text goes on without ">", which markdown would add to the quote instead
func (e *Email) separateQuotes(lines []string) []string {
	separated := make([]string, 0, len(lines))
	for i, line := range lines {
		if i > 0 && e.isQuoteLine(lines[i-1]) && strings.TrimSpace(line) != "" && !e.isQuoteLine(line) {
			separated = append(separated, "")
		}
		separated = append(separated, line)
	}
	return separated
}

func (e *Email) isQuoteLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), ">")
}

// previewEnd returns how many of the first parts of a long body are shown before the rest is folded, -1 if not long
func (b *body) previewEnd(end int) int {
	parts := b.parts[:end]
	lengths := make([]int, len(parts))
	var total int
	for i, part := range parts {
		lengths[i] = b.textLen(part)
		total += lengths[i]
	}
	if total <= foldTextLen {
		return -1
	}
	var shown int
	for i, length := range lengths[:len(lengths)-1] {
		shown += length
		if shown >= previewTextLen {
			return i + 1
		}
	}
	return -1
}

// quotedTail returns the first line of the quote ending a plain text reply, with its "... wrote:" line, or -1
func (b *body) quotedTail() int {
	lines := b.parts
	start := -1
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ">") {
			start = i
			continue
		}
		if start >= 0 && strings.HasSuffix(line, ":") && utf8.RuneCountInString(line) <= maxAttributionLen {
			start = i
			// long attribution lines are wrapped, e.g. "On Mon, 28 Sep 2026, John <john@example.com>\nwrote:"
			if previous := strings.TrimSpace(lines[max(i-1, 0)]); i > 0 && len(line) < 20 && previous != "" &&
				!strings.HasPrefix(previous, ">") {
				start = i - 1
			}
		}
		break
	}
	for _, line := range lines[:max(start, 0)] {
		if strings.TrimSpace(line) != "" {
			return start
		}
	}
	return -1
}

// render renders the first parts of the body; folds are only in the formatted body, the plain text has everything
func (b *body) render(parts int) (formatted, plain string) {
	shown := b.parts[:parts]
	formatted, plain = b.flat(shown)
	quote := parts
	if b.quoteAt >= 0 && b.quoteAt < parts {
		quote = b.quoteAt
	}
	more := quote
	if b.moreAt >= 0 && b.moreAt < quote {
		more = b.moreAt
	}
	if more == parts {
		return formatted, plain
	}
	formatted, _ = b.flat(shown[:more])
	if more < quote {
		folded, _ := b.flat(shown[more:quote])
		formatted += b.details(moreSummary, folded)
	}
	if quote < parts {
		folded, _ := b.flat(shown[quote:])
		formatted += b.details(quoteSummary, folded)
	}
	return formatted, plain
}

// details folds the content under a summary line that opens it
func (b *body) details(summary, content string) string {
	return "<details><summary>" + html.EscapeString(summary) + "</summary>" + content + "</details>"
}
