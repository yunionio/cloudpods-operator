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

	"yunion.io/x/onecloud/pkg/compute/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type regionManager struct {
	*ComponentManager
}

// newRegionManager return *regionManager
func newRegionManager(man *ComponentManager) manager.Manager {
	return &regionManager{man}
}

func (m *regionManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.RegionServer.Disable)
}

func (m *regionManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.RegionServer.DB
}

func (m *regionManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.RegionServer.CloudUser
}

func (m *regionManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return man.Region()
}

func (m *regionManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeComputeV2); err != nil {
		return nil, err
	}
	config := cfg.RegionServer
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceDBCommonOptions.ServiceCommonOptions)
	// TODO: fix this, currently init container can't sync table
	opt.AutoSyncTable = true

	opt.PortV2 = constants.RegionPort
	return m.newServiceConfigMap(v1alpha1.RegionComponentType, oc, opt), nil
}

func (m *regionManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service {
	ports := []corev1.ServicePort{
		NewServiceNodePort("api", constants.RegionPort),
	}
	return m.newNodePortService(v1alpha1.RegionComponentType, oc, ports)
}

func (m *regionManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.RegionComponentType, oc, oc.Spec.RegionServer.DeploymentSpec, constants.RegionPort, true)
}

func (m *regionManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.RegionServer.DeploymentStatus
}
