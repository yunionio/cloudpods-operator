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
	"path"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud/pkg/vpcagent/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type vpcAgentManager struct {
	*ComponentManager
}

func newVpcAgentManager(man *ComponentManager) manager.Manager {
	return &vpcAgentManager{man}
}

func (m *vpcAgentManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *vpcAgentManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.VpcAgentComponentType
}

func (m *vpcAgentManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if oc.Spec.DisableLocalVpc {
		return nil
	}
	return syncComponent(m, oc, oc.Spec.VpcAgent.Disable, "")
}

func (m *vpcAgentManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.VpcAgent.CloudUser
}

func (m *vpcAgentManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opts := &options.Options{}
	if err := option.SetOptionsDefault(opts, constants.ServiceTypeVpcagent); err != nil {
		return nil, false, err
	}
	opts.OvnNorthDatabase = fmt.Sprintf("tcp:%s:%d",
		controller.NewClusterComponentName(
			oc.GetName(),
			v1alpha1.OvnNorthComponentType,
		),
		constants.OvnNorthDbPort,
	)
	config := cfg.VpcAgent
	options.SetOptionsServiceTLS(&opts.BaseOptions, false)
	options.SetServiceCommonOptions(&opts.CommonOptions, oc, config.ServiceCommonOptions)
	opts.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opts.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opts.Port = config.Port
	return m.newServiceConfigMap(v1alpha1.VpcAgentComponentType, "", oc, opts), false, nil
}

func (m *vpcAgentManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man,
		v1alpha1.VpcAgentComponentType,
		constants.ServiceNameVpcagent,
		constants.ServiceTypeVpcagent,
		man.GetCluster().Spec.VpcAgent.Service.NodePort, "")
}

func (m *vpcAgentManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.VpcAgentComponentType, oc, int32(oc.Spec.VpcAgent.Service.NodePort), int32(cfg.VpcAgent.Port))}
}

func (m *vpcAgentManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.VpcAgentComponentType, "", oc, &oc.Spec.VpcAgent.DeploymentSpec, int32(cfg.VpcAgent.Port), false, false)
}

func (m *vpcAgentManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.VpcAgent
}
