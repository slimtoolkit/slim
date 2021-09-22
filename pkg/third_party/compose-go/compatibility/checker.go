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
	"github.com/compose-spec/compose-go/errdefs"
	"github.com/compose-spec/compose-go/types"
)

type Checker interface {
	Errors() []error
	CheckBlkioConfig(build *types.ServiceConfig)
	CheckBlkioWeight(build *types.BlkioConfig)
	CheckBlkioWeightDevice(build *types.BlkioConfig)
	CheckBlkioDeviceReadBps(build *types.BlkioConfig)
	CheckBlkioDeviceReadIOps(build *types.BlkioConfig)
	CheckBlkioDeviceWriteBps(build *types.BlkioConfig)
	CheckBlkioDeviceWriteIOps(build *types.BlkioConfig)
	CheckBuild(build *types.ServiceConfig) bool
	CheckBuildArgs(build *types.BuildConfig)
	CheckBuildLabels(build *types.BuildConfig)
	CheckBuildCacheFrom(build *types.BuildConfig)
	CheckBuildExtraHosts(build *types.BuildConfig)
	CheckBuildIsolation(build *types.BuildConfig)
	CheckBuildNetwork(build *types.BuildConfig)
	CheckBuildTarget(build *types.BuildConfig)
	CheckCapAdd(service *types.ServiceConfig)
	CheckCapDrop(service *types.ServiceConfig)
	CheckCgroupParent(service *types.ServiceConfig)
	CheckCPUCount(service *types.ServiceConfig)
	CheckCPUPercent(service *types.ServiceConfig)
	CheckCPUPeriod(service *types.ServiceConfig)
	CheckCPUQuota(service *types.ServiceConfig)
	CheckCPURTRuntime(service *types.ServiceConfig)
	CheckCPURTPeriod(service *types.ServiceConfig)
	CheckCPUs(service *types.ServiceConfig)
	CheckCPUSet(service *types.ServiceConfig)
	CheckCPUShares(service *types.ServiceConfig)
	CheckCommand(service *types.ServiceConfig)
	CheckConfigs(service *types.ServiceConfig)
	CheckContainerName(service *types.ServiceConfig)
	CheckCredentialSpec(service *types.ServiceConfig)
	CheckDependsOn(service *types.ServiceConfig)
	CheckDevices(service *types.ServiceConfig)
	CheckDNS(service *types.ServiceConfig)
	CheckDNSOpts(service *types.ServiceConfig)
	CheckDNSSearch(service *types.ServiceConfig)
	CheckDomainName(service *types.ServiceConfig)
	CheckEntrypoint(service *types.ServiceConfig)
	CheckEnvironment(service *types.ServiceConfig)
	CheckEnvFile(service *types.ServiceConfig)
	CheckExpose(service *types.ServiceConfig)
	CheckExtends(service *types.ServiceConfig)
	CheckExternalLinks(service *types.ServiceConfig)
	CheckExtraHosts(service *types.ServiceConfig)
	CheckGroupAdd(service *types.ServiceConfig)
	CheckHostname(service *types.ServiceConfig)
	CheckHealthCheckTest(h *types.HealthCheckConfig)
	CheckHealthCheckTimeout(h *types.HealthCheckConfig)
	CheckHealthCheckInterval(h *types.HealthCheckConfig)
	CheckHealthCheckRetries(h *types.HealthCheckConfig)
	CheckHealthCheckStartPeriod(h *types.HealthCheckConfig)
	CheckImage(service *types.ServiceConfig)
	CheckInit(service *types.ServiceConfig)
	CheckIpc(service *types.ServiceConfig)
	CheckIsolation(service *types.ServiceConfig)
	CheckLabels(service *types.ServiceConfig)
	CheckLinks(service *types.ServiceConfig)
	CheckLoggingDriver(logging *types.LoggingConfig)
	CheckLoggingOptions(logging *types.LoggingConfig)
	CheckMemLimit(service *types.ServiceConfig)
	CheckMemReservation(service *types.ServiceConfig)
	CheckMemSwapLimit(service *types.ServiceConfig)
	CheckMemSwappiness(service *types.ServiceConfig)
	CheckMacAddress(service *types.ServiceConfig)
	CheckNet(service *types.ServiceConfig)
	CheckNetworkMode(service *types.ServiceConfig)
	CheckNetworkAliases(n *types.ServiceNetworkConfig)
	CheckNetworkIpv4Address(n *types.ServiceNetworkConfig)
	CheckNetworkIpv6Address(n *types.ServiceNetworkConfig)
	CheckOomKillDisable(service *types.ServiceConfig)
	CheckOomScoreAdj(service *types.ServiceConfig)
	CheckPid(service *types.ServiceConfig)
	CheckPidsLimit(service *types.ServiceConfig)
	CheckPlatform(service *types.ServiceConfig)
	CheckPortsMode(p *types.ServicePortConfig)
	CheckPortsTarget(p *types.ServicePortConfig)
	CheckPortsPublished(p *types.ServicePortConfig)
	CheckPortsProtocol(p *types.ServicePortConfig)
	CheckPrivileged(service *types.ServiceConfig)
	CheckPullPolicy(service *types.ServiceConfig)
	CheckReadOnly(service *types.ServiceConfig)
	CheckRestart(service *types.ServiceConfig)
	CheckRuntime(service *types.ServiceConfig)
	CheckScale(service *types.ServiceConfig)
	CheckSecrets(service *types.ServiceConfig)
	CheckFileReferenceSource(s string, config *types.FileReferenceConfig)
	CheckFileReferenceTarget(s string, config *types.FileReferenceConfig)
	CheckFileReferenceUID(s string, config *types.FileReferenceConfig)
	CheckFileReferenceGID(s string, config *types.FileReferenceConfig)
	CheckFileReferenceMode(s string, config *types.FileReferenceConfig)
	CheckSecurityOpt(service *types.ServiceConfig)
	CheckShmSize(service *types.ServiceConfig)
	CheckStdinOpen(service *types.ServiceConfig)
	CheckStopGracePeriod(service *types.ServiceConfig)
	CheckStopSignal(service *types.ServiceConfig)
	CheckSysctls(service *types.ServiceConfig)
	CheckTmpfs(service *types.ServiceConfig)
	CheckTty(service *types.ServiceConfig)
	CheckUlimits(service *types.ServiceConfig)
	CheckUser(service *types.ServiceConfig)
	CheckUserNSMode(service *types.ServiceConfig)
	CheckUts(service *types.ServiceConfig)
	CheckVolumeDriver(service *types.ServiceConfig)
	CheckVolumesSource(config *types.ServiceVolumeConfig)
	CheckVolumesTarget(config *types.ServiceVolumeConfig)
	CheckVolumesReadOnly(config *types.ServiceVolumeConfig)
	CheckVolumesConsistency(config *types.ServiceVolumeConfig)
	CheckVolumesBind(config *types.ServiceVolumeBind)
	CheckVolumesVolume(config *types.ServiceVolumeVolume)
	CheckVolumesTmpfs(config *types.ServiceVolumeTmpfs)
	CheckVolumesFrom(service *types.ServiceConfig)
	CheckWorkingDir(service *types.ServiceConfig)
	CheckVolumeConfigDriver(config *types.VolumeConfig)
	CheckVolumeConfigDriverOpts(config *types.VolumeConfig)
	CheckVolumeConfigExternal(config *types.VolumeConfig)
	CheckVolumeConfigLabels(config *types.VolumeConfig)
	CheckFileObjectConfigFile(s string, config *types.FileObjectConfig)
	CheckFileObjectConfigExternal(s string, config *types.FileObjectConfig)
	CheckFileObjectConfigLabels(s string, config *types.FileObjectConfig)
	CheckFileObjectConfigDriver(s string, config *types.FileObjectConfig)
	CheckFileObjectConfigDriverOpts(s string, config *types.FileObjectConfig)
	CheckFileObjectConfigTemplateDriver(s string, config *types.FileObjectConfig)
	CheckDeploy(deploy *types.ServiceConfig) bool
	CheckDeployEndpointMode(deploy *types.DeployConfig)
	CheckDeployLabels(deploy *types.DeployConfig)
	CheckDeployMode(deploy *types.DeployConfig)
	CheckDeployReplicas(deploy *types.DeployConfig)
	CheckDeployRestartPolicy(deploy *types.DeployConfig) bool
	CheckDeployRollbackConfig(deploy *types.DeployConfig) bool
	CheckDeployUpdateConfig(deploy *types.DeployConfig) bool
	CheckPlacementConstraints(p *types.Placement)
	CheckPlacementMaxReplicas(p *types.Placement)
	CheckPlacementPreferences(p *types.Placement)
	CheckRestartPolicyDelay(policy *types.RestartPolicy)
	CheckRestartPolicyCondition(policy *types.RestartPolicy)
	CheckRestartPolicyMaxAttempts(policy *types.RestartPolicy)
	CheckRestartPolicyWindow(policy *types.RestartPolicy)
	CheckUpdateConfigDelay(rollback string, config *types.UpdateConfig)
	CheckUpdateConfigFailureAction(rollback string, config *types.UpdateConfig)
	CheckUpdateConfigMaxFailureRatio(rollback string, config *types.UpdateConfig)
	CheckUpdateConfigMonitor(rollback string, config *types.UpdateConfig)
	CheckUpdateConfigOrder(rollback string, config *types.UpdateConfig)
	CheckUpdateConfigParallelism(rollback string, config *types.UpdateConfig)
	CheckDeployResourcesNanoCPUs(s string, resource *types.Resource)
	CheckDeployResourcesMemoryBytes(s string, resource *types.Resource)
	CheckDeployResourcesDevices(s string, resource *types.Resource)
	CheckDeployResourcesDevicesCapabilities(s string, r types.DeviceRequest)
	CheckDeployResourcesDevicesCount(s string, r types.DeviceRequest)
	CheckDeployResourcesDevicesIDs(s string, r types.DeviceRequest)
	CheckDeployResourcesDevicesDriver(s string, r types.DeviceRequest)
	CheckDeployResourcesGenericResources(s string, resource *types.Resource)
	CheckDeployResourcesLimits(deploy *types.DeployConfig) bool
	CheckDeployResourcesReservations(deploy *types.DeployConfig) bool
	CheckHealthCheck(service *types.ServiceConfig) bool
	CheckLogging(service *types.ServiceConfig) bool
	CheckNetworks(service *types.ServiceConfig) bool
	CheckPorts(service *types.ServiceConfig) bool
	CheckServiceVolumes(service *types.ServiceConfig) bool
	CheckNetworkConfigIpam(network *types.NetworkConfig)
	CheckNetworkConfigIpamSubnet(config *types.IPAMPool)
	CheckNetworkConfigIpamGateway(config *types.IPAMPool)
	CheckNetworkConfigIpamIPRange(config *types.IPAMPool)
	CheckNetworkConfigIpamAuxiliaryAddresses(config *types.IPAMPool)
	CheckNetworkConfigDriver(network *types.NetworkConfig)
	CheckNetworkConfigDriverOpts(network *types.NetworkConfig)
	CheckNetworkConfigExternal(network *types.NetworkConfig)
	CheckNetworkConfigInternal(network *types.NetworkConfig)
	CheckNetworkConfigAttachable(network *types.NetworkConfig)
	CheckNetworkConfigLabels(network *types.NetworkConfig)
}

