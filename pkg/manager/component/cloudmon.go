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

package component

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

const (
	CloudMonitorConfigTemplate = `
debug=false
trace=false
logging.level.com.yunion.cloudmon=INFO

yunion.rc.auth.url={{.AuthURL}}
yunion.rc.auth.domain={{.AuthDomain}}
yunion.rc.auth.username={{.AuthUsername}}
yunion.rc.auth.password={{.AuthPassword}}
yunion.rc.auth.project={{.AuthProject}}
yunion.rc.auth.region={{.Region}}
yunion.rc.auth.cache-size=500
yunion.rc.auth.timeout=1000
yunion.rc.auth.debug=true
yunion.rc.auth.insecure=true
yunion.rc.auth.refresh-interval=300000

yunion.rc.async-job.initial-delay=2000
yunion.rc.async-job.fixed-rate=300000
yunion.rc.async-job.fixed-thread-pool=10

yunion.rc.influxdb.database=telegraf
yunion.rc.influxdb.policy=30day_only
yunion.rc.influxdb.measurement=instance

yunion.rc.metrics.ins.providers=Aliyun,Azure,Aws,Qcloud,VMWare,Huawei,Openstack,Ucloud,ZStack
yunion.rc.metrics.eip.providers=Aliyun,Qcloud
`
)

type CloudMonitorConfig struct {
	AuthURL      string
	AuthDomain   string
	AuthUsername string
	AuthPassword string
	AuthProject  string
	Region       string
}

type cloudMonitorManager struct {
	*ComponentManager
}

func newCloudMonitorManager(man *ComponentManager) manager.Manager {
	return &cloudMonitorManager{man}
}

func (m *cloudMonitorManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc)
}

func (m *cloudMonitorManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.CloudMonitor.CloudUser
}

func (m *cloudMonitorManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	config := &CloudMonitorConfig{
		AuthURL:      controller.GetAuthURL(oc),
		AuthProject:  constants.SysAdminProject,
		AuthDomain:   constants.DefaultDomain,
		AuthUsername: constants.CloudMonitorAdminUser,
		AuthPassword: cfg.CloudMonitor.Password,
		Region:       oc.Spec.Region,
	}
	data, err := CompileTemplateFromMap(CloudMonitorConfigTemplate, config)
	if err != nil {
		return nil, err
	}
	return m.newConfigMap(v1alpha1.CloudMonitorComponentType, oc, data), nil
}

func (m *cloudMonitorManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		confVol := volMounts[len(volMounts)-1]
		confVol.MountPath = "/deployments/config"
		volMounts[len(volMounts)-1] = confVol
		return []corev1.Container{
			{
				Name:  "app",
				Image: oc.Spec.CloudMonitor.Image,
				Env: []corev1.EnvVar{
					{
						Name:  "JAVA_APP_JAR",
						Value: "cloudmon.jar",
					},
				},
				VolumeMounts: volMounts,
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.CloudMonitorComponentType, oc,
		NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.CloudMonitorComponentType), v1alpha1.CloudMonitorComponentType),
		oc.Spec.CloudMonitor, cf)
	if err != nil {
		return nil, err
	}
	podSpec := &deploy.Spec.Template.Spec
	config := podSpec.Volumes[len(podSpec.Volumes)-1]
	config.ConfigMap.Items[0].Path = "application.properties"
	podSpec.Volumes[len(podSpec.Volumes)-1] = config
	return deploy, nil
}
