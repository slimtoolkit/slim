package table

import (
	"sort"
	"strconv"
)

// SortBy defines What to sort (Column Name or Number), and How to sort (Mode).
type SortBy struct {
	// Name is the name of the Column as it appears in the first Header row.
	// If a Header is not provided, or the name is not found in the header, this
	// will not work.
	Name string
	// Number is the Column # from left. When specified, it overrides the Name
	// property. If you know the exact Column number, use this instead of Name.
	Number int

	// Mode tells the Writer how to Sort. Asc/Dsc/etc.
	Mode SortMode
}

// SortMode defines How to sort.
type SortMode int

const (
	// Asc sorts the column in Ascending order alphabetically.
	Asc SortMode = iota
	// AscNumeric sorts the column in Ascending order numerically.
	AscNumeric
	// Dsc sorts the column in Descending order alphabetically.
	Dsc
	// DscNumeric sorts the column in Descending order numerically.
	DscNumeric
)

type rowsSorter struct {
	rows          []rowStr
	sortBy        []SortBy
	sortedIndices []int
}

// getSortedRowIndices sorts and returns the row indices in Sorted order as
// directed by Table.sortBy which can be set using Table.SortBy(...)
func (t *Table) getSortedRowIndices() []int {
	sortedIndices := make([]int, len(t.rows))
	for idx := range t.rows {
		sortedIndices[idx] = idx
	}

	if t.sortBy != nil && len(t.sortBy) > 0 {
		sort.Sort(rowsSorter{
			rows:          t.rows,
			sortBy:        t.parseSortBy(t.sortBy),
			sortedIndices: sortedIndices,
		})
	}

	return sortedIndices
}

func (t *Table) parseSortBy(sortBy []SortBy) []SortBy {
	var resSortBy []SortBy
	for _, col := range sortBy {
		colNum := 0
		if col.Number > 0 && col.Number <= t.numColumns {
			colNum = col.Number
		} else if col.Name != "" && len(t.rowsHeader) > 0 {
			for idx, colName := range t.rowsHeader[0] {
				if col.Name == colName {
					colNum = idx + 1
					break
				}
			}
		}
		if colNum > 0 {
			resSortBy = append(resSortBy, SortBy{
				Name:   col.Name,
				Number: colNum,
				Mode:   col.Mode,
			})
		}
	}
	return resSortBy
}

func (rs rowsSorter) Len() int {
	return len(rs.rows)
}

func (rs rowsSorter) Swap(i, j int) {
	rs.sortedIndices[i], rs.sortedIndices[j] = rs.sortedIndices[j], rs.sortedIndices[i]
}

func (rs rowsSorter) Less(i, j int) bool {
	realI, realJ := rs.sortedIndices[i], rs.sortedIndices[j]
	for _, col := range rs.sortBy {
		rowI, rowJ, colIdx := rs.rows[realI], rs.rows[realJ], col.Number-1
		if colIdx < len(rowI) && colIdx < len(rowJ) {
			shouldContinue, returnValue := rs.lessColumns(rowI, rowJ, colIdx, col)
			if !shouldContinue {
				return returnValue
			}
		}
	}
	return false
}

func (rs rowsSorter) lessColumns(rowI rowStr, rowJ rowStr, colIdx int, col SortBy) (bool, bool) {
	if rowI[colIdx] == rowJ[colIdx] {
		return true, false
	} else if col.Mode == Asc {
		return false, rowI[colIdx] < rowJ[colIdx]
	} else if col.Mode == Dsc {
		return false, rowI[colIdx] > rowJ[colIdx]
	}

	iVal, iErr := strconv.ParseFloat(rowI[colIdx], 64)
	jVal, jErr := strconv.ParseFloat(rowJ[colIdx], 64)
	if iErr == nil && jErr == nil {
		if col.Mode == AscNumeric {
			return false, iVal < jVal
		} else if col.Mode == DscNumeric {
			return false, jVal < iVal
		}
	}
	return true, false
}
