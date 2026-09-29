package email

import (
	"html"
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

	// renderer converts email HTML into a flat list of Matrix-compatible HTML blocks
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
	}
)

// renderHTML converts email HTML into Matrix-compatible HTML blocks; flatTables renders data tables as lines
func renderHTML(source string, resolve imageResolver, flatTables bool) []string {
	doc, err := xhtml.ParseWithOptions(strings.NewReader(source), xhtml.ParseOptionEnableScripting(false))
	if err != nil {
		return nil
	}
	state := &renderState{resolve: resolve, flatTables: flatTables, info: map[*xhtml.Node]nodeInfo{}}
	r := &renderer{renderState: state, wrapper: "p"}
	r.walk(doc)
	return r.finish()
}

func (r *renderer) sub() *renderer {
	return &renderer{renderState: r.renderState, wrapper: "p", depth: r.depth}
}

func (r *renderer) finish() []string {
	r.closeLeaf()
	for len(r.blocks) > 0 && r.blocks[len(r.blocks)-1] == "<hr>" {
		r.blocks = r.blocks[:len(r.blocks)-1]
	}
	return r.blocks
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
	if skippedElements[n.DataAtom] || isHidden(n) {
		return
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
		r.block(n, paragraphBreak(n))
	default:
		r.generic(n)
	}
}

func (r *renderer) generic(n *xhtml.Node) {
	if tag := inlineFormat(n.DataAtom); tag != "" {
		r.push("<"+tag+">", "</"+tag+">")
		r.styled(n, func() { r.walk(n) })
		r.pop()
		return
	}
	if blockElements[n.DataAtom] || isDisplayBlock(n) {
		r.block(n, breakLine)
		return
	}
	r.styled(n, func() { r.walk(n) })
}

func (r *renderer) block(n *xhtml.Node, level int) {
	if hasTopBorder(n) {
		r.rule()
	}
	r.breakAtLeast(level)
	r.styled(n, func() { r.walk(n) })
	r.breakAtLeast(level)
}

// styled wraps the content rendered by fn into the formatting set by the inline CSS of the element
func (r *renderer) styled(n *xhtml.Node, fn func()) {
	tags := styleFormats(n)
	for _, tag := range tags {
		r.push("<"+tag+">", "</"+tag+">")
	}
	fn()
	for range tags {
		r.pop()
	}
}

func (r *renderer) text(n *xhtml.Node) {
	if r.pre {
		for i, line := range strings.Split(n.Data, "\n") {
			if i > 0 {
				r.lineBreak()
			}
			r.inlineText(normalizeSpace(line))
		}
		return
	}
	if isBlankLine(n.Data) && r.isBlankBlock(n) {
		r.lineBreak()
		return
	}
	r.inlineText(normalizeSpace(n.Data))
}

