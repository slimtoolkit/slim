# go-tty

Simple tty utility

## Usage

```go
tty, err := tty.Open()
if err != nil {
	log.Fatal(err)
}
defer tty.Close()

for {
	r, err := tty.ReadRune()
	if err != nil {
		log.Fatal(err)
	}
	// handle key event
}
```

if you are on windows and want to display ANSI colors, use <a href="https://github.com/mattn/go-colorable">go-colorable</a>.

```go
tty, err := tty.Open()
if err != nil {
	log.Fatal(err)
}
defer tty.Close()

out := colorable.NewColorable(tty.Output())

fmt.Fprintln(out, "\x1b[2J")
```

## Installation

```
$ go get github.com/mattn/go-tty
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
