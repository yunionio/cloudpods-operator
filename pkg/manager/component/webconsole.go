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
	"strings"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
)

type webconsoleManager struct {
	*ComponentManager
}

func newWebconsoleManager(man *ComponentManager) manager.Manager {
	return &webconsoleManager{man}
}

func (m *webconsoleManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *webconsoleManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.WebconsoleComponentType
}

func (m *webconsoleManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Webconsole.Disable, "")
}

func (m *webconsoleManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Webconsole.DB
}

func (m *webconsoleManager) getClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Webconsole.ClickhouseConf
}

func (m *webconsoleManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Webconsole.CloudUser
}

func (m *webconsoleManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewWebconsole().GetPhaseControl(man)
}

func (m *webconsoleManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewWebconsole().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}
	return m.shouldSyncConfigmap(oc, v1alpha1.WebconsoleComponentType, opt, func(optStr string) bool {
		if !strings.Contains(optStr, "sql_connection") {
			// hack: force update old configmap if not contains sql_connection option
			return true
		}
		return false
	})
}

func (m *webconsoleManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.WebconsoleComponentType, oc, int32(oc.Spec.Webconsole.Service.NodePort), int32(cfg.Webconsole.Port))}
}

func (m *webconsoleManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.WebconsoleComponentType, "", oc, &oc.Spec.Webconsole.DeploymentSpec, constants.WebconsolePort, false, false)
}

func (m *webconsoleManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Webconsole
}
