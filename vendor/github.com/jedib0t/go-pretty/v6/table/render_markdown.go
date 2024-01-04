package table

import (
	"fmt"
	"strings"
)

// RenderMarkdown renders the Table in Markdown format. Example:
//
//	| # | First Name | Last Name | Salary |  |
//	| ---:| --- | --- | ---:| --- |
//	| 1 | Arya | Stark | 3000 |  |
//	| 20 | Jon | Snow | 2000 | You know nothing, Jon Snow! |
//	| 300 | Tyrion | Lannister | 5000 |  |
//	|  |  | Total | 10000 |  |
func (t *Table) RenderMarkdown() string {
	t.initForRender()

	var out strings.Builder
	if t.numColumns > 0 {
		t.markdownRenderTitle(&out)
		t.markdownRenderRowsHeader(&out)
		t.markdownRenderRows(&out, t.rows, renderHint{})
		t.markdownRenderRowsFooter(&out)
		t.markdownRenderCaption(&out)
	}
	return t.render(&out)
}

func (t *Table) markdownRenderCaption(out *strings.Builder) {
	if t.caption != "" {
		out.WriteRune('\n')
		out.WriteRune('_')
		out.WriteString(t.caption)
		out.WriteRune('_')
	}
}

func (t *Table) markdownRenderRow(out *strings.Builder, row rowStr, hint renderHint) {
	// when working on line number 2 or more, insert a newline first
	if out.Len() > 0 {
		out.WriteRune('\n')
	}

	// render each column up to the max. columns seen in all the rows
	out.WriteRune('|')
	for colIdx := 0; colIdx < t.numColumns; colIdx++ {
		t.markdownRenderRowAutoIndex(out, colIdx, hint)

		if hint.isSeparatorRow {
			out.WriteString(t.getAlign(colIdx, hint).MarkdownProperty())
		} else {
			var colStr string
			if colIdx < len(row) {
				colStr = row[colIdx]
			}
			out.WriteRune(' ')
			colStr = strings.ReplaceAll(colStr, "|", "\\|")
			colStr = strings.ReplaceAll(colStr, "\n", "<br/>")
			out.WriteString(colStr)
			out.WriteRune(' ')
		}
		out.WriteRune('|')
	}
}

func (t *Table) markdownRenderRowAutoIndex(out *strings.Builder, colIdx int, hint renderHint) {
	if colIdx == 0 && t.autoIndex {
		out.WriteRune(' ')
		if hint.isSeparatorRow {
			out.WriteString("---:")
		} else if hint.isRegularRow() {
			out.WriteString(fmt.Sprintf("%d ", hint.rowNumber))
		}
		out.WriteRune('|')
	}
}

func (t *Table) markdownRenderRows(out *strings.Builder, rows []rowStr, hint renderHint) {
	if len(rows) > 0 {
		for idx, row := range rows {
			hint.rowNumber = idx + 1
			t.markdownRenderRow(out, row, hint)

			if idx == len(rows)-1 && hint.isHeaderRow {
				t.markdownRenderRow(out, t.rowSeparator, renderHint{isSeparatorRow: true})
			}
		}
	}
}

func (t *Table) markdownRenderRowsFooter(out *strings.Builder) {
	t.markdownRenderRows(out, t.rowsFooter, renderHint{isFooterRow: true})
}

func (t *Table) markdownRenderRowsHeader(out *strings.Builder) {
	if len(t.rowsHeader) > 0 {
		t.markdownRenderRows(out, t.rowsHeader, renderHint{isHeaderRow: true})
	} else if t.autoIndex {
		t.markdownRenderRows(out, []rowStr{t.getAutoIndexColumnIDs()}, renderHint{isAutoIndexRow: true, isHeaderRow: true})
	}
}

func (t *Table) markdownRenderTitle(out *strings.Builder) {
	if t.title != "" {
		out.WriteString("# ")
		out.WriteString(t.title)
	}
}
