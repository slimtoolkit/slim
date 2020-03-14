# Change Log

## v0.3.0 (2018/??/??)

next release.

## v0.2.3 (2018/10/25)

### What's new?

* Add `prompt.FuzzyFilter` for fuzzy matching at [#92](https://github.com/c-bata/go-prompt/pull/92).
* Add `OptionShowCompletionAtStart` to show completion at start at [#100](https://github.com/c-bata/go-prompt/pull/100).
* Add `prompt.NewStderrWriter` at [#102](https://github.com/c-bata/go-prompt/pull/102).

### Fixed

* Fix resetting display attributes (please see [pull #104](https://github.com/c-bata/go-prompt/pull/104) for more details).
* Fix error handling of Flush function in ConsoleWriter (please see [pull #97](https://github.com/c-bata/go-prompt/pull/97) for more details).
* Fix panic problem when reading from stdin before starting the prompt (please see [issue #88](https://github.com/c-bata/go-prompt/issues/88) for more details).

### Removed or Deprecated

* `prompt.NewStandardOutputWriter` is deprecated. Please use `prompt.NewStdoutWriter`.

## v0.2.2 (2018/06/28)

### What's new?

* Support CJK(Chinese, Japanese and Korean) and Cyrillic characters.
* Add OptionCompletionWordSeparator(x string) to customize insertion points for completions.
    * To support this, text query functions by arbitrary word separator are added in Document (please see [here](https://github.com/c-bata/go-prompt/pull/79) for more details).
* Add FilePathCompleter to complete file path on your system.
* Add option to customize ascii code key bindings.
* Add GetWordAfterCursor method in Document.

### Removed or Deprecated

* prompt.Choose shortcut function is deprecated.

## v0.2.1 (2018/02/14)

### What's New?

* ~~It seems that windows support is almost perfect.~~
    * A critical bug is found :( When you change a terminal window size, the layout will be broken because current implementation cannot catch signal for updating window size on Windows.

### Fixed

* Fix a Shift+Tab handling on Windows.
* Fix 4-dimension arrow keys handling on Windows.

## v0.2.0 (2018/02/13)

### What's New?

* Supports scrollbar when there are too many matched suggestions
* Windows support (but please caution because this is still not perfect).
* Add OptionLivePrefix to update the prefix dynamically
* Implement clear screen by `Ctrl+L`.

### Fixed

* Fix the behavior of `Ctrl+W` keybind.
* Fix the panic because when running on a docker container (please see [here](https://github.com/c-bata/go-prompt/pull/32) for details).
* Fix panic when making terminal window small size after input 2 lines of texts. See [here](https://github.com/c-bata/go-prompt/issues/37) for details).
* And also fixed many bugs that layout is broken when using Terminal.app, GNU Terminal and a Goland(IntelliJ).

### News

New core developers are joined (alphabetical order).

* Nao Yonashiro (Github @orisano)
* Ryoma Abe (Github @Allajah)
* Yusuke Nakamura (Github @unasuke)


## v0.1.0 (2017/08/15)

Initial Release
