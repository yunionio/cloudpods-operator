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
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

type ansibleManager struct {
	*ComponentManager
}

func newAnsibleManager(man *ComponentManager) manager.ServiceManager {
	return &ansibleManager{man}
}

func (m *ansibleManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *ansibleManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.AnsibleServerComponentType
}

func (m *ansibleManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.AnsibleServer.Disable || !isInProductVersion(m, oc)
}

func (m *ansibleManager) GetServiceName() string {
	return constants.ServiceNameAnsibleServer
}

func (m *ansibleManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *ansibleManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return component.NewAnsibleServer().GetDefaultDBConfig(cfg)
}

func (m *ansibleManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.AnsibleServer.DbEngine)
}

func (m *ansibleManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return component.NewAnsibleServer().GetDefaultCloudUser(cfg)
}

func (m *ansibleManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewAnsibleServer().GetPhaseControl(man)
}

func (m *ansibleManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewAnsibleServer().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}
	return m.newServiceConfigMap(v1alpha1.AnsibleServerComponentType, "", oc, opt), false, nil
}

func (m *ansibleManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	aCfg := cfg.AnsibleServer
	return m.newSinglePortService(v1alpha1.AnsibleServerComponentType, oc, oc.Spec.AnsibleServer.Service.InternalOnly, int32(oc.Spec.AnsibleServer.Service.NodePort), int32(aCfg.Port), oc.Spec.AnsibleServer.SlaveReplicas > 0)
}

func (m *ansibleManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "server",
				Image:           oc.Spec.AnsibleServer.Image,
				ImagePullPolicy: oc.Spec.AnsibleServer.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/ansibleserver", "--config", "/etc/yunion/ansibleserver.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.AnsibleServerComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.AnsibleServerComponentType), v1alpha1.AnsibleServerComponentType), &oc.Spec.AnsibleServer.DeploymentSpec, cf)
}

func (m *ansibleManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.AnsibleServer
}

func (m *ansibleManager) supportsReadOnlyService() bool {
	return true
}

func (m *ansibleManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return m.genReadonlyDeployment(v1alpha1.AnsibleServerComponentType, oc, deployment, &oc.Spec.AnsibleServer.DeploymentSpec)
}

func (m *ansibleManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return func(s *mcclient.ClientSession) error {
		if m.IsDisabled(oc) {
			return onecloud.EnsureDisableService(s, m.GetServiceName())
		} else {
			return onecloud.EnsureEnableService(s, m.GetServiceName(), m.supportsReadOnlyService() && oc.Spec.AnsibleServer.SlaveReplicas > 0)
		}
	}
}
