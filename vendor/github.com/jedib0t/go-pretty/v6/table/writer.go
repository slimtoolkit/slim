package table

import (
	"io"
)

// Writer declares the interfaces that can be used to set up and render a table.
type Writer interface {
	AppendFooter(row Row, configs ...RowConfig)
	AppendHeader(row Row, configs ...RowConfig)
	AppendRow(row Row, configs ...RowConfig)
	AppendRows(rows []Row, configs ...RowConfig)
	AppendSeparator()
	Length() int
	Render() string
	RenderCSV() string
	RenderHTML() string
	RenderMarkdown() string
	RenderTSV() string
	ResetFooters()
	ResetHeaders()
	ResetRows()
	SetAllowedRowLength(length int)
	SetAutoIndex(autoIndex bool)
	SetCaption(format string, a ...interface{})
	SetColumnConfigs(configs []ColumnConfig)
	SetIndexColumn(colNum int)
	SetOutputMirror(mirror io.Writer)
	SetPageSize(numLines int)
	SetRowPainter(painter RowPainter)
	SetStyle(style Style)
	SetTitle(format string, a ...interface{})
	SortBy(sortBy []SortBy)
	Style() *Style
	SuppressEmptyColumns()

	// deprecated; in favor of Style().HTML.CSSClass
	SetHTMLCSSClass(cssClass string)
}

// NewWriter initializes and returns a Writer.
func NewWriter() Writer {
	return &Table{}
}
