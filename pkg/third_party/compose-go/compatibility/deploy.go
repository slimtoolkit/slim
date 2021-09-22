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

func (c *AllowList) CheckDeploy(service *types.ServiceConfig) bool {
	if !c.supported("services.deploy") && service.Deploy != nil {
		service.Deploy = nil
		c.Unsupported("services.deploy")
		return false
	}
	return true
}

func (c *AllowList) CheckDeployMode(config *types.DeployConfig) {
	if !c.supported("services.deploy.mode") && config.Mode != "" {
		config.Mode = ""
		c.Unsupported("services.deploy.mode")
	}
}
func (c *AllowList) CheckDeployReplicas(config *types.DeployConfig) {
	if !c.supported("services.deploy.replicas") && config.Replicas != nil {
		config.Replicas = nil
		c.Unsupported("services.deploy.replicas")
	}
}
func (c *AllowList) CheckDeployLabels(config *types.DeployConfig) {
	if !c.supported("services.deploy.labels") && len(config.Labels) != 0 {
		config.Labels = nil
		c.Unsupported("services.deploy.labels")
	}
}

const (
	UpdateConfigUpdate   = "update_config"
	UpdateConfigRollback = "rolback_config"
)

func (c *AllowList) CheckDeployUpdateConfig(config *types.DeployConfig) bool {
	if !c.supported("services.deploy.update_config") {
		config.UpdateConfig = nil
		c.Unsupported("services.deploy.update_config")
		return false
	}
	return true
}

func (c *AllowList) CheckDeployRollbackConfig(config *types.DeployConfig) bool {
	if !c.supported("services.deploy.rollback_config") {
		config.RollbackConfig = nil
		c.Unsupported("services.deploy.rollback_config")
		return false
	}
	return true
}

func (c *AllowList) CheckUpdateConfigParallelism(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.parallelism", s)
	if !c.supported(k) && config.Parallelism != nil {
		config.Parallelism = nil
		c.Unsupported(k)
	}
}
func (c *AllowList) CheckUpdateConfigDelay(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.delay", s)
	if !c.supported(k) && config.Delay != 0 {
		config.Delay = 0
		c.Unsupported(k)
	}
}
func (c *AllowList) CheckUpdateConfigFailureAction(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.failure_action", s)
	if !c.supported(k) && config.FailureAction != "" {
		config.FailureAction = ""
		c.Unsupported(k)
	}
}
func (c *AllowList) CheckUpdateConfigMonitor(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.monitor", s)
	if !c.supported(k) && config.Monitor != 0 {
		config.Monitor = 0
		c.Unsupported(k)
	}
}
func (c *AllowList) CheckUpdateConfigMaxFailureRatio(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.max_failure_ratio", s)
	if !c.supported(k) && config.MaxFailureRatio != 0 {
		config.MaxFailureRatio = 0
		c.Unsupported(k)
	}
}
func (c *AllowList) CheckUpdateConfigOrder(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.order", s)
	if !c.supported(k) && config.Order != "" {
		config.Order = ""
		c.Unsupported(k)
	}
}

const (
	ResourceLimits       = "limits"
	ResourceReservations = "reservations"
)

func (c *AllowList) CheckDeployResourcesLimits(config *types.DeployConfig) bool {
	if !c.supported("services.deploy.resources.limits") {
		config.Resources.Limits = nil
		c.Unsupported("services.deploy.resources.limits")
		return false
	}
	return true
}

func (c *AllowList) CheckDeployResourcesReservations(config *types.DeployConfig) bool {
	if !c.supported("services.deploy.resources.reservations") {
		config.Resources.Reservations = nil
		c.Unsupported("services.deploy.resources.reservations")
		return false
	}
	return true
}

