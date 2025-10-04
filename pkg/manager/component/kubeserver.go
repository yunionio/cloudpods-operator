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

type kubeManager struct {
	*ComponentManager
}

func newKubeManager(man *ComponentManager) manager.Manager {
	return &kubeManager{man}
}

func (m *kubeManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *kubeManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.KubeServerComponentType
}

func (m *kubeManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.KubeServer.Disable || !isInProductVersion(m, oc)
}

func (m *kubeManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *kubeManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.KubeServer.DB
}

func (m *kubeManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.KubeServer.DbEngine)
}

func (m *kubeManager) getClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.KubeServer.ClickhouseConf
}

func (m *kubeManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.KubeServer.CloudUser
}

func (m *kubeManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.KubeServer(m.nodeLister)
}

func (m *kubeManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewKubeserver().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}
	return m.newServiceConfigMap(v1alpha1.KubeServerComponentType, "", oc, opt), false, nil
}

func (m *kubeManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSinglePortService(v1alpha1.KubeServerComponentType, oc, oc.Spec.KubeServer.Service.InternalOnly, int32(oc.Spec.KubeServer.Service.NodePort), int32(cfg.KubeServer.Port))}
}

func (m *kubeManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.KubeServerComponentType, "", oc, &oc.Spec.KubeServer.DeploymentSpec, int32(cfg.KubeServer.Port), true, true)
	if err != nil {
		return nil, err
	}
	deploy.Spec.Template.Spec.ServiceAccountName = constants.ServiceAccountOnecloudOperator
	return deploy, nil
}

func (m *kubeManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.KubeServer
}
