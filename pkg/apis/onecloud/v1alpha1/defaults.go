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
	"yunion.io/x/pkg/utils"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/util/passwd"
)

var (
	ClearComponent bool
)

const (
	DefaultVersion                    = "latest"
	DefaultOnecloudRegion             = "region0"
	DefaultOnecloudRegionDNSDomain    = "cloud.onecloud.io"
	DefaultOnecloudZone               = "zone0"
	DefaultOnecloudWire               = "bcast0"
	DefaultImageRepository            = "registry.hub.docker.com/yunion"
	DefaultVPCId                      = "default"
	DefaultGlanceStorageSize          = "100G"
	DefaultMeterStorageSize           = "100G"
	DefaultInfluxdbStorageSize        = "20G"
	DefaultVictoriaMetricsStorageSize = "20G"
	DefaultNotifyStorageSize          = "1G" // for plugin template
	DefaultBaremetalStorageSize       = "100G"
	DefaultEsxiAgentStorageSize       = "30G"
	// rancher local-path-provisioner: https://github.com/rancher/local-path-provisioner
	DefaultStorageClass = "local-path"

	DefaultOvnVersion   = "2.12.4"
	DefaultOvnImageName = "openvswitch"
	DefaultOvnImageTag  = DefaultOvnVersion + "-2"

	DefaultSdnAgentImageName = "sdnagent"

	DefaultWebOverviewImageName = "dashboard-overview"
	DefaultWebDocsImageName     = "docs-ee"

	DefaultNotifyPluginsImageName = "notify-plugins"

	DefaultHostImageName = "host-image"
	DefaultHostImageTag  = "v1.0.6"

	DefaultHostHealthName = "host-health"
	DefaultHostHealthTag  = "v0.0.1"

	DefaultInfluxdbImageVersion = "1.7.7"

	DefaultVictoriaMetricsImageVersion = "v1.95.1"

	DefaultTelegrafImageName     = "telegraf"
	DefaultTelegrafImageTag      = "release-1.19.2-2"
	DefaultTelegrafInitImageName = "telegraf-init"
	DefaultTelegrafInitImageTag  = "release-1.19.2-0"
	DefaultTelegrafRaidImageName = "telegraf-raid-plugin"
	DefaultTelegrafRaidImageTag  = "release-1.6.5"

	DefaultHostQemuVersion = "4.2.0"

	DefaultEChartSSRVersion = "v0.0.1"
	DefaultGuacdVersion     = "1.5.3"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_OnecloudCluster(obj *OnecloudCluster) {
	if _, ok := obj.GetLabels()[constants.InstanceLabelKey]; !ok {
		obj.SetLabels(map[string]string{constants.InstanceLabelKey: fmt.Sprintf("onecloud-cluster-%s", rand.String(4))})
	}

	SetDefaults_OnecloudClusterSpec(&obj.Spec, IsEnterpriseEdition(obj), IsEEOrESEEdition(obj))
}

func GetEdition(oc *OnecloudCluster) string {
	edition := constants.OnecloudCommunityEdition
	if oc.Annotations == nil {
		return edition
	}
	curEdition := oc.Annotations[constants.OnecloudEditionAnnotationKey]
	if curEdition == "" {
		return constants.OnecloudCommunityEdition
	}
	return curEdition
}

func IsEnterpriseEdition(oc *OnecloudCluster) bool {
	return GetEdition(oc) == constants.OnecloudEnterpriseEdition
}

func IsEEOrESEEdition(oc *OnecloudCluster) bool {
	if IsEnterpriseEdition(oc) {
		return true
	}
	if GetEdition(oc) == constants.OnecloudEnterpriseSupportEdition {
		return true
	}
	return false
}

type hyperImagePair struct {
	*DeploymentSpec
	Supported bool
}

func newHyperImagePair(ds *DeploymentSpec, supported bool) *hyperImagePair {
	return &hyperImagePair{
		DeploymentSpec: ds,
		Supported:      supported,
	}
}

func clearContainerSpec(spec *ContainerSpec) {
	if ClearComponent {
		spec.Tag = ""
		spec.ImageName = ""
		spec.Repository = ""
	}
}

func SetDefaults_OnecloudClusterSpec(obj *OnecloudClusterSpec, isEE bool, isEEOrESE bool) {
	setDefaults_Mysql(&obj.Mysql)
	setDefaults_Clickhouse(&obj.Clickhouse)
	setDefaults_Dameng(&obj.Dameng)
	if obj.ProductVersion == "" {
		obj.ProductVersion = ProductVersionFullStack
	}
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

	useHyperImage := obj.UseHyperImage

	SetDefaults_KeystoneSpec(&obj.Keystone, obj.ImageRepository, obj.Version, useHyperImage, isEE)
	SetDefaults_RegionSpec(&obj.RegionServer, obj.ImageRepository, obj.Version, useHyperImage, isEE)
	SetDefaults_RegionDNSSpec(&obj.RegionDNS, obj.ImageRepository, obj.Version)
	setDefaults_Cloudmux(obj, &obj.Cloudmux)

	nHP := newHyperImagePair

	for cType, spec := range map[ComponentType]*hyperImagePair{
		WebconsoleComponentType:      nHP(&obj.Webconsole.DeploymentSpec, useHyperImage),
		SchedulerComponentType:       nHP(&obj.Scheduler.DeploymentSpec, useHyperImage),
		LoggerComponentType:          nHP(&obj.Logger.DeploymentSpec, useHyperImage),
		YunionconfComponentType:      nHP(&obj.Yunionconf.DeploymentSpec, useHyperImage),
		KubeServerComponentType:      nHP(&obj.KubeServer.DeploymentSpec, false),
		AnsibleServerComponentType:   nHP(&obj.AnsibleServer.DeploymentSpec, useHyperImage),
		CloudnetComponentType:        nHP(&obj.Cloudnet.DeploymentSpec, false),
		CloudproxyComponentType:      nHP(&obj.Cloudproxy.DeploymentSpec, false),
		CloudeventComponentType:      nHP(&obj.Cloudevent.DeploymentSpec, useHyperImage),
		S3gatewayComponentType:       nHP(&obj.S3gateway.DeploymentSpec, false),
		DevtoolComponentType:         nHP(&obj.Devtool.DeploymentSpec, useHyperImage),
		AutoUpdateComponentType:      nHP(&obj.AutoUpdate.DeploymentSpec, useHyperImage),
		OvnNorthComponentType:        nHP(&obj.OvnNorth, false),
		VpcAgentComponentType:        nHP(&obj.VpcAgent.DeploymentSpec, useHyperImage),
		MonitorComponentType:         nHP(&obj.Monitor.DeploymentSpec, useHyperImage),
		ServiceOperatorComponentType: nHP(&obj.ServiceOperator.DeploymentSpec, false),
		ItsmComponentType:            nHP(&obj.Itsm.DeploymentSpec, false),
		CloudIdComponentType:         nHP(&obj.CloudId.DeploymentSpec, useHyperImage),
		SuggestionComponentType:      nHP(&obj.Suggestion.DeploymentSpec, useHyperImage),
		ScheduledtaskComponentType:   nHP(&obj.Scheduledtask.DeploymentSpec, useHyperImage),
		ReportComponentType:          nHP(&obj.Report.DeploymentSpec, useHyperImage),
		APIMapComponentType:          nHP(&obj.APIMap.DeploymentSpec, useHyperImage),
		BastionHostComponentType:     nHP(&obj.BastionHost.DeploymentSpec, useHyperImage),
		ExtdbComponentType:           nHP(&obj.Extdb.DeploymentSpec, useHyperImage),
		BillingComponentType:         nHP(&obj.Billing.DeploymentSpec, useHyperImage),
	} {
		SetDefaults_DeploymentSpec(spec.DeploymentSpec, getImage(
			obj.ImageRepository, spec.Repository,
			cType, spec.ImageName,
			obj.Version, spec.Tag,
			spec.Supported, isEE,
		))
	}

	// CE or EE parts
	for cType, spec := range map[ComponentType]*hyperImagePair{
		CloudmuxComponentType: nHP(obj.Cloudmux.ToDeploymentSpec(), false),
		CloudmonComponentType: nHP(&obj.Cloudmon.DeploymentSpec, useHyperImage),
	} {
		SetDefaults_DeploymentSpec(spec.DeploymentSpec,
			getEditionImage(
				obj.ImageRepository, spec.Repository,
				cType, spec.ImageName,
				obj.Version, spec.Tag,
				spec.Supported, isEE,
			))
	}

	// CE/SUPPORT parts
	for cType, spec := range map[ComponentType]*hyperImagePair{
		APIGatewayComponentType: nHP(&obj.APIGateway.DeploymentSpec, useHyperImage),
		ClimcComponentType:      nHP(&obj.Climc, false),
		WebComponentType:        nHP(&obj.Web.DeploymentSpec, false),
	} {
		SetDefaults_DeploymentSpec(spec.DeploymentSpec,
			getEditionImage(
				obj.ImageRepository, spec.Repository,
				cType, spec.ImageName,
				obj.Version, spec.Tag,
				spec.Supported, isEEOrESE,
			))
	}

	// setting web overview image
	obj.Web.Overview.Image = getImage(
		obj.ImageRepository, obj.Web.Overview.Repository,
		DefaultWebOverviewImageName, obj.Web.Overview.ImageName,
		obj.Version, obj.Web.Overview.Tag,
		false, isEE,
	)
	obj.Web.Overview.ImagePullPolicy = corev1.PullIfNotPresent
	// setting web docs image
	obj.Web.Docs.Image = getImage(
		obj.ImageRepository, obj.Web.Docs.Repository,
		DefaultWebDocsImageName, obj.Web.Docs.ImageName,
		obj.Version, obj.Web.Docs.Tag,
		false, isEE,
	)
	obj.Web.Docs.ImagePullPolicy = corev1.PullIfNotPresent

	// setting host spec
	if obj.HostAgent.DefaultQemuVersion == "" {
		obj.HostAgent.DefaultQemuVersion = DefaultHostQemuVersion
	}

	for cType, spec := range map[ComponentType]*DaemonSetSpec{
		HostComponentType:         &obj.HostAgent.DaemonSetSpec,
		HostDeployerComponentType: &obj.HostDeployer,
		YunionagentComponentType:  &obj.Yunionagent.DaemonSetSpec,
		HostImageComponentType:    &obj.HostImage,
		LbagentComponentType:      &obj.Lbagent.DaemonSetSpec,
	} {
		useHI := false
		if utils.IsInStringArray(string(cType), []string{
			string(YunionagentComponentType),
			string(HostComponentType),
			string(LbagentComponentType),
		}) {
			useHI = useHyperImage
		}
		SetDefaults_DaemonSetSpec(spec, getImage(
			obj.ImageRepository, spec.Repository,
			cType, spec.ImageName,
			obj.Version, spec.Tag,
			useHI, isEE,
		))
	}

	// setting webconsole guacd image
	obj.Webconsole.Guacd.Image = getImage(
		obj.ImageRepository, obj.Webconsole.Guacd.Repository,
		GuacdComponentType, obj.Webconsole.Guacd.ImageName,
		DefaultGuacdVersion, obj.Webconsole.Guacd.Tag,
		false, false,
	)
	obj.Webconsole.Guacd.ImagePullPolicy = corev1.PullIfNotPresent
	clearContainerSpec(&obj.Webconsole.Guacd)

	// setting sdnagent image
	obj.HostAgent.SdnAgent.Image = getImage(
		obj.ImageRepository, obj.HostAgent.SdnAgent.Repository,
		DefaultSdnAgentImageName, obj.HostAgent.SdnAgent.ImageName,
		obj.Version, obj.HostAgent.SdnAgent.Tag,
		useHyperImage, isEE,
	)
	obj.HostAgent.SdnAgent.ImagePullPolicy = corev1.PullIfNotPresent
	clearContainerSpec(&obj.HostAgent.SdnAgent)

	// setting ovn image
	obj.HostAgent.OvnController.Image = getImage(
		obj.ImageRepository, obj.HostAgent.OvnController.Repository,
		DefaultOvnImageName, obj.HostAgent.OvnController.ImageName,
		DefaultOvnImageTag, obj.HostAgent.OvnController.Tag,
		false, isEE,
	)
	obj.HostAgent.OvnController.ImagePullPolicy = corev1.PullIfNotPresent
	clearContainerSpec(&obj.HostAgent.OvnController)

	obj.OvnNorth.Image = getImage(
		obj.ImageRepository, obj.OvnNorth.Repository,
		DefaultOvnImageName, obj.OvnNorth.ImageName,
		DefaultOvnImageTag, obj.OvnNorth.Tag,
		false, isEE,
	)
	obj.OvnNorth.ImagePullPolicy = corev1.PullIfNotPresent
	clearContainerSpec(&obj.OvnNorth.ContainerSpec)
	// host-image
	obj.HostImage.Image = getImage(
		obj.ImageRepository, obj.HostImage.Repository,
		DefaultHostImageName, obj.HostImage.ImageName,
		DefaultHostImageTag, obj.HostImage.Tag,
		false, isEE,
	)
	// setting host health image
	obj.HostAgent.HostHealth.Image = getImage(
		obj.ImageRepository, obj.HostAgent.HostHealth.Repository,
		DefaultHostHealthName, obj.HostAgent.HostHealth.ImageName,
		DefaultHostHealthTag, obj.HostAgent.HostHealth.Tag,
		false, isEE,
	)
	clearContainerSpec(&obj.HostImage.ContainerSpec)

	// lbagent ovn-controller
	// setting ovn image
	obj.Lbagent.OvnController.Image = getImage(
		obj.ImageRepository, obj.Lbagent.OvnController.Repository,
		DefaultOvnImageName, obj.Lbagent.OvnController.ImageName,
		DefaultOvnImageTag, obj.Lbagent.OvnController.Tag,
		false, isEE,
	)
	obj.Lbagent.OvnController.ImagePullPolicy = corev1.PullIfNotPresent

	// telegraf spec
	obj.Telegraf.InitContainerImage = getImage(
		obj.ImageRepository, obj.Telegraf.Repository,
		DefaultTelegrafInitImageName, "",
		DefaultTelegrafInitImageTag, "",
		false, isEE,
	)
	// obj.Telegraf.TelegrafRaidImage = getImage(
	//	obj.ImageRepository, obj.Telegraf.Repository,
	//	DefaultTelegrafRaidImageName, "",
	//	DefaultTelegrafRaidImageTag, "",
	//	false, isEE,
	// )
	obj.Telegraf.TelegrafRaid.Image = getImage(
		obj.ImageRepository, obj.Telegraf.TelegrafRaid.Repository,
		DefaultTelegrafRaidImageName, obj.Telegraf.TelegrafRaid.ImageName,
		DefaultTelegrafRaidImageTag, obj.Telegraf.TelegrafRaid.Tag,
		false, isEE,
	)
	obj.Telegraf.TelegrafRaid.ImagePullPolicy = corev1.PullIfNotPresent
	clearContainerSpec(&obj.Telegraf.TelegrafRaid)

	SetDefaults_DaemonSetSpec(
		&obj.Telegraf.DaemonSetSpec,
		getImage(obj.ImageRepository, obj.Telegraf.Repository,
			DefaultTelegrafImageName, obj.Telegraf.ImageName,
			DefaultTelegrafImageTag, obj.Telegraf.Tag,
			false, isEE,
		),
	)

	type stateDeploy struct {
		obj           *StatefulDeploymentSpec
		size          string
		version       string
		useHyperImage bool
	}
	for cType, spec := range map[ComponentType]*stateDeploy{
		GlanceComponentType:          {&obj.Glance.StatefulDeploymentSpec, DefaultGlanceStorageSize, obj.Version, useHyperImage},
		InfluxdbComponentType:        {&obj.Influxdb.StatefulDeploymentSpec, DefaultInfluxdbStorageSize, DefaultInfluxdbImageVersion, false},
		VictoriaMetricsComponentType: {&obj.VictoriaMetrics.StatefulDeploymentSpec, DefaultVictoriaMetricsStorageSize, DefaultVictoriaMetricsImageVersion, false},
		NotifyComponentType:          {&obj.Notify.StatefulDeploymentSpec, DefaultNotifyStorageSize, obj.Version, useHyperImage},
		BaremetalAgentComponentType:  {&obj.BaremetalAgent.StatefulDeploymentSpec, DefaultBaremetalStorageSize, obj.Version, false},
		MeterComponentType:           {&obj.Meter.StatefulDeploymentSpec, DefaultMeterStorageSize, obj.Version, useHyperImage},
		EsxiAgentComponentType:       {&obj.EsxiAgent.StatefulDeploymentSpec, DefaultEsxiAgentStorageSize, obj.Version, useHyperImage},
	} {
		SetDefaults_StatefulDeploymentSpec(cType, spec.obj, spec.size, obj.ImageRepository, spec.version, spec.useHyperImage, isEE)
	}

	if obj.VictoriaMetrics.RententionPeriodDays == 0 {
		obj.VictoriaMetrics.RententionPeriodDays = constants.VictoriaMetricsDefaultRententionPeriod
	}

	// setting web overview image
	obj.Notify.Plugins.Image = getImage(
		obj.ImageRepository, obj.Notify.Plugins.Repository,
		DefaultNotifyPluginsImageName, obj.Notify.Plugins.ImageName,
		obj.Version, obj.Notify.Plugins.Tag,
		false, isEE,
	)
	obj.Notify.Plugins.ImagePullPolicy = corev1.PullIfNotPresent

	// cloudmon spec
	//SetDefaults_DeploymentSpec(&obj.Cloudmon.DeploymentSpec,
	//	getImage(obj.ImageRepository, obj.Cloudmon.Repository, APIGatewayComponentTypeEE,
	//		obj.Cloudmon.ImageName, obj.Version, obj.APIGateway.Tag))

	setDefaults_MonitorStackSpec(&obj.MonitorStack)

	setDefaults_Components_ServicePort(obj)

	setDefaults_EChartsSpec(obj, &obj.EChartsSSR)
}

type serviceSpecPair struct {
	spec        *ServiceSpec
	defaultPort int
}

func setDefaults_Components_ServicePort(obj *OnecloudClusterSpec) {
	newSP := func(spec *ServiceSpec, defaultPort int) *serviceSpecPair {
		return &serviceSpecPair{
			spec:        spec,
			defaultPort: defaultPort,
		}
	}
	for _, spec := range []*serviceSpecPair{
		newSP(&obj.Keystone.AdminService, constants.KeystoneAdminPort),
		newSP(&obj.Keystone.PublicService, constants.KeystonePublicPort),
		newSP(&obj.RegionServer.Service, constants.RegionPort),
		newSP(&obj.Scheduler.Service, constants.SchedulerPort),
		newSP(&obj.Glance.Service, constants.GlanceAPIPort),
		newSP(&obj.Webconsole.Service, constants.WebconsolePort),
		newSP(&obj.Logger.Service, constants.LoggerPort),
		newSP(&obj.Yunionconf.Service, constants.YunionConfPort),
		newSP(&obj.KubeServer.Service, constants.KubeServerPort),
		newSP(&obj.AnsibleServer.Service, constants.AnsibleServerPort),
		newSP(&obj.Cloudnet.Service, constants.CloudnetPort),
		newSP(&obj.Cloudproxy.Service, constants.CloudproxyPort),
		newSP(&obj.Cloudevent.Service, constants.CloudeventPort),
		newSP(&obj.CloudId.Service, constants.CloudIdPort),
		newSP(&obj.Cloudmon.Service, constants.CloudmonPort),
		newSP(&obj.AutoUpdate.Service, constants.AutoUpdatePort),
		newSP(&obj.S3gateway.Service, constants.S3gatewayPort),
		newSP(&obj.Devtool.Service, constants.DevtoolPort),
		newSP(&obj.Meter.Service, constants.MeterPort),
		newSP(&obj.Billing.Service, constants.BillingPort),
		newSP(&obj.Itsm.Service, constants.ItsmPort),
		newSP(&obj.Suggestion.Service, constants.SuggestionPort),
		newSP(&obj.Notify.Service, constants.NotifyPort),
		newSP(&obj.Influxdb.Service, constants.InfluxdbPort),
		newSP(&obj.VictoriaMetrics.Service, constants.VictoriaMetricsPort),
		newSP(&obj.Monitor.Service, constants.MonitorPort),
		newSP(&obj.Scheduledtask.Service, constants.ScheduledtaskPort),
		newSP(&obj.APIMap.Service, constants.APIMapPort),
		newSP(&obj.Report.Service, constants.ReportPort),
		newSP(&obj.APIGateway.APIService, constants.APIGatewayPort),
		newSP(&obj.APIGateway.WSService, constants.APIWebsocketPort),
		newSP(&obj.ServiceOperator.Service, constants.ServiceOperatorPort),
		newSP(&obj.Yunionagent.Service, constants.YunionAgentPort),
		newSP(&obj.VpcAgent.Service, constants.VpcAgentPort),
		newSP(&obj.BastionHost.Service, constants.BastionHostPort),
		newSP(&obj.Extdb.Service, constants.ExtdbPort),
	} {
		SetDefaults_ServiceSpec(spec.spec, spec.defaultPort)
	}
}

func setDefaults_Mysql(obj *Mysql) {
	if obj.Username == "" {
		obj.Username = "root"
	}
	if obj.Port == 0 {
		obj.Port = 3306
	}
}

func setDefaults_Clickhouse(obj *Clickhouse) {
	if obj.Username == "" {
		obj.Username = "default"
	}
	if obj.Port == 0 {
		obj.Port = 9000
	}
}

func setDefaults_Dameng(obj *Dameng) {
	if obj.Username == "" {
		obj.Username = "sysdba"
	}
	if obj.Port == 0 {
		obj.Port = 5236
	}
}

func getHyperImageName(isEE bool) string {
	if isEE {
		return "cloudpods-ee"
	}
	return "cloudpods"
}

func getImage(
	globalRepo, specRepo string,
	componentType ComponentType, componentName string,
	globalVersion, tag string,
	useHyperImage bool, isEE bool) string {
	repo := specRepo
	if specRepo == "" {
		repo = globalRepo
	}
	version := tag
	if version == "" {
		version = globalVersion
	}
	if componentName == "" {
		if useHyperImage {
			componentName = getHyperImageName(isEE)
		} else {
			componentName = componentType.String()
		}
	}
	return fmt.Sprintf("%s/%s:%s", repo, componentName, version)
}

func getEditionImage(globalRepo, specRepo string, componentType ComponentType, componentName string, globalVersion, tag string, useHyperImage bool, isEE bool) string {
	if componentName == "" {
		componentName = componentType.String()
		if isEE {
			componentName = fmt.Sprintf("%s-ee", componentName)
		}
		if useHyperImage {
			componentName = getHyperImageName(isEE)
		}
	}
	return getImage(globalRepo, specRepo, componentType, componentName, globalVersion, tag, useHyperImage, isEE)
}

func SetDefaults_KeystoneSpec(
	obj *KeystoneSpec,
	imageRepo, version string,
	useHyperImage, isEE bool) {
	SetDefaults_DeploymentSpec(&obj.DeploymentSpec, getImage(
		imageRepo, obj.Repository,
		KeystoneComponentType, obj.ImageName,
		version, obj.Tag,
		useHyperImage, isEE,
	))
	if obj.BootstrapPassword == "" {
		obj.BootstrapPassword = passwd.GeneratePassword()
	}

}

func SetDefaults_ServiceSpec(obj *ServiceSpec, port int) {
	if obj.NodePort != 0 {
		return
	}
	obj.NodePort = port
}

func SetDefaults_RegionSpec(
	obj *RegionSpec,
	imageRepo, version string,
	useHyperImage, isEE bool,
) {
	SetDefaults_DeploymentSpec(&obj.DeploymentSpec, getEditionImage(
		imageRepo, obj.Repository,
		RegionComponentType, obj.ImageName,
		version, obj.Tag,
		useHyperImage, isEE,
	))
	if obj.DNSDomain == "" {
		obj.DNSDomain = DefaultOnecloudRegionDNSDomain
	}
	if obj.DNSConfig == nil {
		ndotVal2 := "2"
		obj.DNSConfig = &corev1.PodDNSConfig{
			Options: []corev1.PodDNSConfigOption{
				{
					Name:  "ndots",
					Value: &ndotVal2,
				},
			},
		}
	}
}

func SetDefaults_RegionDNSSpec(obj *RegionDNSSpec, imageRepo, version string) {
	SetDefaults_DaemonSetSpec(&obj.DaemonSetSpec, getImage(
		imageRepo, obj.Repository,
		RegionDNSComponentType, obj.ImageName,
		version, obj.Tag,
		false, false,
	))
}

func setPVCStoreage(obj *ContainerSpec, size string) {
	if obj.Requests == nil {
		obj.Requests = new(ResourceRequirement)
	}
	if obj.Requests.Storage == "" {
		obj.Requests.Storage = size
	}
}

func SetDefaults_StatefulDeploymentSpec(
	ctype ComponentType,
	obj *StatefulDeploymentSpec, defaultSize string,
	imageRepo, version string,
	useHyperImage bool, isEE bool) {
	SetDefaults_DeploymentSpec(&obj.DeploymentSpec, getImage(imageRepo, obj.Repository, ctype, obj.ImageName, version, obj.Tag, useHyperImage, isEE))
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
		obj.ImagePullPolicy = corev1.PullIfNotPresent
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
	clearContainerSpec(&obj.ContainerSpec)
}

func SetDefaults_DaemonSetSpec(obj *DaemonSetSpec, image string) {
	obj.Image = image
	if string(obj.ImagePullPolicy) == "" {
		obj.ImagePullPolicy = corev1.PullIfNotPresent
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
	clearContainerSpec(&obj.ContainerSpec)
}

func SetDefaults_CronJobSpec(obj *CronJobSpec, image string) {
	obj.Image = image
	if string(obj.ImagePullPolicy) == "" {
		obj.ImagePullPolicy = corev1.PullIfNotPresent
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
	clearContainerSpec(&obj.ContainerSpec)
}

func setDefaults_MonitorStackSpec(obj *MonitorStackSpec) {
	// set minio defaults
	minio := &obj.Minio
	if minio.AccessKey == "" {
		minio.AccessKey = "monitor-admin"
	}
	if minio.SecretKey == "" {
		minio.SecretKey = passwd.GeneratePassword()
	}

	// set grafana defaults
	grafana := &obj.Grafana
	if grafana.AdminUser == "" {
		grafana.AdminUser = "admin"
	}
	if grafana.AdminPassword == "" {
		grafana.AdminPassword = passwd.GeneratePassword()
	}
	if grafana.Subpath == "" {
		grafana.Subpath = "grafana"
	}
	oauth := &grafana.OAuth
	if oauth.Scopes == "" {
		oauth.Scopes = "user"
	}
	if oauth.RoleAttributePath == "" {
		oauth.RoleAttributePath = `projectName == 'system' && contains(roles, 'admin') && 'Admin' || 'Editor'`
	}

	// set loki defaults
	loki := &obj.Loki
	if loki.ObjectStoreConfig.Bucket == "" {
		loki.ObjectStoreConfig.Bucket = "loki"
	}

	// set prometheus defaults
	prom := &obj.Prometheus
	if prom.ThanosSidecarSpec.ObjectStoreConfig.Bucket == "" {
		prom.ThanosSidecarSpec.ObjectStoreConfig.Bucket = "thanos"
	}
	trueVar := true
	if prom.Disable == nil {
		prom.Disable = &trueVar
	}

	// set thanos defaults
	thanos := &obj.Thanos
	if thanos.ObjectStoreConfig.Bucket == "" {
		thanos.ObjectStoreConfig.Bucket = "thanos"
	}
}

func SetDefaults_OnecloudClusterConfig(obj *OnecloudClusterConfig) {
	setDefaults_KeystoneConfig(&obj.Keystone)

	type userPort struct {
		user string
		port int
	}

	for opt, userPort := range map[*ServiceCommonOptions]userPort{
		&obj.APIGateway:                          {constants.APIGatewayAdminUser, constants.APIGatewayPort},
		&obj.HostAgent.ServiceCommonOptions:      {constants.HostAdminUser, constants.HostPort},
		&obj.BaremetalAgent.ServiceCommonOptions: {constants.BaremetalAdminUser, constants.BaremetalPort},
		&obj.S3gateway:                           {constants.S3gatewayAdminUser, constants.S3gatewayPort},
		&obj.EsxiAgent.ServiceCommonOptions:      {constants.EsxiAgentAdminUser, constants.EsxiAgentPort},
		&obj.VpcAgent.ServiceCommonOptions:       {constants.VpcAgentAdminUser, constants.VpcAgentPort},
		&obj.ServiceOperator:                     {constants.ServiceOperatorAdminUser, constants.ServiceOperatorPort},
		&obj.Lbagent.ServiceCommonOptions:        {constants.LbagentAdminUser, constants.LbagentPort},
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
		&obj.Yunionagent:                         {constants.YunionAgentAdminUser, constants.YunionAgentPort, constants.YunionAgentDB, constants.YunionAgentDBUser},
		&obj.Yunionconf:                          {constants.YunionConfAdminUser, constants.YunionConfPort, constants.YunionConfDB, constants.YunionConfDBUser},
		&obj.KubeServer:                          {constants.KubeServerAdminUser, constants.KubeServerPort, constants.KubeServerDB, constants.KubeServerDBUser},
		&obj.AnsibleServer:                       {constants.AnsibleServerAdminUser, constants.AnsibleServerPort, constants.AnsibleServerDB, constants.AnsibleServerDBUser},
		&obj.Cloudnet:                            {constants.CloudnetAdminUser, constants.CloudnetPort, constants.CloudnetDB, constants.CloudnetDBUser},
		&obj.Cloudproxy:                          {constants.CloudproxyAdminUser, constants.CloudproxyPort, constants.CloudproxyDB, constants.CloudproxyDBUser},
		&obj.Cloudevent:                          {constants.CloudeventAdminUser, constants.CloudeventPort, constants.CloudeventDB, constants.CloudeventDBUser},
		&obj.Notify:                              {constants.NotifyAdminUser, constants.NotifyPort, constants.NotifyDB, constants.NotifyDBUser},
		&obj.Devtool:                             {constants.DevtoolAdminUser, constants.DevtoolPort, constants.DevtoolDB, constants.DevtoolDBUser},
		&obj.Meter.ServiceDBCommonOptions:        {constants.MeterAdminUser, constants.MeterPort, constants.MeterDB, constants.MeterDBUser},
		&obj.Billing:                             {constants.BillingAdminUser, constants.BillingPort, constants.BillingDB, constants.BillingDBUser},
		&obj.Monitor:                             {constants.MonitorAdminUser, constants.MonitorPort, constants.MonitorDB, constants.MonitorDBUser},
		&obj.Itsm.ServiceDBCommonOptions:         {constants.ItsmAdminUser, constants.ItsmPort, constants.ItsmDB, constants.ItsmDBUser},
		&obj.CloudId:                             {constants.CloudIdAdminUser, constants.CloudIdPort, constants.CloudIdDB, constants.CloudIdDBUser},
		&obj.Webconsole:                          {constants.WebconsoleAdminUser, constants.WebconsolePort, constants.WebconsoleDB, constants.WebconsoleDBUser},
		&obj.Report:                              {constants.ReportAdminUser, constants.ReportPort, constants.ReportDB, constants.ReportDBUser},
		&obj.AutoUpdate:                          {constants.AutoUpdateAdminUser, constants.AutoUpdatePort, constants.AutoUpdateDB, constants.AutoUpdateDBUser},
		&obj.BastionHost:                         {constants.BastionHostAdminUser, constants.BastionHostPort, constants.BastionHostDB, constants.BastionHostDBUser},
		&obj.Extdb:                               {constants.ExtdbAdminUser, constants.ExtdbPort, constants.ExtdbDB, constants.ExtdbDBUser},
	} {
		if user, ok := registryPorts[tmp.port]; ok {
			log.Fatalf("port %d has been registered by %s", tmp.port, user)
		}
		registryPorts[tmp.port] = tmp.user
		SetDefaults_ServiceDBCommonOptions(opt, tmp.db, tmp.dbUser, tmp.user, tmp.port)
	}
	SetDefaults_ServiceCommonOptions(&obj.Cloudmon, constants.CloudmonAdminUser, constants.CloudmonPort)
	setDefaults_ItsmConfig(&obj.Itsm)

	setDefaults_DBConfig(&obj.Grafana.DB, constants.MonitorStackGrafanaDB, constants.MonitorStackGrafanaDBUer)
}

func SetDefaults_ServiceBaseConfig(obj *ServiceBaseConfig, port int) {
	if obj.Port == 0 {
		obj.Port = port
	}
}

func setDefaults_KeystoneConfig(obj *KeystoneConfig) {
	SetDefaults_ServiceBaseConfig(&obj.ServiceBaseConfig, constants.KeystonePublicPort)
	setDefaults_DBConfig(&obj.DB, constants.KeystoneDB, constants.KeystoneDBUser)
	setDefaults_DBConfig(&obj.ClickhouseConf, constants.KeystoneDB, constants.KeystoneDBUser)
}

func SetDefaults_ServiceCommonOptions(obj *ServiceCommonOptions, user string, port int) {
	SetDefaults_ServiceBaseConfig(&obj.ServiceBaseConfig, port)
	setDefaults_CloudUser(&obj.CloudUser, user)
}

func SetDefaults_ServiceDBCommonOptions(obj *ServiceDBCommonOptions, db, dbUser string, svcUser string, port int) {
	setDefaults_DBConfig(&obj.DB, db, dbUser)
	setDefaults_DBConfig(&obj.ClickhouseConf, db, dbUser)
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

func setDefaults_ItsmConfig(obj *ItsmConfig) {
	obj.SecondDatabase = fmt.Sprintf("%s_engine", obj.DB.Database)
	obj.EncryptionKey = passwd.GeneratePassword()
}

func setDefaults_EChartsSpec(spec *OnecloudClusterSpec, obj *EChartsSSRSpec) {
	if obj.Disable == nil {
		trueVar := true
		obj.Disable = &trueVar
	}
	dSpec := obj.ToDeploymentSpec()
	SetDefaults_DeploymentSpec(dSpec, getImage(
		spec.ImageRepository, dSpec.Repository,
		EChartsSSRComponentType, dSpec.ImageName,
		DefaultEChartSSRVersion, dSpec.Tag,
		false, false))
	obj.FillBySpec(dSpec)
}

func setDefaults_Cloudmux(spec *OnecloudClusterSpec, obj *CloudmuxSpec) {
	if obj.Disable == nil {
		trueVar := true
		obj.Disable = &trueVar
	}
	dSpec := obj.ToDeploymentSpec()
	SetDefaults_DeploymentSpec(dSpec, getImage(
		spec.ImageRepository, dSpec.Repository,
		CloudmuxComponentType, dSpec.ImageName,
		spec.Version, dSpec.Tag,
		false, false))
	obj.FillBySpec(dSpec)
}
