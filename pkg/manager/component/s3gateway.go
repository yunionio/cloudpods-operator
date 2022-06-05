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

	"yunion.io/x/onecloud/pkg/s3gateway/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type s3gatewayManager struct {
	*ComponentManager
}

func newS3gatewayManager(man *ComponentManager) manager.Manager {
	return &s3gatewayManager{man}
}

func (m *s3gatewayManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *s3gatewayManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if !oc.Spec.EnableS3Gateway {
		return nil
	}
	return syncComponent(m, oc, oc.Spec.S3gateway.Disable, "")
}

func (m *s3gatewayManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.S3gateway.CloudUser
}

func (m *s3gatewayManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.S3gatewayComponentType,
		constants.ServiceNameS3gateway, constants.ServiceTypeS3gateway,
		constants.S3gatewayPort, "")
}

func (m *s3gatewayManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeS3gateway); err != nil {
		return nil, false, err
	}
	config := cfg.S3gateway
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config)

	return m.newServiceConfigMap(v1alpha1.S3gatewayComponentType, "", oc, opt), false, nil
}

func (m *s3gatewayManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.S3gatewayComponentType, oc, constants.S3gatewayPort)}
}

func (m *s3gatewayManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.S3gatewayComponentType, "", oc, &oc.Spec.S3gateway, constants.S3gatewayPort, false, false)
}

func (m *s3gatewayManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.S3gateway
}
