package table

import (
	"github.com/jedib0t/go-pretty/v6/text"
)

// Style declares how to render the Table and provides very fine-grained control
// on how the Table gets rendered on the Console.
type Style struct {
	Name    string        // name of the Style
	Box     BoxStyle      // characters to use for the boxes
	Color   ColorOptions  // colors to use for the rows and columns
	Format  FormatOptions // formatting options for the rows and columns
	HTML    HTMLOptions   // rendering options for HTML mode
	Options Options       // misc. options for the table
	Title   TitleOptions  // formation options for the title text
}

var (
	// StyleDefault renders a Table like below:
	//  +-----+------------+-----------+--------+-----------------------------+
	//  |   # | FIRST NAME | LAST NAME | SALARY |                             |
	//  +-----+------------+-----------+--------+-----------------------------+
	//  |   1 | Arya       | Stark     |   3000 |                             |
	//  |  20 | Jon        | Snow      |   2000 | You know nothing, Jon Snow! |
	//  | 300 | Tyrion     | Lannister |   5000 |                             |
	//  +-----+------------+-----------+--------+-----------------------------+
	//  |     |            | TOTAL     |  10000 |                             |
	//  +-----+------------+-----------+--------+-----------------------------+
	StyleDefault = Style{
		Name:    "StyleDefault",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsDefault,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsDefault,
		Title:   TitleOptionsDefault,
	}

	// StyleBold renders a Table like below:
	//  ┏━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━━┳━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
	//  ┃   # ┃ FIRST NAME ┃ LAST NAME ┃ SALARY ┃                             ┃
	//  ┣━━━━━╋━━━━━━━━━━━━╋━━━━━━━━━━━╋━━━━━━━━╋━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫
	//  ┃   1 ┃ Arya       ┃ Stark     ┃   3000 ┃                             ┃
	//  ┃  20 ┃ Jon        ┃ Snow      ┃   2000 ┃ You know nothing, Jon Snow! ┃
	//  ┃ 300 ┃ Tyrion     ┃ Lannister ┃   5000 ┃                             ┃
	//  ┣━━━━━╋━━━━━━━━━━━━╋━━━━━━━━━━━╋━━━━━━━━╋━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫
	//  ┃     ┃            ┃ TOTAL     ┃  10000 ┃                             ┃
	//  ┗━━━━━┻━━━━━━━━━━━━┻━━━━━━━━━━━┻━━━━━━━━┻━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
	StyleBold = Style{
		Name:    "StyleBold",
		Box:     StyleBoxBold,
		Color:   ColorOptionsDefault,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsDefault,
		Title:   TitleOptionsDefault,
	}

	// StyleColoredBright renders a Table without any borders or separators,
	// and with Black text on Cyan background for Header/Footer and
	// White background for other rows.
	StyleColoredBright = Style{
		Name:    "StyleColoredBright",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsBright,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsDark,
	}

	// StyleColoredDark renders a Table without any borders or separators, and
	// with Header/Footer in Cyan text and other rows with White text, all on
	// Black background.
	StyleColoredDark = Style{
		Name:    "StyleColoredDark",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsDark,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsBright,
	}

	// StyleColoredBlackOnBlueWhite renders a Table without any borders or
	// separators, and with Black text on Blue background for Header/Footer and
	// White background for other rows.
	StyleColoredBlackOnBlueWhite = Style{
		Name:    "StyleColoredBlackOnBlueWhite",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsBlackOnBlueWhite,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsBlueOnBlack,
	}

	// StyleColoredBlackOnCyanWhite renders a Table without any borders or
	// separators, and with Black text on Cyan background for Header/Footer and
	// White background for other rows.
	StyleColoredBlackOnCyanWhite = Style{
		Name:    "StyleColoredBlackOnCyanWhite",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsBlackOnCyanWhite,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsCyanOnBlack,
	}

	// StyleColoredBlackOnGreenWhite renders a Table without any borders or
	// separators, and with Black text on Green background for Header/Footer and
	// White background for other rows.
	StyleColoredBlackOnGreenWhite = Style{
		Name:    "StyleColoredBlackOnGreenWhite",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsBlackOnGreenWhite,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsGreenOnBlack,
	}

	// StyleColoredBlackOnMagentaWhite renders a Table without any borders or
	// separators, and with Black text on Magenta background for Header/Footer and
	// White background for other rows.
	StyleColoredBlackOnMagentaWhite = Style{
		Name:    "StyleColoredBlackOnMagentaWhite",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsBlackOnMagentaWhite,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsMagentaOnBlack,
	}

	// StyleColoredBlackOnYellowWhite renders a Table without any borders or
	// separators, and with Black text on Yellow background for Header/Footer and
	// White background for other rows.
	StyleColoredBlackOnYellowWhite = Style{
		Name:    "StyleColoredBlackOnYellowWhite",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsBlackOnYellowWhite,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsYellowOnBlack,
	}

	// StyleColoredBlackOnRedWhite renders a Table without any borders or
	// separators, and with Black text on Red background for Header/Footer and
	// White background for other rows.
	StyleColoredBlackOnRedWhite = Style{
		Name:    "StyleColoredBlackOnRedWhite",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsBlackOnRedWhite,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsRedOnBlack,
	}

	// StyleColoredBlueWhiteOnBlack renders a Table without any borders or
	// separators, and with Header/Footer in Blue text and other rows with
	// White text, all on Black background.
	StyleColoredBlueWhiteOnBlack = Style{
		Name:    "StyleColoredBlueWhiteOnBlack",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsBlueWhiteOnBlack,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsBlackOnBlue,
	}

	// StyleColoredCyanWhiteOnBlack renders a Table without any borders or
	// separators, and with Header/Footer in Cyan text and other rows with
	// White text, all on Black background.
	StyleColoredCyanWhiteOnBlack = Style{
		Name:    "StyleColoredCyanWhiteOnBlack",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsCyanWhiteOnBlack,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsBlackOnCyan,
	}

	// StyleColoredGreenWhiteOnBlack renders a Table without any borders or
	// separators, and with Header/Footer in Green text and other rows with
	// White text, all on Black background.
	StyleColoredGreenWhiteOnBlack = Style{
		Name:    "StyleColoredGreenWhiteOnBlack",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsGreenWhiteOnBlack,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsBlackOnGreen,
	}

	// StyleColoredMagentaWhiteOnBlack renders a Table without any borders or
	// separators, and with Header/Footer in Magenta text and other rows with
	// White text, all on Black background.
	StyleColoredMagentaWhiteOnBlack = Style{
		Name:    "StyleColoredMagentaWhiteOnBlack",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsMagentaWhiteOnBlack,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsBlackOnMagenta,
	}

	// StyleColoredRedWhiteOnBlack renders a Table without any borders or
	// separators, and with Header/Footer in Red text and other rows with
	// White text, all on Black background.
	StyleColoredRedWhiteOnBlack = Style{
		Name:    "StyleColoredRedWhiteOnBlack",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsRedWhiteOnBlack,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsBlackOnRed,
	}

	// StyleColoredYellowWhiteOnBlack renders a Table without any borders or
	// separators, and with Header/Footer in Yellow text and other rows with
	// White text, all on Black background.
	StyleColoredYellowWhiteOnBlack = Style{
		Name:    "StyleColoredYellowWhiteOnBlack",
		Box:     StyleBoxDefault,
		Color:   ColorOptionsYellowWhiteOnBlack,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsNoBordersAndSeparators,
		Title:   TitleOptionsBlackOnYellow,
	}

	// StyleDouble renders a Table like below:
	//  ╔═════╦════════════╦═══════════╦════════╦═════════════════════════════╗
	//  ║   # ║ FIRST NAME ║ LAST NAME ║ SALARY ║                             ║
	//  ╠═════╬════════════╬═══════════╬════════╬═════════════════════════════╣
	//  ║   1 ║ Arya       ║ Stark     ║   3000 ║                             ║
	//  ║  20 ║ Jon        ║ Snow      ║   2000 ║ You know nothing, Jon Snow! ║
	//  ║ 300 ║ Tyrion     ║ Lannister ║   5000 ║                             ║
	//  ╠═════╬════════════╬═══════════╬════════╬═════════════════════════════╣
	//  ║     ║            ║ TOTAL     ║  10000 ║                             ║
	//  ╚═════╩════════════╩═══════════╩════════╩═════════════════════════════╝
	StyleDouble = Style{
		Name:    "StyleDouble",
		Box:     StyleBoxDouble,
		Color:   ColorOptionsDefault,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsDefault,
		Title:   TitleOptionsDefault,
	}

	// StyleLight renders a Table like below:
	//  ┌─────┬────────────┬───────────┬────────┬─────────────────────────────┐
	//  │   # │ FIRST NAME │ LAST NAME │ SALARY │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │   1 │ Arya       │ Stark     │   3000 │                             │
	//  │  20 │ Jon        │ Snow      │   2000 │ You know nothing, Jon Snow! │
	//  │ 300 │ Tyrion     │ Lannister │   5000 │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │     │            │ TOTAL     │  10000 │                             │
	//  └─────┴────────────┴───────────┴────────┴─────────────────────────────┘
	StyleLight = Style{
		Name:    "StyleLight",
		Box:     StyleBoxLight,
		Color:   ColorOptionsDefault,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsDefault,
		Title:   TitleOptionsDefault,
	}

	// StyleRounded renders a Table like below:
	//  ╭─────┬────────────┬───────────┬────────┬─────────────────────────────╮
	//  │   # │ FIRST NAME │ LAST NAME │ SALARY │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │   1 │ Arya       │ Stark     │   3000 │                             │
	//  │  20 │ Jon        │ Snow      │   2000 │ You know nothing, Jon Snow! │
	//  │ 300 │ Tyrion     │ Lannister │   5000 │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │     │            │ TOTAL     │  10000 │                             │
	//  ╰─────┴────────────┴───────────┴────────┴─────────────────────────────╯
	StyleRounded = Style{
		Name:    "StyleRounded",
		Box:     StyleBoxRounded,
		Color:   ColorOptionsDefault,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsDefault,
		Title:   TitleOptionsDefault,
	}

	// styleTest renders a Table like below:
	//  (-----^------------^-----------^--------^-----------------------------)
	//  [<  #>|<FIRST NAME>|<LAST NAME>|<SALARY>|<                           >]
	//  {-----+------------+-----------+--------+-----------------------------}
	//  [<  1>|<Arya      >|<Stark    >|<  3000>|<                           >]
	//  [< 20>|<Jon       >|<Snow     >|<  2000>|<You know nothing, Jon Snow!>]
	//  [<300>|<Tyrion    >|<Lannister>|<  5000>|<                           >]
	//  {-----+------------+-----------+--------+-----------------------------}
	//  [<   >|<          >|<TOTAL    >|< 10000>|<                           >]
	//  \-----v------------v-----------v--------v-----------------------------/
	styleTest = Style{
		Name:    "styleTest",
		Box:     styleBoxTest,
		Color:   ColorOptionsDefault,
		Format:  FormatOptionsDefault,
		HTML:    DefaultHTMLOptions,
		Options: OptionsDefault,
		Title:   TitleOptionsDefault,
	}
)

