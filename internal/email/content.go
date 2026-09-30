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
)

const (
	// maxContentSize keeps events below the 64KiB Matrix limit, even after encryption and JSON escaping
	maxContentSize = 40000
	// truncatedNotice is added to messages shortened to fit the Matrix event size limit
	truncatedNotice = "✂️ This email is too long to show in full."
)

var (
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

// body renders the email body, using its HTML part when allowed; strip removes reply quotes and signatures
func (e *Email) body(options *ContentOptions, strip bool) *body {
	if options.HTML && e.HTML != "" {
		if blocks, quoteAt := newRenderer(e.imageResolver(options), strip).render(e.HTML); len(blocks) > 0 {
			if strip {
				markdown := e.stripParser().Parse(strings.Join(blocks, ""), format.NewContext(context.Background()))
				return e.markdownBody(mailstrip.Parse(markdown).String(), true)
			}
			return e.fold(e.htmlBody(blocks), quoteAt, options)
		}
	}
	if strip {
		return e.markdownBody(mailstrip.Parse(e.Text).String(), false)
	}
	text := e.markdownBody(e.Text, false)
	return e.fold(text, text.quotedTail(), options)
}

func (e *Email) htmlBody(blocks []string) *body {
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
func (e *Email) markdownBody(text string, allowHTML bool) *body {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	var lines []string
	if text != "" {
		lines = e.separateQuotes(strings.Split(text, "\n"))
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

// stripParser converts rendered HTML into markdown for stripping quotes, keeping images and escaping text
func (e *Email) stripParser() *format.HTMLParser {
	return &format.HTMLParser{
		TabsToSpaces:   4,
		Newline:        "\n",
		HorizontalLine: "\n---\n",
		TextConverter:  e.escapeMarkdownText,
		LinkConverter: func(text, href string, _ format.Context) string {
			return "[" + text + "](<" + markdownURLEscaper.Replace(href) + ">)"
		},
		ImageConverter: func(src, alt, title, width, height string, _ bool) string {
			return (&Image{URI: src}).tag(alt, title, e.atoi(width), e.atoi(height))
		},
		UnderlineConverter: func(text string, _ format.Context) string {
			return "<u>" + text + "</u>"
		},
	}
}

func (e *Email) escapeMarkdownText(text string, ctx format.Context) string {
	if ctx.TagStack.Has("pre") || ctx.TagStack.Has("code") {
		return text
	}
	return markdownEscaper.Replace(text)
}

func (e *Email) atoi(value string) int {
	number, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return number
}

// render builds the event content, dropping the end of the body if the event would be too large
func (m *message) render() (content *event.Content, truncated bool) {
	if m.body == nil {
		return m.content(0, false), false
	}
	content = m.content(len(m.body.parts), false)
	if m.fits(content) {
		return content, false
	}
	low, high := 0, len(m.body.parts)-1
	for low < high {
		middle := (low + high + 1) / 2
		if m.fits(m.content(middle, true)) {
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

func (m *message) fits(content *event.Content) bool {
	data, err := json.Marshal(content)
	return err == nil && len(data) <= maxContentSize
}
