package ptrace

import (
	"testing"
)

func TestGetIntVal(t *testing.T) {
	tt := []struct {
		input    uint64
		expected int
	}{
		{input: 0, expected: 0},
		{input: 0xFFFFFFFE, expected: -2},  // ENOENT
		{input: 0xFFFFFFEC, expected: -20}, // ENOTDIR
		{input: 1, expected: 1},
		{input: 0xFFFFFFFF, expected: -1}, // Generic error
	}

	for _, test := range tt {
		result := getIntVal(test.input)
		if result != test.expected {
			t.Errorf("getIntVal(0x%x) = %d, want %d", test.input, result, test.expected)
		}
	}
}

func TestCheckFileSyscallProcessorOKReturnStatus(t *testing.T) {
	processor := &checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{},
	}

	tt := []struct {
		retVal   uint64
		expected bool
		desc     string
	}{
		{
			retVal:   0,
			expected: true,
			desc:     "success (0)",
		},
		{
			retVal:   0xFFFFFFFE, // -2 as uint64
			expected: true,
			desc:     "ENOENT (-2) - file not found, should be tracked",
		},
		{
			retVal:   0xFFFFFFEC, // -20 as uint64
			expected: true,
			desc:     "ENOTDIR (-20) - not a directory, should be tracked",
		},
		{
			retVal:   0xFFFFFFFF, // -1 as uint64
			expected: false,
			desc:     "EPERM (-1) - should not be tracked",
		},
		{
			retVal:   0xFFFFFFFD, // -3 as uint64
			expected: false,
			desc:     "ESRCH (-3) - should not be tracked",
		},
		{
			retVal:   0xFFFFFFED, // -19 as uint64
			expected: false,
			desc:     "ENODEV (-19) - should not be tracked",
		},
		{
			retVal:   1,
			expected: false,
			desc:     "positive return value - should not be tracked",
		},
	}

	for _, test := range tt {
		result := processor.OKReturnStatus(test.retVal)
		if result != test.expected {
			t.Errorf("OKReturnStatus(0x%x) [%s] = %v, want %v",
				test.retVal, test.desc, result, test.expected)
		}
	}
}

func TestCheckFileSyscallProcessorFailedReturnStatus(t *testing.T) {
	processor := &checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{},
	}

	tt := []struct {
		retVal   uint64
		expected bool
		desc     string
	}{
		{
			retVal:   0,
			expected: false,
			desc:     "success (0) - not failed",
		},
		{
			retVal:   0xFFFFFFFE, // -2 (ENOENT)
			expected: true,
			desc:     "ENOENT (-2) - failed",
		},
		{
			retVal:   0xFFFFFFFF, // -1 (EPERM)
			expected: true,
			desc:     "EPERM (-1) - failed",
		},
	}

	for _, test := range tt {
		result := processor.FailedReturnStatus(test.retVal)
		if result != test.expected {
			t.Errorf("FailedReturnStatus(0x%x) [%s] = %v, want %v",
				test.retVal, test.desc, result, test.expected)
		}
	}
}

