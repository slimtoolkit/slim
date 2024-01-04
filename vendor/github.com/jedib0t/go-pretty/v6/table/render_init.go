package table

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/text"
)

func (t *Table) analyzeAndStringify(row Row, hint renderHint) rowStr {
	// update t.numColumns if this row is the longest seen till now
	if len(row) > t.numColumns {
		// init the slice for the first time; and pad it the rest of the time
		if t.numColumns == 0 {
			t.columnIsNonNumeric = make([]bool, len(row))
		} else {
			t.columnIsNonNumeric = append(t.columnIsNonNumeric, make([]bool, len(row)-t.numColumns)...)
		}
		// update t.numColumns
		t.numColumns = len(row)
	}

	// convert each column to string and figure out if it has non-numeric data
	rowOut := make(rowStr, len(row))
	for colIdx, col := range row {
		// if the column is not a number, keep track of it
		if !hint.isHeaderRow && !hint.isFooterRow && !t.columnIsNonNumeric[colIdx] && !isNumber(col) {
			t.columnIsNonNumeric[colIdx] = true
		}

		rowOut[colIdx] = t.analyzeAndStringifyColumn(colIdx, col, hint)
	}
	return rowOut
}

func (t *Table) analyzeAndStringifyColumn(colIdx int, col interface{}, hint renderHint) string {
	// convert to a string and store it in the row
	var colStr string
	if transformer := t.getColumnTransformer(colIdx, hint); transformer != nil {
		colStr = transformer(col)
	} else if colStrVal, ok := col.(string); ok {
		colStr = colStrVal
	} else {
		colStr = fmt.Sprint(col)
	}
	colStr = strings.ReplaceAll(colStr, "\t", "    ")
	colStr = strings.ReplaceAll(colStr, "\r", "")
	return fmt.Sprintf("%s%s", t.style.Format.Direction.Modifier(), colStr)
}

func (t *Table) extractMaxColumnLengths(rows []rowStr, hint renderHint) {
	for rowIdx, row := range rows {
		hint.rowNumber = rowIdx + 1
		t.extractMaxColumnLengthsFromRow(row, t.getMergedColumnIndices(row, hint))
	}
}

func (t *Table) extractMaxColumnLengthsFromRow(row rowStr, mci mergedColumnIndices) {
	for colIdx, colStr := range row {
		longestLineLen := text.LongestLineLen(colStr)
		maxColWidth := t.getColumnWidthMax(colIdx)
		if maxColWidth > 0 && maxColWidth < longestLineLen {
			longestLineLen = maxColWidth
		}
		mergedColumnsLength := mci.mergedLength(colIdx, t.maxColumnLengths)
		if longestLineLen > mergedColumnsLength {
			if mergedColumnsLength > 0 {
				t.extractMaxColumnLengthsFromRowForMergedColumns(colIdx, longestLineLen, mci)
			} else {
				t.maxColumnLengths[colIdx] = longestLineLen
			}
		} else if maxColWidth == 0 && longestLineLen > t.maxColumnLengths[colIdx] {
			t.maxColumnLengths[colIdx] = longestLineLen
		}
	}
}

func (t *Table) extractMaxColumnLengthsFromRowForMergedColumns(colIdx int, mergedColumnLength int, mci mergedColumnIndices) {
	numMergedColumns := mci.len(colIdx)
	mergedColumnLength -= (numMergedColumns - 1) * text.RuneWidthWithoutEscSequences(t.style.Box.MiddleSeparator)
	maxLengthSplitAcrossColumns := mergedColumnLength / numMergedColumns
	if maxLengthSplitAcrossColumns > t.maxColumnLengths[colIdx] {
		t.maxColumnLengths[colIdx] = maxLengthSplitAcrossColumns
	}
	for otherColIdx := range mci[colIdx] {
		if maxLengthSplitAcrossColumns > t.maxColumnLengths[otherColIdx] {
			t.maxColumnLengths[otherColIdx] = maxLengthSplitAcrossColumns
		}
	}
}

func (t *Table) initForRender() {
	// pick a default style if none was set until now
	t.Style()

	// reset rendering state
	t.reset()

	// initialize the column configs and normalize them
	t.initForRenderColumnConfigs()

	// initialize and stringify all the raw rows
	t.initForRenderRows()

	// find the longest continuous line in each column
	t.initForRenderColumnLengths()

	// generate a separator row and calculate maximum row length
	t.initForRenderRowSeparator()

	// reset the counter for the number of lines rendered
	t.numLinesRendered = 0
}

func (t *Table) initForRenderColumnConfigs() {
	t.columnConfigMap = map[int]ColumnConfig{}
	for _, colCfg := range t.columnConfigs {
		// find the column number if none provided; this logic can work only if
		// a header row is present and has a column with the given name
		if colCfg.Number == 0 {
			for _, row := range t.rowsHeaderRaw {
				colCfg.Number = row.findColumnNumber(colCfg.Name)
				if colCfg.Number > 0 {
					break
				}
			}
		}
		if colCfg.Number > 0 {
			t.columnConfigMap[colCfg.Number-1] = colCfg
		}
	}
}

func (t *Table) initForRenderColumnLengths() {
	t.maxColumnLengths = make([]int, t.numColumns)
	t.extractMaxColumnLengths(t.rowsHeader, renderHint{isHeaderRow: true})
	t.extractMaxColumnLengths(t.rows, renderHint{})
	t.extractMaxColumnLengths(t.rowsFooter, renderHint{isFooterRow: true})

	// increase the column lengths if any are under the limits
	for colIdx := range t.maxColumnLengths {
		minWidth := t.getColumnWidthMin(colIdx)
		if minWidth > 0 && t.maxColumnLengths[colIdx] < minWidth {
			t.maxColumnLengths[colIdx] = minWidth
		}
	}
}

