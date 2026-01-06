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

	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

type notifyManager struct {
	*ComponentManager
}

func newNotifyManager(man *ComponentManager) manager.ServiceManager {
	return &notifyManager{man}
}

func (m *notifyManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *notifyManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.NotifyComponentType
}

func (m *notifyManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Notify.Disable
}

func (m *notifyManager) GetServiceName() string {
	return constants.ServiceNameNotify
}

func (m *notifyManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *notifyManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Notify.DB
}

func (m *notifyManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Notify.DbEngine)
}

func (m *notifyManager) getClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Notify.ClickhouseConf
}

func (m *notifyManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Notify.CloudUser
}

func (m *notifyManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewNotify().GetPhaseControl(man)
}

func (m *notifyManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewNotify().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, errors.Wrap(err, "set notify option")
	}
	cfgMap := m.newServiceConfigMap(v1alpha1.NotifyComponentType, "", oc, opt)
	return cfgMap, false, nil
}

func (m *notifyManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return m.newSinglePortService(v1alpha1.NotifyComponentType, oc, oc.Spec.Notify.Service.InternalOnly, int32(oc.Spec.Notify.Service.NodePort), int32(cfg.Notify.Port), oc.Spec.Notify.SlaveReplicas > 0)
}

func (m *notifyManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.NotifyComponentType, "", oc, &oc.Spec.Notify.DeploymentSpec, int32(cfg.Notify.Port), true, true)
}

func (m *notifyManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Notify
}

func (m *notifyManager) supportsReadOnlyService() bool {
	return false
}

func (m *notifyManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return nil
}

func (m *notifyManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return func(s *mcclient.ClientSession) error {
		if m.IsDisabled(oc) {
			return onecloud.EnsureDisableService(s, m.GetServiceName())
		} else {
			return onecloud.EnsureEnableService(s, m.GetServiceName(), m.supportsReadOnlyService() && oc.Spec.Notify.SlaveReplicas > 0)
		}
	}
}
