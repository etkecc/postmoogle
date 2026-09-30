package email

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
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

// mustElementSet builds a set of HTML elements from space-separated tag names, for the package-level sets above
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
func (r *renderer) inlineFormat(element atom.Atom) string {
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

func (r *renderer) attr(n *xhtml.Node, key string) string {
	value, _ := r.attrValue(n, key)
	return value
}

func (r *renderer) attrValue(n *xhtml.Node, key string) (string, bool) {
	for _, attribute := range n.Attr {
		if attribute.Key == key && attribute.Namespace == "" {
			return attribute.Val, true
		}
	}
	return "", false
}

func (r *renderer) hasClass(n *xhtml.Node, class string) bool {
	for _, name := range strings.Fields(r.attr(n, "class")) {
		if strings.EqualFold(name, class) {
			return true
		}
	}
	return false
}

// styleOf parses the inline CSS of an element into lowercase property-value pairs
func (r *renderer) styleOf(n *xhtml.Node) map[string]string {
	style := r.attr(n, "style")
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
func (r *renderer) isHidden(n *xhtml.Node) bool {
	if n.Type != xhtml.ElementNode {
		return false
	}
	if _, ok := r.attrValue(n, "hidden"); ok {
		return true
	}
	style := r.styleOf(n)
	if style["display"] == "none" || style["visibility"] == "hidden" || r.isZero(style["opacity"]) || r.isZero(style["max-height"]) {
		return true
	}
	return style["overflow"] == "hidden" && (r.isZero(style["height"]) || r.isZero(style["width"]) || r.isZero(style["max-width"]))
}

// isZero reports whether a CSS length is zero, e.g. 0, 0px, 0%
func (r *renderer) isZero(value string) bool {
	value = strings.TrimRight(strings.TrimSpace(value), "abcdefghijklmnopqrstuvwxyz%")
	if value == "" {
		return false
	}
	number, err := strconv.ParseFloat(value, 64)
	return err == nil && number == 0
}

// allZero reports whether all values of a CSS shorthand are zero, e.g. margin: 0 0 0 0
func (r *renderer) allZero(value string) bool {
	fields := strings.Fields(value)
	return len(fields) > 0 && !slices.ContainsFunc(fields, func(field string) bool { return !r.isZero(field) })
}

// styleFormats returns Matrix formatting tags equivalent to the inline CSS of the element
func (r *renderer) styleFormats(n *xhtml.Node) []string {
	style := r.styleOf(n)
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

func (r *renderer) isDisplayBlock(n *xhtml.Node) bool {
	return blockDisplays[r.styleOf(n)["display"]]
}

func (r *renderer) hasBackground(style map[string]string) bool {
	if color := style["background-color"]; !noBackground[color] {
		return true
	}
	return !noBackground[style["background"]]
}

func (r *renderer) hasPadding(style map[string]string) bool {
	for _, name := range []string{"padding", "padding-top", "padding-right", "padding-bottom", "padding-left"} {
		if value := style[name]; value != "" && !r.allZero(value) {
			return true
		}
	}
	return false
}

func (r *renderer) hasBorder(style map[string]string) bool {
	border := style["border"]
	if border != "" && border != "none" && !r.allZero(border) {
		return true
	}
	radius := style["border-radius"]
	return radius != "" && !r.allZero(radius)
}

// hasTopBorder detects blocks with a line above them, like footers and the quoted part of Outlook replies
func (r *renderer) hasTopBorder(n *xhtml.Node) bool {
	border := r.styleOf(n)["border-top"]
	if border == "" || strings.Contains(border, "none") || strings.Contains(border, "hidden") {
		return false
	}
	return !slices.ContainsFunc(strings.Fields(border), r.isZero)
}
