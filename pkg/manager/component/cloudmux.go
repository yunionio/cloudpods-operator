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

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type cloudmuxManager struct {
	*ComponentManager
}

func newCloudmuxComponentManager(base *ComponentManager) manager.Manager {
	return &cloudmuxManager{
		ComponentManager: base,
	}
}

func (m *cloudmuxManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *cloudmuxManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.CloudmuxComponentType
}

func (m *cloudmuxManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return *oc.Spec.Cloudmux.Disable
}

func (m *cloudmuxManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *cloudmuxManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	spec := oc.Spec.Cloudmux.ToDeploymentSpec()
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "cloudmux",
				Image:           oc.Spec.Cloudmux.Image,
				ImagePullPolicy: oc.Spec.Cloudmux.ImagePullPolicy,
				Command:         []string{"tail", "-f", "/dev/null"},
				VolumeMounts:    volMounts,
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.CloudmuxComponentType, "", oc, NewVolumeHelper(oc, "", v1alpha1.CloudmuxComponentType), spec, containersF)
	if err != nil {
		return nil, err
	}
	oc.Spec.Cloudmux.FillBySpec(spec)
	return deploy, nil
}

func (m *cloudmuxManager) supportsReadOnlyService() bool {
	return false
}

func (m *cloudmuxManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return nil
}

func (m *cloudmuxManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return nil
}
