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

	"yunion.io/x/log"
	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/util/passwd"
)

const (
	DefaultVersion                 = "latest"
	DefaultOnecloudRegion          = "region0"
	DefaultOnecloudRegionDNSDomain = "cloud.onecloud.io"
	DefaultOnecloudZone            = "zone0"
	DefaultOnecloudWire            = "bcast0"
	DefaultImageRepository         = "registry.hub.docker.com/yunion"
	DefaultVPCId                   = "default"
	DefaultGlanceStorageSize       = "100G"
	DefaultMeterStorageSize        = "100G"
	DefaultInfluxdbStorageSize     = "20G"
	DefaultNotifyStorageSize       = "1G" // for plugin template
	DefaultBaremetalStorageSize    = "1G"
	DefaultEsxiAgentStorageSize    = "30G"
	// rancher local-path-provisioner: https://github.com/rancher/local-path-provisioner
	DefaultStorageClass = "local-path"

	DefaultOvnVersion   = "2.9.6"
	DefaultOvnImageName = "openvswitch"
	DefaultOvnImageTag  = DefaultOvnVersion + "-2"

	DefaultInfluxdbImageVersion = "1.7.7"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_OnecloudCluster(obj *OnecloudCluster) {
	if _, ok := obj.GetLabels()[constants.InstanceLabelKey]; !ok {
		obj.SetLabels(map[string]string{constants.InstanceLabelKey: fmt.Sprintf("onecloud-cluster-%s", rand.String(4))})
	}

	SetDefaults_OnecloudClusterSpec(&obj.Spec, IsEnterpriseEdition(obj))
}

func GetEdition(oc *OnecloudCluster) string {
	edition := constants.OnecloudCommunityEdition
	if oc.Annotations == nil {
		return edition
	}
	curEdition := oc.Annotations[constants.OnecloudEditionAnnotationKey]
	if curEdition == constants.OnecloudEnterpriseEdition {
		return curEdition
	}
	return edition
}

func IsEnterpriseEdition(oc *OnecloudCluster) bool {
	return GetEdition(oc) == constants.OnecloudEnterpriseEdition
}

func SetDefaults_OnecloudClusterSpec(obj *OnecloudClusterSpec, isEE bool) {
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
	SetDefaults_RegionDNSSpec(&obj.RegionDNS, obj.ImageRepository, obj.Version)

	for cType, spec := range map[ComponentType]*DeploymentSpec{
		ClimcComponentType:         &obj.Climc,
		WebconsoleComponentType:    &obj.Webconsole,
		SchedulerComponentType:     &obj.Scheduler,
		LoggerComponentType:        &obj.Logger,
		RegisterComponentType:      &obj.Register,
		BillingComponentType:       &obj.Billing,
		YunionconfComponentType:    &obj.Yunionconf,
		KubeServerComponentType:    &obj.KubeServer,
		AnsibleServerComponentType: &obj.AnsibleServer,
		CloudnetComponentType:      &obj.Cloudnet,
		CloudeventComponentType:    &obj.Cloudevent,
		S3gatewayComponentType:     &obj.S3gateway,
		DevtoolComponentType:       &obj.Devtool,
		AutoUpdateComponentType:    &obj.AutoUpdate,
		OvnNorthComponentType:      &obj.OvnNorth,
		VpcAgentComponentType:      &obj.VpcAgent,
		MonitorComponentType:       &obj.Monitor,
	} {
		SetDefaults_DeploymentSpec(spec, getImage(obj.ImageRepository, spec.Repository, cType, spec.ImageName, obj.Version, spec.Tag))
	}

	// CE or EE parts
	for cType, spec := range map[ComponentType]*DeploymentSpec{
		APIGatewayComponentType: &obj.APIGateway,
		WebComponentType:        &obj.Web,
	} {
		SetDefaults_DeploymentSpec(spec,
			getEditionImage(obj.ImageRepository, spec.Repository, cType, spec.ImageName, obj.Version, spec.Tag, isEE))
	}

	for cType, spec := range map[ComponentType]*DaemonSetSpec{
		HostComponentType:         &obj.HostAgent.DaemonSetSpec,
		HostDeployerComponentType: &obj.HostDeployer,
		YunionagentComponentType:  &obj.Yunionagent,
	} {
		SetDefaults_DaemonSetSpec(spec, getImage(obj.ImageRepository, spec.Repository, cType, spec.ImageName, obj.Version, spec.Tag))
	}
	obj.HostAgent.SdnAgent.Image = getImage(obj.ImageRepository, obj.HostAgent.Repository, "sdnagent", obj.HostAgent.ImageName, obj.Version, obj.HostAgent.Tag)

	// setting ovn image
	obj.HostAgent.OvnController.Image = getImage(
		obj.ImageRepository, obj.HostAgent.OvnController.Repository,
		DefaultOvnImageName, obj.HostAgent.OvnController.ImageName,
		DefaultOvnImageTag, obj.HostAgent.OvnController.Tag,
	)
	obj.HostAgent.OvnController.ImagePullPolicy = corev1.PullIfNotPresent
	obj.OvnNorth.Image = getImage(
		obj.ImageRepository, obj.OvnNorth.Repository,
		DefaultOvnImageName, obj.OvnNorth.ImageName,
		DefaultOvnImageTag, obj.OvnNorth.Tag,
	)
	obj.OvnNorth.ImagePullPolicy = corev1.PullIfNotPresent

	type stateDeploy struct {
		obj     *StatefulDeploymentSpec
		size    string
		version string
	}
	for cType, spec := range map[ComponentType]*stateDeploy{
		GlanceComponentType:         {&obj.Glance, DefaultGlanceStorageSize, obj.Version},
		InfluxdbComponentType:       {&obj.Influxdb, DefaultInfluxdbStorageSize, DefaultInfluxdbImageVersion},
		NotifyComponentType:         {&obj.Notify, DefaultNotifyStorageSize, obj.Version},
		BaremetalAgentComponentType: {&obj.BaremetalAgent, DefaultBaremetalStorageSize, obj.Version},
		MeterComponentType:          {&obj.Meter, DefaultMeterStorageSize, obj.Version},
		EsxiAgentComponentType:      {&obj.EsxiAgent, DefaultEsxiAgentStorageSize, obj.Version},
	} {
		SetDefaults_StatefulDeploymentSpec(cType, spec.obj, spec.size, obj.ImageRepository, spec.version)
	}

	SetDefaults_CronJobSpec(&obj.CloudmonPing,
		getImage(obj.ImageRepository, obj.CloudmonPing.Repository, APIGatewayComponentTypeEE,
			obj.CloudmonPing.ImageName, obj.Version, obj.APIGateway.Tag))

	SetDefaults_CronJobSpec(&obj.CloudmonReportUsage,
		getImage(obj.ImageRepository, obj.CloudmonReportUsage.Repository, APIGatewayComponentTypeEE,
			obj.CloudmonReportUsage.ImageName, obj.Version, obj.APIGateway.Tag))

	SetDefaults_CronJobSpec(&obj.CloudmonReportServer,
		getImage(obj.ImageRepository, obj.CloudmonReportServer.Repository, APIGatewayComponentTypeEE,
			obj.CloudmonReportServer.ImageName, obj.Version, obj.APIGateway.Tag))
}

func SetDefaults_Mysql(obj *Mysql) {
	if obj.Username == "" {
		obj.Username = "root"
	}
	if obj.Port == 0 {
		obj.Port = 3306
	}
}

func getImage(globalRepo, specRepo string, componentType ComponentType, componentName string, globalVersion, tag string) string {
	repo := specRepo
	if specRepo == "" {
		repo = globalRepo
	}
	version := tag
	if version == "" {
		version = globalVersion
	}
	if componentName == "" {
		componentName = componentType.String()
	}
	return fmt.Sprintf("%s/%s:%s", repo, componentName, version)
}

func getEditionImage(globalRepo, specRepo string, componentType ComponentType, componentName string, globalVersion, tag string, isEE bool) string {
	if componentName == "" {
		componentName = componentType.String()
		if isEE {
			componentName = fmt.Sprintf("%s-ee", componentName)
		}
	}
	return getImage(globalRepo, specRepo, componentType, componentName, globalVersion, tag)
}

func SetDefaults_KeystoneSpec(obj *KeystoneSpec, imageRepo, version string) {
	SetDefaults_DeploymentSpec(&obj.DeploymentSpec, getImage(imageRepo, obj.Repository, KeystoneComponentType, obj.ImageName, version, obj.Tag))
	if obj.BootstrapPassword == "" {
		obj.BootstrapPassword = passwd.GeneratePassword()
	}
}

func SetDefaults_RegionSpec(obj *RegionSpec, imageRepo, version string) {
	SetDefaults_DeploymentSpec(&obj.DeploymentSpec, getImage(imageRepo, obj.Repository, RegionComponentType, obj.ImageName, version, obj.Tag))
	if obj.DNSDomain == "" {
		obj.DNSDomain = DefaultOnecloudRegionDNSDomain
	}
}

func SetDefaults_RegionDNSSpec(obj *RegionDNSSpec, imageRepo, version string) {
	SetDefaults_DaemonSetSpec(&obj.DaemonSetSpec, getImage(imageRepo, obj.Repository, RegionDNSComponentType, obj.ImageName, version, obj.Tag))
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
	SetDefaults_DeploymentSpec(&obj.DeploymentSpec, getImage(imageRepo, obj.Repository, ctype, obj.ImageName, version, obj.Tag))
	if obj.StorageClassName == "" {
		obj.StorageClassName = DefaultStorageClass
	}
	setPVCStoreage(&obj.ContainerSpec, defaultSize)
}

func SetDefaults_DeploymentSpec(obj *DeploymentSpec, image string) {
	if obj.Replicas <= 0 {
		obj.Replicas = 1
	}
	if obj.Disable {
		obj.Replicas = 0
	}
	obj.Image = image
	if string(obj.ImagePullPolicy) == "" {
		obj.ImagePullPolicy = corev1.PullAlways
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

func SetDefaults_DaemonSetSpec(obj *DaemonSetSpec, image string) {
	obj.Image = image
	if string(obj.ImagePullPolicy) == "" {
		obj.ImagePullPolicy = corev1.PullAlways
	}
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

func SetDefaults_CronJobSpec(obj *CronJobSpec, image string) {
	obj.Image = image
	if string(obj.ImagePullPolicy) == "" {
		obj.ImagePullPolicy = corev1.PullAlways
	}
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
		&obj.Webconsole:                          {constants.WebconsoleAdminUser, constants.WebconsolePort},
		&obj.APIGateway:                          {constants.APIGatewayAdminUser, constants.APIGatewayPort},
		&obj.HostAgent.ServiceCommonOptions:      {constants.HostAdminUser, constants.HostPort},
		&obj.BaremetalAgent.ServiceCommonOptions: {constants.BaremetalAdminUser, constants.BaremetalPort},
		&obj.S3gateway:                           {constants.S3gatewayAdminUser, constants.S3gatewayPort},
		&obj.AutoUpdate:                          {constants.AutoUpdateAdminUser, constants.AutoUpdatePort},
		&obj.EsxiAgent.ServiceCommonOptions:      {constants.EsxiAgentAdminUser, constants.EsxiAgentPort},
		&obj.VpcAgent.ServiceCommonOptions:       {constants.VpcAgentAdminUser, 0},
	} {
		SetDefaults_ServiceCommonOptions(opt, userPort.user, userPort.port)
	}

	type userDBPort struct {
		user   string
		port   int
		db     string
		dbUser string
	}

	registryPorts := map[int]string{}

	for opt, tmp := range map[*ServiceDBCommonOptions]userDBPort{
		&obj.RegionServer.ServiceDBCommonOptions: {constants.RegionAdminUser, constants.RegionPort, constants.RegionDB, constants.RegionDBUser},
		&obj.Glance.ServiceDBCommonOptions:       {constants.GlanceAdminUser, constants.GlanceAPIPort, constants.GlanceDB, constants.GlanceDBUser},
		&obj.Logger:                              {constants.LoggerAdminUser, constants.LoggerPort, constants.LoggerDB, constants.LoggerDBUser},
		&obj.Register:                            {constants.RegisterAdminUser, constants.RegisterPort, constants.RegisterDB, constants.RegisterDBUser},
		&obj.Billing:                             {constants.BillingAdminUser, constants.BillingPort, constants.BillingDB, constants.BillingDBUser},
		&obj.Yunionagent:                         {constants.YunionAgentAdminUser, constants.YunionAgentPort, constants.YunionAgentDB, constants.YunionAgentDBUser},
		&obj.Yunionconf:                          {constants.YunionConfAdminUser, constants.YunionConfPort, constants.YunionConfDB, constants.YunionConfDBUser},
		&obj.KubeServer:                          {constants.KubeServerAdminUser, constants.KubeServerPort, constants.KubeServerDB, constants.KubeServerDBUser},
		&obj.AnsibleServer:                       {constants.AnsibleServerAdminUser, constants.AnsibleServerPort, constants.AnsibleServerDB, constants.AnsibleServerDBUser},
		&obj.Cloudnet:                            {constants.CloudnetAdminUser, constants.CloudnetPort, constants.CloudnetDB, constants.CloudnetDBUser},
		&obj.Cloudevent:                          {constants.CloudeventAdminUser, constants.CloudeventPort, constants.CloudeventDB, constants.CloudeventDBUser},
		&obj.Notify:                              {constants.NotifyAdminUser, constants.NotifyPort, constants.NotifyDB, constants.NotifyDBUser},
		&obj.Devtool:                             {constants.DevtoolAdminUser, constants.DevtoolPort, constants.DevtoolDB, constants.DevtoolDBUser},
		&obj.Meter.ServiceDBCommonOptions:        {constants.MeterAdminUser, constants.MeterPort, constants.MeterDB, constants.MeterDBUser},
		&obj.Monitor:                             {constants.MonitorAdminUser, constants.MonitorPort, constants.MonitorDB, constants.MonitorDBUser},
	} {
		if user, ok := registryPorts[tmp.port]; ok {
			log.Fatalf("port %d has been registered by %s", tmp.port, user)
		}
		registryPorts[tmp.port] = tmp.user
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
