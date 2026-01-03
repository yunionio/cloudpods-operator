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
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *climcManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.ClimcComponentType
}

func (m *climcManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Climc.Disable
}

func (m *climcManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *climcManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	content := component.GetRCAdminContent(oc, false)
	return m.newConfigMap(m.GetComponentType(), "", oc, content), false, nil
}

func (m *climcManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	rcadminName := "rcadmin"
	configMap := controller.ComponentConfigMapName(oc, m.GetComponentType())
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		// find configmap and map it to /etc/yunion/rcadmin
		volMounts = append(volMounts, corev1.VolumeMount{
			Name:      rcadminName,
			ReadOnly:  true,
			MountPath: component.YUNION_ETC_CONFIG_DIR,
		})

		return []corev1.Container{
			{
				Name:            "climc",
				Image:           oc.Spec.Climc.Image,
				ImagePullPolicy: oc.Spec.Climc.ImagePullPolicy,
				Command:         []string{"bash", "/opt/climc-entrypoint.sh"},
				Env:             component.GetRCAdminEnv(oc, true),
				VolumeMounts:    volMounts,
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.ClimcComponentType, "", oc, NewVolumeHelper(oc, "", v1alpha1.ClimcComponentType), &oc.Spec.Climc, containersF)
	if err != nil {
		return nil, err
	}
	rcadminVol := corev1.Volume{
		Name: rcadminName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMap,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  constants.VolumeConfigName,
						Path: rcadminName,
					},
				},
			},
		},
	}
	deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, rcadminVol)
	return deploy, nil
}

func (m *climcManager) supportsReadOnlyService() bool {
	return false
}

func (m *climcManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return nil
}

func (m *climcManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return nil
}