// isBlankBlock detects blocks with nothing but non-breaking spaces, which email clients show as empty lines
func (r *renderer) isBlankBlock(n *xhtml.Node) bool {
	for parent := n.Parent; parent != nil && parent.Type == xhtml.ElementNode; parent = parent.Parent {
		if r.contentInfo(parent).visible() {
			return false
		}
		if blockLike[parent.DataAtom] || blockElements[parent.DataAtom] || isDisplayBlock(parent) {
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
	r.closeLeaf()
	r.breaks = 0
	r.space = false
	r.blocks = append(r.blocks, block)
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

func (r *renderer) rule() {
	r.closeLeaf()
	if len(r.blocks) == 0 || r.blocks[len(r.blocks)-1] == "<hr>" {
		return
	}
	r.addBlock("<hr>")
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
	width, height := elementSize(n)
	if hiddenSize(width, height) {
		return
	}
	alt := attr(n, "alt")
	var img *Image
	if src := strings.TrimSpace(attr(n, "src")); src != "" && r.resolve != nil {
		img = r.resolve(src)
	}
	if img == nil {
		r.inlineText(normalizeSpace(alt))
		return
	}
	width, height = displaySize(width, height, img.Width, img.Height)
	if hiddenSize(width, height) {
		return
	}
	title := strings.TrimSpace(normalizeSpace(attr(n, "title")))
	r.write(imageTag(img.URI, strings.TrimSpace(normalizeSpace(alt)), title, width, height))
}

// link renders a link; links styled as buttons are also bold, so calls to action stand out
func (r *renderer) link(n *xhtml.Node) {
	blockLink := isDisplayBlock(n)
	if blockLink {
		r.breakAtLeast(breakLine)
	}
	href := safeHref(attr(n, "href"))
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
	if hasNoListStyle(n) {
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
		open, closing = "<ol"+listStart(n)+">", "</ol>"
	}
	r.addBlock(open + strings.Join(items, "") + closing)
}

func (r *renderer) listItem(n *xhtml.Node) string {
	sub := r.sub()
	if n.Type == xhtml.ElementNode && n.DataAtom == atom.Li {
		if isHidden(n) {
			return ""
		}
		sub.styled(n, func() { sub.walk(n) })
	} else {
		sub.node(n)
	}
	return tight(sub.finish())
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
	if hasDescendant(n, atom.Code, 0) {
		var code strings.Builder
		textContent(n, &code, 0)
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

func (r *renderer) table(n *xhtml.Node) {
	for _, caption := range childElements(n, atom.Caption) {
		r.block(caption, breakLine)
	}
	rows := tableRows(n)
	if !r.flatTables && r.isDataTable(n, rows) {
		r.dataTable(rows)
		return
	}
	r.styled(n, func() {
		for _, row := range rows {
			r.layoutRow(row)
		}
	})
}

// layoutRow renders a row of a layout table: short inline cells share a line, other cells become paragraphs
func (r *renderer) layoutRow(row *xhtml.Node) {
	cells := r.visible(childElements(row, atom.Td, atom.Th))
	switch {
	case len(cells) == 0:
		return
	case r.joinable(cells):
		r.breakAtLeast(breakLine)
		r.styled(row, func() {
			for i, cell := range cells {
				if i > 0 {
					r.separator(r.infoOf(cells[i-1]))
				}
				r.styled(cell, func() { r.walk(cell) })
			}
		})
		r.breakAtLeast(breakLine)
	default:
		r.styled(row, func() {
			for _, cell := range cells {
				r.block(cell, breakParagraph)
			}
		})
	}
}

// separator separates cells joined on one line, e.g. "Name: value" or "Help · Privacy"
func (r *renderer) separator(previous nodeInfo) {
	r.space = true
	if previous.text > 0 && previous.lastRune != ':' {
		r.write("·")
		r.space = true
	}
}

func (r *renderer) joinable(cells []*xhtml.Node) bool {
	if len(cells) < 2 {
		return false
	}
	for _, cell := range cells {
		info := r.contentInfo(cell)
		if info.block || info.text > maxJoinedCellLen {
			return false
		}
	}
	return true
}

// isDataTable detects tables with headers holding tabular data, other tables are treated as layout
func (r *renderer) isDataTable(table *xhtml.Node, rows []*xhtml.Node) bool {
	role := strings.ToLower(attr(table, "role"))
	if role == "presentation" || role == "none" {
		return false
	}
	var headers, columns int
	for _, row := range rows {
		cells := childElements(row, atom.Td, atom.Th)
		columns = max(columns, len(cells))
		for _, cell := range cells {
			if r.contentInfo(cell).table {
				return false
			}
			if cell.DataAtom == atom.Th {
				headers++
			}
		}
	}
	return headers > 0 && columns > 1
}

func (r *renderer) dataTable(rows []*xhtml.Node) {
	var table strings.Builder
	for _, row := range rows {
		cells := childElements(row, atom.Td, atom.Th)
		if len(cells) == 0 || len(r.visible(cells)) == 0 {
			continue
		}
		table.WriteString("<tr>")
		for _, cell := range cells {
			tag := "td"
			if cell.DataAtom == atom.Th {
				tag = "th"
			}
			sub := r.sub()
			sub.styled(cell, func() { sub.walk(cell) })
			table.WriteString("<" + tag + ">" + joinParagraphs(sub.finish()) + "</" + tag + ">")
		}
		table.WriteString("</tr>")
	}
	if table.Len() > 0 {
		r.addBlock("<table>" + table.String() + "</table>")
	}
}

func (r *renderer) visible(nodes []*xhtml.Node) []*xhtml.Node {
	visible := make([]*xhtml.Node, 0, len(nodes))
	for _, n := range nodes {
		if r.infoOf(n).visible() {
			visible = append(visible, n)
		}
	}
	return visible
}

// isButton detects links styled as buttons, either by themselves or by a colored container around them
func (r *renderer) isButton(link *xhtml.Node) bool {
	if strings.EqualFold(attr(link, "role"), "button") || buttonClassRegex.MatchString(attr(link, "class")) {
		return true
	}
	style := styleOf(link)
	if hasBackground(style) {
		return true
	}
	if hasPadding(style) && (hasBorder(style) || strings.Contains(style["display"], "block")) {
		return true
	}
	parent := link.Parent
	for parent != nil && buttonWrappers[parent.DataAtom] {
		parent = parent.Parent
	}
	if parent == nil || parent.Type != xhtml.ElementNode {
		return false
	}
	if attr(parent, "bgcolor") == "" && !hasBackground(styleOf(parent)) {
		return false
	}
	return r.contentInfo(parent).text == r.infoOf(link).text
}

func (r *renderer) infoOf(n *xhtml.Node) nodeInfo {
	return r.nodeInfo(n, 0)
}

// contentInfo summarizes the children of the node, without the node itself
func (r *renderer) contentInfo(n *xhtml.Node) nodeInfo {
	return r.childrenInfo(n, 0)
}

func (r *renderer) nodeInfo(n *xhtml.Node, depth int) nodeInfo {
	if info, ok := r.info[n]; ok {
		return info
	}
	var info nodeInfo
	switch n.Type {
	case xhtml.TextNode:
		info = textInfo(n.Data)
	case xhtml.ElementNode:
		info = r.elementInfo(n, depth)
	default:
		return info
	}
	r.info[n] = info
	return info
}

func (r *renderer) elementInfo(n *xhtml.Node, depth int) nodeInfo {
	if skippedElements[n.DataAtom] || isHidden(n) || depth > maxRenderDepth {
		return nodeInfo{}
	}
	if n.DataAtom == atom.Img {
		if width, height := elementSize(n); hiddenSize(width, height) {
			return nodeInfo{}
		}
		return nodeInfo{images: 1}
	}
	info := r.childrenInfo(n, depth)
	info.block = info.block || blockLike[n.DataAtom] || isDisplayBlock(n)
	info.table = info.table || n.DataAtom == atom.Table
	return info
}

func (r *renderer) childrenInfo(n *xhtml.Node, depth int) nodeInfo {
	var info nodeInfo
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		info = info.merge(r.nodeInfo(child, depth+1))
	}
	return info
}

func (info nodeInfo) visible() bool {
	return info.text > 0 || info.images > 0
}

func (info nodeInfo) merge(other nodeInfo) nodeInfo {
	info.text += other.text
	info.images += other.images
	info.block = info.block || other.block
	info.table = info.table || other.table
	if other.lastRune != 0 {
		info.lastRune = other.lastRune
	}
	return info
}
