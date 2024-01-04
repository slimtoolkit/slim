package table

import (
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/text"
)

// Row defines a single row in the Table.
type Row []interface{}

func (r Row) findColumnNumber(colName string) int {
	for colIdx, col := range r {
		if fmt.Sprint(col) == colName {
			return colIdx + 1
		}
	}
	return 0
}

// RowPainter is a custom function that takes a Row as input and returns the
// text.Colors{} to use on the entire row
type RowPainter func(row Row) text.Colors

// rowStr defines a single row in the Table comprised of just string objects.
type rowStr []string

// areEqual returns true if the contents of the 2 given columns are the same
func (row rowStr) areEqual(colIdx1 int, colIdx2 int) bool {
	return colIdx1 >= 0 && colIdx2 < len(row) && row[colIdx1] == row[colIdx2]
}

// Table helps print a 2-dimensional array in a human-readable pretty-table.
type Table struct {
	// allowedRowLength is the max allowed length for a row (or line of output)
	allowedRowLength int
	// enable automatic indexing of the rows and columns like a spreadsheet?
	autoIndex bool
	// autoIndexVIndexMaxLength denotes the length in chars for the last row
	autoIndexVIndexMaxLength int
	// caption stores the text to be rendered just below the table; and doesn't
	// get used when rendered as a CSV
	caption string
	// columnIsNonNumeric stores if a column contains non-numbers in all rows
	columnIsNonNumeric []bool
	// columnConfigs stores the custom-configuration for 1 or more columns
	columnConfigs []ColumnConfig
	// columnConfigMap stores the custom-configuration by column
	// number and is generated before rendering
	columnConfigMap map[int]ColumnConfig
	// htmlCSSClass stores the HTML CSS Class to use on the <table> node
	htmlCSSClass string
	// indexColumn stores the number of the column considered as the "index"
	indexColumn int
	// maxColumnLengths stores the length of the longest line in each column
	maxColumnLengths []int
	// maxRowLength stores the length of the longest row
	maxRowLength int
	// numColumns stores the (max.) number of columns seen
	numColumns int
	// numLinesRendered keeps track of the number of lines rendered and helps in
	// paginating long tables
	numLinesRendered int
	// outputMirror stores an io.Writer where the "Render" functions would write
	outputMirror io.Writer
	// pageSize stores the maximum lines to render before rendering the header
	// again (to denote a page break) - useful when you are dealing with really
	// long tables
	pageSize int
	// rows stores the rows that make up the body (in string form)
	rows []rowStr
	// rowsColors stores the text.Colors over-rides for each row as defined by
	// rowPainter
	rowsColors []text.Colors
	// rowsConfigs stores RowConfig for each row
	rowsConfigMap map[int]RowConfig
	// rowsRaw stores the rows that make up the body
	rowsRaw []Row
	// rowsFooter stores the rows that make up the footer (in string form)
	rowsFooter []rowStr
	// rowsFooterConfigs stores RowConfig for each footer row
	rowsFooterConfigMap map[int]RowConfig
	// rowsFooterRaw stores the rows that make up the footer
	rowsFooterRaw []Row
	// rowsHeader stores the rows that make up the header (in string form)
	rowsHeader []rowStr
	// rowsHeaderConfigs stores RowConfig for each header row
	rowsHeaderConfigMap map[int]RowConfig
	// rowsHeaderRaw stores the rows that make up the header
	rowsHeaderRaw []Row
	// rowPainter is a custom function that given a Row, returns the colors to
	// use on the entire row
	rowPainter RowPainter
	// rowSeparator is a dummy row that contains the separator columns (dashes
	// that make up the separator between header/body/footer
	rowSeparator rowStr
	// separators is used to keep track of all rowIndices after which a
	// separator has to be rendered
	separators map[int]bool
	// sortBy stores a map of Column
	sortBy []SortBy
	// style contains all the strings used to draw the table, and more
	style *Style
	// suppressEmptyColumns hides columns which have no content on all regular
	// rows
	suppressEmptyColumns bool
	// title contains the text to appear above the table
	title string
}

