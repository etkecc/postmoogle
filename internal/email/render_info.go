package email

import (
	"html"
	"strings"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	// maxAttributionLen is the longest line introducing a quote, like "On Monday, John <john@example.com> wrote:"
	maxAttributionLen = 300
	// maxReplyHeaderLen is the longest text of the header Outlook puts above a quoted message
	maxReplyHeaderLen = 1000
)

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
		info = r.textInfo(n.Data)
	case xhtml.ElementNode:
		info = r.elementInfo(n, depth)
	default:
		return info
	}
	r.info[n] = info
	return info
}

func (r *renderer) elementInfo(n *xhtml.Node, depth int) nodeInfo {
	if skippedElements[n.DataAtom] || r.isHidden(n) || depth > maxRenderDepth {
		return nodeInfo{}
	}
	if n.DataAtom == atom.Img {
		if width, height := r.elementSize(n); r.hiddenSize(width, height) {
			return nodeInfo{}
		}
		return nodeInfo{images: 1}
	}
	info := r.childrenInfo(n, depth)
	info.block = info.block || blockLike[n.DataAtom] || r.isDisplayBlock(n)
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

func (r *renderer) textInfo(text string) nodeInfo {
	text = strings.TrimSpace(r.normalizeSpace(text))
	if text == "" {
		return nodeInfo{}
	}
	last, _ := utf8.DecodeLastRuneInString(text)
	return nodeInfo{text: utf8.RuneCountInString(text), lastRune: last}
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

// textContent collects the visible text of the node, turning line breaks into newlines
func (r *renderer) textContent(n *xhtml.Node, out *strings.Builder, depth int) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		switch {
		case child.Type == xhtml.TextNode:
			out.WriteString(child.Data)
		case child.Type != xhtml.ElementNode || skippedElements[child.DataAtom] || r.isHidden(child):
			continue
		case child.DataAtom == atom.Br:
			out.WriteByte('\n')
		case depth < maxRenderDepth:
			r.textContent(child, out, depth+1)
		}
	}
}

func (r *renderer) hasDescendant(n *xhtml.Node, element atom.Atom, depth int) bool {
	if depth > maxRenderDepth {
		return false
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode {
			continue
		}
		if child.DataAtom == element || r.hasDescendant(child, element, depth+1) {
			return true
		}
	}
	return false
}

// quoteKind detects where mail apps put the quoted earlier messages of a reply
func (r *renderer) quoteKind(n *xhtml.Node) int {
	if n.DataAtom != atom.Div && n.DataAtom != atom.Blockquote {
		return quoteNone
	}
	id := r.attr(n, "id")
	class := r.attr(n, "class")
	switch {
	case quoteRestIDRegex.MatchString(id):
		return quoteRest
	case quoteBlockIDRegex.MatchString(id):
		return quoteBlock
	case n.DataAtom == atom.Blockquote && (strings.EqualFold(r.attr(n, "type"), "cite") || quoteClassRegex.MatchString(class)):
		return quoteBlock
	case n.DataAtom == atom.Div && quoteClassRegex.MatchString(class):
		if gmailQuoteRegex.MatchString(class) && !r.hasDescendant(n, atom.Blockquote, 0) {
			return quoteNone
		}
		return quoteBlock
	case n.DataAtom == atom.Div && r.hasTopBorder(n) && r.contentInfo(n).text <= maxReplyHeaderLen && r.countLabels(n, 0) >= 2:
		return quoteRest
	default:
		return quoteNone
	}
}

// countLabels counts bold labels, like "From:" and "Sent:" in the header Outlook puts above a quoted message
func (r *renderer) countLabels(n *xhtml.Node, depth int) int {
	if depth > maxRenderDepth {
		return 0
	}
	count := 0
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode || skippedElements[child.DataAtom] || r.isHidden(child) {
			continue
		}
		if child.DataAtom == atom.B || child.DataAtom == atom.Strong {
			var text strings.Builder
			r.textContent(child, &text, depth)
			if strings.HasSuffix(strings.TrimSpace(r.normalizeSpace(text.String())), ":") {
				count++
			}
			continue
		}
		count += r.countLabels(child, depth+1)
	}
	return count
}

// isAttribution detects a short paragraph ending with a colon, like "On Monday, John <john@example.com> wrote:"
func (r *renderer) isAttribution(block string) bool {
	inner, ok := r.unwrapParagraph(block)
	if !ok || strings.Contains(inner, "<br>") {
		return false
	}
	text := strings.TrimSpace(blockText(inner))
	return strings.HasSuffix(text, ":") && utf8.RuneCountInString(text) <= maxAttributionLen
}

func (r *renderer) isCode(n *xhtml.Node) bool {
	return n.Type == xhtml.ElementNode && codeClassRegex.MatchString(r.attr(n, "class"))
}

// blockText returns the visible text of a rendered block; it is shared by the renderer and the message body
func blockText(block string) string {
	return html.UnescapeString(tagRegex.ReplaceAllString(block, ""))
}