func Check(project *types.Project, c Checker) {
	for i, service := range project.Services {
		CheckServiceConfig(&service, c)
		project.Services[i] = service
	}

	for i, network := range project.Networks {
		CheckNetworkConfig(&network, c)
		project.Networks[i] = network
	}

	for i, volume := range project.Volumes {
		CheckVolumeConfig(&volume, c)
		project.Volumes[i] = volume
	}

	for i, config := range project.Configs {
		CheckConfigsConfig(&config, c)
		project.Configs[i] = config
	}

	for i, secret := range project.Secrets {
		CheckSecretsConfig(&secret, c)
		project.Secrets[i] = secret
	}
}

// IsCompatible return true if the checker didn't reported any incompatibility error
func IsCompatible(c Checker) bool {
	for _, err := range c.Errors() {
		if errdefs.IsIncompatibleError(err) {
			return false
		}
	}
	return true
}

func CheckServiceConfig(service *types.ServiceConfig, c Checker) {
	c.CheckBlkioConfig(service)
	if service.Build != nil && c.CheckBuild(service) {
		c.CheckBuildArgs(service.Build)
		c.CheckBuildLabels(service.Build)
		c.CheckBuildCacheFrom(service.Build)
		c.CheckBuildNetwork(service.Build)
		c.CheckBuildTarget(service.Build)
	}
	c.CheckCapAdd(service)
	c.CheckCapDrop(service)
	c.CheckCgroupParent(service)
	c.CheckCPUCount(service)
	c.CheckCPUPercent(service)
	c.CheckCPUPeriod(service)
	c.CheckCPUQuota(service)
	c.CheckCPURTPeriod(service)
	c.CheckCPURTRuntime(service)
	c.CheckCPUs(service)
	c.CheckCPUSet(service)
	c.CheckCPUShares(service)
	c.CheckCommand(service)
	c.CheckConfigs(service)
	c.CheckContainerName(service)
	c.CheckCredentialSpec(service)
	c.CheckDependsOn(service)
	if service.Deploy != nil && c.CheckDeploy(service) {
		c.CheckDeployEndpointMode(service.Deploy)
		c.CheckDeployLabels(service.Deploy)
		c.CheckDeployMode(service.Deploy)
		c.CheckPlacementConstraints(&service.Deploy.Placement)
		c.CheckPlacementMaxReplicas(&service.Deploy.Placement)
		c.CheckPlacementPreferences(&service.Deploy.Placement)
		c.CheckDeployReplicas(service.Deploy)
		if service.Deploy.Resources.Limits != nil && c.CheckDeployResourcesLimits(service.Deploy) {
			c.CheckDeployResourcesNanoCPUs(ResourceLimits, service.Deploy.Resources.Limits)
			c.CheckDeployResourcesMemoryBytes(ResourceLimits, service.Deploy.Resources.Limits)
			c.CheckDeployResourcesGenericResources(ResourceLimits, service.Deploy.Resources.Limits)
		}
		if service.Deploy.Resources.Reservations != nil && c.CheckDeployResourcesReservations(service.Deploy) {
			c.CheckDeployResourcesNanoCPUs(ResourceReservations, service.Deploy.Resources.Reservations)
			c.CheckDeployResourcesMemoryBytes(ResourceReservations, service.Deploy.Resources.Reservations)
			c.CheckDeployResourcesGenericResources(ResourceReservations, service.Deploy.Resources.Reservations)
			c.CheckDeployResourcesDevices(ResourceReservations, service.Deploy.Resources.Reservations)
		}
		if service.Deploy.RestartPolicy != nil && c.CheckDeployRestartPolicy(service.Deploy) {
			c.CheckRestartPolicyCondition(service.Deploy.RestartPolicy)
			c.CheckRestartPolicyDelay(service.Deploy.RestartPolicy)
			c.CheckRestartPolicyMaxAttempts(service.Deploy.RestartPolicy)
			c.CheckRestartPolicyWindow(service.Deploy.RestartPolicy)
		}
		if service.Deploy.UpdateConfig != nil && c.CheckDeployUpdateConfig(service.Deploy) {
			c.CheckUpdateConfigDelay(UpdateConfigUpdate, service.Deploy.UpdateConfig)
			c.CheckUpdateConfigFailureAction(UpdateConfigUpdate, service.Deploy.UpdateConfig)
			c.CheckUpdateConfigMaxFailureRatio(UpdateConfigUpdate, service.Deploy.UpdateConfig)
			c.CheckUpdateConfigMonitor(UpdateConfigUpdate, service.Deploy.UpdateConfig)
			c.CheckUpdateConfigOrder(UpdateConfigUpdate, service.Deploy.UpdateConfig)
			c.CheckUpdateConfigParallelism(UpdateConfigUpdate, service.Deploy.UpdateConfig)
		}
		if service.Deploy.RollbackConfig != nil && c.CheckDeployRollbackConfig(service.Deploy) {
			c.CheckUpdateConfigDelay(UpdateConfigRollback, service.Deploy.RollbackConfig)
			c.CheckUpdateConfigFailureAction(UpdateConfigRollback, service.Deploy.RollbackConfig)
			c.CheckUpdateConfigMaxFailureRatio(UpdateConfigRollback, service.Deploy.RollbackConfig)
			c.CheckUpdateConfigMonitor(UpdateConfigRollback, service.Deploy.RollbackConfig)
			c.CheckUpdateConfigOrder(UpdateConfigRollback, service.Deploy.RollbackConfig)
			c.CheckUpdateConfigParallelism(UpdateConfigRollback, service.Deploy.RollbackConfig)
		}
	}
	c.CheckDevices(service)
	c.CheckDNS(service)
	c.CheckDNSOpts(service)
	c.CheckDNSSearch(service)
	c.CheckDomainName(service)
	c.CheckEntrypoint(service)
	c.CheckEnvironment(service)
	c.CheckEnvFile(service)
	c.CheckExpose(service)
	c.CheckExtends(service)
	c.CheckExternalLinks(service)
	c.CheckExtraHosts(service)
	c.CheckGroupAdd(service)
	c.CheckHostname(service)
	if service.HealthCheck != nil && c.CheckHealthCheck(service) {
		c.CheckHealthCheckInterval(service.HealthCheck)
		c.CheckHealthCheckRetries(service.HealthCheck)
		c.CheckHealthCheckStartPeriod(service.HealthCheck)
		c.CheckHealthCheckTest(service.HealthCheck)
		c.CheckHealthCheckTimeout(service.HealthCheck)
	}
	c.CheckImage(service)
	c.CheckInit(service)
	c.CheckIpc(service)
	c.CheckIsolation(service)
	c.CheckLabels(service)
	c.CheckLinks(service)
	if service.Logging != nil && c.CheckLogging(service) {
		c.CheckLoggingDriver(service.Logging)
		c.CheckLoggingOptions(service.Logging)
	}
	c.CheckMemLimit(service)
	c.CheckMemReservation(service)
	c.CheckMemSwapLimit(service)
	c.CheckMemSwappiness(service)
	c.CheckMacAddress(service)
	c.CheckNet(service)
	c.CheckNetworkMode(service)
	if len(service.Networks) > 0 && c.CheckNetworks(service) {
		for _, n := range service.Networks {
			if n != nil {
				c.CheckNetworkAliases(n)
				c.CheckNetworkIpv4Address(n)
				c.CheckNetworkIpv6Address(n)
			}
		}
	}
	c.CheckOomKillDisable(service)
	c.CheckOomScoreAdj(service)
	c.CheckPid(service)
	c.CheckPidsLimit(service)
	c.CheckPlatform(service)
	if len(service.Ports) > 0 && c.CheckPorts(service) {
		for i, p := range service.Ports {
			c.CheckPortsMode(&p)
			c.CheckPortsTarget(&p)
			c.CheckPortsProtocol(&p)
			c.CheckPortsPublished(&p)
			service.Ports[i] = p
		}
	}
	c.CheckPrivileged(service)
	c.CheckPullPolicy(service)
	c.CheckReadOnly(service)
	c.CheckRestart(service)
	c.CheckRuntime(service)
	c.CheckScale(service)
	c.CheckSecrets(service)
	c.CheckSecurityOpt(service)
	c.CheckShmSize(service)
	c.CheckStdinOpen(service)
	c.CheckStopGracePeriod(service)
	c.CheckStopSignal(service)
	c.CheckSysctls(service)
	c.CheckTmpfs(service)
	c.CheckTty(service)
	c.CheckUlimits(service)
	c.CheckUser(service)
	c.CheckUserNSMode(service)
	c.CheckUts(service)
	c.CheckVolumeDriver(service)
	if len(service.Volumes) > 0 && c.CheckServiceVolumes(service) {
		for i, v := range service.Volumes {
			c.CheckVolumesSource(&v)
			c.CheckVolumesTarget(&v)
			c.CheckVolumesReadOnly(&v)
			switch v.Type {
			case types.VolumeTypeBind:
				c.CheckVolumesBind(v.Bind)
			case types.VolumeTypeVolume:
				c.CheckVolumesVolume(v.Volume)
			case types.VolumeTypeTmpfs:
				c.CheckVolumesTmpfs(v.Tmpfs)
			}
			service.Volumes[i] = v
		}
	}
	c.CheckVolumesFrom(service)
	c.CheckWorkingDir(service)
}

