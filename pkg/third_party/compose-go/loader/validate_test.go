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
	"testing"

	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/types"
)

func TestValidateAnonymousVolume(t *testing.T) {
	project := &types.Project{
		Services: types.Services([]types.ServiceConfig{
			{
				Name:  "myservice",
				Image: "my/service",
				Volumes: []types.ServiceVolumeConfig{
					{
						Type:   types.VolumeTypeVolume,
						Target: "/use/local",
					},
				},
			},
		}),
	}
	err := checkConsistency(project)
	assert.NilError(t, err)
}

func TestValidateNamedVolume(t *testing.T) {
	project := &types.Project{
		Services: types.Services([]types.ServiceConfig{
			{
				Name:  "myservice",
				Image: "my/service",
				Volumes: []types.ServiceVolumeConfig{
					{
						Type:   types.VolumeTypeVolume,
						Source: "myVolume",
						Target: "/use/local",
					},
				},
			},
		}),
	}
	err := checkConsistency(project)
	assert.Error(t, err, `service "myservice" refers to undefined volume myVolume: invalid compose project`)

	project.Volumes = types.Volumes(map[string]types.VolumeConfig{
		"myVolume": {
			Name: "myVolume",
		},
	})
	err = checkConsistency(project)
	assert.NilError(t, err)
}

func TestValidateNoBuildNoImage(t *testing.T) {
	project := &types.Project{
		Services: types.Services([]types.ServiceConfig{
			{
				Name: "myservice",
			},
		}),
	}
	err := checkConsistency(project)
	assert.Error(t, err, `service "myservice" has neither an image nor a build context specified: invalid compose project`)
}

func TestValidateNetworkMode(t *testing.T) {
	t.Run("network_mode service fail", func(t *testing.T) {
		project := &types.Project{
			Services: types.Services([]types.ServiceConfig{
				{
					Name:  "myservice1",
					Image: "scratch",
				},
				{
					Name:        "myservice2",
					Image:       "scratch",
					NetworkMode: "service:myservice1",
				},
			}),
		}
		err := checkConsistency(project)
		assert.NilError(t, err)
	})

	t.Run("network_mode service fail", func(t *testing.T) {
		project := &types.Project{
			Services: types.Services([]types.ServiceConfig{
				{
					Name:  "myservice1",
					Image: "scratch",
				},
				{
					Name:        "myservice2",
					Image:       "scratch",
					NetworkMode: "service:nonexistentservice",
				},
			}),
		}
		err := checkConsistency(project)
		assert.Error(t, err, `service "nonexistentservice" not found for network_mode 'service:nonexistentservice'`)
	})

	t.Run("network_mode container", func(t *testing.T) {
		project := &types.Project{
			Services: types.Services([]types.ServiceConfig{
				{
					Name:          "myservice1",
					ContainerName: "mycontainer_name",
					Image:         "scratch",
				},
				{
					Name:        "myservice2",
					Image:       "scratch",
					NetworkMode: "container:mycontainer_name",
				},
			}),
		}
		err := checkConsistency(project)
		assert.NilError(t, err)
	})
}
