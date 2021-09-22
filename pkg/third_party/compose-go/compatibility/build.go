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

import "github.com/compose-spec/compose-go/types"

func (c *AllowList) CheckBuild(service *types.ServiceConfig) bool {
	if !c.supported("services.build") && service.Build != nil {
		service.Build = nil
		c.Unsupported("services.build")
		return false
	}
	return true
}

func (c *AllowList) CheckBuildArgs(build *types.BuildConfig) {
	if !c.supported("services.build.args") && len(build.Args) != 0 {
		build.Args = nil
		c.Unsupported("services.build.args")
	}
}

func (c *AllowList) CheckBuildLabels(build *types.BuildConfig) {
	if !c.supported("services.build.labels") && len(build.Labels) != 0 {
		build.Labels = nil
		c.Unsupported("services.build.labels")
	}
}

func (c *AllowList) CheckBuildCacheFrom(build *types.BuildConfig) {
	if !c.supported("services.build.cache_from") && len(build.CacheFrom) != 0 {
		build.CacheFrom = nil
		c.Unsupported("services.build.cache_from")
	}
}

func (c *AllowList) CheckBuildExtraHosts(build *types.BuildConfig) {
	if !c.supported("services.build.extra_hosts") && len(build.ExtraHosts) != 0 {
		build.ExtraHosts = nil
		c.Unsupported("services.build.extra_hosts")
	}
}

func (c *AllowList) CheckBuildIsolation(build *types.BuildConfig) {
	if !c.supported("services.build.isolation") && build.Isolation != "" {
		build.Isolation = ""
		c.Unsupported("services.build.isolation")
	}
}

func (c *AllowList) CheckBuildNetwork(build *types.BuildConfig) {
	if !c.supported("services.build.network") && build.Network != "" {
		build.Network = ""
		c.Unsupported("services.build.network")
	}
}

func (c *AllowList) CheckBuildTarget(build *types.BuildConfig) {
	if !c.supported("services.build.target") && build.Target != "" {
		build.Target = ""
		c.Unsupported("services.build.target")
	}
}
