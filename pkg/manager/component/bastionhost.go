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
	"path"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type bastionHostManager struct {
	*ComponentManager
}

func newBastionHostManager(man *ComponentManager) manager.ServiceManager {
	return &bastionHostManager{man}
}

func (m *bastionHostManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *bastionHostManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.BastionHostComponentType
}

func (m *bastionHostManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.BastionHost.Disable || !IsEnterpriseEdition(oc)
}

func (m *bastionHostManager) GetServiceName() string {
	return constants.ServiceNameBastionHost
}

func (m *bastionHostManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if !IsEnterpriseEdition(oc) {
		return nil
	}
	return syncComponent(m, oc, "")
}

func (m *bastionHostManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.BastionHost.DB
}

func (m *bastionHostManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.BastionHost.DbEngine)
}

func (m *bastionHostManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.BastionHost.CloudUser
}

func (m *bastionHostManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.BastionHostComponentType,
		constants.ServiceNameBastionHost, constants.ServiceTypeBastionHost,
		man.GetCluster().Spec.BastionHost.Service.NodePort, "")
}

func (m *bastionHostManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &option.CommonDBOptions{}
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeBastionHost); err != nil {
		return nil, false, err
	}
	config := cfg.BastionHost

	switch oc.Spec.GetDbEngine(oc.Spec.BastionHost.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	// opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = config.Port
	return m.newServiceConfigMap(v1alpha1.BastionHostComponentType, "", oc, opt), false, nil
}

func (m *bastionHostManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSinglePortService(v1alpha1.BastionHostComponentType, oc, oc.Spec.BastionHost.Service.InternalOnly, int32(oc.Spec.BastionHost.Service.NodePort), int32(cfg.BastionHost.Port))}
}

func (m *bastionHostManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.BastionHostComponentType, "", oc, &oc.Spec.BastionHost.DeploymentSpec, int32(cfg.BastionHost.Port), true, false)
}

func (m *bastionHostManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.BastionHost
}