// BoxStyle defines the characters/strings to use to render the borders and
// separators for the Table.
type BoxStyle struct {
	BottomLeft       string
	BottomRight      string
	BottomSeparator  string
	EmptySeparator   string
	Left             string
	LeftSeparator    string
	MiddleHorizontal string
	MiddleSeparator  string
	MiddleVertical   string
	PaddingLeft      string
	PaddingRight     string
	PageSeparator    string
	Right            string
	RightSeparator   string
	TopLeft          string
	TopRight         string
	TopSeparator     string
	UnfinishedRow    string
}

var (
	// StyleBoxDefault defines a Boxed-Table like below:
	//  +-----+------------+-----------+--------+-----------------------------+
	//  |   # | FIRST NAME | LAST NAME | SALARY |                             |
	//  +-----+------------+-----------+--------+-----------------------------+
	//  |   1 | Arya       | Stark     |   3000 |                             |
	//  |  20 | Jon        | Snow      |   2000 | You know nothing, Jon Snow! |
	//  | 300 | Tyrion     | Lannister |   5000 |                             |
	//  +-----+------------+-----------+--------+-----------------------------+
	//  |     |            | TOTAL     |  10000 |                             |
	//  +-----+------------+-----------+--------+-----------------------------+
	StyleBoxDefault = BoxStyle{
		BottomLeft:       "+",
		BottomRight:      "+",
		BottomSeparator:  "+",
		EmptySeparator:   text.RepeatAndTrim(" ", text.RuneWidthWithoutEscSequences("+")),
		Left:             "|",
		LeftSeparator:    "+",
		MiddleHorizontal: "-",
		MiddleSeparator:  "+",
		MiddleVertical:   "|",
		PaddingLeft:      " ",
		PaddingRight:     " ",
		PageSeparator:    "\n",
		Right:            "|",
		RightSeparator:   "+",
		TopLeft:          "+",
		TopRight:         "+",
		TopSeparator:     "+",
		UnfinishedRow:    " ~",
	}

	// StyleBoxBold defines a Boxed-Table like below:
	//  ┏━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━━┳━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
	//  ┃   # ┃ FIRST NAME ┃ LAST NAME ┃ SALARY ┃                             ┃
	//  ┣━━━━━╋━━━━━━━━━━━━╋━━━━━━━━━━━╋━━━━━━━━╋━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫
	//  ┃   1 ┃ Arya       ┃ Stark     ┃   3000 ┃                             ┃
	//  ┃  20 ┃ Jon        ┃ Snow      ┃   2000 ┃ You know nothing, Jon Snow! ┃
	//  ┃ 300 ┃ Tyrion     ┃ Lannister ┃   5000 ┃                             ┃
	//  ┣━━━━━╋━━━━━━━━━━━━╋━━━━━━━━━━━╋━━━━━━━━╋━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫
	//  ┃     ┃            ┃ TOTAL     ┃  10000 ┃                             ┃
	//  ┗━━━━━┻━━━━━━━━━━━━┻━━━━━━━━━━━┻━━━━━━━━┻━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
	StyleBoxBold = BoxStyle{
		BottomLeft:       "┗",
		BottomRight:      "┛",
		BottomSeparator:  "┻",
		EmptySeparator:   text.RepeatAndTrim(" ", text.RuneWidthWithoutEscSequences("╋")),
		Left:             "┃",
		LeftSeparator:    "┣",
		MiddleHorizontal: "━",
		MiddleSeparator:  "╋",
		MiddleVertical:   "┃",
		PaddingLeft:      " ",
		PaddingRight:     " ",
		PageSeparator:    "\n",
		Right:            "┃",
		RightSeparator:   "┫",
		TopLeft:          "┏",
		TopRight:         "┓",
		TopSeparator:     "┳",
		UnfinishedRow:    " ≈",
	}

	// StyleBoxDouble defines a Boxed-Table like below:
	//  ╔═════╦════════════╦═══════════╦════════╦═════════════════════════════╗
	//  ║   # ║ FIRST NAME ║ LAST NAME ║ SALARY ║                             ║
	//  ╠═════╬════════════╬═══════════╬════════╬═════════════════════════════╣
	//  ║   1 ║ Arya       ║ Stark     ║   3000 ║                             ║
	//  ║  20 ║ Jon        ║ Snow      ║   2000 ║ You know nothing, Jon Snow! ║
	//  ║ 300 ║ Tyrion     ║ Lannister ║   5000 ║                             ║
	//  ╠═════╬════════════╬═══════════╬════════╬═════════════════════════════╣
	//  ║     ║            ║ TOTAL     ║  10000 ║                             ║
	//  ╚═════╩════════════╩═══════════╩════════╩═════════════════════════════╝
	StyleBoxDouble = BoxStyle{
		BottomLeft:       "╚",
		BottomRight:      "╝",
		BottomSeparator:  "╩",
		EmptySeparator:   text.RepeatAndTrim(" ", text.RuneWidthWithoutEscSequences("╬")),
		Left:             "║",
		LeftSeparator:    "╠",
		MiddleHorizontal: "═",
		MiddleSeparator:  "╬",
		MiddleVertical:   "║",
		PaddingLeft:      " ",
		PaddingRight:     " ",
		PageSeparator:    "\n",
		Right:            "║",
		RightSeparator:   "╣",
		TopLeft:          "╔",
		TopRight:         "╗",
		TopSeparator:     "╦",
		UnfinishedRow:    " ≈",
	}

	// StyleBoxLight defines a Boxed-Table like below:
	//  ┌─────┬────────────┬───────────┬────────┬─────────────────────────────┐
	//  │   # │ FIRST NAME │ LAST NAME │ SALARY │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │   1 │ Arya       │ Stark     │   3000 │                             │
	//  │  20 │ Jon        │ Snow      │   2000 │ You know nothing, Jon Snow! │
	//  │ 300 │ Tyrion     │ Lannister │   5000 │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │     │            │ TOTAL     │  10000 │                             │
	//  └─────┴────────────┴───────────┴────────┴─────────────────────────────┘
	StyleBoxLight = BoxStyle{
		BottomLeft:       "└",
		BottomRight:      "┘",
		BottomSeparator:  "┴",
		EmptySeparator:   text.RepeatAndTrim(" ", text.RuneWidthWithoutEscSequences("┼")),
		Left:             "│",
		LeftSeparator:    "├",
		MiddleHorizontal: "─",
		MiddleSeparator:  "┼",
		MiddleVertical:   "│",
		PaddingLeft:      " ",
		PaddingRight:     " ",
		PageSeparator:    "\n",
		Right:            "│",
		RightSeparator:   "┤",
		TopLeft:          "┌",
		TopRight:         "┐",
		TopSeparator:     "┬",
		UnfinishedRow:    " ≈",
	}

	// StyleBoxRounded defines a Boxed-Table like below:
	//  ╭─────┬────────────┬───────────┬────────┬─────────────────────────────╮
	//  │   # │ FIRST NAME │ LAST NAME │ SALARY │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │   1 │ Arya       │ Stark     │   3000 │                             │
	//  │  20 │ Jon        │ Snow      │   2000 │ You know nothing, Jon Snow! │
	//  │ 300 │ Tyrion     │ Lannister │   5000 │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │     │            │ TOTAL     │  10000 │                             │
	//  ╰─────┴────────────┴───────────┴────────┴─────────────────────────────╯
	StyleBoxRounded = BoxStyle{
		BottomLeft:       "╰",
		BottomRight:      "╯",
		BottomSeparator:  "┴",
		EmptySeparator:   text.RepeatAndTrim(" ", text.RuneWidthWithoutEscSequences("┼")),
		Left:             "│",
		LeftSeparator:    "├",
		MiddleHorizontal: "─",
		MiddleSeparator:  "┼",
		MiddleVertical:   "│",
		PaddingLeft:      " ",
		PaddingRight:     " ",
		PageSeparator:    "\n",
		Right:            "│",
		RightSeparator:   "┤",
		TopLeft:          "╭",
		TopRight:         "╮",
		TopSeparator:     "┬",
		UnfinishedRow:    " ≈",
	}

	// styleBoxTest defines a Boxed-Table like below:
	//  (-----^------------^-----------^--------^-----------------------------)
	//  [<  #>|<FIRST NAME>|<LAST NAME>|<SALARY>|<                           >]
	//  {-----+------------+-----------+--------+-----------------------------}
	//  [<  1>|<Arya      >|<Stark    >|<  3000>|<                           >]
	//  [< 20>|<Jon       >|<Snow     >|<  2000>|<You know nothing, Jon Snow!>]
	//  [<300>|<Tyrion    >|<Lannister>|<  5000>|<                           >]
	//  {-----+------------+-----------+--------+-----------------------------}
	//  [<   >|<          >|<TOTAL    >|< 10000>|<                           >]
	//  \-----v------------v-----------v--------v-----------------------------/
	styleBoxTest = BoxStyle{
		BottomLeft:       "\\",
		BottomRight:      "/",
		BottomSeparator:  "v",
		EmptySeparator:   text.RepeatAndTrim(" ", text.RuneWidthWithoutEscSequences("+")),
		Left:             "[",
		LeftSeparator:    "{",
		MiddleHorizontal: "--",
		MiddleSeparator:  "+",
		MiddleVertical:   "|",
		PaddingLeft:      "<",
		PaddingRight:     ">",
		PageSeparator:    "\n",
		Right:            "]",
		RightSeparator:   "}",
		TopLeft:          "(",
		TopRight:         ")",
		TopSeparator:     "^",
		UnfinishedRow:    " ~~~",
	}
)

