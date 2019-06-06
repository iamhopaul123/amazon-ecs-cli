// Copyright 2015-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package converter implements the logic to translate an ecs.TaskDefinition
// structure to a docker compose schema, which will be written to a
// docker-compose.local.yml file.

package converter

import (
	"fmt"
	"strings"
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"

	composeV3 "github.com/docker/cli/cli/compose/types"
	"github.com/docker/go-units"
	"gopkg.in/yaml.v2"
)

// TODO wrap compose type?
// type Compose struct {
// }

// type Config struct {
// 	Filename string `yaml:"-"`
// 	Version  string
// 	Services Services
// 	Networks map[string]NetworkConfig   `yaml:",omitempty"`
// 	Volumes  map[string]VolumeConfig    `yaml:",omitempty"`
// 	Secrets  map[string]SecretConfig    `yaml:",omitempty"`
// 	Configs  map[string]ConfigObjConfig `yaml:",omitempty"`
// 	Extras   map[string]interface{}     `yaml:",inline"`
// }

// LinuxParams is a shim between members of ecs.LinuxParamters and their
// corresponding fields in the Docker Compose V3 ServiceConfig
type LinuxParams struct {
		CapAdd  []string
		CapDrop []string
		Devices []string
		Init    *bool
		Tmpfs   []string
		ShmSize string
}

// ConvertToDockerCompose creates the payload from an ECS Task Definition to be written as a docker compose file
func ConvertToDockerCompose(taskDefinition *ecs.TaskDefinition) ([]byte, error) {
	services := []composeV3.ServiceConfig{}
	for _, containerDefinition := range taskDefinition.ContainerDefinitions {
		service, err := convertToComposeService(containerDefinition)
		if err == nil {
			services = append(services, service)
		}
	}

	data, err := yaml.Marshal(&composeV3.Config{
		Filename: "docker-compose.local.yml",
		Version: "3.0",
		Services: services,
		// Volumes: taskDefinition.Volumes,
	})

	if err != nil {
		return nil, err
	}

	return data, nil
}

func convertToComposeService(containerDefinition *ecs.ContainerDefinition) (composeV3.ServiceConfig, error) {
	linuxParams := convertLinuxParameters(containerDefinition.LinuxParameters)
	tmpfs := linuxParams.Tmpfs
	init := linuxParams.Init
	// devices := aws.StringValueSlice(containerDefinition.LinuxParameters.Devices)

	ulimits, _ := convertUlimits(containerDefinition.Ulimits)

	service := composeV3.ServiceConfig{
		Name: aws.StringValue(containerDefinition.Name),
		Image: aws.StringValue(containerDefinition.Image),
		DNS: aws.StringValueSlice(containerDefinition.DnsServers),
		DNSSearch: aws.StringValueSlice(containerDefinition.DnsSearchDomains),
		Command: aws.StringValueSlice(containerDefinition.Command),
		Entrypoint: aws.StringValueSlice(containerDefinition.EntryPoint),
		Links: aws.StringValueSlice(containerDefinition.Links),
		Hostname: aws.StringValue(containerDefinition.Hostname),
		SecurityOpt: aws.StringValueSlice(containerDefinition.DockerSecurityOptions),
		WorkingDir: aws.StringValue(containerDefinition.WorkingDirectory),
		User: aws.StringValue(containerDefinition.User),
		Tty: aws.BoolValue(containerDefinition.PseudoTerminal),
		Privileged: aws.BoolValue(containerDefinition.Privileged),
		ReadOnly: aws.BoolValue(containerDefinition.ReadonlyRootFilesystem),
		Ulimits: ulimits,

		Tmpfs: tmpfs,
		Init: init,

		// ShmSize: shmSize,
		// Devices: devices,
		// CapAdd: containerDefinition.LinuxParameters.KernalCapabilities.Add
		// CapDrop: containerDefinition.LinuxParameters.KernalCapabilities.Drop
		// =======

		// CapAdd          []string                         `mapstructure:"cap_add" yaml:"cap_add,omitempty"`
		// CapDrop         []string                         `mapstructure:"cap_drop" yaml:"cap_drop,omitempty"`
		// Devices         []string                         `yaml:",omitempty"`
		// Init            *bool                            `yaml:",omitempty"`


		// Environment     MappingWithEquals                `yaml:",omitempty"`
		// ExtraHosts      HostsList                        `mapstructure:"extra_hosts" yaml:"extra_hosts,omitempty"`
		// HealthCheck     *HealthCheckConfig               `yaml:",omitempty"`
		// Labels          Labels                           `yaml:",omitempty"`
		// Logging         *LoggingConfig                   `yaml:",omitempty"`
		// Volumes         []ServiceVolumeConfig            `yaml:",omitempty"`
	}


	fmt.Printf("\nCOMPOSE SERVICE: %+v\n\n", service)
	return service, nil
}

// FIXME
func convertLinuxParameters(params *ecs.LinuxParameters) LinuxParams {
	if params == nil {
		return LinuxParams{}
	}

	tmpfs, _ := convertToTmpfs(params.Tmpfs)
	init := params.InitProcessEnabled

	return LinuxParams {
		Tmpfs: tmpfs,
		Init: init,
	}
}

func convertToTmpfs(mounts []*ecs.Tmpfs) ([]string, error) {
	if len(mounts) == 0 {
		return nil, nil
	}

	out := []string{}

	for _, mount := range mounts {

		if mount.ContainerPath == nil || mount.Size == nil {
			return nil, errors.New("You must specify the path and size for tmpfs mounts")
		}

		path := aws.StringValue(mount.ContainerPath)
		size := aws.Int64Value(mount.Size) * units.MiB

		composeSize := fmt.Sprintf("size=%s", units.BytesSize(float64(size)))

		tmpfs := strings.Join([]string{path, composeSize}, ":")

		if mount.MountOptions != nil {
			opts := aws.StringValueSlice(mount.MountOptions)
			composeOpts := strings.Join(opts, ",")
			tmpfs = strings.Join([]string{tmpfs, composeOpts}, ",")
		}

		out = append(out, tmpfs)
	}

	return out, nil
}

func convertUlimits(ulimits []*ecs.Ulimit) (map[string]*composeV3.UlimitsConfig, error) {
	if len(ulimits) == 0 {
		return nil, nil
	}

	out := make(map[string]*composeV3.UlimitsConfig)

	for _, ulimit := range ulimits {
		out[aws.StringValue(ulimit.Name)] = &composeV3.UlimitsConfig{
			Soft: int(aws.Int64Value(ulimit.SoftLimit)),
			Hard: int(aws.Int64Value(ulimit.HardLimit)),
		}
	}

	return out, nil
}