// AppendFooter appends the row to the List of footers to render.
//
// Only the first item in the "config" will be tagged against this row.
func (t *Table) AppendFooter(row Row, config ...RowConfig) {
	t.rowsFooterRaw = append(t.rowsFooterRaw, row)
	if len(config) > 0 {
		if t.rowsFooterConfigMap == nil {
			t.rowsFooterConfigMap = make(map[int]RowConfig)
		}
		t.rowsFooterConfigMap[len(t.rowsFooterRaw)-1] = config[0]
	}
}

// AppendHeader appends the row to the List of headers to render.
//
// Only the first item in the "config" will be tagged against this row.
func (t *Table) AppendHeader(row Row, config ...RowConfig) {
	t.rowsHeaderRaw = append(t.rowsHeaderRaw, row)
	if len(config) > 0 {
		if t.rowsHeaderConfigMap == nil {
			t.rowsHeaderConfigMap = make(map[int]RowConfig)
		}
		t.rowsHeaderConfigMap[len(t.rowsHeaderRaw)-1] = config[0]
	}
}

// AppendRow appends the row to the List of rows to render.
//
// Only the first item in the "config" will be tagged against this row.
func (t *Table) AppendRow(row Row, config ...RowConfig) {
	t.rowsRaw = append(t.rowsRaw, row)
	if len(config) > 0 {
		if t.rowsConfigMap == nil {
			t.rowsConfigMap = make(map[int]RowConfig)
		}
		t.rowsConfigMap[len(t.rowsRaw)-1] = config[0]
	}
}

// AppendRows appends the rows to the List of rows to render.
//
// Only the first item in the "config" will be tagged against all the rows.
func (t *Table) AppendRows(rows []Row, config ...RowConfig) {
	for _, row := range rows {
		t.AppendRow(row, config...)
	}
}

// AppendSeparator helps render a separator row after the current last row. You
// could call this function over and over, but it will be a no-op unless you
// call AppendRow or AppendRows in between. Likewise, if the last thing you
// append is a separator, it will not be rendered in addition to the usual table
// separator.
//
// ******************************************************************************
// Please note the following caveats:
//  1. SetPageSize(): this may end up creating consecutive separator rows near
//     the end of a page or at the beginning of a page
//  2. SortBy(): since SortBy could inherently alter the ordering of rows, the
//     separators may not appear after the row it was originally intended to
//     follow
//
// ******************************************************************************
func (t *Table) AppendSeparator() {
	if t.separators == nil {
		t.separators = make(map[int]bool)
	}
	if len(t.rowsRaw) > 0 {
		t.separators[len(t.rowsRaw)-1] = true
	}
}

// Length returns the number of rows to be rendered.
func (t *Table) Length() int {
	return len(t.rowsRaw)
}

// ResetFooters resets and clears all the Footer rows appended earlier.
func (t *Table) ResetFooters() {
	t.rowsFooterRaw = nil
}

// ResetHeaders resets and clears all the Header rows appended earlier.
func (t *Table) ResetHeaders() {
	t.rowsHeaderRaw = nil
}

// ResetRows resets and clears all the rows appended earlier.
func (t *Table) ResetRows() {
	t.rowsRaw = nil
	t.separators = nil
}

// SetAllowedRowLength sets the maximum allowed length or a row (or line of
// output) when rendered as a table. Rows that are longer than this limit will
// be "snipped" to the length. Length has to be a positive value to take effect.
func (t *Table) SetAllowedRowLength(length int) {
	t.allowedRowLength = length
}

// SetAutoIndex adds a generated header with columns such as "A", "B", "C", etc.
// and a leading column with the row number similar to what you'd see on any
// spreadsheet application. NOTE: Appending a Header will void this
// functionality.
func (t *Table) SetAutoIndex(autoIndex bool) {
	t.autoIndex = autoIndex
}

// SetCaption sets the text to be rendered just below the table. This will not
// show up when the Table is rendered as a CSV.
func (t *Table) SetCaption(format string, a ...interface{}) {
	t.caption = fmt.Sprintf(format, a...)
}

