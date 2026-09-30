package email

import (
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	// maxRenderDepth limits how deep nested markup is rendered; the HTML parser rejects deeper documents anyway
	maxRenderDepth = 512
	// maxJoinedCellLen is the longest table cell text that may share a line with neighbor cells
	maxJoinedCellLen = 100
	// maxHeadingLen is the longest text rendered as a heading, longer "headings" are just bold paragraphs
	maxHeadingLen = 160
	// maxLeafSize is the size of a paragraph after which line breaks start new paragraphs, to allow truncation
	maxLeafSize = 2000

	breakLine      = 1
	breakParagraph = 2
)

// quote kinds, see quoteKind
const (
	quoteNone  = iota
	quoteBlock // the element holds the quoted messages
	quoteRest  // the quoted messages start with the element and go on to the end of the email
)

type (
	// imageResolver returns the uploaded image for an email image source, or nil if it cannot be shown
	imageResolver func(src string) *Image

	// inlineTag is an inline element that is written lazily, right before its first content
	inlineTag struct {
		open    string
		closing string
		written bool
	}

	// nodeInfo summarizes the visible content of an HTML subtree
	nodeInfo struct {
		text     int  // visible text length, in runes
		images   int  // visible images
		block    bool // has block-level elements or line breaks
		table    bool // has tables
		lastRune rune // last visible text character
	}

	// renderState is shared between a renderer and its nested renderers
	renderState struct {
		resolve    imageResolver
		flatTables bool
		info       map[*xhtml.Node]nodeInfo
	}

	// renderer converts one email HTML document into a flat list of Matrix-compatible HTML blocks
	renderer struct {
		*renderState
		blocks    []string
		leaf      strings.Builder
		wrapper   string
		inline    []*inlineTag
		breaks    int
		space     bool
		lineStart bool
		pre       bool
		depth     int
		top       bool // renders the whole document, not a list item, quote, or table cell
		quoteAt   int  // index of the first block of quoted earlier messages, -1 if there are none
		inQuote   int  // how many quote elements are being rendered
		quoteRest bool // the quote goes on to the end of the email, as in Outlook
		quoteDone bool // the email goes on after its quote, like replies written between quotes, so nothing is folded
	}
)

// newRenderer creates a renderer for one document; flatTables renders data tables as lines, for quote stripping
func newRenderer(resolve imageResolver, flatTables bool) *renderer {
	state := &renderState{resolve: resolve, flatTables: flatTables, info: map[*xhtml.Node]nodeInfo{}}
	return &renderer{renderState: state, wrapper: "p", top: true, quoteAt: -1}
}

// render converts email HTML into Matrix-compatible HTML blocks, and finds the first block of quoted messages, or -1
func (r *renderer) render(source string) (blocks []string, quoteAt int) {
	doc, err := xhtml.ParseWithOptions(strings.NewReader(source), xhtml.ParseOptionEnableScripting(false))
	if err != nil {
		return nil, -1
	}
	r.walk(doc)
	blocks = r.finish()
	// a quote with nothing before it is the whole email, e.g. a forward, and a quote with nothing in it is no quote
	if r.quoteAt <= 0 || r.quoteAt >= len(blocks) {
		return blocks, -1
	}
	return blocks, r.quoteAt
}

func (r *renderer) sub() *renderer {
	return &renderer{renderState: r.renderState, wrapper: "p", depth: r.depth, quoteAt: -1}
}

func (r *renderer) finish() []string {
	r.closeLeaf()
	for len(r.blocks) > 0 && r.blocks[len(r.blocks)-1] == "<hr>" {
		r.blocks = r.blocks[:len(r.blocks)-1]
	}
	return r.blocks
}

// startQuote marks the start of quoted earlier messages, taking in a line like "On Monday, John wrote:" before them
func (r *renderer) startQuote() {
	r.closeLeaf()
	r.breaks = 0
	for len(r.blocks) > 0 && r.blocks[len(r.blocks)-1] == "<hr>" {
		r.blocks = r.blocks[:len(r.blocks)-1]
	}
	r.quoteAt = len(r.blocks)
	if r.quoteAt > 0 && r.isAttribution(r.blocks[r.quoteAt-1]) {
		r.quoteAt--
	}
}

func (r *renderer) walk(n *xhtml.Node) {
	if r.depth >= maxRenderDepth {
		return
	}
	r.depth++
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		r.node(child)
	}
	r.depth--
}

func (r *renderer) node(n *xhtml.Node) {
	switch n.Type {
	case xhtml.TextNode:
		r.text(n)
	case xhtml.ElementNode:
		r.element(n)
	case xhtml.DocumentNode:
		r.walk(n)
	default:
		return
	}
}

func (r *renderer) element(n *xhtml.Node) {
	if skippedElements[n.DataAtom] || r.isHidden(n) {
		return
	}
	if r.top {
		if kind := r.quoteKind(n); kind != quoteNone {
			r.enterQuote(kind)
			defer func() { r.inQuote-- }()
		}
	}
	switch n.DataAtom {
	case atom.Br:
		r.lineBreak()
	case atom.Hr:
		r.rule()
	case atom.Img:
		r.image(n)
	case atom.A:
		r.link(n)
	case atom.H1:
		r.heading(n, "h3")
	case atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		r.heading(n, "h4")
	case atom.Ul, atom.Ol:
		r.list(n)
	case atom.Blockquote:
		r.quote(n)
	case atom.Pre:
		r.preformatted(n)
	case atom.Table:
		r.table(n)
	case atom.P:
		r.block(n, r.paragraphBreak(n))
	default:
		r.generic(n)
	}
}

func (r *renderer) generic(n *xhtml.Node) {
	if tag := r.inlineFormat(n.DataAtom); tag != "" {
		r.push("<"+tag+">", "</"+tag+">")
		r.styled(n, func() { r.walk(n) })
		r.pop()
		return
	}
	if blockElements[n.DataAtom] || r.isDisplayBlock(n) {
		r.block(n, breakLine)
		return
	}
	r.styled(n, func() { r.walk(n) })
}

func (r *renderer) block(n *xhtml.Node, level int) {
	if r.hasTopBorder(n) {
		r.rule()
	}
	r.breakAtLeast(level)
	r.styled(n, func() { r.walk(n) })
	r.breakAtLeast(level)
}

// styled wraps the content rendered by fn into the formatting set by the inline CSS of the element
func (r *renderer) styled(n *xhtml.Node, fn func()) {
	tags := r.styleFormats(n)
	for _, tag := range tags {
		r.push("<"+tag+">", "</"+tag+">")
	}
	fn()
	for range tags {
		r.pop()
	}
}
