package email

import (
	"html"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	// maxImageWidth is the widest an embedded image is displayed, in pixels
	maxImageWidth = 600
	// maxDimension caps image dimensions parsed from HTML, in pixels
	maxDimension = 10000
	// maxAttributionLen is the longest line introducing a quote, like "On Monday, John <john@example.com> wrote:"
	maxAttributionLen = 300
	// maxReplyHeaderLen is the longest text of the header Outlook puts above a quoted message
	maxReplyHeaderLen = 1000
)

var (
	// skippedElements are never shown to the reader
	skippedElements = mustElementSet("head title meta link style script template iframe frame frameset object embed " +
		"applet param svg math canvas audio video source track map area input select textarea option datalist colgroup col")

	// blockElements start on a new line
	blockElements = mustElementSet("html body div section article header footer main aside nav center address figure " +
		"figcaption dl dt dd form fieldset legend details summary caption li tbody thead tfoot tr td th")

	// blockLike are elements that break the text flow, used to decide whether table cells fit on one line
	blockLike = mustElementSet("p h1 h2 h3 h4 h5 h6 ul ol blockquote pre table hr br div center section article " +
		"header footer li dl address figure form")

	// buttonWrappers may sit between a button link and its colored container
	buttonWrappers = mustElementSet("span font b strong i em center")

	boldWeights = map[string]bool{"bold": true, "bolder": true, "600": true, "700": true, "800": true, "900": true}

	blockDisplays = map[string]bool{
		"block": true, "flex": true, "grid": true, "table": true, "list-item": true, "table-row": true, "table-cell": true,
	}

	noBackground = map[string]bool{"": true, "none": true, "transparent": true, "inherit": true, "initial": true, "unset": true}

	// safeSchemes are the link schemes allowed by the Matrix spec
	safeSchemes = map[string]bool{"http": true, "https": true, "ftp": true, "mailto": true, "magnet": true}

	buttonClassRegex = regexp.MustCompile(`(?i)(?:^|[\s_-])(?:btn|button|cta)(?:$|[\s_-])`)

	// codeClassRegex matches classes of code blocks, like "highlight" on GitHub or "language-go"
	codeClassRegex = regexp.MustCompile(`(?i)(?:^|[\s_-])(?:highlight|code|syntax|sourcecode|prettyprint|hljs|language|lang)(?:$|[\s_-])`)

	// quote markers of Gmail, Yahoo, Proton Mail, Thunderbird, and Zoho; a Gmail quote without a blockquote is a forward
	quoteClassRegex = regexp.MustCompile(`(?i)(?:^|\s)(?:gmail_quote|yahoo_quoted|protonmail_quote|moz-cite-prefix|zmail_extra)(?:$|\s)`)
	gmailQuoteRegex = regexp.MustCompile(`(?i)(?:^|\s)gmail_quote(?:$|\s)`)
	// Outlook puts a header above the quoted message, which goes on to the end of the email
	quoteRestIDRegex = regexp.MustCompile(`(?i)^(?:x_)?(?:divRplyFwdMsg|appendonsend)$`)
	// Outlook for Mac and the new Outlook wrap the quoted message into one element
	quoteBlockIDRegex = regexp.MustCompile(`(?i)^(?:x_)?(?:mail-editor-reference-message-container|OLK_SRC_BODY_SECTION)$`)

	tagRegex = regexp.MustCompile(`<[^>]*>`)
)

// mustElementSet builds a set of HTML elements from space-separated tag names
func mustElementSet(names string) map[atom.Atom]bool {
	set := map[atom.Atom]bool{}
	for _, name := range strings.Fields(names) {
		element := atom.Lookup([]byte(name))
		if element == 0 {
			panic("unknown HTML element: " + name)
		}
		set[element] = true
	}
	return set
}

// inlineFormat returns the Matrix formatting tag of an HTML formatting element, or an empty string
func inlineFormat(element atom.Atom) string {
	switch element {
	case atom.B, atom.Strong:
		return "strong"
	case atom.I, atom.Em, atom.Cite, atom.Dfn, atom.Var:
		return "em"
	case atom.U, atom.Ins:
		return "u"
	case atom.S, atom.Strike, atom.Del:
		return "del"
	case atom.Code, atom.Tt, atom.Kbd, atom.Samp:
		return "code"
	case atom.Sup:
		return "sup"
	case atom.Sub:
		return "sub"
	default:
		return ""
	}
}

func attr(n *xhtml.Node, key string) string {
	value, _ := attrValue(n, key)
	return value
}