// SetColumnConfigs sets the configs for each Column.
func (t *Table) SetColumnConfigs(configs []ColumnConfig) {
	t.columnConfigs = configs
}

// SetHTMLCSSClass sets the HTML CSS Class to use on the <table> node
// when rendering the Table in HTML format.
//
// Deprecated: in favor of Style().HTML.CSSClass
func (t *Table) SetHTMLCSSClass(cssClass string) {
	t.htmlCSSClass = cssClass
}

// SetIndexColumn sets the given Column # as the column that has the row
// "Number". Valid values range from 1 to N. Note that this is not 0-indexed.
func (t *Table) SetIndexColumn(colNum int) {
	t.indexColumn = colNum
}

// SetOutputMirror sets an io.Writer for all the Render functions to "Write" to
// in addition to returning a string.
func (t *Table) SetOutputMirror(mirror io.Writer) {
	t.outputMirror = mirror
}

// SetPageSize sets the maximum number of lines to render before rendering the
// header rows again. This can be useful when dealing with tables containing a
// long list of rows that can span pages. Please note that the pagination logic
// will not consider Header/Footer lines for paging.
func (t *Table) SetPageSize(numLines int) {
	t.pageSize = numLines
}

// SetRowPainter sets the RowPainter function which determines the colors to use
// on a row. Before rendering, this function is invoked on all rows and the
// color of each row is determined. This color takes precedence over other ways
// to set color (ColumnConfig.Color*, SetColor*()).
func (t *Table) SetRowPainter(painter RowPainter) {
	t.rowPainter = painter
}

// SetStyle overrides the DefaultStyle with the provided one.
func (t *Table) SetStyle(style Style) {
	t.style = &style
}

// SetTitle sets the title text to be rendered above the table.
func (t *Table) SetTitle(format string, a ...interface{}) {
	t.title = fmt.Sprintf(format, a...)
}

// SortBy sets the rules for sorting the Rows in the order specified. i.e., the
// first SortBy instruction takes precedence over the second and so on. Any
// duplicate instructions on the same column will be discarded while sorting.
func (t *Table) SortBy(sortBy []SortBy) {
	t.sortBy = sortBy
}

// Style returns the current style.
func (t *Table) Style() *Style {
	if t.style == nil {
		tempStyle := StyleDefault
		t.style = &tempStyle
	}
	return t.style
}

// SuppressEmptyColumns hides columns when the column is empty in ALL the
// regular rows.
func (t *Table) SuppressEmptyColumns() {
	t.suppressEmptyColumns = true
}

func (t *Table) getAlign(colIdx int, hint renderHint) text.Align {
	align := text.AlignDefault
	if cfg, ok := t.columnConfigMap[colIdx]; ok {
		if hint.isHeaderRow {
			align = cfg.AlignHeader
		} else if hint.isFooterRow {
			align = cfg.AlignFooter
		} else {
			align = cfg.Align
		}
	}
	if align == text.AlignDefault {
		if !t.columnIsNonNumeric[colIdx] {
			align = text.AlignRight
		} else if hint.isAutoIndexRow {
			align = text.AlignCenter
		}
	}
	return align
}

func (t *Table) getAutoIndexColumnIDs() rowStr {
	row := make(rowStr, t.numColumns)
	for colIdx := range row {
		row[colIdx] = AutoIndexColumnID(colIdx)
	}
	return row
}

func (t *Table) getBorderColors(hint renderHint) text.Colors {
	if t.style.Options.DoNotColorBordersAndSeparators {
		return nil
	} else if t.style.Color.Border != nil {
		return t.style.Color.Border
	} else if hint.isTitleRow {
		return t.style.Title.Colors
	} else if hint.isHeaderRow {
		return t.style.Color.Header
	} else if hint.isFooterRow {
		return t.style.Color.Footer
	} else if t.autoIndex {
		return t.style.Color.IndexColumn
	} else if hint.rowNumber%2 == 0 && t.style.Color.RowAlternate != nil {
		return t.style.Color.RowAlternate
	}
	return t.style.Color.Row
}

