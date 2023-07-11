// +build gofuzz

package robotstxt

import "testing/quick"

func Fuzz(data []byte) int {
	r, err := FromBytes(data)
	if err != nil {
		if r != nil {
			panic("r != nil on error")
		}
		return 0
	}

	// FindGroup must never return nil
	f1 := func(agent string) bool { return r.FindGroup(agent) != nil }
	if err := quick.Check(f1, nil); err != nil {
		panic(err)
	}

	// just check TestAgent doesn't panic
	f2 := func(path, agent string) bool { r.TestAgent(path, agent); return true }
	if err := quick.Check(f2, nil); err != nil {
		panic(err)
	}

	return 1
}