func attrValue(n *xhtml.Node, key string) (string, bool) {
	for _, attribute := range n.Attr {
		if attribute.Key == key && attribute.Namespace == "" {
			return attribute.Val, true
		}
	}
	return "", false
}

func hasClass(n *xhtml.Node, class string) bool {
	for _, name := range strings.Fields(attr(n, "class")) {
		if strings.EqualFold(name, class) {
			return true
		}
	}
	return false
}

// styleOf parses the inline CSS of an element into lowercase property-value pairs
func styleOf(n *xhtml.Node) map[string]string {
	style := attr(n, "style")
	if style == "" {
		return nil
	}
	props := map[string]string{}
	for _, declaration := range strings.Split(style, ";") {
		name, value, ok := strings.Cut(declaration, ":")
		if !ok {
			continue
		}
		value = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), "!important")
		props[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
	}
	return props
}

// isHidden detects elements invisible to the reader, like preheaders and content for other devices
func isHidden(n *xhtml.Node) bool {
	if n.Type != xhtml.ElementNode {
		return false
	}
	if _, ok := attrValue(n, "hidden"); ok {
		return true
	}
	style := styleOf(n)
	if style["display"] == "none" || style["visibility"] == "hidden" || isZero(style["opacity"]) || isZero(style["max-height"]) {
		return true
	}
	return style["overflow"] == "hidden" && (isZero(style["height"]) || isZero(style["width"]) || isZero(style["max-width"]))
}

// isZero reports whether a CSS length is zero, e.g. 0, 0px, 0%
func isZero(value string) bool {
	value = strings.TrimRight(strings.TrimSpace(value), "abcdefghijklmnopqrstuvwxyz%")
	if value == "" {
		return false
	}
	number, err := strconv.ParseFloat(value, 64)
	return err == nil && number == 0
}

// allZero reports whether all values of a CSS shorthand are zero, e.g. margin: 0 0 0 0
func allZero(value string) bool {
	fields := strings.Fields(value)
	return len(fields) > 0 && !slices.ContainsFunc(fields, func(field string) bool { return !isZero(field) })
}

// styleFormats returns Matrix formatting tags equivalent to the inline CSS of the element
func styleFormats(n *xhtml.Node) []string {
	style := styleOf(n)
	if style == nil {
		return nil
	}
	var tags []string
	if boldWeights[style["font-weight"]] {
		tags = append(tags, "strong")
	}
	if style["font-style"] == "italic" || style["font-style"] == "oblique" {
		tags = append(tags, "em")
	}
	decoration := style["text-decoration"] + " " + style["text-decoration-line"]
	if strings.Contains(decoration, "underline") && n.DataAtom != atom.A {
		tags = append(tags, "u")
	}
	if strings.Contains(decoration, "line-through") {
		tags = append(tags, "del")
	}
	return tags
}

func isDisplayBlock(n *xhtml.Node) bool {
	return blockDisplays[styleOf(n)["display"]]
}

func hasBackground(style map[string]string) bool {
	if color := style["background-color"]; !noBackground[color] {
		return true
	}
	return !noBackground[style["background"]]
}

func hasPadding(style map[string]string) bool {
	for _, name := range []string{"padding", "padding-top", "padding-right", "padding-bottom", "padding-left"} {
		if value := style[name]; value != "" && !allZero(value) {
			return true
		}
	}
	return false
}

func hasBorder(style map[string]string) bool {
	border := style["border"]
	if border != "" && border != "none" && !allZero(border) {
		return true
	}
	radius := style["border-radius"]
	return radius != "" && !allZero(radius)
}

// hasTopBorder detects blocks with a line above them, like footers and the quoted part of Outlook replies
func hasTopBorder(n *xhtml.Node) bool {
	border := styleOf(n)["border-top"]
	if border == "" || strings.Contains(border, "none") || strings.Contains(border, "hidden") {
		return false
	}
	return !slices.ContainsFunc(strings.Fields(border), isZero)
}

// quoteKind detects where mail apps put the quoted earlier messages of a reply
func (r *renderer) quoteKind(n *xhtml.Node) int {
	if n.DataAtom != atom.Div && n.DataAtom != atom.Blockquote {
		return quoteNone
	}
	id := attr(n, "id")
	class := attr(n, "class")
	switch {
	case quoteRestIDRegex.MatchString(id):
		return quoteRest
	case quoteBlockIDRegex.MatchString(id):
		return quoteBlock
	case n.DataAtom == atom.Blockquote && (strings.EqualFold(attr(n, "type"), "cite") || quoteClassRegex.MatchString(class)):
		return quoteBlock
	case n.DataAtom == atom.Div && quoteClassRegex.MatchString(class):
		if gmailQuoteRegex.MatchString(class) && !hasDescendant(n, atom.Blockquote, 0) {
			return quoteNone
		}
		return quoteBlock
	case n.DataAtom == atom.Div && hasTopBorder(n) && r.contentInfo(n).text <= maxReplyHeaderLen && countLabels(n, 0) >= 2:
		return quoteRest
	default:
		return quoteNone
	}
}