func (t *Table) getBorderLeft(hint renderHint) string {
	border := t.style.Box.Left
	if hint.isBorderTop {
		if t.title != "" {
			border = t.style.Box.LeftSeparator
		} else {
			border = t.style.Box.TopLeft
		}
	} else if hint.isBorderBottom {
		border = t.style.Box.BottomLeft
	} else if hint.isSeparatorRow {
		if t.autoIndex && hint.isHeaderOrFooterSeparator() {
			border = t.style.Box.Left
		} else if !t.autoIndex && t.shouldMergeCellsVertically(0, hint) {
			border = t.style.Box.Left
		} else {
			border = t.style.Box.LeftSeparator
		}
	}
	return border
}

func (t *Table) getBorderRight(hint renderHint) string {
	border := t.style.Box.Right
	if hint.isBorderTop {
		if t.title != "" {
			border = t.style.Box.RightSeparator
		} else {
			border = t.style.Box.TopRight
		}
	} else if hint.isBorderBottom {
		border = t.style.Box.BottomRight
	} else if hint.isSeparatorRow {
		if t.shouldMergeCellsVertically(t.numColumns-1, hint) {
			border = t.style.Box.Right
		} else {
			border = t.style.Box.RightSeparator
		}
	}
	return border
}

func (t *Table) getColumnColors(colIdx int, hint renderHint) text.Colors {
	if hint.isBorderOrSeparator() {
		if colors := t.getColumnColorsForBorderOrSeparator(hint); colors != nil {
			return colors
		}
	}
	if t.rowPainter != nil && hint.isRegularNonSeparatorRow() && !t.isIndexColumn(colIdx, hint) {
		if colors := t.rowsColors[hint.rowNumber-1]; colors != nil {
			return colors
		}
	}
	if cfg, ok := t.columnConfigMap[colIdx]; ok {
		if hint.isSeparatorRow {
			return nil
		} else if hint.isHeaderRow {
			return cfg.ColorsHeader
		} else if hint.isFooterRow {
			return cfg.ColorsFooter
		}
		return cfg.Colors
	}
	return nil
}

func (t *Table) getColumnColorsForBorderOrSeparator(hint renderHint) text.Colors {
	if t.style.Options.DoNotColorBordersAndSeparators {
		return text.Colors{} // not nil to force caller to paint with no colors
	}
	if (hint.isBorderBottom || hint.isBorderTop) && t.style.Color.Border != nil {
		return t.style.Color.Border
	}
	if hint.isSeparatorRow && t.style.Color.Separator != nil {
		return t.style.Color.Separator
	}
	return nil
}

func (t *Table) getColumnSeparator(row rowStr, colIdx int, hint renderHint) string {
	separator := t.style.Box.MiddleVertical
	if hint.isSeparatorRow {
		if hint.isBorderTop {
			if t.shouldMergeCellsHorizontallyBelow(row, colIdx, hint) {
				separator = t.style.Box.MiddleHorizontal
			} else {
				separator = t.style.Box.TopSeparator
			}
		} else if hint.isBorderBottom {
			if t.shouldMergeCellsHorizontallyAbove(row, colIdx, hint) {
				separator = t.style.Box.MiddleHorizontal
			} else {
				separator = t.style.Box.BottomSeparator
			}
		} else {
			sm1 := t.shouldMergeCellsHorizontallyAbove(row, colIdx, hint)
			sm2 := t.shouldMergeCellsHorizontallyBelow(row, colIdx, hint)
			separator = t.getColumnSeparatorNonBorder(sm1, sm2, colIdx, hint)
		}
	}
	return separator
}

func (t *Table) getColumnSeparatorNonBorder(mergeCellsAbove bool, mergeCellsBelow bool, colIdx int, hint renderHint) string {
	mergeNextCol := t.shouldMergeCellsVertically(colIdx, hint)
	if hint.isAutoIndexColumn {
		return t.getColumnSeparatorNonBorderAutoIndex(mergeNextCol, hint)
	}

	mergeCurrCol := t.shouldMergeCellsVertically(colIdx-1, hint)
	return t.getColumnSeparatorNonBorderNonAutoIndex(mergeCellsAbove, mergeCellsBelow, mergeCurrCol, mergeNextCol)
}

