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

type mcpServerManager struct {
	*ComponentManager
}

func newMcpServerManager(man *ComponentManager) manager.ServiceManager {
	return &mcpServerManager{man}
}

func (m *mcpServerManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *mcpServerManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.McpServerComponentType
}

func (m *mcpServerManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.McpServer.Disable
}

func (m *mcpServerManager) GetServiceName() string {
	return constants.ServiceNameMcpServer
}

func (m *mcpServerManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *mcpServerManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewMcpServer().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}

	return m.newServiceConfigMap(v1alpha1.McpServerComponentType, zone, oc, opt), false, nil
}

func (m *mcpServerManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return m.newSinglePortService(v1alpha1.McpServerComponentType, oc, oc.Spec.McpServer.Service.InternalOnly, int32(oc.Spec.McpServer.Service.NodePort), int32(constants.McpServerPort), oc.Spec.McpServer.SlaveReplicas > 0)
}

func (m *mcpServerManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.McpServerComponentType, "", oc, &oc.Spec.McpServer.DeploymentSpec, int32(constants.McpServerPort), false, false)
}

func (m *mcpServerManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.McpServer
}

func (m *mcpServerManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewMcpServer().GetPhaseControl(man)
}

func (m *mcpServerManager) supportsReadOnlyService() bool {
	return false
}

func (m *mcpServerManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return nil
}

func (m *mcpServerManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return func(s *mcclient.ClientSession) error {
		if m.IsDisabled(oc) {
			return onecloud.EnsureDisableService(s, m.GetServiceName())
		} else {
			return onecloud.EnsureEnableService(s, m.GetServiceName(), m.supportsReadOnlyService() && oc.Spec.McpServer.SlaveReplicas > 0)
		}
	}
}
