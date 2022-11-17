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
	"path"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud/pkg/monitor/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type monitorManager struct {
	*ComponentManager
}

func newMonitorManager(man *ComponentManager) manager.Manager {
	return &monitorManager{man}
}

func (m *monitorManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *monitorManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.MonitorComponentType
}

func (m *monitorManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Monitor.Disable, "")
}

func (m *monitorManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Monitor.DB
}

func (m *monitorManager) getClickhouseConf(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Monitor.ClickhouseConf
}

func (m *monitorManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Monitor.CloudUser
}

func (m *monitorManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.Monitor()
}

func (m *monitorManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeMonitor); err != nil {
		return nil, false, err
	}
	config := cfg.Monitor
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = config.Port
	return m.newServiceConfigMap(v1alpha1.MonitorComponentType, "", oc, opt), false, nil
}

func (m *monitorManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.MonitorComponentType, oc, int32(oc.Spec.Monitor.Service.NodePort), int32(cfg.Monitor.Port))}
}

func (m *monitorManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "monitor",
				Image:           oc.Spec.Monitor.Image,
				ImagePullPolicy: oc.Spec.Monitor.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/monitor", "--config", "/etc/yunion/monitor.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.MonitorComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.MonitorComponentType), v1alpha1.MonitorComponentType), &oc.Spec.Monitor.DeploymentSpec, cf)
}

func (m *monitorManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Monitor
}
