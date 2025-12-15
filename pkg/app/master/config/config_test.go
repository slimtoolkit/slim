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
package config

import (
	"testing"
)

func TestContainerRunOptionsDeviceRequest(t *testing.T) {
	tt := []struct {
		deviceRequest string
		expectEmpty   bool
	}{
		{
			deviceRequest: "",
			expectEmpty:   true,
		},
		{
			deviceRequest: `{"Driver":"nvidia","Count":-1,"Capabilities":[["gpu"]]}`,
			expectEmpty:   false,
		},
		{
			deviceRequest: `{"Driver":"nvidia","DeviceIDs":["0","1"],"Capabilities":[["gpu"]]}`,
			expectEmpty:   false,
		},
	}

	for _, test := range tt {
		cro := ContainerRunOptions{
			DeviceRequest: test.deviceRequest,
		}

		isEmpty := cro.DeviceRequest == ""
		if isEmpty != test.expectEmpty {
			t.Errorf("DeviceRequest isEmpty = %v for %q, want %v", isEmpty, test.deviceRequest, test.expectEmpty)
		}

		if !test.expectEmpty && cro.DeviceRequest != test.deviceRequest {
			t.Errorf("DeviceRequest = %q, want %q", cro.DeviceRequest, test.deviceRequest)
		}
	}
}

func TestContainerRunOptionsAllFields(t *testing.T) {
	// Test that ContainerRunOptions can be created with all fields including DeviceRequest
	cro := ContainerRunOptions{
		Runtime:       "nvidia",
		SysctlParams:  map[string]string{"net.core.somaxconn": "1024"},
		ShmSize:       67108864, // 64MB
		DeviceRequest: `{"Driver":"nvidia","Count":-1,"Capabilities":[["gpu"]]}`,
	}

	if cro.Runtime != "nvidia" {
		t.Errorf("Runtime = %q, want 'nvidia'", cro.Runtime)
	}

	if cro.SysctlParams["net.core.somaxconn"] != "1024" {
		t.Errorf("SysctlParams[net.core.somaxconn] = %q, want '1024'", cro.SysctlParams["net.core.somaxconn"])
	}

	if cro.ShmSize != 67108864 {
		t.Errorf("ShmSize = %d, want 67108864", cro.ShmSize)
	}

	if cro.DeviceRequest == "" {
		t.Error("DeviceRequest should not be empty")
	}
}