func CheckNetworkConfig(network *types.NetworkConfig, c Checker) {
	c.CheckNetworkConfigDriver(network)
	c.CheckNetworkConfigDriverOpts(network)
	c.CheckNetworkConfigIpam(network)
	c.CheckNetworkConfigExternal(network)
	c.CheckNetworkConfigInternal(network)
	c.CheckNetworkConfigAttachable(network)
	c.CheckNetworkConfigLabels(network)
}

func CheckVolumeConfig(config *types.VolumeConfig, c Checker) {
	c.CheckVolumeConfigDriver(config)
	c.CheckVolumeConfigDriverOpts(config)
	c.CheckVolumeConfigExternal(config)
	c.CheckVolumeConfigLabels(config)
}

func CheckConfigsConfig(config *types.ConfigObjConfig, c Checker) {
	ref := types.FileObjectConfig(*config)
	CheckFileObjectConfig("configs", &ref, c)
}

func CheckSecretsConfig(config *types.SecretConfig, c Checker) {
	ref := types.FileObjectConfig(*config)
	CheckFileObjectConfig("secrets", &ref, c)
}

func CheckFileObjectConfig(s string, config *types.FileObjectConfig, c Checker) {
	c.CheckFileObjectConfigDriver(s, config)
	c.CheckFileObjectConfigDriverOpts(s, config)
	c.CheckFileObjectConfigExternal(s, config)
	c.CheckFileObjectConfigFile(s, config)
	c.CheckFileObjectConfigLabels(s, config)
	c.CheckFileObjectConfigTemplateDriver(s, config)
}
