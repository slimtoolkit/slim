package table

import (
	"fmt"
	"html"
	"strings"
)

const (
	// DefaultHTMLCSSClass stores the css-class to use when none-provided via
	// SetHTMLCSSClass(cssClass string).
	DefaultHTMLCSSClass = "go-pretty-table"
)

// RenderHTML renders the Table in HTML format. Example:
//
//	<table class="go-pretty-table">
//	  <thead>
//	  <tr>
//	    <th align="right">#</th>
//	    <th>First Name</th>
//	    <th>Last Name</th>
//	    <th align="right">Salary</th>
//	    <th>&nbsp;</th>
//	  </tr>
//	  </thead>
//	  <tbody>
//	  <tr>
//	    <td align="right">1</td>
//	    <td>Arya</td>
//	    <td>Stark</td>
//	    <td align="right">3000</td>
//	    <td>&nbsp;</td>
//	  </tr>
//	  <tr>
//	    <td align="right">20</td>
//	    <td>Jon</td>
//	    <td>Snow</td>
//	    <td align="right">2000</td>
//	    <td>You know nothing, Jon Snow!</td>
//	  </tr>
//	  <tr>
//	    <td align="right">300</td>
//	    <td>Tyrion</td>
//	    <td>Lannister</td>
//	    <td align="right">5000</td>
//	    <td>&nbsp;</td>
//	  </tr>
//	  </tbody>
//	  <tfoot>
//	  <tr>
//	    <td align="right">&nbsp;</td>
//	    <td>&nbsp;</td>
//	    <td>Total</td>
//	    <td align="right">10000</td>
//	    <td>&nbsp;</td>
//	  </tr>
//	  </tfoot>
//	</table>
func (t *Table) RenderHTML() string {
	t.initForRender()

	var out strings.Builder
	if t.numColumns > 0 {
		out.WriteString("<table class=\"")
		if t.htmlCSSClass != "" {
			out.WriteString(t.htmlCSSClass)
		} else {
			out.WriteString(t.style.HTML.CSSClass)
		}
		out.WriteString("\">\n")
		t.htmlRenderTitle(&out)
		t.htmlRenderRowsHeader(&out)
		t.htmlRenderRows(&out, t.rows, renderHint{})
		t.htmlRenderRowsFooter(&out)
		t.htmlRenderCaption(&out)
		out.WriteString("</table>")
	}
	return t.render(&out)
}

func (t *Table) htmlGetColStrAndTag(row rowStr, colIdx int, hint renderHint) (string, string) {
	// get the column contents
	var colStr string
	if colIdx < len(row) {
		colStr = row[colIdx]
	}

	// header uses "th" instead of "td"
	colTagName := "td"
	if hint.isHeaderRow {
		colTagName = "th"
	}

	return colStr, colTagName
}

func (t *Table) htmlRenderCaption(out *strings.Builder) {
	if t.caption != "" {
		out.WriteString("  <caption class=\"caption\" style=\"caption-side: bottom;\">")
		out.WriteString(t.caption)
		out.WriteString("</caption>\n")
	}
}

func (t *Table) htmlRenderColumn(out *strings.Builder, colStr string) {
	if t.style.HTML.EscapeText {
		colStr = html.EscapeString(colStr)
	}
	if t.style.HTML.Newline != "\n" {
		colStr = strings.Replace(colStr, "\n", t.style.HTML.Newline, -1)
	}
	out.WriteString(colStr)
}

func (t *Table) htmlRenderColumnAttributes(out *strings.Builder, colIdx int, hint renderHint) {
	// determine the HTML "align"/"valign" property values
	align := t.getAlign(colIdx, hint).HTMLProperty()
	vAlign := t.getVAlign(colIdx, hint).HTMLProperty()
	// determine the HTML "class" property values for the colors
	class := t.getColumnColors(colIdx, hint).HTMLProperty()

	if align != "" {
		out.WriteRune(' ')
		out.WriteString(align)
	}
	if class != "" {
		out.WriteRune(' ')
		out.WriteString(class)
	}
	if vAlign != "" {
		out.WriteRune(' ')
		out.WriteString(vAlign)
	}
}

