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

package loader

import (
	"github.com/compose-spec/compose-go/types"
)

func withVersionExampleConfig(workingDir, homeDir string) *types.Config {
	return &types.Config{
		Services: withVersionServices(workingDir, homeDir),
		Networks: withVersionNetworks(),
		Volumes:  withVersionVolumes(),
	}
}

func withVersionServices(workingDir, homeDir string) []types.ServiceConfig {
	return []types.ServiceConfig{
		{
			Name: "web",

			Build: &types.BuildConfig{
				Context: "./Dockerfile",
			},
			Environment: types.MappingWithEquals{},
			Networks: map[string]*types.ServiceNetworkConfig{
				"front":   nil,
				"default": nil,
			},
			VolumesFrom: []string{"other"},
			Scale:       1,
		},
		{
			Name: "other",

			Image:       "busybox:1.31.0-uclibc",
			Command:     []string{"top"},
			Environment: types.MappingWithEquals{},
			Volumes: []types.ServiceVolumeConfig{
				{Target: "/data", Type: "volume", Volume: &types.ServiceVolumeVolume{}},
			},
			Scale: 1,
		},
	}
}

func withVersionNetworks() map[string]types.NetworkConfig {
	return map[string]types.NetworkConfig{
		"front": {},
	}
}

func withVersionVolumes() map[string]types.VolumeConfig {
	return map[string]types.VolumeConfig{
		"data": {
			Driver: "local",
		},
	}
}
