package table

import (
	"github.com/jedib0t/go-pretty/v6/text"
)

// ColumnConfig contains configurations that determine and modify the way the
// contents of the column get rendered.
type ColumnConfig struct {
	// Name is the name of the Column as it appears in the first Header row.
	// If a Header is not provided, or the name is not found in the header, this
	// will not work.
	Name string
	// Number is the Column # from left. When specified, it overrides the Name
	// property. If you know the exact Column number, use this instead of Name.
	Number int

	// Align defines the horizontal alignment
	Align text.Align
	// AlignFooter defines the horizontal alignment of Footer rows
	AlignFooter text.Align
	// AlignHeader defines the horizontal alignment of Header rows
	AlignHeader text.Align

	// AutoMerge merges cells with similar values and prevents separators from
	// being drawn. Caveats:
	// * VAlign is applied on the individual cell and not on the merged cell
	// * Does not work in CSV/HTML/Markdown render modes
	// * Does not work well with horizontal auto-merge (RowConfig.AutoMerge)
	//
	// Works best when:
	// * Style().Options.SeparateRows == true
	// * Style().Color.Row == Style().Color.RowAlternate (or not set)
	AutoMerge bool

	// Colors defines the colors to be used on the column
	Colors text.Colors
	// ColorsFooter defines the colors to be used on the column in Footer rows
	ColorsFooter text.Colors
	// ColorsHeader defines the colors to be used on the column in Header rows
	ColorsHeader text.Colors

	// Hidden when set to true will prevent the column from being rendered.
	// This is useful in cases like needing a column for sorting, but not for
	// display.
	Hidden bool

	// Transformer is a custom-function that changes the way the value gets
	// rendered to the console. Refer to text/transformer.go for ready-to-use
	// Transformer functions.
	Transformer text.Transformer
	// TransformerFooter is like Transformer but for Footer rows
	TransformerFooter text.Transformer
	// TransformerHeader is like Transformer but for Header rows
	TransformerHeader text.Transformer

	// VAlign defines the vertical alignment
	VAlign text.VAlign
	// VAlignFooter defines the vertical alignment in Footer rows
	VAlignFooter text.VAlign
	// VAlignHeader defines the vertical alignment in Header rows
	VAlignHeader text.VAlign

	// WidthMax defines the maximum character length of the column
	WidthMax int
	// WidthEnforcer enforces the WidthMax value on the column contents;
	// default: text.WrapText
	WidthMaxEnforcer WidthEnforcer
	// WidthMin defines the minimum character length of the column
	WidthMin int
}

func (c ColumnConfig) getWidthMaxEnforcer() WidthEnforcer {
	if c.WidthMax == 0 {
		return widthEnforcerNone
	}
	if c.WidthMaxEnforcer != nil {
		return c.WidthMaxEnforcer
	}
	return text.WrapText
}

// RowConfig contains configurations that determine and modify the way the
// contents of a row get rendered.
type RowConfig struct {
	// AutoMerge merges cells with similar values and prevents separators from
	// being drawn. Caveats:
	// * Align is overridden to text.AlignCenter on the merged cell (unless set
	//   by AutoMergeAlign value below)
	// * Does not work in CSV/HTML/Markdown render modes
	// * Does not work well with vertical auto-merge (ColumnConfig.AutoMerge)
	AutoMerge bool

	// Alignment to use on a merge (defaults to text.AlignCenter)
	AutoMergeAlign text.Align
}

func (rc RowConfig) getAutoMergeAlign() text.Align {
	if rc.AutoMergeAlign == text.AlignDefault {
		return text.AlignCenter
	}
	return rc.AutoMergeAlign
}
