// Copyright 2020 Yunion
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
	"k8s.io/apimachinery/pkg/util/intstr"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type ovnNorthManager struct {
	*ComponentManager
}

func newOvnNorthManager(man *ComponentManager) manager.Manager {
	return &ovnNorthManager{man}
}

func (m *ovnNorthManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *ovnNorthManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.OvnNorthComponentType
}

func (m *ovnNorthManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if oc.Spec.DisableLocalVpc {
		return nil
	}
	return syncComponent(m, oc, oc.Spec.OvnNorth.Disable, "")
}

func (m *ovnNorthManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	np0 := NewServiceNodePort("north-db", constants.OvnNorthDbPort)
	np0.TargetPort = intstr.FromInt(6641)
	np1 := NewServiceNodePort("south-db", constants.OvnSouthDbPort)
	np1.TargetPort = intstr.FromInt(6642)
	ports := []corev1.ServicePort{
		np0,
		np1,
	}
	return []*corev1.Service{m.newNodePortService(v1alpha1.OvnNorthComponentType, oc, ports)}
}

func (m *ovnNorthManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            v1alpha1.OvnNorthComponentType.String(),
				Image:           oc.Spec.OvnNorth.Image,
				ImagePullPolicy: oc.Spec.OvnNorth.ImagePullPolicy,
				Command:         []string{"/start.sh", "north"},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{
							corev1.Capability("SYS_NICE"),
						},
					},
				},
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.OvnNorthComponentType, "", oc, NewVolumeHelper(oc, "", v1alpha1.OvnNorthComponentType), &oc.Spec.OvnNorth, containersF)
	if err != nil {
		return nil, err
	}
	return deploy, nil
}

func (m *ovnNorthManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.OvnNorth
}
