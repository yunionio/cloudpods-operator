// Copyright 2020 Yunion
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

	"yunion.io/x/onecloud/pkg/vpcagent/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type vpcAgentManager struct {
	*ComponentManager
}

func newVpcAgentManager(man *ComponentManager) manager.Manager {
	return &vpcAgentManager{man}
}

func (m *vpcAgentManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.VpcAgent.Disable)
}

func (m *vpcAgentManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.VpcAgent.CloudUser
}

func (m *vpcAgentManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	var (
		opts = &options.Options{}
		prog = "vpcagent"
	)
	if err := SetOptionsDefault(opts, prog); err != nil {
		return nil, err
	}
	opts.OvnNorthDatabase = fmt.Sprintf("tcp:%s:%d",
		controller.NewClusterComponentName(
			oc.GetName(),
			v1alpha1.OvnNorthComponentType,
		),
		constants.OvnNorthDbPort,
	)
	config := cfg.VpcAgent
	SetServiceCommonOptions(&opts.CommonOptions, oc, config.ServiceCommonOptions)
	return m.newServiceConfigMap(v1alpha1.VpcAgentComponentType, oc, opts), nil
}

func (m *vpcAgentManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	return m.newCloudServiceDeployment(v1alpha1.VpcAgentComponentType, oc, oc.Spec.VpcAgent, nil, nil, false)
}

func (m *vpcAgentManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.VpcAgent
}