// ColorOptions defines the ANSI colors to use for parts of the Table.
type ColorOptions struct {
	Border       text.Colors // borders (if nil, uses one of the below)
	Footer       text.Colors // footer row(s) colors
	Header       text.Colors // header row(s) colors
	IndexColumn  text.Colors // index-column colors (row #, etc.)
	Row          text.Colors // regular row(s) colors
	RowAlternate text.Colors // regular row(s) colors for the even-numbered rows
	Separator    text.Colors // separators (if nil, uses one of the above)
}

var (
	// ColorOptionsDefault defines sensible ANSI color options - basically NONE.
	ColorOptionsDefault = ColorOptions{}

	// ColorOptionsBright renders dark text on bright background.
	ColorOptionsBright = ColorOptionsBlackOnCyanWhite

	// ColorOptionsDark renders bright text on dark background.
	ColorOptionsDark = ColorOptionsCyanWhiteOnBlack

	// ColorOptionsBlackOnBlueWhite renders Black text on Blue/White background.
	ColorOptionsBlackOnBlueWhite = ColorOptions{
		Footer:       text.Colors{text.BgBlue, text.FgBlack},
		Header:       text.Colors{text.BgHiBlue, text.FgBlack},
		IndexColumn:  text.Colors{text.BgHiBlue, text.FgBlack},
		Row:          text.Colors{text.BgHiWhite, text.FgBlack},
		RowAlternate: text.Colors{text.BgWhite, text.FgBlack},
	}

	// ColorOptionsBlackOnCyanWhite renders Black text on Cyan/White background.
	ColorOptionsBlackOnCyanWhite = ColorOptions{
		Footer:       text.Colors{text.BgCyan, text.FgBlack},
		Header:       text.Colors{text.BgHiCyan, text.FgBlack},
		IndexColumn:  text.Colors{text.BgHiCyan, text.FgBlack},
		Row:          text.Colors{text.BgHiWhite, text.FgBlack},
		RowAlternate: text.Colors{text.BgWhite, text.FgBlack},
	}

	// ColorOptionsBlackOnGreenWhite renders Black text on Green/White
	// background.
	ColorOptionsBlackOnGreenWhite = ColorOptions{
		Footer:       text.Colors{text.BgGreen, text.FgBlack},
		Header:       text.Colors{text.BgHiGreen, text.FgBlack},
		IndexColumn:  text.Colors{text.BgHiGreen, text.FgBlack},
		Row:          text.Colors{text.BgHiWhite, text.FgBlack},
		RowAlternate: text.Colors{text.BgWhite, text.FgBlack},
	}

	// ColorOptionsBlackOnMagentaWhite renders Black text on Magenta/White
	// background.
	ColorOptionsBlackOnMagentaWhite = ColorOptions{
		Footer:       text.Colors{text.BgMagenta, text.FgBlack},
		Header:       text.Colors{text.BgHiMagenta, text.FgBlack},
		IndexColumn:  text.Colors{text.BgHiMagenta, text.FgBlack},
		Row:          text.Colors{text.BgHiWhite, text.FgBlack},
		RowAlternate: text.Colors{text.BgWhite, text.FgBlack},
	}

	// ColorOptionsBlackOnRedWhite renders Black text on Red/White background.
	ColorOptionsBlackOnRedWhite = ColorOptions{
		Footer:       text.Colors{text.BgRed, text.FgBlack},
		Header:       text.Colors{text.BgHiRed, text.FgBlack},
		IndexColumn:  text.Colors{text.BgHiRed, text.FgBlack},
		Row:          text.Colors{text.BgHiWhite, text.FgBlack},
		RowAlternate: text.Colors{text.BgWhite, text.FgBlack},
	}

	// ColorOptionsBlackOnYellowWhite renders Black text on Yellow/White
	// background.
	ColorOptionsBlackOnYellowWhite = ColorOptions{
		Footer:       text.Colors{text.BgYellow, text.FgBlack},
		Header:       text.Colors{text.BgHiYellow, text.FgBlack},
		IndexColumn:  text.Colors{text.BgHiYellow, text.FgBlack},
		Row:          text.Colors{text.BgHiWhite, text.FgBlack},
		RowAlternate: text.Colors{text.BgWhite, text.FgBlack},
	}

	// ColorOptionsBlueWhiteOnBlack renders Blue/White text on Black background.
	ColorOptionsBlueWhiteOnBlack = ColorOptions{
		Footer:       text.Colors{text.FgBlue, text.BgHiBlack},
		Header:       text.Colors{text.FgHiBlue, text.BgHiBlack},
		IndexColumn:  text.Colors{text.FgHiBlue, text.BgHiBlack},
		Row:          text.Colors{text.FgHiWhite, text.BgBlack},
		RowAlternate: text.Colors{text.FgWhite, text.BgBlack},
	}

	// ColorOptionsCyanWhiteOnBlack renders Cyan/White text on Black background.
	ColorOptionsCyanWhiteOnBlack = ColorOptions{
		Footer:       text.Colors{text.FgCyan, text.BgHiBlack},
		Header:       text.Colors{text.FgHiCyan, text.BgHiBlack},
		IndexColumn:  text.Colors{text.FgHiCyan, text.BgHiBlack},
		Row:          text.Colors{text.FgHiWhite, text.BgBlack},
		RowAlternate: text.Colors{text.FgWhite, text.BgBlack},
	}

	// ColorOptionsGreenWhiteOnBlack renders Green/White text on Black
	// background.
	ColorOptionsGreenWhiteOnBlack = ColorOptions{
		Footer:       text.Colors{text.FgGreen, text.BgHiBlack},
		Header:       text.Colors{text.FgHiGreen, text.BgHiBlack},
		IndexColumn:  text.Colors{text.FgHiGreen, text.BgHiBlack},
		Row:          text.Colors{text.FgHiWhite, text.BgBlack},
		RowAlternate: text.Colors{text.FgWhite, text.BgBlack},
	}

	// ColorOptionsMagentaWhiteOnBlack renders Magenta/White text on Black
	// background.
	ColorOptionsMagentaWhiteOnBlack = ColorOptions{
		Footer:       text.Colors{text.FgMagenta, text.BgHiBlack},
		Header:       text.Colors{text.FgHiMagenta, text.BgHiBlack},
		IndexColumn:  text.Colors{text.FgHiMagenta, text.BgHiBlack},
		Row:          text.Colors{text.FgHiWhite, text.BgBlack},
		RowAlternate: text.Colors{text.FgWhite, text.BgBlack},
	}

	// ColorOptionsRedWhiteOnBlack renders Red/White text on Black background.
	ColorOptionsRedWhiteOnBlack = ColorOptions{
		Footer:       text.Colors{text.FgRed, text.BgHiBlack},
		Header:       text.Colors{text.FgHiRed, text.BgHiBlack},
		IndexColumn:  text.Colors{text.FgHiRed, text.BgHiBlack},
		Row:          text.Colors{text.FgHiWhite, text.BgBlack},
		RowAlternate: text.Colors{text.FgWhite, text.BgBlack},
	}

	// ColorOptionsYellowWhiteOnBlack renders Yellow/White text on Black
	// background.
	ColorOptionsYellowWhiteOnBlack = ColorOptions{
		Footer:       text.Colors{text.FgYellow, text.BgHiBlack},
		Header:       text.Colors{text.FgHiYellow, text.BgHiBlack},
		IndexColumn:  text.Colors{text.FgHiYellow, text.BgHiBlack},
		Row:          text.Colors{text.FgHiWhite, text.BgBlack},
		RowAlternate: text.Colors{text.FgWhite, text.BgBlack},
	}
)

