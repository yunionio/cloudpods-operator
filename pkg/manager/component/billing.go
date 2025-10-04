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
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type billingManager struct {
	*ComponentManager
}

func newBillingManager(man *ComponentManager) manager.ServiceManager {
	return &billingManager{man}
}

func (b *billingManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (b *billingManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.BillingComponentType
}

func (m *billingManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Billing.Disable || !IsEnterpriseEdition(oc) || !isInProductVersion(m, oc)
}

func (m *billingManager) GetServiceName() string {
	return constants.ServiceNameBilling
}

func (b *billingManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(b, oc, "")
}

func (b *billingManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Billing.DB
}

func (m *billingManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Billing.DbEngine)
}

func (b *billingManager) getClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Billing.ClickhouseConf
}

func (b *billingManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Billing.CloudUser
}

func (b *billingManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man,
		v1alpha1.BillingComponentType,
		constants.ServiceNameBilling,
		constants.ServiceTypeBilling,
		man.GetCluster().Spec.Billing.Service.NodePort, "")
}

func (b *billingManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{b.newSinglePortService(v1alpha1.BillingComponentType, oc, oc.Spec.Billing.Service.InternalOnly, int32(oc.Spec.Billing.Service.NodePort), int32(cfg.Billing.Port))}
}

func (b *billingManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &option.CommonDBOptions{}
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeBilling); err != nil {
		return nil, false, err
	}

	config := cfg.Billing

	switch oc.Spec.GetDbEngine(oc.Spec.Billing.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)

	// TODO: fix this
	// opt.AutoSyncTable = true
	opt.Port = config.Port

	return b.newServiceConfigMap(v1alpha1.BillingComponentType, "", oc, opt), false, nil
}

func (b *billingManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return b.newCloudServiceSinglePortDeployment(v1alpha1.BillingComponentType, "", oc, &oc.Spec.Billing.DeploymentSpec, int32(cfg.Billing.Port), true, false)
}

func (b *billingManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.BillingStatus
}
