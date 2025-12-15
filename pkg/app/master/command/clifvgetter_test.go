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
package command

import (
	"flag"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestGetContainerRunOptionsDeviceRequest(t *testing.T) {
	tt := []struct {
		deviceRequestFlag     string
		expectedDeviceRequest string
	}{
		{
			deviceRequestFlag:     "",
			expectedDeviceRequest: "",
		},
		{
			deviceRequestFlag:     `{"Driver":"nvidia","Count":-1,"Capabilities":[["gpu"]]}`,
			expectedDeviceRequest: `{"Driver":"nvidia","Count":-1,"Capabilities":[["gpu"]]}`,
		},
		{
			deviceRequestFlag:     `{"Driver":"nvidia","DeviceIDs":["0","1"],"Capabilities":[["gpu"]]}`,
			expectedDeviceRequest: `{"Driver":"nvidia","DeviceIDs":["0","1"],"Capabilities":[["gpu"]]}`,
		},
		{
			deviceRequestFlag:     `{"Driver":"nvidia","Count":2,"DeviceIDs":["GPU-123"],"Capabilities":[["gpu","compute"]],"Options":{"visible":"true"}}`,
			expectedDeviceRequest: `{"Driver":"nvidia","Count":2,"DeviceIDs":["GPU-123"],"Capabilities":[["gpu","compute"]],"Options":{"visible":"true"}}`,
		},
	}

	for _, test := range tt {
		flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
		flagSet.String(FlagCRODeviceRequest, test.deviceRequestFlag, "")
		flagSet.String(FlagCRORuntime, "", "")
		flagSet.String(FlagCROHostConfigFile, "", "")
		flagSet.Int64(FlagCROShmSize, 0, "")

		app := &cli.App{}
		ctx := cli.NewContext(app, flagSet, nil)

		cro, err := GetContainerRunOptions(ctx)
		if err != nil {
			t.Fatalf("GetContainerRunOptions returned error: %v", err)
		}

		if cro.DeviceRequest != test.expectedDeviceRequest {
			t.Errorf("DeviceRequest = %q, want %q", cro.DeviceRequest, test.expectedDeviceRequest)
		}
	}
}

func TestGetContainerRunOptionsAllFields(t *testing.T) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String(FlagCRODeviceRequest, `{"Driver":"nvidia","Count":-1,"Capabilities":[["gpu"]]}`, "")
	flagSet.String(FlagCRORuntime, "nvidia", "")
	flagSet.String(FlagCROHostConfigFile, "", "")
	flagSet.Int64(FlagCROShmSize, 67108864, "")

	app := &cli.App{}
	ctx := cli.NewContext(app, flagSet, nil)

	cro, err := GetContainerRunOptions(ctx)
	if err != nil {
		t.Fatalf("GetContainerRunOptions returned error: %v", err)
	}

	if cro.Runtime != "nvidia" {
		t.Errorf("Runtime = %q, want 'nvidia'", cro.Runtime)
	}

	if cro.ShmSize != 67108864 {
		t.Errorf("ShmSize = %d, want 67108864", cro.ShmSize)
	}

	if cro.DeviceRequest != `{"Driver":"nvidia","Count":-1,"Capabilities":[["gpu"]]}` {
		t.Errorf("DeviceRequest = %q, want JSON string", cro.DeviceRequest)
	}
}

func TestFlagCRODeviceRequestDefinition(t *testing.T) {
	flagDef, exists := CommonFlags[FlagCRODeviceRequest]
	if !exists {
		t.Fatal("FlagCRODeviceRequest not found in CommonFlags")
	}

	stringFlag, ok := flagDef.(*cli.StringFlag)
	if !ok {
		t.Fatal("FlagCRODeviceRequest is not a StringFlag")
	}

	if stringFlag.Name != FlagCRODeviceRequest {
		t.Errorf("Flag name = %q, want %q", stringFlag.Name, FlagCRODeviceRequest)
	}

	if stringFlag.Usage != FlagCRODeviceRequestUsage {
		t.Errorf("Flag usage = %q, want %q", stringFlag.Usage, FlagCRODeviceRequestUsage)
	}

	expectedEnvVar := "DSLIM_CRO_DEVICE_REQUEST"
	hasEnvVar := false
	for _, env := range stringFlag.EnvVars {
		if env == expectedEnvVar {
			hasEnvVar = true
			break
		}
	}
	if !hasEnvVar {
		t.Errorf("Flag missing expected EnvVar %q, has %v", expectedEnvVar, stringFlag.EnvVars)
	}
}

func TestFlagCRODeviceRequestConstants(t *testing.T) {
	if FlagCRODeviceRequest != "cro-device-request" {
		t.Errorf("FlagCRODeviceRequest = %q, want 'cro-device-request'", FlagCRODeviceRequest)
	}

	expectedUsage := "JSON string specifying device request configuration for the container"
	if FlagCRODeviceRequestUsage != expectedUsage {
		t.Errorf("FlagCRODeviceRequestUsage = %q, want %q", FlagCRODeviceRequestUsage, expectedUsage)
	}
}
