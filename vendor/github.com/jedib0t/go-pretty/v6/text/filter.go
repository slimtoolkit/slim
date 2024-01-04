package text

// Filter filters the slice 's' to items which return truth when passed to 'f'.
func Filter(s []string, f func(string) bool) []string {
	var out []string
	for _, item := range s {
		if f(item) {
			out = append(out, item)
		}
	}
	return out
}
