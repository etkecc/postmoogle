package email

import (
	"slices"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func (r *renderer) table(n *xhtml.Node) {
	for _, caption := range r.childElements(n, atom.Caption) {
		r.block(caption, breakLine)
	}
	rows := r.tableRows(n)
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
	cells := r.visible(r.childElements(row, atom.Td, atom.Th))
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
	role := strings.ToLower(r.attr(table, "role"))
	if role == "presentation" || role == "none" {
		return false
	}
	var headers, columns int
	for _, row := range rows {
		cells := r.childElements(row, atom.Td, atom.Th)
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
		cells := r.childElements(row, atom.Td, atom.Th)
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
			table.WriteString("<" + tag + ">" + r.joinParagraphs(sub.finish()) + "</" + tag + ">")
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

// tableRows returns visible rows of the table, without rows of nested tables
func (r *renderer) tableRows(table *xhtml.Node) []*xhtml.Node {
	rows := r.childElements(table, atom.Tr)
	for _, section := range r.childElements(table, atom.Thead, atom.Tbody, atom.Tfoot) {
		rows = append(rows, r.childElements(section, atom.Tr)...)
	}
	return rows
}

// childElements returns visible child elements of the given types
func (r *renderer) childElements(n *xhtml.Node, elements ...atom.Atom) []*xhtml.Node {
	var children []*xhtml.Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && slices.Contains(elements, child.DataAtom) && !r.isHidden(child) {
			children = append(children, child)
		}
	}
	return children
}

// tight unwraps a single paragraph, so list items and cells do not get paragraph spacing
func (r *renderer) tight(blocks []string) string {
	if len(blocks) == 1 {
		if inner, ok := r.unwrapParagraph(blocks[0]); ok {
			return inner
		}
	}
	return strings.Join(blocks, "")
}

func (r *renderer) joinParagraphs(blocks []string) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		inner, _ := r.unwrapParagraph(block)
		parts = append(parts, inner)
	}
	return strings.Join(parts, "<br>")
}

func (r *renderer) unwrapParagraph(block string) (string, bool) {
	if strings.HasPrefix(block, "<p>") && strings.HasSuffix(block, "</p>") {
		return block[len("<p>") : len(block)-len("</p>")], true
	}
	return block, false
}
