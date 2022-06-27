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
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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

func (m *regionManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *regionManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.RegionComponentType
}

func (m *regionManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.RegionServer.Disable, "")
}

func (m *regionManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.RegionServer.DB
}

func (m *regionManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.RegionServer.CloudUser
}

func (m *regionManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.Region()
}

func (m *regionManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*v1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeComputeV2); err != nil {
		return nil, false, err
	}
	config := cfg.RegionServer
	spec := oc.Spec.RegionServer
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceDBCommonOptions.ServiceCommonOptions)
	// TODO: fix this, currently init container can't sync table
	opt.AutoSyncTable = true

	opt.DNSDomain = spec.DNSDomain
	if spec.DNSServer == "" {
		spec.DNSServer = oc.Spec.LoadBalancerEndpoint
	}
	oc.Spec.RegionServer = spec
	opt.DNSServer = spec.DNSServer

	opt.PortV2 = constants.RegionPort
	err := m.setBaremetalPrepareConfigure(oc, cfg, opt)
	if err != nil {
		return nil, false, err
	}
	return m.newServiceConfigMap(v1alpha1.RegionComponentType, "", oc, opt), false, nil
}

func (m *regionManager) setBaremetalPrepareConfigure(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, opt *options.ComputeOptions) error {
	masterAddress := oc.Spec.LoadBalancerEndpoint
	opt.BaremetalPreparePackageUrl = fmt.Sprintf("https://%s/baremetal-prepare/baremetal_prepare.tar.gz", masterAddress)
	return nil
}

func (m *regionManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*v1.Service {
	ports := []corev1.ServicePort{
		NewServiceNodePort("api", constants.RegionPort),
	}
	return []*corev1.Service{m.newNodePortService(v1alpha1.RegionComponentType, oc, ports)}
}

func (m *regionManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.RegionComponentType, "", oc, &oc.Spec.RegionServer.DeploymentSpec, constants.RegionPort, true, true)
	if err != nil {
		return nil, err
	}
	deploy.Spec.Template.Spec.ServiceAccountName = constants.ServiceAccountOnecloudOperator
	return deploy, nil
}

func (m *regionManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.RegionServer.DeploymentStatus
}
