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

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

type aiProxyManager struct {
	*ComponentManager
}

func newAiProxyManager(man *ComponentManager) manager.ServiceManager {
	return &aiProxyManager{man}
}

func (m *aiProxyManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionAI,
	}
}

func (m *aiProxyManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.AiProxyComponentType
}

func (m *aiProxyManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.AiProxy.Disable
}

func (m *aiProxyManager) GetServiceName() string {
	return constants.ServiceNameAiProxy
}

func (m *aiProxyManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *aiProxyManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.AiProxy.DB
}

func (m *aiProxyManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.AiProxy.DbEngine)
}

func (m *aiProxyManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.AiProxy.CloudUser
}

func (m *aiProxyManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewAiProxy().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}

	return m.newServiceConfigMap(v1alpha1.AiProxyComponentType, zone, oc, opt), false, nil
}

func (m *aiProxyManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return m.newSinglePortService(v1alpha1.AiProxyComponentType, oc, oc.Spec.AiProxy.Service.InternalOnly, int32(oc.Spec.AiProxy.Service.NodePort), int32(cfg.AiProxy.Port), oc.Spec.AiProxy.SlaveReplicas > 0)
}

func (m *aiProxyManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.AiProxyComponentType, "", oc, &oc.Spec.AiProxy.DeploymentSpec, int32(cfg.AiProxy.Port), false, false)
}

func (m *aiProxyManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.AiProxy
}

func (m *aiProxyManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewAiProxy().GetPhaseControl(man)
}

func (m *aiProxyManager) supportsReadOnlyService() bool {
	return false
}

func (m *aiProxyManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return nil
}

func (m *aiProxyManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return func(s *mcclient.ClientSession) error {
		if m.IsDisabled(oc) {
			return onecloud.EnsureDisableService(s, m.GetServiceName())
		}
		return onecloud.EnsureEnableService(s, m.GetServiceName(), m.supportsReadOnlyService() && oc.Spec.AiProxy.SlaveReplicas > 0)
	}
}
