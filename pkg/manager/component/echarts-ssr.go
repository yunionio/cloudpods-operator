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
	"k8s.io/apimachinery/pkg/util/intstr"

	"yunion.io/x/onecloud/pkg/mcclient"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

type echartsSSRManager struct {
	*ComponentManager
}

func newEChartsSSR(man *ComponentManager) manager.Manager {
	return &echartsSSRManager{man}
}

func (m *echartsSSRManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *echartsSSRManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.EChartsSSRComponentType
}

func (m *echartsSSRManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if oc.Spec.EChartsSSR.Disable != nil && *oc.Spec.EChartsSSR.Disable {
		controller.RunWithSession(oc, func(s *mcclient.ClientSession) error {
			return onecloud.EnsureDisableService(s, constants.ServiceNameEChartsSSR)
		})
	}
	if !IsEnterpriseEdition(oc) {
		return nil
	}
	return syncComponent(m, oc, *oc.Spec.EChartsSSR.Disable, "")
}

func (m *echartsSSRManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	ports := []corev1.ServicePort{
		{
			Name:       "server",
			Protocol:   corev1.ProtocolTCP,
			Port:       constants.EChartsSSRPort,
			TargetPort: intstr.FromInt(constants.EChartsSSRPort),
		},
	}
	return []*corev1.Service{m.newService(v1alpha1.EChartsSSRComponentType, oc, corev1.ServiceTypeClusterIP, ports)}
}

func (m *echartsSSRManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	spec := oc.Spec.EChartsSSR.ToDeploymentSpec()
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            string(v1alpha1.EChartsSSRComponentType),
				Image:           spec.Image,
				ImagePullPolicy: spec.ImagePullPolicy,
				Ports: []corev1.ContainerPort{
					{
						Name:          "server",
						ContainerPort: constants.EChartsSSRPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: volMounts,
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.EChartsSSRComponentType, "", oc, NewVolumeHelper(oc, "", v1alpha1.EChartsSSRComponentType), spec, containersF)
	if err != nil {
		return nil, err
	}
	oc.Spec.EChartsSSR.FillBySpec(spec)
	return deploy, nil
}

func (m *echartsSSRManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.ECharts
}

func (m *echartsSSRManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.EChartsSSR()
}
