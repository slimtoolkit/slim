package table

// renderHint has hints for the Render*() logic
type renderHint struct {
	isAutoIndexColumn bool // auto-index column?
	isAutoIndexRow    bool // auto-index row?
	isBorderBottom    bool // bottom-border?
	isBorderTop       bool // top-border?
	isFirstRow        bool // first-row of header/footer/regular-rows?
	isFooterRow       bool // footer row?
	isHeaderRow       bool // header row?
	isLastLineOfRow   bool // last-line of the current row?
	isLastRow         bool // last-row of header/footer/regular-rows?
	isSeparatorRow    bool // separator row?
	isTitleRow        bool // title row?
	rowLineNumber     int  // the line number for a multi-line row
	rowNumber         int  // the row number/index
}

func (h *renderHint) isBorderOrSeparator() bool {
	return h.isBorderTop || h.isSeparatorRow || h.isBorderBottom
}

func (h *renderHint) isRegularRow() bool {
	return !h.isHeaderRow && !h.isFooterRow
}

func (h *renderHint) isRegularNonSeparatorRow() bool {
	return !h.isHeaderRow && !h.isFooterRow && !h.isSeparatorRow
}

func (h *renderHint) isHeaderOrFooterSeparator() bool {
	return h.isSeparatorRow && !h.isBorderBottom && !h.isBorderTop &&
		((h.isHeaderRow && !h.isLastRow) || (h.isFooterRow && (!h.isFirstRow || h.rowNumber > 0)))
}

func (h *renderHint) isLastLineOfLastRow() bool {
	return h.isLastLineOfRow && h.isLastRow
}
