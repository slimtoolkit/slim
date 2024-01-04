package table

import (
	"fmt"
	"strings"
)

func (t *Table) RenderTSV() string {
	t.initForRender()

	var out strings.Builder

	if t.numColumns > 0 {
		if t.title != "" {
			out.WriteString(t.title)
		}

		if t.autoIndex && len(t.rowsHeader) == 0 {
			t.tsvRenderRow(&out, t.getAutoIndexColumnIDs(), renderHint{isAutoIndexRow: true, isHeaderRow: true})
		}

		t.tsvRenderRows(&out, t.rowsHeader, renderHint{isHeaderRow: true})
		t.tsvRenderRows(&out, t.rows, renderHint{})
		t.tsvRenderRows(&out, t.rowsFooter, renderHint{isFooterRow: true})

		if t.caption != "" {
			out.WriteRune('\n')
			out.WriteString(t.caption)
		}
	}

	return t.render(&out)
}

func (t *Table) tsvRenderRow(out *strings.Builder, row rowStr, hint renderHint) {
	if out.Len() > 0 {
		out.WriteRune('\n')
	}

	for idx, col := range row {
		if idx == 0 && t.autoIndex {
			if hint.isRegularRow() {
				out.WriteString(fmt.Sprint(hint.rowNumber))
			}
			out.WriteRune('\t')
		}

		if idx > 0 {
			out.WriteRune('\t')
		}

		if strings.ContainsAny(col, "\t\n\"") || strings.Contains(col, "    ") {
			out.WriteString(fmt.Sprintf("\"%s\"", t.tsvFixDoubleQuotes(col)))
		} else {
			out.WriteString(col)
		}
	}

	for colIdx := len(row); colIdx < t.numColumns; colIdx++ {
		out.WriteRune('\t')
	}
}

func (t *Table) tsvFixDoubleQuotes(str string) string {
	return strings.Replace(str, "\"", "\"\"", -1)
}

func (t *Table) tsvRenderRows(out *strings.Builder, rows []rowStr, hint renderHint) {
	for idx, row := range rows {
		hint.rowNumber = idx + 1
		t.tsvRenderRow(out, row, hint)
	}
}
