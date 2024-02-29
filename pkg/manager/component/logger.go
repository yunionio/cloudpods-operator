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
)

type loggerManager struct {
	*ComponentManager
}

func newLoggerManager(man *ComponentManager) manager.Manager {
	return &loggerManager{man}
}

func (m *loggerManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *loggerManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.LoggerComponentType
}

func (m *loggerManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Logger.Disable, "")
}

func (m *loggerManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Logger.DB
}

func (m *loggerManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Logger.DbEngine)
}

func (m *loggerManager) getClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Logger.ClickhouseConf
}

func (m *loggerManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Logger.CloudUser
}

func (m *loggerManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewLogger().GetPhaseControl(man)
}

func (m *loggerManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewLogger().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}

	return m.newServiceConfigMap(v1alpha1.LoggerComponentType, zone, oc, opt), false, nil
}

func (m *loggerManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.LoggerComponentType, oc, int32(oc.Spec.Logger.Service.NodePort), int32(cfg.Logger.Port))}
}

func (m *loggerManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.LoggerComponentType, "", oc, &oc.Spec.Logger.DeploymentSpec, constants.LoggerPort, true, false)
}

func (m *loggerManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Logger
}
