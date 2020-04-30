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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"yunion.io/x/onecloud/pkg/esxi/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type esxiManager struct {
	*ComponentManager
}

func newEsxiManager(m *ComponentManager) manager.Manager {
	return &esxiManager{m}
}

func (m *esxiManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.EsxiAgent.Disable)
}

func (m *esxiManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.EsxiAgent.CloudUser
}

func (m *esxiManager) getConfigMap(
	oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig,
) (*corev1.ConfigMap, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, ""); err != nil {
		return nil, err
	}
	// fill options
	opt.Zone = oc.Spec.Zone
	opt.ListenInterface = "eth0"

	config := cfg.EsxiAgent
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.Port = constants.EsxiAgentPort
	return m.newServiceConfigMap(v1alpha1.EsxiAgentComponentType, oc, opt), nil
}

func (m *esxiManager) getPVC(oc *v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.EsxiAgent
	return m.ComponentManager.newPVC(v1alpha1.EsxiAgentComponentType, oc, cfg)
}

func (m *esxiManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.EsxiAgent
}

func (m *esxiManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.
	Deployment, error) {

	dm, err := m.newCloudServiceSinglePortDeployment(v1alpha1.EsxiAgentComponentType, oc,
		oc.Spec.EsxiAgent.DeploymentSpec, constants.EsxiAgentPort, false, false)
	if err != nil {
		return nil, err
	}

	// workspace
	podTemplate := &dm.Spec.Template.Spec
	podVols := podTemplate.Volumes
	podVols = append(podVols, corev1.Volume{
		Name: "opt",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: controller.NewClusterComponentName(oc.GetName(), v1alpha1.EsxiAgentComponentType),
				ReadOnly:  false,
			},
		},
	})
	volMounts := podTemplate.Containers[0].VolumeMounts
	volMounts = append(volMounts, corev1.VolumeMount{
		Name:      "opt",
		MountPath: constants.EsxiAgentDataStore,
	})

	// /var/run
	var hostPathDirectory = corev1.HostPathDirectory

	podVols = append(podVols, corev1.Volume{
		Name: "run",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/run",
				Type: &hostPathDirectory,
			},
		},
	})
	volMounts = append(volMounts, corev1.VolumeMount{
		Name:      "run",
		ReadOnly:  false,
		MountPath: "/var/run",
	})
	podTemplate.Containers[0].VolumeMounts = volMounts
	podTemplate.Volumes = podVols

	// add pod label for pod affinity
	if dm.Spec.Template.ObjectMeta.Labels == nil {
		dm.Spec.Template.ObjectMeta.Labels = make(map[string]string)
	}
	dm.Spec.Template.ObjectMeta.Labels[constants.OnecloudHostDeployerLabelKey] = ""
	if dm.Spec.Selector == nil {
		dm.Spec.Selector = &metav1.LabelSelector{}
	}
	if dm.Spec.Selector.MatchLabels == nil {
		dm.Spec.Selector.MatchLabels = make(map[string]string)
	}
	dm.Spec.Selector.MatchLabels[constants.OnecloudHostDeployerLabelKey] = ""
	return dm, nil
}
