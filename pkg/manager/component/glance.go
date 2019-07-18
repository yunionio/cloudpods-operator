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
	"yunion.io/x/onecloud/pkg/image/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type glanceManager struct {
	*ComponentManager
}

func newGlanceManager(man *ComponentManager) manager.Manager {
	return &glanceManager{man}
}

func (m *glanceManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc)
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

func (m *glanceManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service {
	return m.newSingleNodePortService(v1alpha1.GlanceComponentType, oc, constants.GlanceAPIPort)
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
	opt.TorrentStoreDir = constants.GlanceTorrentStoreDir
	opt.EnableTorrentService = false

	return m.newServiceConfigMap(v1alpha1.GlanceComponentType, oc, opt), nil
}

func (m *glanceManager) getPVC(oc *v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.Glance
	return m.ComponentManager.newPVC(v1alpha1.GlanceComponentType, oc, cfg)
}

func (m *glanceManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.GlanceComponentType, oc, oc.Spec.Glance.DeploymentSpec, constants.GlanceAPIPort, true)
	if err != nil {
		return nil, err
	}
	podTemplate := &deploy.Spec.Template.Spec
	podVols := podTemplate.Volumes
	podVols = append(podVols, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: controller.NewClusterComponentName(oc.GetName(), v1alpha1.GlanceComponentType),
				ReadOnly:  false,
			},
		},
	})
	volMounts := podTemplate.Containers[0].VolumeMounts
	volMounts = append(volMounts, corev1.VolumeMount{
		Name:      "data",
		MountPath: constants.GlanceDataStore,
	})
	podTemplate.Containers[0].VolumeMounts = volMounts
	podTemplate.Volumes = podVols
	return deploy, nil
}

func (m *glanceManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.Glance.DeploymentStatus
}
