// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/util/passwd"
)

const (
	DefaultVersion             = "latest"
	DefaultOnecloudRegion      = "region0"
	DefaultOnecloudZone        = "zone0"
	DefaultImageRepository     = "registry.hub.docker.com/yunion"
	DefaultVPCId               = "default"
	DefaultGlanceStoreageSize  = "100G"
	DefaultInfluxdbStorageSize = "20G"
	// rancher local-path-provisioner: https://github.com/rancher/local-path-provisioner
	DefaultStorageClass = "local-path"

	DefaultInfluxdbImageVersion = "1.7.7"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_OnecloudCluster(obj *OnecloudCluster) {
	if _, ok := obj.GetLabels()[constants.InstanceLabelKey]; !ok {
		obj.SetLabels(map[string]string{constants.InstanceLabelKey: fmt.Sprintf("onecloud-cluster-%s", rand.String(4))})
	}
	SetDefaults_OnecloudClusterSpec(&obj.Spec)
}

func SetDefaults_OnecloudClusterSpec(obj *OnecloudClusterSpec) {
	SetDefaults_Mysql(&obj.Mysql)
	if obj.Region == "" {
		obj.Region = DefaultOnecloudRegion
	}
	if obj.Zone == "" {
		obj.Zone = DefaultOnecloudZone
	}
	if obj.Version == "" {
		obj.Version = DefaultVersion
	}
	if obj.ImageRepository == "" {
		obj.ImageRepository = DefaultImageRepository
	}

	SetDefaults_KeystoneSpec(&obj.Keystone, obj.ImageRepository, obj.Version)
	SetDefaults_RegionSpec(&obj.RegionServer, obj.ImageRepository, obj.Version)

	for cType, spec := range map[ComponentType]*DeploymentSpec{
		ClimcComponentType:        &obj.Climc,
		WebconsoleComponentType:   &obj.Webconsole,
		SchedulerComponentType:    &obj.Scheduler,
		LoggerComponentType:       &obj.Logger,
		YunionconfComponentType:   &obj.Yunionconf,
		APIGatewayComponentType:   &obj.APIGateway,
		WebComponentType:          &obj.Web,
		KubeServerComponentType:   &obj.KubeServer,
		CloudMonitorComponentType: &obj.CloudMonitor,
	} {
		SetDefaults_DeploymentSpec(spec, getImage(obj.ImageRepository, cType, obj.Version))
	}

	type stateDeploy struct {
		obj     *StatefulDeploymentSpec
		size    string
		version string
	}
	for cType, spec := range map[ComponentType]*stateDeploy{
		GlanceComponentType:      &stateDeploy{&obj.Glance, DefaultGlanceStoreageSize, obj.Version},
		InfluxdbComponentType:    &stateDeploy{&obj.Influxdb, DefaultInfluxdbStorageSize, DefaultInfluxdbImageVersion},
		YunionagentComponentType: &stateDeploy{&obj.Yunionagent, "1G", obj.Version},
	} {
		SetDefaults_StatefulDeploymentSpec(cType, spec.obj, spec.size, obj.ImageRepository, spec.version)
	}
}

func SetDefaults_Mysql(obj *Mysql) {
	if obj.Username == "" {
		obj.Username = "root"
	}
	if obj.Port == 0 {
		obj.Port = 3306
	}
}

func getImage(repo string, componentType ComponentType, version string) string {
	return fmt.Sprintf("%s/%s:%s", repo, componentType, version)
}

func SetDefaults_KeystoneSpec(obj *KeystoneSpec, imageRepo, version string) {
	SetDefaults_DeploymentSpec(&obj.DeploymentSpec, getImage(imageRepo, KeystoneComponentType, version))
	if obj.BootstrapPassword == "" {
		obj.BootstrapPassword = passwd.GeneratePassword()
	}
}

func SetDefaults_RegionSpec(obj *RegionSpec, imageRepo, version string) {
	SetDefaults_DeploymentSpec(&obj.DeploymentSpec, getImage(imageRepo, RegionComponentType, version))
}

func setPVCStoreage(obj *ContainerSpec, size string) {
	if obj.Requests == nil {
		obj.Requests = new(ResourceRequirement)
	}
	if obj.Requests.Storage == "" {
		obj.Requests.Storage = size
	}
}

func SetDefaults_StatefulDeploymentSpec(ctype ComponentType, obj *StatefulDeploymentSpec, defaultSize string, imageRepo, version string) {
	SetDefaults_DeploymentSpec(&obj.DeploymentSpec, getImage(imageRepo, ctype, version))
	if obj.StorageClassName == "" {
		obj.StorageClassName = DefaultStorageClass
	}
	setPVCStoreage(&obj.ContainerSpec, defaultSize)
}

func SetDefaults_DeploymentSpec(obj *DeploymentSpec, image string) {
	if obj.Replicas == 0 {
		obj.Replicas = 1
	}
	if obj.Image == "" {
		obj.Image = image
	}
	// add tolerations
	if len(obj.Tolerations) == 0 {
		obj.Tolerations = append(obj.Tolerations, []corev1.Toleration{
			{
				Key:    "node-role.kubernetes.io/master",
				Effect: corev1.TaintEffectNoSchedule,
			},
			{
				Key:    "node-role.kubernetes.io/controlplane",
				Effect: corev1.TaintEffectNoSchedule,
			},
		}...)
	}
}

func SetDefaults_OnecloudClusterConfig(obj *OnecloudClusterConfig) {
	SetDefaults_KeystoneConfig(&obj.Keystone)

	type userPort struct {
		user string
		port int
	}

	for opt, userPort := range map[*ServiceCommonOptions]userPort{
		&obj.Webconsole:   {constants.WebconsoleAdminUser, constants.WebconsolePort},
		&obj.APIGateway:   {constants.APIGatewayAdminUser, constants.APIGatewayPort},
		&obj.CloudMonitor: {constants.CloudMonitorAdminUser, 0},
	} {
		SetDefaults_ServiceCommonOptions(opt, userPort.user, userPort.port)
	}

	type userDBPort struct {
		user   string
		port   int
		db     string
		dbUser string
	}

	for opt, tmp := range map[*ServiceDBCommonOptions]userDBPort{
		&obj.RegionServer.ServiceDBCommonOptions: {constants.RegionAdminUser, constants.RegionPort, constants.RegionDB, constants.RegionDBUser},
		&obj.Glance.ServiceDBCommonOptions:       {constants.GlanceAdminUser, constants.GlanceAPIPort, constants.GlanceDB, constants.GlanceDBUser},
		&obj.Logger:                              {constants.LoggerAdminUser, constants.LoggerPort, constants.LoggerDB, constants.LoggerDBUser},
		&obj.Yunionagent:                         {constants.YunionAgentAdminUser, constants.YunionAgentPort, constants.YunionAgentDB, constants.YunionAgentDBUser},
		&obj.Yunionconf:                          {constants.YunionConfAdminUser, constants.YunionConfPort, constants.YunionConfDB, constants.YunionConfDBUser},
		&obj.KubeServer:                          {constants.KubeServerAdminUser, constants.KubeServerPort, constants.KubeServerDB, constants.KubeServerDBUser},
	} {
		SetDefaults_ServiceDBCommonOptions(opt, tmp.db, tmp.dbUser, tmp.user, tmp.port)
	}
}

func SetDefaults_ServiceBaseConfig(obj *ServiceBaseConfig, port int) {
	if obj.Port == 0 {
		obj.Port = port
	}
}

func SetDefaults_KeystoneConfig(obj *KeystoneConfig) {
	SetDefaults_ServiceBaseConfig(&obj.ServiceBaseConfig, constants.KeystonePublicPort)
	setDefaults_DBConfig(&obj.DB, constants.KeystoneDB, constants.KeystoneDBUser)
}

func SetDefaults_ServiceCommonOptions(obj *ServiceCommonOptions, user string, port int) {
	SetDefaults_ServiceBaseConfig(&obj.ServiceBaseConfig, port)
	setDefaults_CloudUser(&obj.CloudUser, user)
}

func SetDefaults_ServiceDBCommonOptions(obj *ServiceDBCommonOptions, db, dbUser string, svcUser string, port int) {
	setDefaults_DBConfig(&obj.DB, db, dbUser)
	SetDefaults_ServiceCommonOptions(&obj.ServiceCommonOptions, svcUser, port)
}

func setDefaults_DBConfig(obj *DBConfig, database string, username string) {
	if obj.Database == "" {
		obj.Database = database
	}
	if obj.Username == "" {
		obj.Username = username
	}
	if obj.Password == "" {
		obj.Password = passwd.GeneratePassword()
	}
}

func setDefaults_CloudUser(obj *CloudUser, username string) {
	if obj.Username == "" {
		obj.Username = username
	}
	if obj.Password == "" {
		obj.Password = passwd.GeneratePassword()
	}
}