// FormatOptions defines the text-formatting to perform on parts of the Table.
type FormatOptions struct {
	Direction text.Direction // (forced) BiDi direction for each Column
	Footer    text.Format    // footer row(s) text format
	Header    text.Format    // header row(s) text format
	Row       text.Format    // (data) row(s) text format
}

// FormatOptionsDefault defines sensible formatting options.
var FormatOptionsDefault = FormatOptions{
	Footer: text.FormatUpper,
	Header: text.FormatUpper,
	Row:    text.FormatDefault,
}

// HTMLOptions defines the global options to control HTML rendering.
type HTMLOptions struct {
	CSSClass    string // CSS class to set on the overall <table> tag
	EmptyColumn string // string to replace "" columns with (entire content being "")
	EscapeText  bool   // escape text into HTML-safe content?
	Newline     string // string to replace "\n" characters with
}

// DefaultHTMLOptions defines sensible HTML rendering defaults.
var DefaultHTMLOptions = HTMLOptions{
	CSSClass:    DefaultHTMLCSSClass,
	EmptyColumn: "&nbsp;",
	EscapeText:  true,
	Newline:     "<br/>",
}

// Options defines the global options that determine how the Table is
// rendered.
type Options struct {
	// DoNotColorBordersAndSeparators disables coloring all the borders and row
	// or column separators.
	DoNotColorBordersAndSeparators bool

	// DrawBorder enables or disables drawing the border around the Table.
	// Example of a table where it is disabled:
	//     # │ FIRST NAME │ LAST NAME │ SALARY │
	//  ─────┼────────────┼───────────┼────────┼─────────────────────────────
	//     1 │ Arya       │ Stark     │   3000 │
	//    20 │ Jon        │ Snow      │   2000 │ You know nothing, Jon Snow!
	//   300 │ Tyrion     │ Lannister │   5000 │
	//  ─────┼────────────┼───────────┼────────┼─────────────────────────────
	//       │            │ TOTAL     │  10000 │
	DrawBorder bool

	// SeparateColumns enables or disable drawing border between columns.
	// Example of a table where it is disabled:
	//  ┌─────────────────────────────────────────────────────────────────┐
	//  │   #  FIRST NAME  LAST NAME  SALARY                              │
	//  ├─────────────────────────────────────────────────────────────────┤
	//  │   1  Arya        Stark        3000                              │
	//  │  20  Jon         Snow         2000  You know nothing, Jon Snow! │
	//  │ 300  Tyrion      Lannister    5000                              │
	//  │                  TOTAL       10000                              │
	//  └─────────────────────────────────────────────────────────────────┘
	SeparateColumns bool

	// SeparateFooter enables or disable drawing border between the footer and
	// the rows. Example of a table where it is disabled:
	//  ┌─────┬────────────┬───────────┬────────┬─────────────────────────────┐
	//  │   # │ FIRST NAME │ LAST NAME │ SALARY │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │   1 │ Arya       │ Stark     │   3000 │                             │
	//  │  20 │ Jon        │ Snow      │   2000 │ You know nothing, Jon Snow! │
	//  │ 300 │ Tyrion     │ Lannister │   5000 │                             │
	//  │     │            │ TOTAL     │  10000 │                             │
	//  └─────┴────────────┴───────────┴────────┴─────────────────────────────┘
	SeparateFooter bool

	// SeparateHeader enables or disable drawing border between the header and
	// the rows. Example of a table where it is disabled:
	//  ┌─────┬────────────┬───────────┬────────┬─────────────────────────────┐
	//  │   # │ FIRST NAME │ LAST NAME │ SALARY │                             │
	//  │   1 │ Arya       │ Stark     │   3000 │                             │
	//  │  20 │ Jon        │ Snow      │   2000 │ You know nothing, Jon Snow! │
	//  │ 300 │ Tyrion     │ Lannister │   5000 │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │     │            │ TOTAL     │  10000 │                             │
	//  └─────┴────────────┴───────────┴────────┴─────────────────────────────┘
	SeparateHeader bool

	// SeparateRows enables or disables drawing separators between each row.
	// Example of a table where it is enabled:
	//  ┌─────┬────────────┬───────────┬────────┬─────────────────────────────┐
	//  │   # │ FIRST NAME │ LAST NAME │ SALARY │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │   1 │ Arya       │ Stark     │   3000 │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │  20 │ Jon        │ Snow      │   2000 │ You know nothing, Jon Snow! │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │ 300 │ Tyrion     │ Lannister │   5000 │                             │
	//  ├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
	//  │     │            │ TOTAL     │  10000 │                             │
	//  └─────┴────────────┴───────────┴────────┴─────────────────────────────┘
	SeparateRows bool
}