func (t *Table) htmlRenderColumnAutoIndex(out *strings.Builder, hint renderHint) {
	if hint.isHeaderRow {
		out.WriteString("    <th>")
		out.WriteString(t.style.HTML.EmptyColumn)
		out.WriteString("</th>\n")
	} else if hint.isFooterRow {
		out.WriteString("    <td>")
		out.WriteString(t.style.HTML.EmptyColumn)
		out.WriteString("</td>\n")
	} else {
		out.WriteString("    <td align=\"right\">")
		out.WriteString(fmt.Sprint(hint.rowNumber))
		out.WriteString("</td>\n")
	}
}

func (t *Table) htmlRenderRow(out *strings.Builder, row rowStr, hint renderHint) {
	out.WriteString("  <tr>\n")
	for colIdx := 0; colIdx < t.numColumns; colIdx++ {
		// auto-index column
		if colIdx == 0 && t.autoIndex {
			t.htmlRenderColumnAutoIndex(out, hint)
		}

		colStr, colTagName := t.htmlGetColStrAndTag(row, colIdx, hint)
		// write the row
		out.WriteString("    <")
		out.WriteString(colTagName)
		t.htmlRenderColumnAttributes(out, colIdx, hint)
		out.WriteString(">")
		if len(colStr) == 0 {
			out.WriteString(t.style.HTML.EmptyColumn)
		} else {
			t.htmlRenderColumn(out, colStr)
		}
		out.WriteString("</")
		out.WriteString(colTagName)
		out.WriteString(">\n")
	}
	out.WriteString("  </tr>\n")
}

func (t *Table) htmlRenderRows(out *strings.Builder, rows []rowStr, hint renderHint) {
	if len(rows) > 0 {
		// determine that tag to use based on the type of the row
		rowsTag := "tbody"
		if hint.isHeaderRow {
			rowsTag = "thead"
		} else if hint.isFooterRow {
			rowsTag = "tfoot"
		}

		var renderedTagOpen, shouldRenderTagClose bool
		for idx, row := range rows {
			hint.rowNumber = idx + 1
			if len(row) > 0 {
				if !renderedTagOpen {
					out.WriteString("  <")
					out.WriteString(rowsTag)
					out.WriteString(">\n")
					renderedTagOpen = true
				}
				t.htmlRenderRow(out, row, hint)
				shouldRenderTagClose = true
			}
		}
		if shouldRenderTagClose {
			out.WriteString("  </")
			out.WriteString(rowsTag)
			out.WriteString(">\n")
		}
	}
}

func (t *Table) htmlRenderRowsFooter(out *strings.Builder) {
	if len(t.rowsFooter) > 0 {
		t.htmlRenderRows(out, t.rowsFooter, renderHint{isFooterRow: true})
	}
}

func (t *Table) htmlRenderRowsHeader(out *strings.Builder) {
	if len(t.rowsHeader) > 0 {
		t.htmlRenderRows(out, t.rowsHeader, renderHint{isHeaderRow: true})
	} else if t.autoIndex {
		hint := renderHint{isAutoIndexRow: true, isHeaderRow: true}
		t.htmlRenderRows(out, []rowStr{t.getAutoIndexColumnIDs()}, hint)
	}
}

func (t *Table) htmlRenderTitle(out *strings.Builder) {
	if t.title != "" {
		align := t.style.Title.Align.HTMLProperty()
		colors := t.style.Title.Colors.HTMLProperty()
		title := t.style.Title.Format.Apply(t.title)

		out.WriteString("  <caption class=\"title\"")
		if align != "" {
			out.WriteRune(' ')
			out.WriteString(align)
		}
		if colors != "" {
			out.WriteRune(' ')
			out.WriteString(colors)
		}
		out.WriteRune('>')
		out.WriteString(title)
		out.WriteString("</caption>\n")
	}
}