func (c *AllowList) CheckDeployResourcesNanoCPUs(s string, r *types.Resource) {
	k := fmt.Sprintf("services.deploy.resources.%s.cpus", s)
	if !c.supported(k) && r.NanoCPUs != "" {
		r.NanoCPUs = ""
		c.Unsupported(k)
	}
}
func (c *AllowList) CheckDeployResourcesMemoryBytes(s string, r *types.Resource) {
	k := fmt.Sprintf("services.deploy.resources.%s.memory", s)
	if !c.supported(k) && r.MemoryBytes != 0 {
		r.MemoryBytes = 0
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckDeployResourcesDevices(s string, r *types.Resource) {
	if len(r.Devices) == 0 {
		return
	}
	k := fmt.Sprintf("services.deploy.resources.%s.devices", s)
	if !c.supported(k) {
		r.Devices = nil
		c.Unsupported(k)
		return
	}
	for _, d := range r.Devices {
		c.CheckDeployResourcesDevicesCapabilities(s, d)
		c.CheckDeployResourcesDevicesCount(s, d)
		c.CheckDeployResourcesDevicesIDs(s, d)
		c.CheckDeployResourcesDevicesDriver(s, d)
	}
}

func (c *AllowList) CheckDeployResourcesDevicesCapabilities(s string, r types.DeviceRequest) {
	k := fmt.Sprintf("services.deploy.resources.%s.devices.capabilities", s)
	if !c.supported(k) && len(r.Capabilities) != 0 {
		r.Capabilities = nil
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckDeployResourcesDevicesCount(s string, r types.DeviceRequest) {
	k := fmt.Sprintf("services.deploy.resources.%s.devices.count", s)
	if !c.supported(k) && r.Count != 0 {
		r.Count = 0
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckDeployResourcesDevicesIDs(s string, r types.DeviceRequest) {
	k := fmt.Sprintf("services.deploy.resources.%s.devices.device_ids", s)
	if !c.supported(k) && len(r.IDs) != 0 {
		r.IDs = nil
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckDeployResourcesDevicesDriver(s string, r types.DeviceRequest) {
	k := fmt.Sprintf("services.deploy.resources.%s.devices.driver", s)
	if !c.supported(k) && r.Driver != "" {
		r.Driver = ""
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckDeployResourcesGenericResources(s string, r *types.Resource) {
	k := fmt.Sprintf("services.deploy.resources.%s.generic_resources", s)
	if !c.supported(k) && len(r.GenericResources) != 0 {
		r.GenericResources = nil
		c.Unsupported(k)
	}
}

func (c *AllowList) CheckDeployRestartPolicy(config *types.DeployConfig) bool {
	if !c.supported("services.deploy.restart_policy") {
		config.RestartPolicy = nil
		c.Unsupported("services.deploy.restart_policy")
		return false
	}
	return true
}

func (c *AllowList) CheckRestartPolicyCondition(p *types.RestartPolicy) {
	if !c.supported("services.deploy.restart_policy.condition") && p.Condition != "" {
		p.Condition = ""
		c.Unsupported("services.deploy.restart_policy.condition")
	}
}
func (c *AllowList) CheckRestartPolicyDelay(p *types.RestartPolicy) {
	if !c.supported("services.deploy.restart_policy.delay") && p.Delay != nil {
		p.Delay = nil
		c.Unsupported("services.deploy.restart_policy.delay")
	}
}
func (c *AllowList) CheckRestartPolicyMaxAttempts(p *types.RestartPolicy) {
	if !c.supported("services.deploy.restart_policy.max_attempts") && p.MaxAttempts != nil {
		p.MaxAttempts = nil
		c.Unsupported("services.deploy.restart_policy.max_attempts")
	}
}
func (c *AllowList) CheckRestartPolicyWindow(p *types.RestartPolicy) {
	if !c.supported("services.deploy.restart_policy.window") && p.Window != nil {
		p.Window = nil
		c.Unsupported("services.deploy.restart_policy.window")
	}
}

func (c *AllowList) CheckPlacementConstraints(p *types.Placement) {
	if !c.supported("services.deploy.placement", "services.deploy.placement.constraints") && len(p.Constraints) != 0 {
		p.Constraints = nil
		c.Unsupported("services.deploy.restart_policy.constraints")
	}
}

func (c *AllowList) CheckPlacementPreferences(p *types.Placement) {
	if !c.supported("services.deploy.placement", "services.deploy.placement.preferences") && p.Preferences != nil {
		p.Preferences = nil
		c.Unsupported("services.deploy.restart_policy.preferences")
	}
}

func (c *AllowList) CheckPlacementMaxReplicas(p *types.Placement) {
	if !c.supported("services.deploy.placement", "services.deploy.placement.max_replicas_per_node") && p.MaxReplicas != 0 {
		p.MaxReplicas = 0
		c.Unsupported("services.deploy.restart_policy.max_replicas_per_node")
	}
}

func (c *AllowList) CheckDeployEndpointMode(config *types.DeployConfig) {
	if !c.supported("services.deploy.endpoint_mode") && config.EndpointMode != "" {
		config.EndpointMode = ""
		c.Unsupported("services.deploy.endpoint_mode")
	}
}
