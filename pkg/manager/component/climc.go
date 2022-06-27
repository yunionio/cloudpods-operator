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
)

type climcManager struct {
	*ComponentManager
}

func newClimcComponentManager(base *ComponentManager) manager.Manager {
	return &climcManager{
		ComponentManager: base,
	}
}

func (m *climcManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *climcManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.ClimcComponentType
}

func (m *climcManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Climc.Disable, "")
}

func GetRCAdminEnv(oc *v1alpha1.OnecloudCluster) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "OS_USERNAME",
			Value: constants.SysAdminUsername,
		},
		{
			Name:  "OS_USERNAME",
			Value: constants.SysAdminUsername,
		},
		{
			Name:  "OS_PASSWORD",
			Value: oc.Spec.Keystone.BootstrapPassword,
		},
		{
			Name:  "OS_REGION_NAME",
			Value: oc.GetRegion(),
		},
		{
			Name:  "OS_AUTH_URL",
			Value: controller.GetAuthURL(oc),
		},
		{
			Name:  "OS_PROJECT_NAME",
			Value: constants.SysAdminProject,
		},
		{
			Name:  "YUNION_INSECURE",
			Value: "true",
		},
	}
}

func (m *climcManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "climc",
				Image:           oc.Spec.Climc.Image,
				ImagePullPolicy: oc.Spec.Climc.ImagePullPolicy,
				Command:         []string{"tail", "-f", "/dev/null"},
				Env:             GetRCAdminEnv(oc),
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.ClimcComponentType, "", oc, NewVolumeHelper(oc, "", v1alpha1.ClimcComponentType), &oc.Spec.Climc, containersF)
}
