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
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
)

type yunoinconfManager struct {
	*ComponentManager
}

func newYunionconfManager(man *ComponentManager) manager.ServiceManager {
	return &yunoinconfManager{man}
}

func (m *yunoinconfManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *yunoinconfManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.YunionconfComponentType
}

func (m *yunoinconfManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Yunionconf.Disable
}

func (m *yunoinconfManager) GetServiceName() string {
	return constants.ServiceNameYunionConf
}

func (m *yunoinconfManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *yunoinconfManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Yunionconf.DB
}

func (m *yunoinconfManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Yunionconf.DbEngine)
}

func (m *yunoinconfManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Yunionconf.CloudUser
}

func (m *yunoinconfManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewYunionconf().GetPhaseControl(man)
}

func (m *yunoinconfManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewYunionconf().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}

	return m.newServiceConfigMap(v1alpha1.YunionconfComponentType, "", oc, opt), false, nil
}

func (m *yunoinconfManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.YunionconfComponentType, oc, int32(oc.Spec.Yunionconf.Service.NodePort), int32(cfg.Yunionconf.Port))}
}

func (m *yunoinconfManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.YunionconfComponentType, "", oc, &oc.Spec.Yunionconf.DeploymentSpec, int32(cfg.Yunionconf.Port), false, false)
}

func (m *yunoinconfManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Yunionconf
}