func (t *Table) getColumnSeparatorNonBorderAutoIndex(mergeNextCol bool, hint renderHint) string {
	if hint.isHeaderOrFooterSeparator() {
		if mergeNextCol {
			return t.style.Box.MiddleVertical
		}
		return t.style.Box.LeftSeparator
	} else if mergeNextCol {
		return t.style.Box.RightSeparator
	}
	return t.style.Box.MiddleSeparator
}

func (t *Table) getColumnSeparatorNonBorderNonAutoIndex(mergeCellsAbove bool, mergeCellsBelow bool, mergeCurrCol bool, mergeNextCol bool) string {
	if mergeCellsAbove && mergeCellsBelow && mergeCurrCol && mergeNextCol {
		return t.style.Box.EmptySeparator
	} else if mergeCellsAbove && mergeCellsBelow {
		return t.style.Box.MiddleHorizontal
	} else if mergeCellsAbove {
		return t.style.Box.TopSeparator
	} else if mergeCellsBelow {
		return t.style.Box.BottomSeparator
	} else if mergeCurrCol && mergeNextCol {
		return t.style.Box.MiddleVertical
	} else if mergeCurrCol {
		return t.style.Box.LeftSeparator
	} else if mergeNextCol {
		return t.style.Box.RightSeparator
	}
	return t.style.Box.MiddleSeparator
}

func (t *Table) getColumnTransformer(colIdx int, hint renderHint) text.Transformer {
	var transformer text.Transformer
	if cfg, ok := t.columnConfigMap[colIdx]; ok {
		if hint.isHeaderRow {
			transformer = cfg.TransformerHeader
		} else if hint.isFooterRow {
			transformer = cfg.TransformerFooter
		} else {
			transformer = cfg.Transformer
		}
	}
	return transformer
}

func (t *Table) getColumnWidthMax(colIdx int) int {
	if cfg, ok := t.columnConfigMap[colIdx]; ok {
		return cfg.WidthMax
	}
	return 0
}

func (t *Table) getColumnWidthMin(colIdx int) int {
	if cfg, ok := t.columnConfigMap[colIdx]; ok {
		return cfg.WidthMin
	}
	return 0
}

func (t *Table) getFormat(hint renderHint) text.Format {
	if hint.isSeparatorRow {
		return text.FormatDefault
	} else if hint.isHeaderRow {
		return t.style.Format.Header
	} else if hint.isFooterRow {
		return t.style.Format.Footer
	}
	return t.style.Format.Row
}

func (t *Table) getMaxColumnLengthForMerging(colIdx int) int {
	maxColumnLength := t.maxColumnLengths[colIdx]
	maxColumnLength += text.RuneWidthWithoutEscSequences(t.style.Box.PaddingRight + t.style.Box.PaddingLeft)
	if t.style.Options.SeparateColumns {
		maxColumnLength += text.RuneWidthWithoutEscSequences(t.style.Box.EmptySeparator)
	}
	return maxColumnLength
}

// getMergedColumnIndices returns a map of colIdx values to all the other colIdx
// values (that are being merged) and their lengths.
func (t *Table) getMergedColumnIndices(row rowStr, hint renderHint) mergedColumnIndices {
	if !t.getRowConfig(hint).AutoMerge {
		return nil
	}

	mci := make(mergedColumnIndices)
	for colIdx := 0; colIdx < t.numColumns-1; colIdx++ {
		// look backward
		for otherColIdx := colIdx - 1; colIdx >= 0 && otherColIdx >= 0; otherColIdx-- {
			if row[colIdx] != row[otherColIdx] {
				break
			}
			mci.safeAppend(colIdx, otherColIdx)
		}
		// look forward
		for otherColIdx := colIdx + 1; colIdx < len(row) && otherColIdx < len(row); otherColIdx++ {
			if row[colIdx] != row[otherColIdx] {
				break
			}
			mci.safeAppend(colIdx, otherColIdx)
		}
	}
	return mci
}

