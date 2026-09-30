package email

import (
	"html"
	"strconv"
	"strings"
	"time"

	"maunium.net/go/mautrix/id"

	"github.com/etkecc/postmoogle/internal/utils"
)

const (
	// subjectIcon starts every email, so emails that follow each other in a room are easy to tell apart
	subjectIcon = "✉️ "
	// mutedColor is the color of header labels and addresses, readable on light and dark themes
	mutedColor = "#8D99A5"
	// dateShownAfter is how late an email has to arrive to show when it was sent, e.g. when old emails are imported
	dateShownAfter = 30 * time.Minute
	dateLayout     = "Mon, 2 Jan 2006 15:04 MST"
)

// limits of header values, so that huge headers cannot make the event too large for Matrix
const (
	maxSubjectLen = 300  // characters of the subject
	maxNameLen    = 200  // characters of the sender name
	maxAddressLen = 320  // characters of one address, the longest valid email address
	maxIDLen      = 1000 // characters of a message ID
	maxListLen    = 1000 // bytes of an address list, or of the references
)

// header renders the email subject, sender, recipients, and the date if the email came late
func (e *Email) header(threadID id.EventID, options *ContentOptions) (formatted, plain string) {
	htmlParts := make([]string, 0, 2)
	textParts := make([]string, 0, 2)
	if subject := e.subject(); options.Subject && threadID == "" && subject != "" {
		htmlParts = append(htmlParts, "<h3>"+subjectIcon+html.EscapeString(subject)+"</h3>")
		textParts = append(textParts, subject)
	}

	htmlLines := make([]string, 0, 4)
	textLines := make([]string, 0, 4)
	line := func(label, htmlValue, textValue string) {
		htmlLines = append(htmlLines, e.muted(label+":")+" "+htmlValue)
		textLines = append(textLines, label+": "+textValue)
	}
	if options.Sender && e.From != "" {
		line("From", e.senderHTML(), e.senderText())
	}
	if options.Recipient && e.showRecipient() {
		to := e.recipients()
		line("To", html.EscapeString(to), to)
	}
	if options.CC && len(e.CC) > 0 {
		cc := e.addressList(e.CC)
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

// raw returns the email metadata stored in the event, used to reply and to thread emails
func (e *Email) raw(options *ContentOptions) map[string]any {
	return map[string]any{
		options.MessageIDKey:  utils.Truncate(e.MessageID, maxIDLen),
		options.InReplyToKey:  utils.Truncate(e.InReplyTo, maxIDLen),
		options.ReferencesKey: e.references(),
		options.SubjectKey:    e.subject(),
		options.RcptToKey:     utils.Truncate(e.RcptTo, maxAddressLen),
		options.FromKey:       utils.Truncate(e.From, maxAddressLen),
		options.ToKey:         strings.Join(e.fitAddresses(strings.Split(e.To, ","), ","), ","),
		options.CcKey:         strings.Join(e.fitAddresses(e.CC, ", "), ", "),
	}
}

func (e *Email) subject() string {
	return utils.Truncate(e.Subject, maxSubjectLen)
}

// muted shows secondary text, like labels and addresses, in a softer color; the text must be escaped already
func (e *Email) muted(text string) string {
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
	return e.Sent.UTC().Format(dateLayout)
}

func (e *Email) hasSenderName() bool {
	return e.FromName != "" && !strings.EqualFold(e.FromName, e.From)
}

func (e *Email) senderHTML() string {
	from := html.EscapeString(utils.Truncate(e.From, maxAddressLen))
	if !e.hasSenderName() {
		return "<strong>" + from + "</strong>"
	}
	return "<strong>" + html.EscapeString(utils.Truncate(e.FromName, maxNameLen)) + "</strong> " + e.muted("&lt;"+from+"&gt;")
}

func (e *Email) senderText() string {
	from := utils.Truncate(e.From, maxAddressLen)
	if !e.hasSenderName() {
		return from
	}
	return utils.Truncate(e.FromName, maxNameLen) + " <" + from + ">"
}

// recipients formats the addresses of the To header, showing subaddresses separately
func (e *Email) recipients() string {
	addresses := strings.Split(e.To, ",")
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
	return e.addressList(addresses)
}

// addressList joins addresses to show them, telling how many more addresses did not fit
func (e *Email) addressList(addresses []string) string {
	shown := e.fitAddresses(addresses, ", ")
	list := strings.Join(shown, ", ")
	if hidden := len(addresses) - len(shown); hidden > 0 {
		list += " and " + strconv.Itoa(hidden) + " more"
	}
	return list
}

// fitAddresses returns the first addresses that fit into maxListLen together, each shortened to maxAddressLen
func (e *Email) fitAddresses(addresses []string, separator string) []string {
	fitting := make([]string, 0, len(addresses))
	size := 0
	for _, address := range addresses {
		address = utils.Truncate(address, maxAddressLen)
		if size+len(address) > maxListLen {
			break
		}
		size += len(address) + len(separator)
		fitting = append(fitting, address)
	}
	return fitting
}

// references returns the references to store in the event, the most recent ones that fit into maxListLen
func (e *Email) references() string {
	if len(e.References) <= maxListLen {
		return e.References
	}
	refs := strings.Fields(e.References)
	start, size := len(refs), 0
	for start > 0 && size+len(refs[start-1]) <= maxListLen {
		start--
		size += len(refs[start]) + 1
	}
	return strings.Join(refs[start:], " ")
}