func (t *Table) initForRenderHideColumns() {
	if !t.hasHiddenColumns() {
		return
	}
	colIdxMap := t.hideColumns()

	// re-create columnIsNonNumeric with new column indices
	columnIsNonNumeric := make([]bool, t.numColumns)
	for oldColIdx, nonNumeric := range t.columnIsNonNumeric {
		if newColIdx, ok := colIdxMap[oldColIdx]; ok {
			columnIsNonNumeric[newColIdx] = nonNumeric
		}
	}
	t.columnIsNonNumeric = columnIsNonNumeric

	// re-create columnConfigMap with new column indices
	columnConfigMap := make(map[int]ColumnConfig)
	for oldColIdx, cc := range t.columnConfigMap {
		if newColIdx, ok := colIdxMap[oldColIdx]; ok {
			columnConfigMap[newColIdx] = cc
		}
	}
	t.columnConfigMap = columnConfigMap
}

func (t *Table) initForRenderRows() {
	// auto-index: calc the index column's max length
	t.autoIndexVIndexMaxLength = len(fmt.Sprint(len(t.rowsRaw)))

	// stringify all the rows to make it easy to render
	if t.rowPainter != nil {
		t.rowsColors = make([]text.Colors, len(t.rowsRaw))
	}
	t.rows = t.initForRenderRowsStringify(t.rowsRaw, renderHint{})
	t.rowsFooter = t.initForRenderRowsStringify(t.rowsFooterRaw, renderHint{isFooterRow: true})
	t.rowsHeader = t.initForRenderRowsStringify(t.rowsHeaderRaw, renderHint{isHeaderRow: true})

	// sort the rows as requested
	t.initForRenderSortRows()

	// suppress columns without any content
	t.initForRenderSuppressColumns()

	// strip out hidden columns
	t.initForRenderHideColumns()
}

func (t *Table) initForRenderRowsStringify(rows []Row, hint renderHint) []rowStr {
	rowsStr := make([]rowStr, len(rows))
	for idx, row := range rows {
		if t.rowPainter != nil && hint.isRegularRow() {
			t.rowsColors[idx] = t.rowPainter(row)
		}
		rowsStr[idx] = t.analyzeAndStringify(row, hint)
	}
	return rowsStr
}

func (t *Table) initForRenderRowSeparator() {
	t.maxRowLength = 0
	if t.autoIndex {
		t.maxRowLength += text.RuneWidthWithoutEscSequences(t.style.Box.PaddingLeft)
		t.maxRowLength += len(fmt.Sprint(len(t.rows)))
		t.maxRowLength += text.RuneWidthWithoutEscSequences(t.style.Box.PaddingRight)
		if t.style.Options.SeparateColumns {
			t.maxRowLength += text.RuneWidthWithoutEscSequences(t.style.Box.MiddleSeparator)
		}
	}
	if t.style.Options.SeparateColumns {
		t.maxRowLength += text.RuneWidthWithoutEscSequences(t.style.Box.MiddleSeparator) * (t.numColumns - 1)
	}
	t.rowSeparator = make(rowStr, t.numColumns)
	for colIdx, maxColumnLength := range t.maxColumnLengths {
		maxColumnLength += text.RuneWidthWithoutEscSequences(t.style.Box.PaddingLeft + t.style.Box.PaddingRight)
		t.maxRowLength += maxColumnLength
		t.rowSeparator[colIdx] = text.RepeatAndTrim(t.style.Box.MiddleHorizontal, maxColumnLength)
	}
	if t.style.Options.DrawBorder {
		t.maxRowLength += text.RuneWidthWithoutEscSequences(t.style.Box.Left + t.style.Box.Right)
	}
}

func (t *Table) initForRenderSortRows() {
	if len(t.sortBy) == 0 {
		return
	}

	// sort the rows
	sortedRowIndices := t.getSortedRowIndices()
	sortedRows := make([]rowStr, len(t.rows))
	for idx := range t.rows {
		sortedRows[idx] = t.rows[sortedRowIndices[idx]]
	}
	t.rows = sortedRows

	// sort the rowsColors
	if len(t.rowsColors) > 0 {
		sortedRowsColors := make([]text.Colors, len(t.rows))
		for idx := range t.rows {
			sortedRowsColors[idx] = t.rowsColors[sortedRowIndices[idx]]
		}
		t.rowsColors = sortedRowsColors
	}
}

func (t *Table) initForRenderSuppressColumns() {
	shouldSuppressColumn := func(colIdx int) bool {
		for _, row := range t.rows {
			if colIdx < len(row) && row[colIdx] != "" {
				return false
			}
		}
		return true
	}

	if t.suppressEmptyColumns {
		for colIdx := 0; colIdx < t.numColumns; colIdx++ {
			if shouldSuppressColumn(colIdx) {
				cc := t.columnConfigMap[colIdx]
				cc.Hidden = true
				t.columnConfigMap[colIdx] = cc
			}
		}
	}
}

// reset initializes all the variables used to maintain rendering information
// that are written to in this file
func (t *Table) reset() {
	t.autoIndexVIndexMaxLength = 0
	t.columnConfigMap = nil
	t.columnIsNonNumeric = nil
	t.maxColumnLengths = nil
	t.maxRowLength = 0
	t.numColumns = 0
	t.numLinesRendered = 0
	t.rowSeparator = nil
	t.rows = nil
	t.rowsColors = nil
	t.rowsFooter = nil
	t.rowsHeader = nil
}