func (t *Table) getRow(rowIdx int, hint renderHint) rowStr {
	switch {
	case hint.isHeaderRow:
		if rowIdx >= 0 && rowIdx < len(t.rowsHeader) {
			return t.rowsHeader[rowIdx]
		}
	case hint.isFooterRow:
		if rowIdx >= 0 && rowIdx < len(t.rowsFooter) {
			return t.rowsFooter[rowIdx]
		}
	default:
		if rowIdx >= 0 && rowIdx < len(t.rows) {
			return t.rows[rowIdx]
		}
	}
	return rowStr{}
}

func (t *Table) getRowConfig(hint renderHint) RowConfig {
	rowIdx := hint.rowNumber - 1
	if rowIdx < 0 {
		rowIdx = 0
	}

	switch {
	case hint.isHeaderRow:
		return t.rowsHeaderConfigMap[rowIdx]
	case hint.isFooterRow:
		return t.rowsFooterConfigMap[rowIdx]
	default:
		return t.rowsConfigMap[rowIdx]
	}
}

func (t *Table) getSeparatorColors(hint renderHint) text.Colors {
	if t.style.Options.DoNotColorBordersAndSeparators {
		return nil
	} else if (hint.isBorderBottom || hint.isBorderTop) && t.style.Color.Border != nil {
		return t.style.Color.Border
	} else if t.style.Color.Separator != nil {
		return t.style.Color.Separator
	} else if hint.isHeaderRow {
		return t.style.Color.Header
	} else if hint.isFooterRow {
		return t.style.Color.Footer
	} else if hint.isAutoIndexColumn {
		return t.style.Color.IndexColumn
	} else if hint.rowNumber > 0 && hint.rowNumber%2 == 0 {
		return t.style.Color.RowAlternate
	}
	return t.style.Color.Row
}

func (t *Table) getVAlign(colIdx int, hint renderHint) text.VAlign {
	vAlign := text.VAlignDefault
	if cfg, ok := t.columnConfigMap[colIdx]; ok {
		if hint.isHeaderRow {
			vAlign = cfg.VAlignHeader
		} else if hint.isFooterRow {
			vAlign = cfg.VAlignFooter
		} else {
			vAlign = cfg.VAlign
		}
	}
	return vAlign
}

func (t *Table) hasHiddenColumns() bool {
	for _, cc := range t.columnConfigMap {
		if cc.Hidden {
			return true
		}
	}
	return false
}

func (t *Table) hideColumns() map[int]int {
	colIdxMap := make(map[int]int)
	numColumns := 0
	hideColumnsInRows := func(rows []rowStr) []rowStr {
		var rsp []rowStr
		for _, row := range rows {
			var rowNew rowStr
			for colIdx, col := range row {
				cc := t.columnConfigMap[colIdx]
				if !cc.Hidden {
					rowNew = append(rowNew, col)
					colIdxMap[colIdx] = len(rowNew) - 1
				}
			}
			if len(rowNew) > numColumns {
				numColumns = len(rowNew)
			}
			rsp = append(rsp, rowNew)
		}
		return rsp
	}

	// hide columns as directed
	t.rows = hideColumnsInRows(t.rows)
	t.rowsFooter = hideColumnsInRows(t.rowsFooter)
	t.rowsHeader = hideColumnsInRows(t.rowsHeader)

	// reset numColumns to the new number of columns
	t.numColumns = numColumns

	return colIdxMap
}

func (t *Table) isIndexColumn(colIdx int, hint renderHint) bool {
	return t.indexColumn == colIdx+1 || hint.isAutoIndexColumn
}

func (t *Table) render(out *strings.Builder) string {
	outStr := out.String()
	if t.outputMirror != nil && len(outStr) > 0 {
		_, _ = t.outputMirror.Write([]byte(outStr))
		_, _ = t.outputMirror.Write([]byte("\n"))
	}
	return outStr
}

