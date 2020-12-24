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

	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type billingManager struct {
	*ComponentManager
}

func newBillingManager(man *ComponentManager) manager.Manager {
	return &billingManager{man}
}

func (m *billingManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Billing.Disable, "")
}

func (m *billingManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Billing.DB
}

func (m *billingManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Billing.CloudUser
}

func (m *billingManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.BillingComponentType,
		constants.ServiceNameBilling, constants.ServiceTypeBilling,
		constants.BillingPort, "")
}

type billingOptions struct {
	common_options.CommonOptions
	common_options.DBOptions

	AutoSubscriptionCheckSeconds int    `help:"how frequently to checkout subscriptions" default:"60"`
	DefaultGracePeriod           string `help:"default graceful period for paying dued bill, default is 30minutes" default:"30i"`
}

func (m *billingManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &billingOptions{}
	if err := SetOptionsDefault(opt, constants.ServiceTypeBilling); err != nil {
		return nil, false, err
	}
	config := cfg.Billing
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.Port = constants.BillingPort

	return m.newServiceConfigMap(v1alpha1.BillingComponentType, "", oc, opt), false, nil
}

func (m *billingManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.BillingComponentType, oc, constants.BillingPort)}
}

func (m *billingManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.BillingComponentType, "", oc, oc.Spec.Billing, constants.BillingPort, true, false)
}

func (m *billingManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Billing
}
