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
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/types"
)

func fullExampleConfig(workingDir, homeDir string) *types.Config {
	return &types.Config{
		Services: services(workingDir, homeDir),
		Networks: networks(),
		Volumes:  volumes(),
		Configs:  configs(workingDir),
		Secrets:  secrets(workingDir),
		Extensions: map[string]interface{}{
			"x-foo": "bar",
			"x-bar": "baz",
			"x-nested": map[string]interface{}{
				"foo": "bar",
				"bar": "baz",
			},
		},
	}
}

func services(workingDir, homeDir string) []types.ServiceConfig {
	return []types.ServiceConfig{
		{
			Name: "foo",

			Build: &types.BuildConfig{
				Context:    "./dir",
				Dockerfile: "Dockerfile",
				Args:       map[string]*string{"foo": strPtr("bar")},
				Target:     "foo",
				Network:    "foo",
				CacheFrom:  []string{"foo", "bar"},
				Labels:     map[string]string{"FOO": "BAR"},
			},
			CapAdd:       []string{"ALL"},
			CapDrop:      []string{"NET_ADMIN", "SYS_ADMIN"},
			CgroupParent: "m-executor-abcd",
			Command:      []string{"bundle", "exec", "thin", "-p", "3000"},
			Configs: []types.ServiceConfigObjConfig{
				{
					Source: "config1",
				},
				{
					Source: "config2",
					Target: "/my_config",
					UID:    "103",
					GID:    "103",
					Mode:   uint32Ptr(0440),
				},
			},
			ContainerName: "my-web-container",
			DependsOn: types.DependsOnConfig{
				"db":    {Condition: types.ServiceConditionStarted},
				"redis": {Condition: types.ServiceConditionStarted},
			},
			Deploy: &types.DeployConfig{
				Mode:     "replicated",
				Replicas: uint64Ptr(6),
				Labels:   map[string]string{"FOO": "BAR"},
				RollbackConfig: &types.UpdateConfig{
					Parallelism:     uint64Ptr(3),
					Delay:           types.Duration(10 * time.Second),
					FailureAction:   "continue",
					Monitor:         types.Duration(60 * time.Second),
					MaxFailureRatio: 0.3,
					Order:           "start-first",
				},
				UpdateConfig: &types.UpdateConfig{
					Parallelism:     uint64Ptr(3),
					Delay:           types.Duration(10 * time.Second),
					FailureAction:   "continue",
					Monitor:         types.Duration(60 * time.Second),
					MaxFailureRatio: 0.3,
					Order:           "start-first",
				},
				Resources: types.Resources{
					Limits: &types.Resource{
						NanoCPUs:    "0.001",
						MemoryBytes: 50 * 1024 * 1024,
					},
					Reservations: &types.Resource{
						NanoCPUs:    "0.0001",
						MemoryBytes: 20 * 1024 * 1024,
						GenericResources: []types.GenericResource{
							{
								DiscreteResourceSpec: &types.DiscreteGenericResource{
									Kind:  "gpu",
									Value: 2,
								},
							},
							{
								DiscreteResourceSpec: &types.DiscreteGenericResource{
									Kind:  "ssd",
									Value: 1,
								},
							},
						},
					},
				},
				RestartPolicy: &types.RestartPolicy{
					Condition:   types.RestartPolicyOnFailure,
					Delay:       durationPtr(5 * time.Second),
					MaxAttempts: uint64Ptr(3),
					Window:      durationPtr(2 * time.Minute),
				},
				Placement: types.Placement{
					Constraints: []string{"node=foo"},
					MaxReplicas: uint64(5),
					Preferences: []types.PlacementPreferences{
						{
							Spread: "node.labels.az",
						},
					},
				},
				EndpointMode: "dnsrr",
			},
			Devices:    []string{"/dev/ttyUSB0:/dev/ttyUSB0"},
			DNS:        []string{"8.8.8.8", "9.9.9.9"},
			DNSSearch:  []string{"dc1.example.com", "dc2.example.com"},
			DomainName: "foo.com",
			Entrypoint: []string{"/code/entrypoint.sh", "-p", "3000"},
			Environment: map[string]*string{
				"FOO": strPtr("foo_from_env_file"),
				"BAR": strPtr("bar_from_env_file_2"),
				"BAZ": strPtr("baz_from_service_def"),
				"QUX": strPtr("qux_from_environment"),
			},
			EnvFile: []string{
				"./example1.env",
				"./example2.env",
			},
			Expose: []string{"3000", "8000"},
			ExternalLinks: []string{
				"redis_1",
				"project_db_1:mysql",
				"project_db_1:postgresql",
			},
			ExtraHosts: []string{
				"somehost:162.242.195.82",
				"otherhost:50.31.209.229",
			},
			Extensions: map[string]interface{}{
				"x-bar": "baz",
				"x-foo": "bar",
			},
			HealthCheck: &types.HealthCheckConfig{
				Test:        types.HealthCheckTest([]string{"CMD-SHELL", "echo \"hello world\""}),
				Interval:    durationPtr(10 * time.Second),
				Timeout:     durationPtr(1 * time.Second),
				Retries:     uint64Ptr(5),
				StartPeriod: durationPtr(15 * time.Second),
			},
			Hostname: "foo",
			Image:    "redis",
			Ipc:      "host",
			Labels: map[string]string{
				"com.example.description": "Accounting webapp",
				"com.example.number":      "42",
				"com.example.empty-label": "",
			},
			Links: []string{
				"db",
				"db:database",
				"redis",
			},
			Logging: &types.LoggingConfig{
				Driver: "syslog",
				Options: map[string]string{
					"syslog-address": "tcp://192.168.0.42:123",
				},
			},
			MacAddress:  "02:42:ac:11:65:43",
			NetworkMode: "container:0cfeab0f748b9a743dc3da582046357c6ef497631c1a016d28d2bf9b4f899f7b",
			Networks: map[string]*types.ServiceNetworkConfig{
				"some-network": {
					Aliases:     []string{"alias1", "alias3"},
					Ipv4Address: "",
					Ipv6Address: "",
				},
				"other-network": {
					Ipv4Address: "172.16.238.10",
					Ipv6Address: "2001:3984:3989::10",
				},
				"other-other-network": nil,
			},
			Pid: "host",
			Ports: []types.ServicePortConfig{
				//"3000",
				{
					Mode:     "ingress",
					Target:   3000,
					Protocol: "tcp",
				},
				{
					Mode:     "ingress",
					Target:   3001,
					Protocol: "tcp",
				},
				{
					Mode:     "ingress",
					Target:   3002,
					Protocol: "tcp",
				},
				{
					Mode:     "ingress",
					Target:   3003,
					Protocol: "tcp",
				},
				{
					Mode:     "ingress",
					Target:   3004,
					Protocol: "tcp",
				},
				{
					Mode:     "ingress",
					Target:   3005,
					Protocol: "tcp",
				},
				//"8000:8000",
				{
					Mode:      "ingress",
					Target:    8000,
					Published: 8000,
					Protocol:  "tcp",
				},
				//"9090-9091:8080-8081",
				{
					Mode:      "ingress",
					Target:    8080,
					Published: 9090,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					Target:    8081,
					Published: 9091,
					Protocol:  "tcp",
				},
				//"49100:22",
				{
					Mode:      "ingress",
					Target:    22,
					Published: 49100,
					Protocol:  "tcp",
				},
				//"127.0.0.1:8001:8001",
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    8001,
					Published: 8001,
					Protocol:  "tcp",
				},
				//"127.0.0.1:5000-5010:5000-5010",
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5000,
					Published: 5000,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5001,
					Published: 5001,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5002,
					Published: 5002,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5003,
					Published: 5003,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5004,
					Published: 5004,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5005,
					Published: 5005,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5006,
					Published: 5006,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5007,
					Published: 5007,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5008,
					Published: 5008,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5009,
					Published: 5009,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					HostIP:    "127.0.0.1",
					Target:    5010,
					Published: 5010,
					Protocol:  "tcp",
				},
			},
			Privileged: true,
			ReadOnly:   true,
			Restart:    types.RestartPolicyAlways,
			Scale:      1,
			Secrets: []types.ServiceSecretConfig{
				{
					Source: "secret1",
				},
				{
					Source: "secret2",
					Target: "my_secret",
					UID:    "103",
					GID:    "103",
					Mode:   uint32Ptr(0440),
				},
			},
			SecurityOpt: []string{
				"label=level:s0:c100,c200",
				"label=type:svirt_apache_t",
			},
			StdinOpen:       true,
			StopSignal:      "SIGUSR1",
			StopGracePeriod: durationPtr(20 * time.Second),
			Sysctls: map[string]string{
				"net.core.somaxconn":      "1024",
				"net.ipv4.tcp_syncookies": "0",
			},
			Tmpfs: []string{"/run", "/tmp"},
			Tty:   true,
			Ulimits: map[string]*types.UlimitsConfig{
				"nproc": {
					Single: 65535,
				},
				"nofile": {
					Soft: 20000,
					Hard: 40000,
				},
			},
			User: "someone",
			Volumes: []types.ServiceVolumeConfig{
				{Target: "/var/lib/mysql", Type: "volume", Volume: &types.ServiceVolumeVolume{}},
				{Source: "/opt/data", Target: "/var/lib/mysql", Type: "bind", Bind: &types.ServiceVolumeBind{CreateHostPath: true}},
				{Source: workingDir, Target: "/code", Type: "bind", Bind: &types.ServiceVolumeBind{CreateHostPath: true}},
				{Source: filepath.Join(workingDir, "static"), Target: "/var/www/html", Type: "bind", Bind: &types.ServiceVolumeBind{CreateHostPath: true}},
				{Source: filepath.Join(homeDir, "/configs"), Target: "/etc/configs/", Type: "bind", ReadOnly: true, Bind: &types.ServiceVolumeBind{CreateHostPath: true}},
				{Source: "datavolume", Target: "/var/lib/mysql", Type: "volume", Volume: &types.ServiceVolumeVolume{}},
				{Source: filepath.Join(workingDir, "opt"), Target: "/opt", Consistency: "cached", Type: "bind"},
				{Target: "/opt", Type: "tmpfs", Tmpfs: &types.ServiceVolumeTmpfs{
					Size: int64(10000),
				}},
			},
			WorkingDir: "/code",
		},
	}
}