func (t *Table) shouldMergeCellsHorizontallyAbove(row rowStr, colIdx int, hint renderHint) bool {
	if hint.isAutoIndexColumn || hint.isAutoIndexRow {
		return false
	}

	rowConfig := t.getRowConfig(hint)
	if hint.isSeparatorRow {
		if hint.isHeaderRow && hint.rowNumber == 1 {
			rowConfig = t.getRowConfig(hint)
			row = t.getRow(hint.rowNumber-1, hint)
		} else if hint.isFooterRow && hint.isFirstRow {
			rowConfig = t.getRowConfig(renderHint{isLastRow: true, rowNumber: len(t.rows)})
			row = t.getRow(len(t.rows)-1, renderHint{})
		} else if hint.isFooterRow && hint.isBorderBottom {
			row = t.getRow(len(t.rowsFooter)-1, renderHint{isFooterRow: true})
		} else {
			row = t.getRow(hint.rowNumber-1, hint)
		}
	}

	if rowConfig.AutoMerge {
		return row.areEqual(colIdx-1, colIdx)
	}
	return false
}

func (t *Table) shouldMergeCellsHorizontallyBelow(row rowStr, colIdx int, hint renderHint) bool {
	if hint.isAutoIndexColumn || hint.isAutoIndexRow {
		return false
	}

	var rowConfig RowConfig
	if hint.isSeparatorRow {
		if hint.isRegularRow() {
			rowConfig = t.getRowConfig(renderHint{rowNumber: hint.rowNumber + 1})
			row = t.getRow(hint.rowNumber, renderHint{})
		} else if hint.isHeaderRow && hint.rowNumber == 0 {
			rowConfig = t.getRowConfig(renderHint{isHeaderRow: true, rowNumber: 1})
			row = t.getRow(0, hint)
		} else if hint.isHeaderRow && hint.isLastRow {
			rowConfig = t.getRowConfig(renderHint{rowNumber: 1})
			row = t.getRow(0, renderHint{})
		} else if hint.isHeaderRow {
			rowConfig = t.getRowConfig(renderHint{isHeaderRow: true, rowNumber: hint.rowNumber + 1})
			row = t.getRow(hint.rowNumber, hint)
		} else if hint.isFooterRow && hint.rowNumber >= 0 {
			rowConfig = t.getRowConfig(renderHint{isFooterRow: true, rowNumber: 1})
			row = t.getRow(hint.rowNumber, renderHint{isFooterRow: true})
		}
	}

	if rowConfig.AutoMerge {
		return row.areEqual(colIdx-1, colIdx)
	}
	return false
}

func (t *Table) shouldMergeCellsVertically(colIdx int, hint renderHint) bool {
	if t.columnConfigMap[colIdx].AutoMerge && colIdx < t.numColumns {
		if hint.isSeparatorRow {
			rowPrev := t.getRow(hint.rowNumber-1, hint)
			rowNext := t.getRow(hint.rowNumber, hint)
			if colIdx < len(rowPrev) && colIdx < len(rowNext) {
				return rowPrev[colIdx] == rowNext[colIdx]
			}
		} else {
			rowPrev := t.getRow(hint.rowNumber-2, hint)
			rowCurr := t.getRow(hint.rowNumber-1, hint)
			if colIdx < len(rowPrev) && colIdx < len(rowCurr) {
				return rowPrev[colIdx] == rowCurr[colIdx]
			}
		}
	}
	return false
}

func (t *Table) wrapRow(row rowStr) (int, rowStr) {
	colMaxLines := 0
	rowWrapped := make(rowStr, len(row))
	for colIdx, colStr := range row {
		widthEnforcer := t.columnConfigMap[colIdx].getWidthMaxEnforcer()
		maxWidth := t.getColumnWidthMax(colIdx)
		if maxWidth == 0 {
			maxWidth = t.maxColumnLengths[colIdx]
		}
		rowWrapped[colIdx] = widthEnforcer(colStr, maxWidth)
		colNumLines := strings.Count(rowWrapped[colIdx], "\n") + 1
		if colNumLines > colMaxLines {
			colMaxLines = colNumLines
		}
	}
	return colMaxLines, rowWrapped
}