// countLabels counts bold labels, like "From:" and "Sent:" in the header Outlook puts above a quoted message
func countLabels(n *xhtml.Node, depth int) int {
	if depth > maxRenderDepth {
		return 0
	}
	count := 0
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode || skippedElements[child.DataAtom] || isHidden(child) {
			continue
		}
		if child.DataAtom == atom.B || child.DataAtom == atom.Strong {
			var text strings.Builder
			textContent(child, &text, depth)
			if strings.HasSuffix(strings.TrimSpace(normalizeSpace(text.String())), ":") {
				count++
			}
			continue
		}
		count += countLabels(child, depth+1)
	}
	return count
}

// isAttribution detects a short paragraph ending with a colon, like "On Monday, John <john@example.com> wrote:"
func isAttribution(block string) bool {
	inner, ok := unwrapParagraph(block)
	if !ok || strings.Contains(inner, "<br>") {
		return false
	}
	text := strings.TrimSpace(blockText(inner))
	return strings.HasSuffix(text, ":") && utf8.RuneCountInString(text) <= maxAttributionLen
}

// blockText returns the visible text of a rendered block
func blockText(block string) string {
	return html.UnescapeString(tagRegex.ReplaceAllString(block, ""))
}

func isCode(n *xhtml.Node) bool {
	return n.Type == xhtml.ElementNode && codeClassRegex.MatchString(attr(n, "class"))
}

// hasNoListStyle detects lists without bullets or numbers, which are usually menus or link rows
func hasNoListStyle(n *xhtml.Node) bool {
	style := styleOf(n)
	return style["list-style"] == "none" || style["list-style-type"] == "none"
}

func listStart(n *xhtml.Node) string {
	start, err := strconv.Atoi(strings.TrimSpace(attr(n, "start")))
	if err != nil || start == 1 {
		return ""
	}
	return ` start="` + strconv.Itoa(start) + `"`
}

// paragraphBreak returns the spacing of a paragraph; Outlook uses margin-less paragraphs for single lines
func paragraphBreak(n *xhtml.Node) int {
	if hasClass(n, "MsoNormal") {
		return breakLine
	}
	style := styleOf(n)
	if allZero(style["margin"]) || (isZero(style["margin-top"]) && isZero(style["margin-bottom"])) {
		return breakLine
	}
	return breakParagraph
}

// safeHref returns the link target if it uses a scheme allowed in Matrix messages, or an empty string
func safeHref(href string) string {
	href = strings.TrimSpace(href)
	parsed, err := url.Parse(href)
	if err != nil || !safeSchemes[strings.ToLower(parsed.Scheme)] {
		return ""
	}
	if parsed.Host == "" && parsed.Opaque == "" {
		return ""
	}
	return href
}

