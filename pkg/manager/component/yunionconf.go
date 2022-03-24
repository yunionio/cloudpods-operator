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
	"yunion.io/x/onecloud/pkg/yunionconf/options"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type yunoinconfManager struct {
	*ComponentManager
}

func newYunionconfManager(man *ComponentManager) manager.Manager {
	return &yunoinconfManager{man}
}

func (m *yunoinconfManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *yunoinconfManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Yunionconf.Disable, "")
}

func (m *yunoinconfManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Yunionconf.DB
}

func (m *yunoinconfManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Yunionconf.CloudUser
}

func (m *yunoinconfManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.YunionconfComponentType,
		constants.ServiceNameYunionConf, constants.ServiceTypeYunionConf,
		constants.YunionConfPort, "")
}

func (m *yunoinconfManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeYunionConf); err != nil {
		return nil, false, err
	}
	config := cfg.Yunionconf
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.Port = constants.YunionConfPort

	return m.newServiceConfigMap(v1alpha1.YunionconfComponentType, "", oc, opt), false, nil
}

func (m *yunoinconfManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.YunionconfComponentType, oc, constants.YunionConfPort)}
}

func (m *yunoinconfManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.YunionconfComponentType, "", oc, &oc.Spec.Yunionconf, constants.YunionConfPort, false, false)
}

func (m *yunoinconfManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Yunionconf
}
