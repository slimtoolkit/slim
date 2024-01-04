package text

import "fmt"

func Hyperlink(url, text string) string {
	if url == "" {
		return text
	}
	if text == "" {
		return url
	}
	// source https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}
