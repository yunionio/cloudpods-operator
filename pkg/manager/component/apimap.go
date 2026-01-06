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

	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/mcclient"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

type apiMapManager struct {
	*ComponentManager
}

func newAPIMapManager(man *ComponentManager) manager.ServiceManager {
	return &apiMapManager{man}
}

func (m *apiMapManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *apiMapManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.APIMapComponentType
}

func (m *apiMapManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.APIMap.Disable || !isInProductVersion(m, oc)
}

func (m *apiMapManager) GetServiceName() string {
	return constants.ServiceNameAPIMap
}

func (m *apiMapManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *apiMapManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewAPIMap().GetPhaseControl(man)
}

func (m *apiMapManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewAPIMap().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, errors.Wrap(err, "apimap: SetOptionsDefault")
	}
	return m.newServiceConfigMap(v1alpha1.APIMapComponentType, "", oc, opt), false, nil
}
func (m *apiMapManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return m.newSinglePortService(v1alpha1.APIMapComponentType, oc, oc.Spec.APIMap.Service.InternalOnly, int32(oc.Spec.APIMap.Service.NodePort), constants.APIMapPort, oc.Spec.APIMap.SlaveReplicas > 0)
}

func (m *apiMapManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.APIMapComponentType, "", oc, &oc.Spec.APIMap.DeploymentSpec, constants.APIMapPort, false, false)
}

func (m *apiMapManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.APIMap
}

func (m *apiMapManager) supportsReadOnlyService() bool {
	return false
}

func (m *apiMapManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return nil
}

func (m *apiMapManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return func(s *mcclient.ClientSession) error {
		if m.IsDisabled(oc) {
			return onecloud.EnsureDisableService(s, m.GetServiceName())
		} else {
			return onecloud.EnsureEnableService(s, m.GetServiceName(), m.supportsReadOnlyService() && oc.Spec.APIMap.SlaveReplicas > 0)
		}
	}
}
