package email

import (
	"context"
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kvannotten/mailstrip"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"github.com/etkecc/postmoogle/internal/utils"
)

const (
	// maxContentSize keeps events below the 64KiB Matrix limit, even after encryption and JSON escaping
	maxContentSize = 40000
	// truncatedNotice is added to messages shortened to fit the Matrix event size limit
	truncatedNotice = "✂️ This email is too long to show in full."
	// subjectIcon starts every email, so emails that follow each other in a room are easy to tell apart
	subjectIcon = "✉️ "
	// mutedColor is the color of header labels and addresses, readable on light and dark themes
	mutedColor = "#8D99A5"
	// dateShownAfter is how late an email has to arrive to show when it was sent, e.g. when old emails are imported
	dateShownAfter = 30 * time.Minute
	dateLayout     = "Mon, 2 Jan 2006 15:04"
	// foldTextLen is how much text a body needs to have its end folded, shorter bodies are shown in full
	foldTextLen = 4000
	// previewTextLen is how much text of a folded body is shown before the fold
	previewTextLen = 1500
	quoteSummary   = "Show quoted text"
	moreSummary    = "Show the rest of the email"
)

var (
	// stripParser converts rendered HTML into markdown for stripping quotes, keeping images and escaping text
	stripParser = &format.HTMLParser{
		TabsToSpaces:   4,
		Newline:        "\n",
		HorizontalLine: "\n---\n",
		TextConverter:  escapeMarkdownText,
		LinkConverter: func(text, href string, _ format.Context) string {
			return "[" + text + "](<" + markdownURLEscaper.Replace(href) + ">)"
		},
		ImageConverter: func(src, alt, title, width, height string, _ bool) string {
			return imageTag(src, alt, title, atoi(width), atoi(height))
		},
		UnderlineConverter: func(text string, _ format.Context) string {
			return "<u>" + text + "</u>"
		},
	}

	// textParser converts rendered HTML into the plain text body of a message, dropping links without text
	textParser = &format.HTMLParser{
		TabsToSpaces:   4,
		Newline:        "\n",
		HorizontalLine: "\n---\n",
		LinkConverter: func(text, href string, _ format.Context) string {
			switch strings.TrimSpace(text) {
			case "":
				return ""
			case href, strings.TrimPrefix(href, "mailto:"):
				return text
			default:
				return "[" + text + "](" + href + ")"
			}
		},
	}

	markdownEscaper    = strings.NewReplacer(`\`, `\\`, "`", "\\`", "*", `\*`, "[", `\[`, "]", `\]`, "&", "&amp;", "<", "&lt;")
	markdownURLEscaper = strings.NewReplacer("<", "%3C", ">", "%3E", "\n", "")

	// subject prefixes of replies and forwards in common languages
	replySubjectRegex   = regexp.MustCompile(`(?i)^\s*(?:re|aw|sv|vs|ynt|odp|antw|antwort|rif)\s*(?:\[\d+\])?\s*:`)
	forwardSubjectRegex = regexp.MustCompile(`(?i)^\s*(?:fwd?|wg|tr|rv|enc|doorst|[iİ]lt)\s*(?:\[\d+\])?\s*:`)

	blockStartRegex = regexp.MustCompile(`^<(?:p|h[1-6]|ul|ol|blockquote|pre|table|hr|details|div)[\s>]`)

	// now returns the current time, tests replace it
	now = time.Now
)

type (
	// body is an email body rendered for Matrix, split into parts that can be dropped from the end to fit size limits
	body struct {
		parts   []string
		quoteAt int                                            // first part of quoted earlier messages, -1 if none
		moreAt  int                                            // first part folded in a very long body, -1 if none
		flat    func(parts []string) (formatted, plain string) // renders parts without folding
		textLen func(part string) int                          // visible text length of a part
	}

	// message assembles the Matrix event content of an email
	message struct {
		headerHTML string
		headerText string
		body       *body
		raw        map[string]any
		relatesTo  *event.RelatesTo
	}
)

// header renders the email subject, sender, recipients, and the date if the email came late
func (e *Email) header(threadID id.EventID, options *ContentOptions) (formatted, plain string) {
	htmlParts := make([]string, 0, 2)
	textParts := make([]string, 0, 2)
	if options.Subject && threadID == "" && e.Subject != "" {
		htmlParts = append(htmlParts, "<h3>"+subjectIcon+html.EscapeString(e.Subject)+"</h3>")
		textParts = append(textParts, e.Subject)
	}

	htmlLines := make([]string, 0, 4)
	textLines := make([]string, 0, 4)
	line := func(label, htmlValue, textValue string) {
		htmlLines = append(htmlLines, muted(label+":")+" "+htmlValue)
		textLines = append(textLines, label+": "+textValue)
	}
	if options.Sender && e.From != "" {
		line("From", e.senderHTML(), e.senderText())
	}
	if options.Recipient && e.showRecipient() {
		to := recipients(e.To)
		line("To", html.EscapeString(to), to)
	}
	if options.CC && len(e.CC) > 0 {
		cc := strings.Join(e.CC, ", ")
		line("Cc", html.EscapeString(cc), cc)
	}
	if sent := e.sentDate(); sent != "" {
		line("Date", html.EscapeString(sent), sent)
	}
	if len(htmlLines) > 0 {
		htmlParts = append(htmlParts, "<p>"+strings.Join(htmlLines, "<br>")+"</p>")
		textParts = append(textParts, strings.Join(textLines, "\n"))
	}
	return strings.Join(htmlParts, ""), strings.Join(textParts, "\n")
}

// muted shows secondary text, like labels and addresses, in a softer color; the text must be escaped already
func muted(text string) string {
	return `<font color="` + mutedColor + `" data-mx-color="` + mutedColor + `">` + text + `</font>`
}

// showRecipient hides the To line when it only repeats the address of the mailbox, unless it has a subaddress
func (e *Email) showRecipient() bool {
	return e.To != "" && (!strings.EqualFold(e.To, e.RcptTo) || utils.Subaddress(e.To) != "")
}

// sentDate returns when the email was sent if it arrived much later than that, e.g. when older emails are imported
func (e *Email) sentDate() string {
	if e.Sent.IsZero() {
		return ""
	}
	if delay := now().Sub(e.Sent); delay < dateShownAfter && delay > -dateShownAfter {
		return ""
	}
	return e.Sent.In(time.Local).Format(dateLayout)
}

func (e *Email) hasSenderName() bool {
	return e.FromName != "" && !strings.EqualFold(e.FromName, e.From)
}

func (e *Email) senderHTML() string {
	if !e.hasSenderName() {
		return "<strong>" + html.EscapeString(e.From) + "</strong>"
	}
	return "<strong>" + html.EscapeString(e.FromName) + "</strong> " + muted("&lt;"+html.EscapeString(e.From)+"&gt;")
}

func (e *Email) senderText() string {
	if !e.hasSenderName() {
		return e.From
	}
	return e.FromName + " <" + e.From + ">"
}

// recipients formats a comma-separated list of addresses, showing subaddresses separately
func recipients(list string) string {
	addresses := strings.Split(list, ",")
	for i, address := range addresses {
		address = strings.TrimSpace(address)
		if !strings.Contains(address, "@") {
			addresses[i] = address
			continue
		}
		mailbox, sub, host := utils.EmailParts(address)
		addresses[i] = mailbox + "@" + host
		if sub != "" {
			addresses[i] += " (" + sub + ")"
		}
	}
	return strings.Join(addresses, ", ")
}

// body renders the email body, using its HTML part when allowed; strip removes reply quotes and signatures
func (e *Email) body(options *ContentOptions, strip bool) *body {
	if options.HTML && e.HTML != "" {
		if blocks, quoteAt := renderDocument(e.HTML, e.imageResolver(options), strip); len(blocks) > 0 {
			if strip {
				markdown := stripParser.Parse(strings.Join(blocks, ""), format.NewContext(context.Background()))
				return markdownBody(mailstrip.Parse(markdown).String(), true)
			}
			return e.fold(htmlBody(blocks), quoteAt, options)
		}
	}
	if strip {
		return markdownBody(mailstrip.Parse(e.Text).String(), false)
	}
	text := markdownBody(e.Text, false)
	return e.fold(text, quotedTail(text.parts), options)
}

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
	b.moreAt = previewEnd(b.parts[:end], b.textLen)
	return b
}

func (e *Email) isReply() bool {
	return e.InReplyTo != "" || e.References != "" || replySubjectRegex.MatchString(e.Subject)
}

func (e *Email) isForward() bool {
	return forwardSubjectRegex.MatchString(e.Subject)
}

// previewEnd returns how many parts of a long body are shown before the rest is folded, -1 if it is not long
func previewEnd(parts []string, textLen func(string) int) int {
	lengths := make([]int, len(parts))
	var total int
	for i, part := range parts {
		lengths[i] = textLen(part)
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
func quotedTail(lines []string) int {
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

func htmlBody(blocks []string) *body {
	return &body{
		parts:   blocks,
		quoteAt: -1,
		moreAt:  -1,
		flat: func(parts []string) (formatted, plain string) {
			formatted = strings.Join(parts, "")
			plain, _ = format.HTMLToMarkdownFull(textParser, formatted)
			return formatted, plain
		},
		textLen: func(part string) int { return utf8.RuneCountInString(blockText(part)) },
	}
}

// markdownBody renders markdown split by lines; allowHTML keeps raw HTML (only used for our own image tags)
func markdownBody(text string, allowHTML bool) *body {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	var lines []string
	if text != "" {
		lines = separateQuotes(strings.Split(text, "\n"))
	}
	return &body{
		parts:   lines,
		quoteAt: -1,
		moreAt:  -1,
		flat: func(parts []string) (formatted, plain string) {
			content := format.RenderMarkdown(strings.Join(parts, "\n"), true, allowHTML)
			formatted = content.FormattedBody
			if content.Format != event.FormatHTML || formatted == "" {
				formatted = strings.ReplaceAll(html.EscapeString(content.Body), "\n", "<br>")
			}
			// a single paragraph comes without its <p>, which leaves it glued to the line above
			if formatted != "" && !blockStartRegex.MatchString(formatted) {
				formatted = "<p>" + formatted + "</p>"
			}
			return formatted, content.Body
		},
		textLen: utf8.RuneCountInString,
	}
}

// separateQuotes ends a quote where the text goes on without ">", which markdown would add to the quote instead
func separateQuotes(lines []string) []string {
	separated := make([]string, 0, len(lines))
	for i, line := range lines {
		if i > 0 && isQuoteLine(lines[i-1]) && strings.TrimSpace(line) != "" && !isQuoteLine(line) {
			separated = append(separated, "")
		}
		separated = append(separated, line)
	}
	return separated
}

func isQuoteLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), ">")
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
		formatted += details(moreSummary, folded)
	}
	if quote < parts {
		folded, _ := b.flat(shown[quote:])
		formatted += details(quoteSummary, folded)
	}
	return formatted, plain
}

// details folds the content under a summary line that opens it
func details(summary, content string) string {
	return "<details><summary>" + html.EscapeString(summary) + "</summary>" + content + "</details>"
}

// render builds the event content, dropping the end of the body if the event would be too large
func (m *message) render() (content *event.Content, truncated bool) {
	if m.body == nil {
		return m.content(0, false), false
	}
	content = m.content(len(m.body.parts), false)
	if fits(content) {
		return content, false
	}
	low, high := 0, len(m.body.parts)-1
	for low < high {
		middle := (low + high + 1) / 2
		if fits(m.content(middle, true)) {
			low = middle
		} else {
			high = middle - 1
		}
	}
	return m.content(low, true), true
}

func (m *message) content(parts int, truncated bool) *event.Content {
	formatted, plain := m.headerHTML, m.headerText
	if m.body != nil {
		bodyHTML, bodyText := m.body.render(parts)
		if truncated {
			bodyHTML += "<p><em>" + html.EscapeString(truncatedNotice) + "</em></p>"
			bodyText = strings.TrimSpace(bodyText + "\n\n" + truncatedNotice)
		}
		if formatted != "" && bodyHTML != "" {
			formatted += "<hr>"
			plain += "\n\n"
		}
		formatted += bodyHTML
		plain += bodyText
	}

	parsed := &event.MessageEventContent{
		MsgType:   event.MsgText,
		Body:      plain,
		Mentions:  &event.Mentions{},
		RelatesTo: m.relatesTo,
	}
	if formatted != "" {
		parsed.Format = event.FormatHTML
		parsed.FormattedBody = formatted
	}
	return &event.Content{Raw: m.raw, Parsed: parsed}
}

func fits(content *event.Content) bool {
	data, err := json.Marshal(content)
	return err == nil && len(data) <= maxContentSize
}

func escapeMarkdownText(text string, ctx format.Context) string {
	if ctx.TagStack.Has("pre") || ctx.TagStack.Has("code") {
		return text
	}
	return markdownEscaper.Replace(text)
}

func atoi(value string) int {
	number, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return number
}
