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

func (c *AllowList) CheckNetworkConfig(network *types.NetworkConfig) {
	c.CheckNetworkConfigDriver(network)
	c.CheckNetworkConfigDriverOpts(network)
	c.CheckNetworkConfigIpam(network)
	c.CheckNetworkConfigExternal(network)
	c.CheckNetworkConfigInternal(network)
	c.CheckNetworkConfigAttachable(network)
	c.CheckNetworkConfigLabels(network)
}

func (c *AllowList) CheckNetworkConfigDriver(config *types.NetworkConfig) {
	if !c.supported("networks.driver") && config.Driver != "" {
		config.Driver = ""
		c.Unsupported("networks.driver")
	}
}

func (c *AllowList) CheckNetworkConfigDriverOpts(config *types.NetworkConfig) {
	if !c.supported("networks.driver_opts") && len(config.DriverOpts) != 0 {
		config.DriverOpts = nil
		c.Unsupported("networks.driver_opts")
	}
}

func (c *AllowList) CheckNetworkConfigIpam(config *types.NetworkConfig) {
	c.CheckNetworkConfigIpamDriver(&config.Ipam)
	if len(config.Ipam.Config) != 0 {
		if !c.supported("networks.ipam.config") {
			c.Unsupported("networks.ipam.config")
			return
		}
		for _, p := range config.Ipam.Config {
			c.CheckNetworkConfigIpamSubnet(p)
			c.CheckNetworkConfigIpamGateway(p)
			c.CheckNetworkConfigIpamIPRange(p)
			c.CheckNetworkConfigIpamAuxiliaryAddresses(p)
		}
	}
}

func (c *AllowList) CheckNetworkConfigIpamDriver(config *types.IPAMConfig) {
	if !c.supported("networks.ipam.driver") && config.Driver != "" {
		config.Driver = ""
		c.Unsupported("networks.ipam.driver")
	}
}

func (c *AllowList) CheckNetworkConfigIpamSubnet(config *types.IPAMPool) {
	if !c.supported("networks.ipam.config.subnet") && config.Subnet != "" {
		config.Subnet = ""
		c.Unsupported("networks.ipam.config.subnet")
	}
}

func (c *AllowList) CheckNetworkConfigIpamGateway(config *types.IPAMPool) {
	if !c.supported("networks.ipam.config.gateway") && config.Gateway != "" {
		config.Gateway = ""
		c.Unsupported("networks.ipam.config.gateway")
	}
}

func (c *AllowList) CheckNetworkConfigIpamIPRange(config *types.IPAMPool) {
	if !c.supported("networks.ipam.config.ip_range") && config.IPRange != "" {
		config.IPRange = ""
		c.Unsupported("networks.ipam.config.ip_range")
	}
}

func (c *AllowList) CheckNetworkConfigIpamAuxiliaryAddresses(config *types.IPAMPool) {
	if !c.supported("networks.ipam.config.aux_addresses") && len(config.AuxiliaryAddresses) > 0 {
		config.AuxiliaryAddresses = nil
		c.Unsupported("networks.ipam.config.aux_addresses")
	}
}

func (c *AllowList) CheckNetworkConfigExternal(config *types.NetworkConfig) {
	if !c.supported("networks.external") && config.External.External {
		config.External.External = false
		c.Unsupported("networks.external")
	}
}

func (c *AllowList) CheckNetworkConfigInternal(config *types.NetworkConfig) {
	if !c.supported("networks.internal") && config.Internal {
		config.Internal = false
		c.Unsupported("networks.internal")
	}
}

func (c *AllowList) CheckNetworkConfigAttachable(config *types.NetworkConfig) {
	if !c.supported("networks.attachable") && config.Attachable {
		config.Attachable = false
		c.Unsupported("networks.attachable")
	}
}

func (c *AllowList) CheckNetworkConfigLabels(config *types.NetworkConfig) {
	if !c.supported("networks.labels") && len(config.Labels) != 0 {
		config.Labels = nil
		c.Unsupported("networks.labels")
	}
}
