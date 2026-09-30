package email

import (
	"html"
	"math"
	"net/url"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// rule adds a separator line, unless it would start the text, the quote, or follow another line
func (r *renderer) rule() {
	r.closeLeaf()
	if len(r.blocks) == 0 || r.blocks[len(r.blocks)-1] == "<hr>" || r.quoteAt == len(r.blocks) {
		return
	}
	r.appendBlock("<hr>")
}

func (r *renderer) heading(n *xhtml.Node, tag string) {
	if r.infoOf(n).text > maxHeadingLen {
		r.push("<strong>", "</strong>")
		r.block(n, breakParagraph)
		r.pop()
		return
	}
	r.closeLeaf()
	r.breaks = 0
	wrapper := r.wrapper
	r.wrapper = tag
	r.styled(n, func() { r.walk(n) })
	r.closeLeaf()
	r.wrapper = wrapper
	r.breakAtLeast(breakParagraph)
}

func (r *renderer) image(n *xhtml.Node) {
	width, height := r.elementSize(n)
	if r.hiddenSize(width, height) {
		return
	}
	alt := r.attr(n, "alt")
	var img *Image
	if src := strings.TrimSpace(r.attr(n, "src")); src != "" && r.resolve != nil {
		img = r.resolve(src)
	}
	if img == nil {
		r.inlineText(r.normalizeSpace(alt))
		return
	}
	width, height = img.displaySize(width, height)
	if r.hiddenSize(width, height) {
		return
	}
	title := strings.TrimSpace(r.normalizeSpace(r.attr(n, "title")))
	r.write(img.tag(strings.TrimSpace(r.normalizeSpace(alt)), title, width, height))
}

// link renders a link; links styled as buttons are also bold, so calls to action stand out
func (r *renderer) link(n *xhtml.Node) {
	blockLink := r.isDisplayBlock(n)
	if blockLink {
		r.breakAtLeast(breakLine)
	}
	href := r.safeHref(r.attr(n, "href"))
	button := href != "" && r.isButton(n)
	if href != "" {
		r.push(`<a href="`+html.EscapeString(href)+`">`, "</a>")
	}
	if button {
		r.push("<strong>", "</strong>")
	}
	r.styled(n, func() { r.walk(n) })
	if button {
		r.pop()
	}
	if href != "" {
		r.pop()
	}
	if blockLink {
		r.breakAtLeast(breakLine)
	}
}

func (r *renderer) list(n *xhtml.Node) {
	if r.hasNoListStyle(n) {
		r.block(n, breakLine)
		return
	}
	items := make([]string, 0, 8)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if item := r.listItem(child); item != "" {
			items = append(items, "<li>"+item+"</li>")
		}
	}
	if len(items) == 0 {
		return
	}
	open, closing := "<ul>", "</ul>"
	if n.DataAtom == atom.Ol {
		open, closing = "<ol"+r.listStart(n)+">", "</ol>"
	}
	r.addBlock(open + strings.Join(items, "") + closing)
}

func (r *renderer) listItem(n *xhtml.Node) string {
	sub := r.sub()
	if n.Type == xhtml.ElementNode && n.DataAtom == atom.Li {
		if r.isHidden(n) {
			return ""
		}
		sub.styled(n, func() { sub.walk(n) })
	} else {
		sub.node(n)
	}
	return r.tight(sub.finish())
}

func (r *renderer) quote(n *xhtml.Node) {
	sub := r.sub()
	sub.styled(n, func() { sub.walk(n) })
	if blocks := sub.finish(); len(blocks) > 0 {
		r.addBlock("<blockquote>" + strings.Join(blocks, "") + "</blockquote>")
	}
}

// preformatted renders code as a code block, and other preformatted text as text with its line breaks
func (r *renderer) preformatted(n *xhtml.Node) {
	if r.hasDescendant(n, atom.Code, 0) || r.isCode(n) || (n.Parent != nil && r.isCode(n.Parent)) {
		var code strings.Builder
		r.textContent(n, &code, 0)
		text := strings.Trim(strings.ReplaceAll(code.String(), "\r\n", "\n"), "\n")
		if strings.TrimSpace(text) != "" {
			r.addBlock("<pre><code>" + html.EscapeString(text) + "</code></pre>")
		}
		return
	}
	pre := r.pre
	r.pre = true
	r.block(n, breakParagraph)
	r.pre = pre
}

// isButton detects links styled as buttons, either by themselves or by a colored container around them
func (r *renderer) isButton(link *xhtml.Node) bool {
	if strings.EqualFold(r.attr(link, "role"), "button") || buttonClassRegex.MatchString(r.attr(link, "class")) {
		return true
	}
	style := r.styleOf(link)
	if r.hasBackground(style) {
		return true
	}
	if r.hasPadding(style) && (r.hasBorder(style) || strings.Contains(style["display"], "block")) {
		return true
	}
	parent := link.Parent
	for parent != nil && buttonWrappers[parent.DataAtom] {
		parent = parent.Parent
	}
	if parent == nil || parent.Type != xhtml.ElementNode {
		return false
	}
	if r.attr(parent, "bgcolor") == "" && !r.hasBackground(r.styleOf(parent)) {
		return false
	}
	return r.contentInfo(parent).text == r.infoOf(link).text
}

// hasNoListStyle detects lists without bullets or numbers, which are usually menus or link rows
func (r *renderer) hasNoListStyle(n *xhtml.Node) bool {
	style := r.styleOf(n)
	return style["list-style"] == "none" || style["list-style-type"] == "none"
}

func (r *renderer) listStart(n *xhtml.Node) string {
	start, err := strconv.Atoi(strings.TrimSpace(r.attr(n, "start")))
	if err != nil || start == 1 {
		return ""
	}
	return ` start="` + strconv.Itoa(start) + `"`
}

// paragraphBreak returns the spacing of a paragraph; Outlook uses margin-less paragraphs for single lines
func (r *renderer) paragraphBreak(n *xhtml.Node) int {
	if r.hasClass(n, "MsoNormal") {
		return breakLine
	}
	style := r.styleOf(n)
	if r.allZero(style["margin"]) || (r.isZero(style["margin-top"]) && r.isZero(style["margin-bottom"])) {
		return breakLine
	}
	return breakParagraph
}

// safeHref returns the link target if it uses a scheme allowed in Matrix messages, or an empty string
func (r *renderer) safeHref(href string) string {
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

// elementSize returns the width and height set by CSS or attributes of an element, -1 when not set
func (r *renderer) elementSize(n *xhtml.Node) (width, height int) {
	style := r.styleOf(n)
	return r.dimension(style["width"], r.attr(n, "width")), r.dimension(style["height"], r.attr(n, "height"))
}

func (r *renderer) dimension(css, attribute string) int {
	if value, ok := r.pixels(css); ok {
		return value
	}
	if value, ok := r.pixels(attribute); ok {
		return value
	}
	return -1
}

// pixels parses a length in pixels, like 180 or 180px; other units and percents are not supported
func (r *renderer) pixels(value string) (int, bool) {
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
func (r *renderer) hiddenSize(width, height int) bool {
	return (width >= 0 && width <= 2) || (height >= 0 && height <= 2)
}
