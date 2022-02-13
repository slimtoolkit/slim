/*
   Copyright 2020 The Compose Specification Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package types

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParsePortConfig(t *testing.T) {
	testCases := []struct {
		value         string
		expectedError string
		expected      []ServicePortConfig
	}{
		{
			value: "80",
			expected: []ServicePortConfig{
				{
					Protocol: "tcp",
					Target:   80,
					Mode:     "ingress",
				},
			},
		},
		{
			value: "80:8080",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "8080:80/tcp",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    80,
					Published: 8080,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80:8080/udp",
			expected: []ServicePortConfig{
				{
					Protocol:  "udp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-81:8080-8081/tcp",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
				{
					Protocol:  "tcp",
					Target:    8081,
					Published: 81,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-82:8080-8082/udp",
			expected: []ServicePortConfig{
				{
					Protocol:  "udp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
				{
					Protocol:  "udp",
					Target:    8081,
					Published: 81,
					Mode:      "ingress",
				},
				{
					Protocol:  "udp",
					Target:    8082,
					Published: 82,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-82:8080/udp",
			expected: []ServicePortConfig{
				{
					Protocol:  "udp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
				{
					Protocol:  "udp",
					Target:    8080,
					Published: 81,
					Mode:      "ingress",
				},
				{
					Protocol:  "udp",
					Target:    8080,
					Published: 82,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-80:8080/tcp",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
			},
		},
		{
			value:         "9999999",
			expectedError: "Invalid containerPort: 9999999",
		},
		{
			value:         "80/xyz",
			expectedError: "Invalid proto: xyz",
		},
		{
			value:         "tcp",
			expectedError: "Invalid containerPort: tcp",
		},
		{
			value:         "udp",
			expectedError: "Invalid containerPort: udp",
		},
		{
			value:         "",
			expectedError: "No port specified: <empty>",
		},
		{
			value: "1.1.1.1:80:80",
			expected: []ServicePortConfig{
				{
					HostIP:    "1.1.1.1",
					Protocol:  "tcp",
					Target:    80,
					Published: 80,
					Mode:      "ingress",
				},
			},
		},
	}
	for _, tc := range testCases {
		ports, err := ParsePortConfig(tc.value)
		if tc.expectedError != "" {
			assert.Error(t, err, tc.expectedError)
			continue
		}
		assert.NilError(t, err)
		assert.Check(t, is.Len(ports, len(tc.expected)))
		for _, expectedPortConfig := range tc.expected {
			assertContains(t, ports, expectedPortConfig)
		}
	}
}

func assertContains(t *testing.T, portConfigs []ServicePortConfig, expected ServicePortConfig) {
	var contains = false
	for _, portConfig := range portConfigs {
		if is.DeepEqual(portConfig, expected)().Success() {
			contains = true
			break
		}
	}
	if !contains {
		t.Errorf("expected %v to contain %v, did not", portConfigs, expected)
	}
}

func TestSet(t *testing.T) {
	s := make(set)
	s.append("one")
	s.append("two")
	s.append("three")
	s.append("two")
	assert.Equal(t, len(s.toSlice()), 3)
}

type foo struct {
	Bar string
}

func TestExtension(t *testing.T) {
	x := Extensions{
		"foo": map[string]interface{}{
			"bar": "zot",
		},
	}
	var foo foo
	ok, err := x.Get("foo", &foo)
	assert.NilError(t, err)
	assert.Check(t, ok == true)
	assert.Check(t, foo.Bar == "zot")

	ok, err = x.Get("qiz", &foo)
	assert.NilError(t, err)
	assert.Check(t, ok == false)
}

func TestNewMapping(t *testing.T) {
	m := NewMapping([]string{
		"FOO=BAR",
		"ZOT=",
		"QIX",
	})
	mw := NewMappingWithEquals([]string{
		"FOO=BAR",
		"ZOT=",
		"QIX",
	})
	assert.Check(t, m["FOO"] == "BAR")
	assert.Check(t, m["ZOT"] == "")
	assert.Check(t, m["QIX"] == "")
	assert.Check(t, *mw["FOO"] == "BAR")
	assert.Check(t, *mw["ZOT"] == "")
	assert.Check(t, mw["QIX"] == nil)
}

func TestNetworksByPriority(t *testing.T) {
	s := ServiceConfig{
		Networks: map[string]*ServiceNetworkConfig{
			"foo": nil,
			"bar": {
				Priority: 10,
			},
			"zot": {
				Priority: 100,
			},
			"qix": {
				Priority: 1000,
			},
		},
	}
	assert.DeepEqual(t, s.NetworksByPriority(), []string{"qix", "zot", "bar", "foo"})
}