var (
	// OptionsDefault defines sensible global options.
	OptionsDefault = Options{
		DrawBorder:      true,
		SeparateColumns: true,
		SeparateFooter:  true,
		SeparateHeader:  true,
		SeparateRows:    false,
	}

	// OptionsNoBorders sets up a table without any borders.
	OptionsNoBorders = Options{
		DrawBorder:      false,
		SeparateColumns: true,
		SeparateFooter:  true,
		SeparateHeader:  true,
		SeparateRows:    false,
	}

	// OptionsNoBordersAndSeparators sets up a table without any borders or
	// separators.
	OptionsNoBordersAndSeparators = Options{
		DrawBorder:      false,
		SeparateColumns: false,
		SeparateFooter:  false,
		SeparateHeader:  false,
		SeparateRows:    false,
	}
)

// TitleOptions defines the way the title text is to be rendered.
type TitleOptions struct {
	Align  text.Align
	Colors text.Colors
	Format text.Format
}

var (
	// TitleOptionsDefault defines sensible title options - basically NONE.
	TitleOptionsDefault = TitleOptions{}

	// TitleOptionsBright renders Bright Bold text on Dark background.
	TitleOptionsBright = TitleOptionsBlackOnCyan

	// TitleOptionsDark renders Dark Bold text on Bright background.
	TitleOptionsDark = TitleOptionsCyanOnBlack

	// TitleOptionsBlackOnBlue renders Black text on Blue background.
	TitleOptionsBlackOnBlue = TitleOptions{
		Colors: append(ColorOptionsBlackOnBlueWhite.Header, text.Bold),
	}

	// TitleOptionsBlackOnCyan renders Black Bold text on Cyan background.
	TitleOptionsBlackOnCyan = TitleOptions{
		Colors: append(ColorOptionsBlackOnCyanWhite.Header, text.Bold),
	}

	// TitleOptionsBlackOnGreen renders Black Bold text onGreen background.
	TitleOptionsBlackOnGreen = TitleOptions{
		Colors: append(ColorOptionsBlackOnGreenWhite.Header, text.Bold),
	}

	// TitleOptionsBlackOnMagenta renders Black Bold text on Magenta background.
	TitleOptionsBlackOnMagenta = TitleOptions{
		Colors: append(ColorOptionsBlackOnMagentaWhite.Header, text.Bold),
	}

	// TitleOptionsBlackOnRed renders Black Bold text on Red background.
	TitleOptionsBlackOnRed = TitleOptions{
		Colors: append(ColorOptionsBlackOnRedWhite.Header, text.Bold),
	}

	// TitleOptionsBlackOnYellow renders Black Bold text on Yellow background.
	TitleOptionsBlackOnYellow = TitleOptions{
		Colors: append(ColorOptionsBlackOnYellowWhite.Header, text.Bold),
	}

	// TitleOptionsBlueOnBlack renders Blue Bold text on Black background.
	TitleOptionsBlueOnBlack = TitleOptions{
		Colors: append(ColorOptionsBlueWhiteOnBlack.Header, text.Bold),
	}

	// TitleOptionsCyanOnBlack renders Cyan Bold text on Black background.
	TitleOptionsCyanOnBlack = TitleOptions{
		Colors: append(ColorOptionsCyanWhiteOnBlack.Header, text.Bold),
	}

	// TitleOptionsGreenOnBlack renders Green Bold text on Black background.
	TitleOptionsGreenOnBlack = TitleOptions{
		Colors: append(ColorOptionsGreenWhiteOnBlack.Header, text.Bold),
	}

	// TitleOptionsMagentaOnBlack renders Magenta Bold text on Black background.
	TitleOptionsMagentaOnBlack = TitleOptions{
		Colors: append(ColorOptionsMagentaWhiteOnBlack.Header, text.Bold),
	}

	// TitleOptionsRedOnBlack renders Red Bold text on Black background.
	TitleOptionsRedOnBlack = TitleOptions{
		Colors: append(ColorOptionsRedWhiteOnBlack.Header, text.Bold),
	}

	// TitleOptionsYellowOnBlack renders Yellow Bold text on Black background.
	TitleOptionsYellowOnBlack = TitleOptions{
		Colors: append(ColorOptionsYellowWhiteOnBlack.Header, text.Bold),
	}
)
