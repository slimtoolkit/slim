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

package compatibility

import (
	"testing"

	"github.com/compose-spec/compose-go/errdefs"
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
)

func TestAllowList(t *testing.T) {
	var checker Checker = customChecker{
		&AllowList{
			Supported: []string{
				"services.image",
				"services.network_mode",
				"services.privileged",
				"services.networks",
				"services.scale",
			},
		},
	}
	dict := []byte(`
services:
  foo:
    image: busybox
    network_mode: host
    privileged: true
    mac_address: "a:b:c:d"
`)

	project, err := loader.Load(types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "filename.yml", Content: dict},
		},
	})
	assert.NilError(t, err)

	Check(project, checker)
	errors := checker.Errors()
	assert.Check(t, len(errors) == 2)
	assert.Check(t, errdefs.IsUnsupportedError(errors[0]))
	assert.Equal(t, errors[0].Error(), "services.mac_address: unsupported attribute")

	assert.Check(t, errdefs.IsUnsupportedError(errors[1]))
	assert.Equal(t, errors[1].Error(), "services.network_mode=host: unsupported attribute")

	service, err := project.GetService("foo")
	assert.NilError(t, err)
	assert.Check(t, service.MacAddress == "")
}

type customChecker struct {
	*AllowList
}

func (c customChecker) CheckNetworkMode(service *types.ServiceConfig) {
	if service.NetworkMode == "host" {
		c.Unsupported("services.network_mode=host")
	}
}
