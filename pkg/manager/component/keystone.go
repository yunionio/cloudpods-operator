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
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
	"yunion.io/x/onecloud/pkg/mcclient"
)

type keystoneManager struct {
	*ComponentManager
}

// newKeystoneComponentManager return *keystoneManager
func newKeystoneComponentManager(baseMan *ComponentManager) manager.Manager {
	return &keystoneManager{
		ComponentManager: baseMan,
	}
}

func (m *keystoneManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *keystoneManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.KeystoneComponentType
}

func (m *keystoneManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Keystone.Disable || !isInProductVersion(m, oc)
}

func (m *keystoneManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *keystoneManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	dbCfg := component.NewKeystone().GetDefaultDBConfig(cfg)
	return dbCfg
}

func (m *keystoneManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Keystone.DbEngine)
}

func (m *keystoneManager) getClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return component.NewKeystone().GetDefaultClickhouseConfig(cfg)
}

func (m *keystoneManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.Keystone()
}

func (m *keystoneManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Keystone.DeploymentStatus
}

func (m *keystoneManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	spec := oc.Spec.Keystone
	ksCfg := cfg.Keystone
	ports := []corev1.ServicePort{
		NewServiceNodePort("public", spec.PublicService.InternalOnly, int32(spec.PublicService.NodePort), int32(ksCfg.Port)),
		NewServiceNodePort("admin", spec.AdminService.InternalOnly, int32(spec.AdminService.NodePort), constants.KeystoneAdminPort),
	}

	return m.newNodePortService(v1alpha1.KeystoneComponentType, oc, spec.AdminService.InternalOnly, ports, oc.Spec.Keystone.SlaveReplicas > 0)
}

func (m *keystoneManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewKeystone().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}

	return m.newServiceConfigMap(v1alpha1.KeystoneComponentType, "", oc, opt), false, nil
}

func (m *keystoneManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	ksConfigMap := controller.ComponentConfigMapName(oc, v1alpha1.KeystoneComponentType)

	initContainersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "init",
				Image:           oc.Spec.Keystone.Image,
				ImagePullPolicy: oc.Spec.Keystone.ImagePullPolicy,
				Command: []string{
					"/opt/yunion/bin/keystone",
					"--config",
					"/etc/yunion/keystone.conf",
					"--auto-sync-table",
					// "--reset-admin-user-password",
					"--exit-after-db-init",
				},
				VolumeMounts: volMounts,
			},
		}
	}

	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:  v1alpha1.KeystoneComponentType.String(),
				Image: oc.Spec.Keystone.Image,
				Command: []string{
					"/opt/yunion/bin/keystone",
					"--config",
					"/etc/yunion/keystone.conf",
				},
				ImagePullPolicy: oc.Spec.Keystone.ImagePullPolicy,
				Ports: []corev1.ContainerPort{
					{
						Name:          "public",
						ContainerPort: int32(constants.KeystonePublicPort),
						Protocol:      corev1.ProtocolTCP,
					},
					{
						Name:          "admin",
						ContainerPort: int32(constants.KeystoneAdminPort),
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts:   volMounts,
				ReadinessProbe: generateReadinessProbe("/ping", int32(constants.KeystonePublicPort)),
				// LivenessProbe:  generateLivenessProbe("/ping", int32(constants.KeystonePublicPort)),
			},
		}
	}

	return m.newDefaultDeployment(v1alpha1.KeystoneComponentType, v1alpha1.KeystoneComponentType, oc, NewVolumeHelper(oc, ksConfigMap, v1alpha1.KeystoneComponentType), &oc.Spec.Keystone.DeploymentSpec, initContainersF, containersF)
}

func (m *keystoneManager) supportsReadOnlyService() bool {
	return true
}

func (m *keystoneManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return m.genReadonlyDeployment(v1alpha1.KeystoneComponentType, oc, deployment, &oc.Spec.Keystone.DeploymentSpec)
}

func (m *keystoneManager) GetServiceName() string {
	return constants.ServiceNameKeystone
}

func (m *keystoneManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return func(s *mcclient.ClientSession) error {
		if m.IsDisabled(oc) {
			return onecloud.EnsureDisableService(s, m.GetServiceName())
		} else {
			return onecloud.EnsureEnableService(s, m.GetServiceName(), m.supportsReadOnlyService() && oc.Spec.Keystone.SlaveReplicas > 0)
		}
	}
}