// normalizeSpace collapses whitespace like browsers do, and removes invisible characters used for padding
func normalizeSpace(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	var space bool
	var last rune
	runes := []rune(text)
	for i, char := range runes {
		switch {
		case unicode.IsSpace(char):
			space = true
		case isInvisible(char):
			continue
		case char == '\u200c' || char == '\u200d':
			if space || last == 0 || i+1 == len(runes) || unicode.IsSpace(runes[i+1]) || isInvisible(runes[i+1]) {
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
func isInvisible(char rune) bool {
	switch char {
	case '\u200b', '\u2060', '\ufeff', '\u00ad', '\u034f', '\u180e', '\u2061', '\u2062', '\u2063', '\u2064':
		return true
	default:
		return false
	}
}

// isBlankLine detects text made of non-breaking spaces only
func isBlankLine(text string) bool {
	return strings.ContainsRune(text, '\u00a0') && strings.TrimSpace(normalizeSpace(text)) == ""
}

func textInfo(text string) nodeInfo {
	text = strings.TrimSpace(normalizeSpace(text))
	if text == "" {
		return nodeInfo{}
	}
	last, _ := utf8.DecodeLastRuneInString(text)
	return nodeInfo{text: utf8.RuneCountInString(text), lastRune: last}
}

// textContent collects the visible text of the node, turning line breaks into newlines
func textContent(n *xhtml.Node, out *strings.Builder, depth int) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		switch {
		case child.Type == xhtml.TextNode:
			out.WriteString(child.Data)
		case child.Type != xhtml.ElementNode || skippedElements[child.DataAtom] || isHidden(child):
			continue
		case child.DataAtom == atom.Br:
			out.WriteByte('\n')
		case depth < maxRenderDepth:
			textContent(child, out, depth+1)
		}
	}
}

func hasDescendant(n *xhtml.Node, element atom.Atom, depth int) bool {
	if depth > maxRenderDepth {
		return false
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode {
			continue
		}
		if child.DataAtom == element || hasDescendant(child, element, depth+1) {
			return true
		}
	}
	return false
}

// childElements returns visible child elements of the given types
func childElements(n *xhtml.Node, elements ...atom.Atom) []*xhtml.Node {
	var children []*xhtml.Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && slices.Contains(elements, child.DataAtom) && !isHidden(child) {
			children = append(children, child)
		}
	}
	return children
}

// tableRows returns visible rows of the table, without rows of nested tables
func tableRows(table *xhtml.Node) []*xhtml.Node {
	rows := childElements(table, atom.Tr)
	for _, section := range childElements(table, atom.Thead, atom.Tbody, atom.Tfoot) {
		rows = append(rows, childElements(section, atom.Tr)...)
	}
	return rows
}

// tight unwraps a single paragraph, so list items and cells do not get paragraph spacing
func tight(blocks []string) string {
	if len(blocks) == 1 {
		if inner, ok := unwrapParagraph(blocks[0]); ok {
			return inner
		}
	}
	return strings.Join(blocks, "")
}

func joinParagraphs(blocks []string) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		inner, _ := unwrapParagraph(block)
		parts = append(parts, inner)
	}
	return strings.Join(parts, "<br>")
}

func unwrapParagraph(block string) (string, bool) {
	if strings.HasPrefix(block, "<p>") && strings.HasSuffix(block, "</p>") {
		return block[len("<p>") : len(block)-len("</p>")], true
	}
	return block, false
}

// elementSize returns the width and height set by CSS or attributes of an element, -1 when not set
func elementSize(n *xhtml.Node) (width, height int) {
	style := styleOf(n)
	return dimension(style["width"], attr(n, "width")), dimension(style["height"], attr(n, "height"))
}

func dimension(css, attribute string) int {
	if value, ok := pixels(css); ok {
		return value
	}
	if value, ok := pixels(attribute); ok {
		return value
	}
	return -1
}

// pixels parses a length in pixels, like 180 or 180px; other units and percents are not supported
func pixels(value string) (int, bool) {
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "px"))
	if value == "" {
		return 0, false
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return int(math.Round(math.Min(number, maxDimension))), true
}

// hiddenSize detects (almost) invisible dimensions used by tracking pixels and spacers; -1 means unknown
func hiddenSize(width, height int) bool {
	return (width >= 0 && width <= 2) || (height >= 0 && height <= 2)
}

// displaySize calculates the size of an embedded image, keeping its aspect ratio; -1 means unknown
func displaySize(width, height, naturalWidth, naturalHeight int) (displayWidth, displayHeight int) {
	if naturalWidth > 0 && naturalHeight > 0 {
		switch {
		case width < 0 && height < 0:
			width, height = scale(naturalWidth, 1, 1), scale(naturalHeight, 1, 1)
		case height < 0:
			height = scale(width, naturalHeight, naturalWidth)
		case width < 0:
			width = scale(height, naturalWidth, naturalHeight)
		}
	}
	if width > maxImageWidth {
		if height > 0 {
			height = scale(height, maxImageWidth, width)
		}
		width = maxImageWidth
	}
	return width, height
}

// scale returns value * numerator / denominator, limited to maxDimension to stay safe with huge images
func scale(value, numerator, denominator int) int {
	return int(math.Min(float64(value)*float64(numerator)/float64(denominator), maxDimension))
}

func imageTag(uri, alt, title string, width, height int) string {
	var tag strings.Builder
	tag.WriteString(`<img src="` + html.EscapeString(uri) + `"`)
	if alt != "" {
		tag.WriteString(` alt="` + html.EscapeString(alt) + `"`)
	}
	if title != "" {
		tag.WriteString(` title="` + html.EscapeString(title) + `"`)
	}
	if width > 0 {
		tag.WriteString(` width="` + strconv.Itoa(width) + `"`)
	}
	if height > 0 {
		tag.WriteString(` height="` + strconv.Itoa(height) + `"`)
	}
	tag.WriteString(">")
	return tag.String()
}
