# Table
[![Go Reference](https://pkg.go.dev/badge/github.com/jedib0t/go-pretty/v6/table.svg)](https://pkg.go.dev/github.com/jedib0t/go-pretty/v6/table)

Pretty-print tables into ASCII/Unicode strings.

  - Add Rows one-by-one or as a group (`AppendRow`/`AppendRows`)
  - Add Header(s) and Footer(s) (`AppendHeader`/`AppendFooter`)
  - Add a Separator manually after any Row (`AppendSeparator`)
  - Auto Index Rows (1, 2, 3 ...) and Columns (A, B, C, ...) (`SetAutoIndex`)
  - Auto Merge
    - Cells in a Row (`RowConfig.AutoMerge`)
    - Columns (`ColumnConfig.AutoMerge`)
  - Limit the length of
    - Rows (`SetAllowedRowLength`)
    - Columns (`ColumnConfig.Width*`)
  - Page results by a specified number of Lines (`SetPageSize`)
  - Alignment - Horizontal & Vertical
    - Auto (horizontal) Align (numeric columns aligned Right)
    - Custom (horizontal) Align per column (`ColumnConfig.Align*`)
    - Custom (vertical) VAlign per column with multi-line cell support (`ColumnConfig.VAlign*`)
  - Mirror output to an `io.Writer` (ex. `os.StdOut`) (`SetOutputMirror`)
  - Sort by one or more Columns (`SortBy`)
  - Suppress/hide columns with no content (`SuppressEmptyColumns`) 
  - Customizable Cell rendering per Column (`ColumnConfig.Transformer*`)
  - Hide any columns that you don't want displayed (`ColumnConfig.Hidden`)
  - Reset Headers/Rows/Footers at will to reuse the same Table Writer (`Reset*`)
  - Completely customizable styles (`SetStyle`/`Style`)
    - Many ready-to-use styles: [style.go](style.go)
    - Colorize Headers/Body/Footers using [../text/color.go](../text/color.go)
    - Custom text-case for Headers/Body/Footers
    - Enable separators between each row
    - Render table without a Border
    - and a lot more...
  - Render as:
    - (ASCII/Unicode) Table
    - CSV
    - HTML Table (with custom CSS Class)
    - Markdown Table

```
+---------------------------------------------------------------------+
| Game of Thrones                                                     +
+-----+------------+-----------+--------+-----------------------------+
|   # | FIRST NAME | LAST NAME | SALARY |                             |
+-----+------------+-----------+--------+-----------------------------+
|   1 | Arya       | Stark     |   3000 |                             |
|  20 | Jon        | Snow      |   2000 | You know nothing, Jon Snow! |
| 300 | Tyrion     | Lannister |   5000 |                             |
+-----+------------+-----------+--------+-----------------------------+
|     |            | TOTAL     |  10000 |                             |
+-----+------------+-----------+--------+-----------------------------+
```

A demonstration of all the capabilities can be found here:
[../cmd/demo-table](../cmd/demo-table)

If you want very specific examples, read ahead.

**Hint**: I've tried to ensure that almost all supported use-cases are covered
by unit-tests and that they print the table rendered. Run
`go test -v github.com/jedib0t/go-pretty/v6/table` to see the test outputs and
help you figure out how to do something.

# Examples

All the examples below are going to start with the following block, although
nothing except a single Row is mandatory for the `Render()` function to render
something:
```golang
package main

import (
    "os"

    "github.com/jedib0t/go-pretty/v6/table"
)

func main() {
    t := table.NewWriter()
    t.SetOutputMirror(os.Stdout)
    t.AppendHeader(table.Row{"#", "First Name", "Last Name", "Salary"})
    t.AppendRows([]table.Row{
        {1, "Arya", "Stark", 3000},
        {20, "Jon", "Snow", 2000, "You know nothing, Jon Snow!"},
    })
    t.AppendSeparator()
    t.AppendRow([]interface{}{300, "Tyrion", "Lannister", 5000})
    t.AppendFooter(table.Row{"", "", "Total", 10000})
    t.Render()
}
```
Running the above will result in:
```
+-----+------------+-----------+--------+-----------------------------+
|   # | FIRST NAME | LAST NAME | SALARY |                             |
+-----+------------+-----------+--------+-----------------------------+
|   1 | Arya       | Stark     |   3000 |                             |
|  20 | Jon        | Snow      |   2000 | You know nothing, Jon Snow! |
+-----+------------+-----------+--------+-----------------------------+
| 300 | Tyrion     | Lannister |   5000 |                             |
+-----+------------+-----------+--------+-----------------------------+
|     |            | TOTAL     |  10000 |                             |
+-----+------------+-----------+--------+-----------------------------+
```

## Styles

You can customize almost every single thing about the table above. The previous
example just defaulted to `StyleDefault` during `Render()`. You can use a
ready-to-use style (as in [style.go](style.go)) or customize it as you want.

### Ready-to-use Styles

Table comes with a bunch of ready-to-use Styles that make the table look really
good. Set or Change the style using:
```golang
    t.SetStyle(table.StyleLight)
    t.Render()
```
to get:
```
┌─────┬────────────┬───────────┬────────┬─────────────────────────────┐
│   # │ FIRST NAME │ LAST NAME │ SALARY │                             │
├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
│   1 │ Arya       │ Stark     │   3000 │                             │
│  20 │ Jon        │ Snow      │   2000 │ You know nothing, Jon Snow! │
├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
│ 300 │ Tyrion     │ Lannister │   5000 │                             │
├─────┼────────────┼───────────┼────────┼─────────────────────────────┤
│     │            │ TOTAL     │  10000 │                             │
└─────┴────────────┴───────────┴────────┴─────────────────────────────┘
```

Or if you want to use a full-color mode, and don't care for boxes, use:
```golang
    t.SetStyle(table.StyleColoredBright)
    t.Render()
```
to get:

<img src="images/table-StyleColoredBright.png" width="640px" alt="Colored Table"/>

### Roll your own Style

You can also roll your own style:
```golang
    t.SetStyle(table.Style{
        Name: "myNewStyle",
        Box: table.BoxStyle{
            BottomLeft:       "\\",
            BottomRight:      "/",
            BottomSeparator:  "v",
            Left:             "[",
            LeftSeparator:    "{",
            MiddleHorizontal: "-",
            MiddleSeparator:  "+",
            MiddleVertical:   "|",
            PaddingLeft:      "<",
            PaddingRight:     ">",
            Right:            "]",
            RightSeparator:   "}",
            TopLeft:          "(",
            TopRight:         ")",
            TopSeparator:     "^",
            UnfinishedRow:    " ~~~",
        },
        Color: table.ColorOptions{
            IndexColumn:     text.Colors{text.BgCyan, text.FgBlack},
            Footer:          text.Colors{text.BgCyan, text.FgBlack},
            Header:          text.Colors{text.BgHiCyan, text.FgBlack},
            Row:             text.Colors{text.BgHiWhite, text.FgBlack},
            RowAlternate:    text.Colors{text.BgWhite, text.FgBlack},
        },
        Format: table.FormatOptions{
            Footer: text.FormatUpper,
            Header: text.FormatUpper,
            Row:    text.FormatDefault,
        },
        Options: table.Options{
            DrawBorder:      true,
            SeparateColumns: true,
            SeparateFooter:  true,
            SeparateHeader:  true,
            SeparateRows:    false,
        },
    })
```

Or you can use one of the ready-to-use Styles, and just make a few tweaks:
```golang
    t.SetStyle(table.StyleLight)
    t.Style().Color.Header = text.Colors{text.BgHiCyan, text.FgBlack}
    t.Style().Color.IndexColumn = text.Colors{text.BgHiCyan, text.FgBlack}
    t.Style().Format.Footer = text.FormatLower
    t.Style().Options.DrawBorder = false
```

## Auto-Merge

You can auto-merge cells horizontally and vertically, but you have request for
it specifically for each row/column using `RowConfig` or `ColumnConfig`.

```golang
    rowConfigAutoMerge := table.RowConfig{AutoMerge: true}

    t := table.NewWriter()
    t.AppendHeader(table.Row{"Node IP", "Pods", "Namespace", "Container", "RCE", "RCE"}, rowConfigAutoMerge)
    t.AppendHeader(table.Row{"", "", "", "", "EXE", "RUN"})
    t.AppendRow(table.Row{"1.1.1.1", "Pod 1A", "NS 1A", "C 1", "Y", "Y"}, rowConfigAutoMerge)
    t.AppendRow(table.Row{"1.1.1.1", "Pod 1A", "NS 1A", "C 2", "Y", "N"}, rowConfigAutoMerge)
    t.AppendRow(table.Row{"1.1.1.1", "Pod 1A", "NS 1B", "C 3", "N", "N"}, rowConfigAutoMerge)
    t.AppendRow(table.Row{"1.1.1.1", "Pod 1B", "NS 2", "C 4", "N", "N"}, rowConfigAutoMerge)
    t.AppendRow(table.Row{"1.1.1.1", "Pod 1B", "NS 2", "C 5", "Y", "N"}, rowConfigAutoMerge)
    t.AppendRow(table.Row{"2.2.2.2", "Pod 2", "NS 3", "C 6", "Y", "Y"}, rowConfigAutoMerge)
    t.AppendRow(table.Row{"2.2.2.2", "Pod 2", "NS 3", "C 7", "Y", "Y"}, rowConfigAutoMerge)
    t.AppendFooter(table.Row{"", "", "", 7, 5, 3})
    t.SetAutoIndex(true)
    t.SetColumnConfigs([]table.ColumnConfig{
        {Number: 1, AutoMerge: true},
        {Number: 2, AutoMerge: true},
        {Number: 3, AutoMerge: true},
        {Number: 4, AutoMerge: true},
        {Number: 5, Align: text.AlignCenter, AlignFooter: text.AlignCenter, AlignHeader: text.AlignCenter},
        {Number: 6, Align: text.AlignCenter, AlignFooter: text.AlignCenter, AlignHeader: text.AlignCenter},
    })
    t.SetOutputMirror(os.Stdout)
    t.SetStyle(table.StyleLight)
    t.Style().Options.SeparateRows = true
    fmt.Println(t.Render())
```
to get:
```
┌───┬─────────┬────────┬───────────┬───────────┬───────────┐
│   │ NODE IP │ PODS   │ NAMESPACE │ CONTAINER │    RCE    │
│   │         │        │           │           ├─────┬─────┤
│   │         │        │           │           │ EXE │ RUN │
├───┼─────────┼────────┼───────────┼───────────┼─────┴─────┤
│ 1 │ 1.1.1.1 │ Pod 1A │ NS 1A     │ C 1       │     Y     │
├───┤         │        │           ├───────────┼─────┬─────┤
│ 2 │         │        │           │ C 2       │  Y  │  N  │
├───┤         │        ├───────────┼───────────┼─────┴─────┤
│ 3 │         │        │ NS 1B     │ C 3       │     N     │
├───┤         ├────────┼───────────┼───────────┼───────────┤
│ 4 │         │ Pod 1B │ NS 2      │ C 4       │     N     │
├───┤         │        │           ├───────────┼─────┬─────┤
│ 5 │         │        │           │ C 5       │  Y  │  N  │
├───┼─────────┼────────┼───────────┼───────────┼─────┴─────┤
│ 6 │ 2.2.2.2 │ Pod 2  │ NS 3      │ C 6       │     Y     │
├───┤         │        │           ├───────────┼───────────┤
│ 7 │         │        │           │ C 7       │     Y     │
├───┼─────────┼────────┼───────────┼───────────┼─────┬─────┤
│   │         │        │           │ 7         │  5  │  3  │
└───┴─────────┴────────┴───────────┴───────────┴─────┴─────┘
```

## Paging

You can limit then number of lines rendered in a single "Page". This logic
can handle rows with multiple lines too. Here is a simple example:
```golang
    t.SetPageSize(1)
    t.Render()
```
to get:
```
+-----+------------+-----------+--------+-----------------------------+
|   # | FIRST NAME | LAST NAME | SALARY |                             |
+-----+------------+-----------+--------+-----------------------------+
|   1 | Arya       | Stark     |   3000 |                             |
+-----+------------+-----------+--------+-----------------------------+
|     |            | TOTAL     |  10000 |                             |
+-----+------------+-----------+--------+-----------------------------+

+-----+------------+-----------+--------+-----------------------------+
|   # | FIRST NAME | LAST NAME | SALARY |                             |
+-----+------------+-----------+--------+-----------------------------+
|  20 | Jon        | Snow      |   2000 | You know nothing, Jon Snow! |
+-----+------------+-----------+--------+-----------------------------+
|     |            | TOTAL     |  10000 |                             |
+-----+------------+-----------+--------+-----------------------------+

+-----+------------+-----------+--------+-----------------------------+
|   # | FIRST NAME | LAST NAME | SALARY |                             |
+-----+------------+-----------+--------+-----------------------------+
| 300 | Tyrion     | Lannister |   5000 |                             |
+-----+------------+-----------+--------+-----------------------------+
|     |            | TOTAL     |  10000 |                             |
+-----+------------+-----------+--------+-----------------------------+
```

## Sorting

Sorting can be done on one or more columns. The following code will make the
rows be sorted first by "First Name" and then by "Last Name" (in case of similar
"First Name" entries).
```golang
    t.SortBy([]table.SortBy{
	    {Name: "First Name", Mode: table.Asc},
	    {Name: "Last Name", Mode: table.Asc},
    })
```

## Wrapping (or) Row/Column Width restrictions

You can restrict the maximum (text) width for a Row:
```golang
    t.SetAllowedRowLength(50)
    t.Render()
```
to get:
```
+-----+------------+-----------+--------+------- ~
|   # | FIRST NAME | LAST NAME | SALARY |        ~
+-----+------------+-----------+--------+------- ~
|   1 | Arya       | Stark     |   3000 |        ~
|  20 | Jon        | Snow      |   2000 | You kn ~
+-----+------------+-----------+--------+------- ~
| 300 | Tyrion     | Lannister |   5000 |        ~
+-----+------------+-----------+--------+------- ~
|     |            | TOTAL     |  10000 |        ~
+-----+------------+-----------+--------+------- ~
```

## Column Control - Alignment, Colors, Width and more

You can control a lot of things about individual cells/columns which overrides
global properties/styles using the `SetColumnConfig()` interface:
- Alignment (horizontal & vertical)
- Colorization
- Transform individual cells based on the content
- Visibility
- Width (minimum & maximum)

```golang
    nameTransformer := text.Transformer(func(val interface{}) string {
        return text.Bold.Sprint(val)
    })

    t.SetColumnConfigs([]ColumnConfig{
        {
            Name:              "First Name",
            Align:             text.AlignLeft,
            AlignFooter:       text.AlignLeft,
            AlignHeader:       text.AlignLeft,
            Colors:            text.Colors{text.BgBlack, text.FgRed},
            ColorsHeader:      text.Colors{text.BgRed, text.FgBlack, text.Bold},
            ColorsFooter:      text.Colors{text.BgRed, text.FgBlack},
            Hidden:            false,
            Transformer:       nameTransformer,
            TransformerFooter: nameTransformer,
            TransformerHeader: nameTransformer,
            VAlign:            text.VAlignMiddle,
            VAlignFooter:      text.VAlignTop,
            VAlignHeader:      text.VAlignBottom,
            WidthMin:          6,
            WidthMax:          64,
        }
    })
```

## Render As ...

Tables can be rendered in other common formats such as:

### ... CSV

```golang
    t.RenderCSV()
```
to get:
```
,First Name,Last Name,Salary,
1,Arya,Stark,3000,
20,Jon,Snow,2000,"You know nothing\, Jon Snow!"
300,Tyrion,Lannister,5000,
,,Total,10000,
```

### ... HTML Table

```golang
    t.Style().HTML = table.HTMLOptions{
        CSSClass:    "game-of-thrones",
        EmptyColumn: "&nbsp;",
        EscapeText:  true,
        Newline:     "<br/>",
    }
    t.RenderHTML()
```
to get:
```html
<table class="game-of-thrones">
  <thead>
  <tr>
    <th align="right">#</th>
    <th>First Name</th>
    <th>Last Name</th>
    <th align="right">Salary</th>
    <th>&nbsp;</th>
  </tr>
  </thead>
  <tbody>
  <tr>
    <td align="right">1</td>
    <td>Arya</td>
    <td>Stark</td>
    <td align="right">3000</td>
    <td>&nbsp;</td>
  </tr>
  <tr>
    <td align="right">20</td>
    <td>Jon</td>
    <td>Snow</td>
    <td align="right">2000</td>
    <td>You know nothing, Jon Snow!</td>
  </tr>
  <tr>
    <td align="right">300</td>
    <td>Tyrion</td>
    <td>Lannister</td>
    <td align="right">5000</td>
    <td>&nbsp;</td>
  </tr>
  </tbody>
  <tfoot>
  <tr>
    <td align="right">&nbsp;</td>
    <td>&nbsp;</td>
    <td>Total</td>
    <td align="right">10000</td>
    <td>&nbsp;</td>
  </tr>
  </tfoot>
</table>
```

### ... Markdown Table

```golang
    t.RenderMarkdown()
```
to get:
```markdown
| # | First Name | Last Name | Salary |  |
| ---:| --- | --- | ---:| --- |
| 1 | Arya | Stark | 3000 |  |
| 20 | Jon | Snow | 2000 | You know nothing, Jon Snow! |
| 300 | Tyrion | Lannister | 5000 |  |
|  |  | Total | 10000 |  |
```
