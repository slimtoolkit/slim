/*
 * SPDX-FileCopyrightText: Copyright (c) 2025 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package container

import (
	"encoding/json"
	"reflect"
	"testing"

	dockerapi "github.com/fsouza/go-dockerclient"
)

func TestParseDeviceRequestJSON(t *testing.T) {
	tt := []struct {
		input       string
		expected    dockerapi.DeviceRequest
		expectError bool
	}{
		{
			input: `{"Driver":"nvidia","Count":-1,"Capabilities":[["gpu"]]}`,
			expected: dockerapi.DeviceRequest{
				Driver:       "nvidia",
				Count:        -1,
				Capabilities: [][]string{{"gpu"}},
			},
			expectError: false,
		},
		{
			input: `{"Driver":"nvidia","DeviceIDs":["0","1"],"Capabilities":[["gpu"]]}`,
			expected: dockerapi.DeviceRequest{
				Driver:       "nvidia",
				DeviceIDs:    []string{"0", "1"},
				Capabilities: [][]string{{"gpu"}},
			},
			expectError: false,
		},
		{
			input: `{"Driver":"nvidia","Count":2,"DeviceIDs":["GPU-123"],"Capabilities":[["gpu","compute"]],"Options":{"visible":"true"}}`,
			expected: dockerapi.DeviceRequest{
				Driver:       "nvidia",
				Count:        2,
				DeviceIDs:    []string{"GPU-123"},
				Capabilities: [][]string{{"gpu", "compute"}},
				Options:      map[string]string{"visible": "true"},
			},
			expectError: false,
		},
		{
			input: `{"Count":-1,"Capabilities":[["gpu"],["nvidia","compute"]]}`,
			expected: dockerapi.DeviceRequest{
				Count:        -1,
				Capabilities: [][]string{{"gpu"}, {"nvidia", "compute"}},
			},
			expectError: false,
		},
		{
			input: `{}`,
			expected: dockerapi.DeviceRequest{
				Driver:       "",
				Count:        0,
				DeviceIDs:    nil,
				Capabilities: nil,
				Options:      nil,
			},
			expectError: false,
		},
		{
			input:       `{"Driver":"nvidia"`,
			expectError: true,
		},
		{
			input:       `["gpu"]`,
			expectError: true,
		},
		{
			input:       `{Driver:nvidia}`,
			expectError: true,
		},
		{
			input:       `{"Count":"all"}`,
			expectError: true,
		},
	}

	for _, test := range tt {
		var deviceRequest dockerapi.DeviceRequest
		err := json.Unmarshal([]byte(test.input), &deviceRequest)

		if test.expectError {
			if err == nil {
				t.Errorf("expected error for input %q, but got none", test.input)
			}
			continue
		}

		if err != nil {
			t.Errorf("unexpected error for input %q: %v", test.input, err)
			continue
		}

		if !reflect.DeepEqual(deviceRequest, test.expected) {
			t.Errorf("parsed device request mismatch for %q:\n  got:      %+v\n  expected: %+v",
				test.input, deviceRequest, test.expected)
		}
	}
}

func TestDeviceRequestToHostConfig(t *testing.T) {
	tt := []struct {
		deviceRequestJSON    string
		expectDeviceRequests int
		expectError          bool
	}{
		{
			deviceRequestJSON:    `{"Driver":"nvidia","Count":-1,"Capabilities":[["gpu"]]}`,
			expectDeviceRequests: 1,
			expectError:          false,
		},
		{
			deviceRequestJSON:    "",
			expectDeviceRequests: 0,
			expectError:          false,
		},
		{
			deviceRequestJSON:    `{invalid}`,
			expectDeviceRequests: 0,
			expectError:          true,
		},
	}

	for _, test := range tt {
		hostConfig := &dockerapi.HostConfig{}

		if test.deviceRequestJSON != "" {
			var deviceRequest dockerapi.DeviceRequest
			err := json.Unmarshal([]byte(test.deviceRequestJSON), &deviceRequest)

			if test.expectError {
				if err == nil {
					t.Errorf("expected error for input %q, but got none", test.deviceRequestJSON)
				}
				if len(hostConfig.DeviceRequests) != 0 {
					t.Errorf("expected no device requests on error, got %d", len(hostConfig.DeviceRequests))
				}
				continue
			}

			if err != nil {
				t.Errorf("unexpected error for input %q: %v", test.deviceRequestJSON, err)
				continue
			}

			hostConfig.DeviceRequests = []dockerapi.DeviceRequest{deviceRequest}
		}

		if len(hostConfig.DeviceRequests) != test.expectDeviceRequests {
			t.Errorf("expected %d device requests for %q, got %d",
				test.expectDeviceRequests, test.deviceRequestJSON, len(hostConfig.DeviceRequests))
		}
	}
}

func TestDeviceRequestFieldValidation(t *testing.T) {
	// Test NVIDIA all GPUs request
	input := `{"Driver":"nvidia","Count":-1,"Capabilities":[["gpu"]]}`
	var dr dockerapi.DeviceRequest
	if err := json.Unmarshal([]byte(input), &dr); err != nil {
		t.Fatalf("failed to parse device request: %v", err)
	}
	if dr.Driver != "nvidia" {
		t.Errorf("expected Driver 'nvidia', got %q", dr.Driver)
	}
	if dr.Count != -1 {
		t.Errorf("expected Count -1 (all GPUs), got %d", dr.Count)
	}
	if len(dr.Capabilities) != 1 || len(dr.Capabilities[0]) != 1 || dr.Capabilities[0][0] != "gpu" {
		t.Errorf("expected Capabilities [[gpu]], got %v", dr.Capabilities)
	}

	// Test NVIDIA specific GPU by ID
	input = `{"Driver":"nvidia","DeviceIDs":["0"],"Capabilities":[["gpu"]]}`
	dr = dockerapi.DeviceRequest{}
	if err := json.Unmarshal([]byte(input), &dr); err != nil {
		t.Fatalf("failed to parse device request: %v", err)
	}
	if len(dr.DeviceIDs) != 1 || dr.DeviceIDs[0] != "0" {
		t.Errorf("expected DeviceIDs [0], got %v", dr.DeviceIDs)
	}
	if dr.Count != 0 {
		t.Errorf("expected Count 0 when DeviceIDs specified, got %d", dr.Count)
	}

	// Test NVIDIA multiple GPUs by UUID
	input = `{"Driver":"nvidia","DeviceIDs":["GPU-abc123","GPU-def456"],"Capabilities":[["gpu","compute"]]}`
	dr = dockerapi.DeviceRequest{}
	if err := json.Unmarshal([]byte(input), &dr); err != nil {
		t.Fatalf("failed to parse device request: %v", err)
	}
	if len(dr.DeviceIDs) != 2 {
		t.Errorf("expected 2 DeviceIDs, got %d", len(dr.DeviceIDs))
	}
	if len(dr.Capabilities) != 1 || len(dr.Capabilities[0]) != 2 {
		t.Errorf("expected Capabilities [[gpu compute]], got %v", dr.Capabilities)
	}

	// Test count of specific number of GPUs
	input = `{"Count":2,"Capabilities":[["gpu"]]}`
	dr = dockerapi.DeviceRequest{}
	if err := json.Unmarshal([]byte(input), &dr); err != nil {
		t.Fatalf("failed to parse device request: %v", err)
	}
	if dr.Count != 2 {
		t.Errorf("expected Count 2, got %d", dr.Count)
	}
}
