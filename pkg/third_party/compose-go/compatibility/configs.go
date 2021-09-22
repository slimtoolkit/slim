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
	"fmt"

	"github.com/compose-spec/compose-go/types"
)

func (c *AllowList) CheckFileObjectConfigFile(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.file", s)
	if !c.supported(k) && config.File != "" {
		config.File = ""
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckFileObjectConfigExternal(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.external", s)
	if !c.supported(k) && config.External.External {
		config.External.External = false
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckFileObjectConfigLabels(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.labels", s)
	if !c.supported(k) && len(config.Labels) != 0 {
		config.Labels = nil
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckFileObjectConfigDriver(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.driver", s)
	if !c.supported(k) && config.Driver != "" {
		config.Driver = ""
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckFileObjectConfigDriverOpts(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.driver_opts", s)
	if !c.supported(k) && len(config.DriverOpts) != 0 {
		config.DriverOpts = nil
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckFileObjectConfigTemplateDriver(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.template_driver", s)
	if !c.supported(k) && config.TemplateDriver != "" {
		config.TemplateDriver = ""
		c.Unsupported(k)
	}
}
