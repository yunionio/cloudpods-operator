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

type cloudidManager struct {
	*ComponentManager
}

func newCloudIdManager(man *ComponentManager) manager.ServiceManager {
	return &cloudidManager{man}
}

func (m *cloudidManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
	}
}

func (m *cloudidManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.CloudIdComponentType
}

func (m *cloudidManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.CloudId.Disable
}

func (m *cloudidManager) GetServiceName() string {
	return constants.ServiceNameCloudId
}

func (m *cloudidManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *cloudidManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return component.NewCloudId().GetDefaultDBConfig(cfg)
}

func (m *cloudidManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.CloudId.DbEngine)
}

func (m *cloudidManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return component.NewCloudId().GetDefaultCloudUser(cfg)
}

func (m *cloudidManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewCloudId().GetPhaseControl(man)
}

func (m *cloudidManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewCloudId().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}
	return m.newServiceConfigMap(v1alpha1.CloudIdComponentType, "", oc, opt), false, nil
}

func (m *cloudidManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.CloudIdComponentType, oc, int32(oc.Spec.CloudId.Service.NodePort), int32(cfg.CloudId.Port))}
}

func (m *cloudidManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "server",
				Image:           oc.Spec.CloudId.Image,
				ImagePullPolicy: oc.Spec.CloudId.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/cloudid", "--config", "/etc/yunion/cloudid.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.CloudIdComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.CloudIdComponentType), v1alpha1.CloudIdComponentType), &oc.Spec.CloudId.DeploymentSpec, cf)
}

func (m *cloudidManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.CloudId
}
