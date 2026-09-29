package email

import (
	"context"
	"encoding/json"
	"html"
	"strconv"
	"strings"

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
)

type (
	// body is an email body rendered for Matrix, split into parts that can be dropped from the end to fit size limits
	body struct {
		parts  []string
		render func(parts []string) (formatted, plain string)
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

// header renders the email subject, sender, and recipients
func (e *Email) header(threadID id.EventID, options *ContentOptions) (formatted, plain string) {
	htmlParts := make([]string, 0, 2)
	textParts := make([]string, 0, 2)
	if options.Subject && threadID == "" && e.Subject != "" {
		htmlParts = append(htmlParts, "<h3>"+html.EscapeString(e.Subject)+"</h3>")
		textParts = append(textParts, e.Subject)
	}

	htmlLines := make([]string, 0, 3)
	textLines := make([]string, 0, 3)
	if options.Sender && e.From != "" {
		htmlLines = append(htmlLines, "From: "+e.senderHTML())
		textLines = append(textLines, "From: "+e.senderText())
	}
	if options.Recipient && e.To != "" {
		to := recipients(e.To)
		htmlLines = append(htmlLines, "To: "+html.EscapeString(to))
		textLines = append(textLines, "To: "+to)
	}
	if options.CC && len(e.CC) > 0 {
		cc := strings.Join(e.CC, ", ")
		htmlLines = append(htmlLines, "Cc: "+html.EscapeString(cc))
		textLines = append(textLines, "Cc: "+cc)
	}
	if len(htmlLines) > 0 {
		htmlParts = append(htmlParts, "<p>"+strings.Join(htmlLines, "<br>")+"</p>")
		textParts = append(textParts, strings.Join(textLines, "\n"))
	}
	return strings.Join(htmlParts, ""), strings.Join(textParts, "\n")
}

func (e *Email) hasSenderName() bool {
	return e.FromName != "" && !strings.EqualFold(e.FromName, e.From)
}

func (e *Email) senderHTML() string {
	if !e.hasSenderName() {
		return "<strong>" + html.EscapeString(e.From) + "</strong>"
	}
	return "<strong>" + html.EscapeString(e.FromName) + "</strong> &lt;" + html.EscapeString(e.From) + "&gt;"
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
		if blocks := renderHTML(e.HTML, e.imageResolver(options), strip); len(blocks) > 0 {
			return htmlBody(blocks, strip)
		}
	}
	text := e.Text
	if strip {
		text = mailstrip.Parse(text).String()
	}
	return markdownBody(text, false)
}

func htmlBody(blocks []string, strip bool) *body {
	if strip {
		markdown := stripParser.Parse(strings.Join(blocks, ""), format.NewContext(context.Background()))
		return markdownBody(mailstrip.Parse(markdown).String(), true)
	}
	return &body{parts: blocks, render: func(parts []string) (formatted, plain string) {
		formatted = strings.Join(parts, "")
		plain, _ = format.HTMLToMarkdownFull(textParser, formatted)
		return formatted, plain
	}}
}

// markdownBody renders markdown split by lines; allowHTML keeps raw HTML (only used for our own image tags)
func markdownBody(text string, allowHTML bool) *body {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	return &body{parts: lines, render: func(parts []string) (formatted, plain string) {
		content := format.RenderMarkdown(strings.Join(parts, "\n"), true, allowHTML)
		formatted = content.FormattedBody
		if content.Format != event.FormatHTML || formatted == "" {
			formatted = strings.ReplaceAll(html.EscapeString(content.Body), "\n", "<br>")
		}
		return formatted, content.Body
	}}
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
		bodyHTML, bodyText := m.body.render(m.body.parts[:parts])
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
