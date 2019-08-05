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

	"yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type yunionagentManager struct {
	*ComponentManager
}

func newYunionagentManager(man *ComponentManager) manager.Manager {
	return &yunionagentManager{man}
}

type yunionagentOptions struct {
	options.CommonOptions
	options.DBOptions
}

func (m *yunionagentManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc)
}

func (m *yunionagentManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Yunionagent.DB
}

func (m *yunionagentManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Yunionagent.CloudUser
}

func (m *yunionagentManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return man.YunionAgent()
}

func (m *yunionagentManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	opt := &yunionagentOptions{}
	if err := SetOptionsDefault(opt, constants.ServiceTypeYunionAgent); err != nil {
		return nil, err
	}
	config := cfg.Yunionagent
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true

	return m.newServiceConfigMap(v1alpha1.YunionagentComponentType, oc, opt), nil
}

func (m *yunionagentManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service {
	return m.newSingleNodePortService(v1alpha1.YunionagentComponentType, oc, constants.YunionAgentPort)
}

func (m *yunionagentManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.YunionagentComponentType, oc, oc.Spec.Yunionagent, constants.YunionAgentPort, false)
}
