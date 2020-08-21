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

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud/pkg/image/options"
)

type glanceManager struct {
	*ComponentManager
}

func newGlanceManager(man *ComponentManager) manager.Manager {
	return &glanceManager{man}
}

func (m *glanceManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Glance.Disable)
}

func (m *glanceManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Glance.DB
}

func (m *glanceManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Glance.CloudUser
}

func (m *glanceManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return man.Glance()
}

func (m *glanceManager) getService(oc *v1alpha1.OnecloudCluster) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.GlanceComponentType, oc, constants.GlanceAPIPort)}
}

func (m *glanceManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeGlance); err != nil {
		return nil, err
	}
	config := cfg.Glance
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceDBCommonOptions.ServiceCommonOptions)

	opt.FilesystemStoreDatadir = constants.GlanceFileStoreDir
	//opt.TorrentStoreDir = constants.GlanceTorrentStoreDir
	opt.EnableTorrentService = false
	// TODO: fix this
	opt.AutoSyncTable = true
	opt.Port = constants.GlanceAPIPort

	return m.newServiceConfigMap(v1alpha1.GlanceComponentType, oc, opt), nil
}

func (m *glanceManager) getPVC(oc *v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.Glance
	pvc, err := m.ComponentManager.newPVC(v1alpha1.GlanceComponentType, oc, cfg)
	if err != nil {
		return nil, err
	}
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Annotations[constants.SpecifiedPresistentVolumePath] = constants.GlanceDataStore
	return pvc, nil
}

func (m *glanceManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.GlanceComponentType, oc, oc.Spec.Glance.DeploymentSpec, constants.GlanceAPIPort, true, false)
	if err != nil {
		return nil, err
	}
	deploy = m.removeDeploymentAffinity(deploy)
	podTemplate := &deploy.Spec.Template.Spec
	podVols := podTemplate.Volumes
	volMounts := podTemplate.Containers[0].VolumeMounts

	// if we are not use local path, propagation mount '/opt/cloud' to glance
	// and persistent volume will propagate to host, then host deployer can find it
	if oc.Spec.Glance.StorageClassName != v1alpha1.DefaultStorageClass {
		var (
			hostPathDirOrCreate = corev1.HostPathDirectoryOrCreate
			mountMode           = corev1.MountPropagationBidirectional
			privileged          = true
		)
		podVols = append(podVols, corev1.Volume{
			Name: "opt-cloud",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/opt/cloud",
					Type: &hostPathDirOrCreate,
				},
			},
		})
		volMounts = append(volMounts, corev1.VolumeMount{
			Name:             "opt-cloud",
			MountPath:        "/opt/cloud",
			MountPropagation: &mountMode,
		})
		podTemplate.Containers[0].SecurityContext = &corev1.SecurityContext{
			Privileged: &privileged,
		}
		deploy.Spec.Strategy.Type = apps.RecreateDeploymentStrategyType
	}

	// data store pvc, mount path: /opt/cloud/workspace/data/glance
	podVols = append(podVols, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: m.newPvcName(oc.GetName(), oc.Spec.Glance.StorageClassName, v1alpha1.GlanceComponentType),
				ReadOnly:  false,
			},
		},
	})
	volMounts = append(volMounts, corev1.VolumeMount{
		Name:      "data",
		MountPath: constants.GlanceDataStore,
	})

	// var run
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
	if deploy.Spec.Template.ObjectMeta.Labels == nil {
		deploy.Spec.Template.ObjectMeta.Labels = make(map[string]string)
	}
	deploy.Spec.Template.ObjectMeta.Labels[constants.OnecloudHostDeployerLabelKey] = ""
	if deploy.Spec.Selector == nil {
		deploy.Spec.Selector = &metav1.LabelSelector{}
	}
	if deploy.Spec.Selector.MatchLabels == nil {
		deploy.Spec.Selector.MatchLabels = make(map[string]string)
	}
	deploy.Spec.Selector.MatchLabels[constants.OnecloudHostDeployerLabelKey] = ""
	return deploy, nil
}

func (m *glanceManager) getPodLabels() map[string]string {
	return map[string]string{
		constants.OnecloudHostDeployerLabelKey: "",
	}
}

func (m *glanceManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.Glance.DeploymentStatus
}
