package email

import (
	"html"
	"strings"
	"unicode"

	xhtml "golang.org/x/net/html"
)

const (
	noBreakSpace       = '\U000000A0'
	zeroWidthNonJoiner = '\U0000200C'
	zeroWidthJoiner    = '\U0000200D'
)

func (r *renderer) text(n *xhtml.Node) {
	if r.pre {
		for i, line := range strings.Split(n.Data, "\n") {
			if i > 0 {
				r.lineBreak()
			}
			r.inlineText(r.normalizeSpace(line))
		}
		return
	}
	if r.isBlankLine(n.Data) && r.isBlankBlock(n) {
		r.lineBreak()
		return
	}
	r.inlineText(r.normalizeSpace(n.Data))
}

// isBlankBlock detects blocks with nothing but non-breaking spaces, which email clients show as empty lines
func (r *renderer) isBlankBlock(n *xhtml.Node) bool {
	for parent := n.Parent; parent != nil && parent.Type == xhtml.ElementNode; parent = parent.Parent {
		if r.contentInfo(parent).visible() {
			return false
		}
		if blockLike[parent.DataAtom] || blockElements[parent.DataAtom] || r.isDisplayBlock(parent) {
			return true
		}
	}
	return false
}

// inlineText writes whitespace-normalized text, keeping at most one space between words
func (r *renderer) inlineText(text string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		r.space = r.space || text != ""
		return
	}
	if text[0] == ' ' {
		r.space = true
	}
	r.write(html.EscapeString(trimmed))
	r.space = text[len(text)-1] == ' '
}

func (r *renderer) lineBreak() {
	if r.breaks < breakParagraph {
		r.breaks++
	}
}

func (r *renderer) breakAtLeast(level int) {
	r.breaks = max(r.breaks, level)
}

// write adds an HTML fragment to the current paragraph, opening the paragraph and pending inline tags if needed
func (r *renderer) write(fragment string) {
	r.outsideQuote()
	r.flushBreaks()
	if r.leaf.Len() == 0 {
		r.leaf.WriteString("<" + r.wrapper + ">")
		r.lineStart = true
	}
	if r.space && !r.lineStart {
		r.leaf.WriteByte(' ')
	}
	r.space = false
	for _, tag := range r.inline {
		if !tag.written {
			r.leaf.WriteString(tag.open)
			tag.written = true
		}
	}
	r.leaf.WriteString(fragment)
	r.lineStart = false
}

func (r *renderer) flushBreaks() {
	breaks := r.breaks
	r.breaks = 0
	if breaks == 0 || r.leaf.Len() == 0 {
		return
	}
	if breaks >= breakParagraph || r.leaf.Len() > maxLeafSize {
		r.closeLeaf()
		return
	}
	r.leaf.WriteString("<br>")
	r.lineStart = true
}

func (r *renderer) closeLeaf() {
	if r.leaf.Len() == 0 {
		return
	}
	for i := len(r.inline) - 1; i >= 0; i-- {
		if r.inline[i].written {
			r.leaf.WriteString(r.inline[i].closing)
			r.inline[i].written = false
		}
	}
	r.leaf.WriteString("</" + r.wrapper + ">")
	r.blocks = append(r.blocks, r.leaf.String())
	r.leaf.Reset()
	r.space = false
}

// addBlock adds a complete block element (list, quote, table, etc.) after the current paragraph
func (r *renderer) addBlock(block string) {
	r.outsideQuote()
	r.appendBlock(block)
}

func (r *renderer) appendBlock(block string) {
	r.closeLeaf()
	r.breaks = 0
	r.space = false
	r.blocks = append(r.blocks, block)
}

func (r *renderer) enterQuote(kind int) {
	if r.quoteAt < 0 && !r.quoteDone {
		r.startQuote()
	}
	r.quoteRest = r.quoteRest || kind == quoteRest
	r.inQuote++
}

// outsideQuote stops folding the quote when the email goes on after it
func (r *renderer) outsideQuote() {
	if r.top && r.quoteAt >= 0 && r.inQuote == 0 && !r.quoteRest {
		r.quoteAt = -1
		r.quoteDone = true
	}
}

// push opens an inline element; nested duplicates (e.g. bold inside bold, link inside link) are ignored
func (r *renderer) push(open, closing string) {
	for _, tag := range r.inline {
		if tag.open == open || (strings.HasPrefix(open, "<a ") && strings.HasPrefix(tag.open, "<a ")) {
			open, closing = "", ""
			break
		}
	}
	r.inline = append(r.inline, &inlineTag{open: open, closing: closing})
}

func (r *renderer) pop() {
	last := r.inline[len(r.inline)-1]
	r.inline = r.inline[:len(r.inline)-1]
	if last.written {
		r.leaf.WriteString(last.closing)
	}
}

// normalizeSpace collapses whitespace like browsers do, and removes invisible characters used for padding
func (r *renderer) normalizeSpace(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	var space bool
	var last rune
	runes := []rune(text)
	for i, char := range runes {
		switch {
		case unicode.IsSpace(char):
			space = true
		case r.isInvisible(char):
			continue
		case char == zeroWidthNonJoiner || char == zeroWidthJoiner:
			if space || last == 0 || i+1 == len(runes) || unicode.IsSpace(runes[i+1]) || r.isInvisible(runes[i+1]) {
				continue
			}
			out.WriteRune(char)
		default:
			if space {
				out.WriteByte(' ')
				space = false
			}
			out.WriteRune(char)
			last = char
		}
	}
	if space {
		out.WriteByte(' ')
	}
	return out.String()
}

// isInvisible detects zero-width characters that emails use to pad preview texts
func (r *renderer) isInvisible(char rune) bool {
	switch char {
	case '\U0000200B', '\U00002060', '\U0000FEFF', '\U000000AD', '\U0000034F',
		'\U0000180E', '\U00002061', '\U00002062', '\U00002063', '\U00002064':
		return true
	default:
		return false
	}
}

// isBlankLine detects text made of non-breaking spaces only
func (r *renderer) isBlankLine(text string) bool {
	return strings.ContainsRune(text, noBreakSpace) && strings.TrimSpace(r.normalizeSpace(text)) == ""
}
