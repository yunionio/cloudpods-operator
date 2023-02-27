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
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud/pkg/notify/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type notifyManager struct {
	*ComponentManager
}

func newNotifyManager(man *ComponentManager) manager.Manager {
	return &notifyManager{man}
}

func (m *notifyManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *notifyManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.NotifyComponentType
}

func (m *notifyManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Notify.Disable, "")
}

func (m *notifyManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Notify.DB
}

func (m *notifyManager) getClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Notify.ClickhouseConf
}

func (m *notifyManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Notify.CloudUser
}

func (m *notifyManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.NotifyComponentType,
		constants.ServiceNameNotify, constants.ServiceTypeNotify,
		man.GetCluster().Spec.Notify.Service.NodePort, "")
}

func (m *notifyManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeNotify); err != nil {
		return nil, false, errors.Wrap(err, "set notify option")
	}
	config := cfg.Notify
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.Port = config.Port
	cfgMap := m.newServiceConfigMap(v1alpha1.NotifyComponentType, "", oc, opt)
	return cfgMap, false, nil
}

func (m *notifyManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.NotifyComponentType, oc, int32(oc.Spec.Notify.Service.NodePort), int32(cfg.Notify.Port))}
}

func (m *notifyManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "notify",
				Image:           oc.Spec.Notify.Image,
				ImagePullPolicy: oc.Spec.Notify.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/notify", "--config", "/etc/yunion/notify.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.NotifyComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.NotifyComponentType), v1alpha1.NotifyComponentType), &oc.Spec.Notify.DeploymentSpec, cf)
}

func (m *notifyManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Notify
}
