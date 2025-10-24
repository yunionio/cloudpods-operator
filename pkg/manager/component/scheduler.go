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

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
)

type schedulerManager struct {
	*ComponentManager
}

func newSchedulerManager(man *ComponentManager) manager.Manager {
	return &schedulerManager{man}
}

func (m *schedulerManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *schedulerManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.SchedulerComponentType
}

func (m *schedulerManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Scheduler.Disable || !isInProductVersion(m, oc)
}

func (m *schedulerManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *schedulerManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewScheduler().GetPhaseControl(man)
}

func (m *schedulerManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewScheduler().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, errors.Wrap(err, "scheduler: SetOptionsDefault")
	}
	return m.newServiceConfigMap(v1alpha1.SchedulerComponentType, "", oc, opt), false, nil
}

func (m *schedulerManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSinglePortService(v1alpha1.SchedulerComponentType, oc, oc.Spec.Scheduler.Service.InternalOnly, int32(oc.Spec.Scheduler.Service.NodePort), constants.SchedulerPort)}
}

func (m *schedulerManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.SchedulerComponentType, "", oc, &oc.Spec.Scheduler.DeploymentSpec, constants.SchedulerPort, false, false)
}

func (m *schedulerManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Scheduler
}
