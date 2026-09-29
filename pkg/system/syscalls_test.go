package system

import "testing"

func TestCallNumTablesAreConsistent(t *testing.T) {
	if !callNumTableIsOkX86Family64() {
		t.Error("x86_64 syscall table does not match its max number and last name")
	}
	if !callNumTableIsOkX86Family32() {
		t.Error("i386 syscall table does not match its max number and last name")
	}
}

func TestCallNameX86Family64(t *testing.T) {
	// Syscalls added after clone3, which is where the table used to stop
	tests := map[uint32]string{
		435: "clone3",
		436: "close_range",
		437: "openat2",
		439: "faccessat2",
		449: "futex_waitv",
		452: "fchmodat2",
		467: "open_tree_attr",
	}

	for num, name := range tests {
		if got := callNameX86Family64(num); got != name {
			t.Errorf("callNameX86Family64(%d) = %q, want %q", num, got, name)
		}
	}

	if got := callNameX86Family64(SyscallX86MaxNum64 + 1); got != SyscallX86UnknownName {
		t.Errorf("callNameX86Family64 past the end = %q, want %q", got, SyscallX86UnknownName)
	}
}

func TestCallNumberX86Family64RoundTrip(t *testing.T) {
	for _, name := range []string{"close_range", "openat2", "futex_waitv", "open_tree_attr"} {
		num, ok := callNumberX86Family64(name)
		if !ok {
			t.Fatalf("callNumberX86Family64(%q) not found", name)
		}
		if got := callNameX86Family64(num); got != name {
			t.Errorf("round trip for %q gave %q", name, got)
		}
	}
}