func networks() map[string]types.NetworkConfig {
	return map[string]types.NetworkConfig{
		"some-network": {},

		"other-network": {
			Driver: "overlay",
			DriverOpts: map[string]string{
				"foo": "bar",
				"baz": "1",
			},
			Ipam: types.IPAMConfig{
				Driver: "overlay",
				Config: []*types.IPAMPool{
					{
						Subnet:  "172.28.0.0/16",
						IPRange: "172.28.5.0/24",
						Gateway: "172.28.5.254",
						AuxiliaryAddresses: map[string]string{
							"host1": "172.28.1.5",
							"host2": "172.28.1.6",
							"host3": "172.28.1.7",
						},
					},
					{
						Subnet:  "2001:3984:3989::/64",
						Gateway: "2001:3984:3989::1",
					},
				},
			},
			Labels: map[string]string{
				"foo": "bar",
			},
		},

		"external-network": {
			Name:     "external-network",
			External: types.External{External: true},
		},

		"other-external-network": {
			Name:     "my-cool-network",
			External: types.External{External: true},
			Extensions: map[string]interface{}{
				"x-bar": "baz",
				"x-foo": "bar",
			},
		},
	}
}

func volumes() map[string]types.VolumeConfig {
	return map[string]types.VolumeConfig{
		"some-volume": {},
		"other-volume": {
			Driver: "flocker",
			DriverOpts: map[string]string{
				"foo": "bar",
				"baz": "1",
			},
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		"another-volume": {
			Name:   "user_specified_name",
			Driver: "vsphere",
			DriverOpts: map[string]string{
				"foo": "bar",
				"baz": "1",
			},
		},
		"external-volume": {
			Name:     "external-volume",
			External: types.External{External: true},
		},
		"other-external-volume": {
			Name:     "my-cool-volume",
			External: types.External{External: true},
		},
		"external-volume3": {
			Name:     "this-is-volume3",
			External: types.External{External: true},
			Extensions: map[string]interface{}{
				"x-bar": "baz",
				"x-foo": "bar",
			},
		},
	}
}

func configs(workingDir string) map[string]types.ConfigObjConfig {
	return map[string]types.ConfigObjConfig{
		"config1": {
			File: filepath.Join(workingDir, "config_data"),
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		"config2": {
			Name:     "my_config",
			External: types.External{External: true},
		},
		"config3": {
			Name:     "config3",
			External: types.External{External: true},
		},
		"config4": {
			Name: "foo",
			File: workingDir,
			Extensions: map[string]interface{}{
				"x-bar": "baz",
				"x-foo": "bar",
			},
		},
	}
}

func secrets(workingDir string) map[string]types.SecretConfig {
	return map[string]types.SecretConfig{
		"secret1": {
			File: filepath.Join(workingDir, "secret_data"),
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		"secret2": {
			Name:     "my_secret",
			External: types.External{External: true},
		},
		"secret3": {
			Name:     "secret3",
			External: types.External{External: true},
		},
		"secret4": {
			Name: "bar",
			File: workingDir,
			Extensions: map[string]interface{}{
				"x-bar": "baz",
				"x-foo": "bar",
			},
		},
	}
}

func fullExampleYAML(workingDir, homeDir string) string {
	return fmt.Sprintf(`services:
  foo:
    build:
      context: ./dir
      dockerfile: Dockerfile
      args:
        foo: bar
      labels:
        FOO: BAR
      cache_from:
      - foo
      - bar
      network: foo
      target: foo
    cap_add:
    - ALL
    cap_drop:
    - NET_ADMIN
    - SYS_ADMIN
    cgroup_parent: m-executor-abcd
    command:
    - bundle
    - exec
    - thin
    - -p
    - "3000"
    configs:
    - source: config1
    - source: config2
      target: /my_config
      uid: "103"
      gid: "103"
      mode: 288
    container_name: my-web-container
    depends_on:
      db:
        condition: service_started
      redis:
        condition: service_started
    deploy:
      mode: replicated
      replicas: 6
      labels:
        FOO: BAR
      update_config:
        parallelism: 3
        delay: 10s
        failure_action: continue
        monitor: 1m0s
        max_failure_ratio: 0.3
        order: start-first
      rollback_config:
        parallelism: 3
        delay: 10s
        failure_action: continue
        monitor: 1m0s
        max_failure_ratio: 0.3
        order: start-first
      resources:
        limits:
          cpus: "0.001"
          memory: "52428800"
        reservations:
          cpus: "0.0001"
          memory: "20971520"
          generic_resources:
          - discrete_resource_spec:
              kind: gpu
              value: 2
          - discrete_resource_spec:
              kind: ssd
              value: 1
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3
        window: 2m0s
      placement:
        constraints:
        - node=foo
        preferences:
        - spread: node.labels.az
        max_replicas_per_node: 5
      endpoint_mode: dnsrr
    devices:
    - /dev/ttyUSB0:/dev/ttyUSB0
    dns:
    - 8.8.8.8
    - 9.9.9.9
    dns_search:
    - dc1.example.com
    - dc2.example.com
    domainname: foo.com
    entrypoint:
    - /code/entrypoint.sh
    - -p
    - "3000"
    environment:
      BAR: bar_from_env_file_2
      BAZ: baz_from_service_def
      FOO: foo_from_env_file
      QUX: qux_from_environment
    env_file:
    - ./example1.env
    - ./example2.env
    expose:
    - "3000"
    - "8000"
    external_links:
    - redis_1
    - project_db_1:mysql
    - project_db_1:postgresql
    extra_hosts:
    - somehost:162.242.195.82
    - otherhost:50.31.209.229
    hostname: foo
    healthcheck:
      test:
      - CMD-SHELL
      - echo "hello world"
      timeout: 1s
      interval: 10s
      retries: 5
      start_period: 15s
    image: redis
    ipc: host
    labels:
      com.example.description: Accounting webapp
      com.example.empty-label: ""
      com.example.number: "42"
    links:
    - db
    - db:database
    - redis
    logging:
      driver: syslog
      options:
        syslog-address: tcp://192.168.0.42:123
    mac_address: 02:42:ac:11:65:43
    network_mode: container:0cfeab0f748b9a743dc3da582046357c6ef497631c1a016d28d2bf9b4f899f7b
    networks:
      other-network:
        ipv4_address: 172.16.238.10
        ipv6_address: 2001:3984:3989::10
      other-other-network: null
      some-network:
        aliases:
        - alias1
        - alias3
    pid: host
    ports:
    - mode: ingress
      target: 3000
      protocol: tcp
    - mode: ingress
      target: 3001
      protocol: tcp
    - mode: ingress
      target: 3002
      protocol: tcp
    - mode: ingress
      target: 3003
      protocol: tcp
    - mode: ingress
      target: 3004
      protocol: tcp
    - mode: ingress
      target: 3005
      protocol: tcp
    - mode: ingress
      target: 8000
      published: 8000
      protocol: tcp
    - mode: ingress
      target: 8080
      published: 9090
      protocol: tcp
    - mode: ingress
      target: 8081
      published: 9091
      protocol: tcp
    - mode: ingress
      target: 22
      published: 49100
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 8001
      published: 8001
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5000
      published: 5000
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5001
      published: 5001
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5002
      published: 5002
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5003
      published: 5003
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5004
      published: 5004
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5005
      published: 5005
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5006
      published: 5006
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5007
      published: 5007
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5008
      published: 5008
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5009
      published: 5009
      protocol: tcp
    - mode: ingress
      host_ip: 127.0.0.1
      target: 5010
      published: 5010
      protocol: tcp
    privileged: true
    read_only: true
    restart: always
    secrets:
    - source: secret1
    - source: secret2
      target: my_secret
      uid: "103"
      gid: "103"
      mode: 288
    security_opt:
    - label=level:s0:c100,c200
    - label=type:svirt_apache_t
    stdin_open: true
    stop_grace_period: 20s
    stop_signal: SIGUSR1
    sysctls:
      net.core.somaxconn: "1024"
      net.ipv4.tcp_syncookies: "0"
    tmpfs:
    - /run
    - /tmp
    tty: true
    ulimits:
      nofile:
        soft: 20000
        hard: 40000
      nproc: 65535
    user: someone
    volumes:
    - type: volume
      target: /var/lib/mysql
      volume: {}
    - type: bind
      source: /opt/data
      target: /var/lib/mysql
      bind:
        create_host_path: true
    - type: bind
      source: %s
      target: /code
      bind:
        create_host_path: true
    - type: bind
      source: %s
      target: /var/www/html
      bind:
        create_host_path: true
    - type: bind
      source: %s
      target: /etc/configs/
      read_only: true
      bind:
        create_host_path: true
    - type: volume
      source: datavolume
      target: /var/lib/mysql
      volume: {}
    - type: bind
      source: %s
      target: /opt
      consistency: cached
    - type: tmpfs
      target: /opt
      tmpfs:
        size: 10000
    working_dir: /code
    x-bar: baz
    x-foo: bar
networks:
  external-network:
    name: external-network
    external: true
  other-external-network:
    name: my-cool-network
    external: true
    x-bar: baz
    x-foo: bar
  other-network:
    driver: overlay
    driver_opts:
      baz: "1"
      foo: bar
    ipam:
      driver: overlay
      config:
      - subnet: 172.28.0.0/16
        gateway: 172.28.5.254
        ip_range: 172.28.5.0/24
        aux_addresses:
          host1: 172.28.1.5
          host2: 172.28.1.6
          host3: 172.28.1.7
      - subnet: 2001:3984:3989::/64
        gateway: 2001:3984:3989::1
    labels:
      foo: bar
  some-network: {}
volumes:
  another-volume:
    name: user_specified_name
    driver: vsphere
    driver_opts:
      baz: "1"
      foo: bar
  external-volume:
    name: external-volume
    external: true
  external-volume3:
    name: this-is-volume3
    external: true
    x-bar: baz
    x-foo: bar
  other-external-volume:
    name: my-cool-volume
    external: true
  other-volume:
    driver: flocker
    driver_opts:
      baz: "1"
      foo: bar
    labels:
      foo: bar
  some-volume: {}
secrets:
  secret1:
    file: %s
    labels:
      foo: bar
  secret2:
    name: my_secret
    external: true
  secret3:
    name: secret3
    external: true
  secret4:
    name: bar
    file: %s
    x-bar: baz
    x-foo: bar
configs:
  config1:
    file: %s
    labels:
      foo: bar
  config2:
    name: my_config
    external: true
  config3:
    name: config3
    external: true
  config4:
    name: foo
    file: %s
    x-bar: baz
    x-foo: bar
x-bar: baz
x-foo: bar
x-nested:
  bar: baz
  foo: bar
`,
		filepath.Join(workingDir),
		filepath.Join(workingDir, "static"),
		filepath.Join(homeDir, "configs"),
		filepath.Join(workingDir, "opt"),
		filepath.Join(workingDir, "secret_data"),
		filepath.Join(workingDir),
		filepath.Join(workingDir, "config_data"),
		filepath.Join(workingDir))
}

func fullExampleJSON(workingDir, homeDir string) string {
	return fmt.Sprintf(`{
  "configs": {
    "config1": {
      "file": "%s",
      "external": false,
      "labels": {
        "foo": "bar"
      }
    },
    "config2": {
      "name": "my_config",
      "external": true
    },
    "config3": {
      "name": "config3",
      "external": true
    },
    "config4": {
      "name": "foo",
      "file": "%s",
      "external": false
    }
  },
  "networks": {
    "external-network": {
      "name": "external-network",
      "ipam": {},
      "external": true
    },
    "other-external-network": {
      "name": "my-cool-network",
      "ipam": {},
      "external": true
    },
    "other-network": {
      "driver": "overlay",
      "driver_opts": {
        "baz": "1",
        "foo": "bar"
      },
      "ipam": {
        "driver": "overlay",
        "config": [
          {
            "subnet": "172.28.0.0/16",
            "gateway": "172.28.5.254",
            "ip_range": "172.28.5.0/24",
            "aux_addresses": {
              "host1": "172.28.1.5",
              "host2": "172.28.1.6",
              "host3": "172.28.1.7"
            }
          },
          {
            "subnet": "2001:3984:3989::/64",
            "gateway": "2001:3984:3989::1"
          }
        ]
      },
      "external": false,
      "labels": {
        "foo": "bar"
      }
    },
    "some-network": {
      "ipam": {},
      "external": false
    }
  },
  "secrets": {
    "secret1": {
      "file": "%s",
      "external": false,
      "labels": {
        "foo": "bar"
      }
    },
    "secret2": {
      "name": "my_secret",
      "external": true
    },
    "secret3": {
      "name": "secret3",
      "external": true
    },
    "secret4": {
      "name": "bar",
      "file": "%s",
      "external": false
    }
  },
  "services": {
    "foo": {
      "build": {
        "context": "./dir",
        "dockerfile": "Dockerfile",
        "args": {
          "foo": "bar"
        },
        "labels": {
          "FOO": "BAR"
        },
        "cache_from": [
          "foo",
          "bar"
        ],
        "network": "foo",
        "target": "foo"
      },
      "cap_add": [
        "ALL"
      ],
      "cap_drop": [
        "NET_ADMIN",
        "SYS_ADMIN"
      ],
      "cgroup_parent": "m-executor-abcd",
      "command": [
        "bundle",
        "exec",
        "thin",
        "-p",
        "3000"
      ],
      "configs": [
        {
          "source": "config1"
        },
        {
          "source": "config2",
          "target": "/my_config",
          "uid": "103",
          "gid": "103",
          "mode": 288
        }
      ],
      "container_name": "my-web-container",
      "depends_on": {
        "db": {
          "condition": "service_started"
        },
        "redis": {
          "condition": "service_started"
        }
      },
      "deploy": {
        "mode": "replicated",
        "replicas": 6,
        "labels": {
          "FOO": "BAR"
        },
        "update_config": {
          "parallelism": 3,
          "delay": "10s",
          "failure_action": "continue",
          "monitor": "1m0s",
          "max_failure_ratio": 0.3,
          "order": "start-first"
        },
        "rollback_config": {
          "parallelism": 3,
          "delay": "10s",
          "failure_action": "continue",
          "monitor": "1m0s",
          "max_failure_ratio": 0.3,
          "order": "start-first"
        },
        "resources": {
          "limits": {
            "cpus": "0.001",
            "memory": "52428800"
          },
          "reservations": {
            "cpus": "0.0001",
            "memory": "20971520",
            "generic_resources": [
              {
                "discrete_resource_spec": {
                  "kind": "gpu",
                  "value": 2
                }
              },
              {
                "discrete_resource_spec": {
                  "kind": "ssd",
                  "value": 1
                }
              }
            ]
          }
        },
        "restart_policy": {
          "condition": "on-failure",
          "delay": "5s",
          "max_attempts": 3,
          "window": "2m0s"
        },
        "placement": {
          "constraints": [
            "node=foo"
          ],
          "preferences": [
            {
              "spread": "node.labels.az"
            }
          ],
          "max_replicas_per_node": 5
        },
        "endpoint_mode": "dnsrr"
      },
      "devices": [
        "/dev/ttyUSB0:/dev/ttyUSB0"
      ],
      "dns": [
        "8.8.8.8",
        "9.9.9.9"
      ],
      "dns_search": [
        "dc1.example.com",
        "dc2.example.com"
      ],
      "domainname": "foo.com",
      "entrypoint": [
        "/code/entrypoint.sh",
        "-p",
        "3000"
      ],
      "environment": {
        "BAR": "bar_from_env_file_2",
        "BAZ": "baz_from_service_def",
        "FOO": "foo_from_env_file",
        "QUX": "qux_from_environment"
      },
      "env_file": [
        "./example1.env",
        "./example2.env"
      ],
      "expose": [
        "3000",
        "8000"
      ],
      "external_links": [
        "redis_1",
        "project_db_1:mysql",
        "project_db_1:postgresql"
      ],
      "extra_hosts": [
        "somehost:162.242.195.82",
        "otherhost:50.31.209.229"
      ],
      "hostname": "foo",
      "healthcheck": {
        "test": [
          "CMD-SHELL",
          "echo \"hello world\""
        ],
        "timeout": "1s",
        "interval": "10s",
        "retries": 5,
        "start_period": "15s"
      },
      "image": "redis",
      "ipc": "host",
      "labels": {
        "com.example.description": "Accounting webapp",
        "com.example.empty-label": "",
        "com.example.number": "42"
      },
      "links": [
        "db",
        "db:database",
        "redis"
      ],
      "logging": {
        "driver": "syslog",
        "options": {
          "syslog-address": "tcp://192.168.0.42:123"
        }
      },
      "mac_address": "02:42:ac:11:65:43",
      "network_mode": "container:0cfeab0f748b9a743dc3da582046357c6ef497631c1a016d28d2bf9b4f899f7b",
      "networks": {
        "other-network": {
          "ipv4_address": "172.16.238.10",
          "ipv6_address": "2001:3984:3989::10"
        },
        "other-other-network": null,
        "some-network": {
          "aliases": [
            "alias1",
            "alias3"
          ]
        }
      },
      "pid": "host",
      "ports": [
        {
          "mode": "ingress",
          "target": 3000,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "target": 3001,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "target": 3002,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "target": 3003,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "target": 3004,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "target": 3005,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "target": 8000,
          "published": 8000,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "target": 8080,
          "published": 9090,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "target": 8081,
          "published": 9091,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "target": 22,
          "published": 49100,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 8001,
          "published": 8001,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5000,
          "published": 5000,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5001,
          "published": 5001,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5002,
          "published": 5002,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5003,
          "published": 5003,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5004,
          "published": 5004,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5005,
          "published": 5005,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5006,
          "published": 5006,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5007,
          "published": 5007,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5008,
          "published": 5008,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5009,
          "published": 5009,
          "protocol": "tcp"
        },
        {
          "mode": "ingress",
          "host_ip": "127.0.0.1",
          "target": 5010,
          "published": 5010,
          "protocol": "tcp"
        }
      ],
      "privileged": true,
      "read_only": true,
      "restart": "always",
      "secrets": [
        {
          "source": "secret1"
        },
        {
          "source": "secret2",
          "target": "my_secret",
          "uid": "103",
          "gid": "103",
          "mode": 288
        }
      ],
      "security_opt": [
        "label=level:s0:c100,c200",
        "label=type:svirt_apache_t"
      ],
      "stdin_open": true,
      "stop_grace_period": "20s",
      "stop_signal": "SIGUSR1",
      "sysctls": {
        "net.core.somaxconn": "1024",
        "net.ipv4.tcp_syncookies": "0"
      },
      "tmpfs": [
        "/run",
        "/tmp"
      ],
      "tty": true,
      "ulimits": {
        "nofile": {
          "soft": 20000,
          "hard": 40000
        },
        "nproc": 65535
      },
      "user": "someone",
      "volumes": [
        {
          "type": "volume",
          "target": "/var/lib/mysql",
          "volume": {}
        },
        {
          "type": "bind",
          "source": "/opt/data",
          "target": "/var/lib/mysql",
          "bind": {
            "create_host_path": true
          }
        },
        {
          "type": "bind",
          "source": "%s",
          "target": "/code",
          "bind": {
            "create_host_path": true
          }
        },
        {
          "type": "bind",
          "source": "%s",
          "target": "/var/www/html",
          "bind": {
            "create_host_path": true
          }
        },
        {
          "type": "bind",
          "source": "%s",
          "target": "/etc/configs/",
          "read_only": true,
          "bind": {
            "create_host_path": true
          }
        },
        {
          "type": "volume",
          "source": "datavolume",
          "target": "/var/lib/mysql",
          "volume": {}
        },
        {
          "type": "bind",
          "source": "%s",
          "target": "/opt",
          "consistency": "cached"
        },
        {
          "type": "tmpfs",
          "target": "/opt",
          "tmpfs": {
            "size": 10000
          }
        }
      ],
      "working_dir": "/code"
    }
  },
  "volumes": {
    "another-volume": {
      "name": "user_specified_name",
      "driver": "vsphere",
      "driver_opts": {
        "baz": "1",
        "foo": "bar"
      },
      "external": false
    },
    "external-volume": {
      "name": "external-volume",
      "external": true
    },
    "external-volume3": {
      "name": "this-is-volume3",
      "external": true
    },
    "other-external-volume": {
      "name": "my-cool-volume",
      "external": true
    },
    "other-volume": {
      "driver": "flocker",
      "driver_opts": {
        "baz": "1",
        "foo": "bar"
      },
      "external": false,
      "labels": {
        "foo": "bar"
      }
    },
    "some-volume": {
      "external": false
    }
  },
  "x-bar": "baz",
  "x-foo": "bar",
  "x-nested": {
    "bar": "baz",
    "foo": "bar"
  }
}`,
		toPath(workingDir, "config_data"),
		toPath(workingDir),
		toPath(workingDir, "secret_data"),
		toPath(workingDir),
		toPath(workingDir),
		toPath(workingDir, "static"),
		toPath(homeDir, "configs"),
		toPath(workingDir, "opt"))
}

func toPath(path ...string) string {
	return strings.ReplaceAll(filepath.Join(path...), "\\", "\\\\")
}
