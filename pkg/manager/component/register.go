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
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type registerManager struct {
	*ComponentManager
}

func newRegisterManager(man *ComponentManager) manager.Manager {
	return &registerManager{man}
}

func (b *registerManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
	}
}

func (b *registerManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.RegisterComponentType
}

func (m *registerManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *registerManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Register.Disable
}

func (m *registerManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Register.DB
}

func (m *registerManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Register.CloudUser
}

func (m *registerManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.RegisterComponentType,
		constants.ServiceNameRegister, constants.ServiceTypeRegister,
		constants.RegisterPort, "")
}

type registerOptions struct {
	common_options.CommonOptions
	common_options.DBOptions

	AccessKeyId     string `help:"aliyun access key id"`
	AccessKeySecret string `help:"aliyun access key secret"`

	// 用户注册验证码
	ShowCaptcha bool `help:"show captcha or not. " default:"true"`
}

func (m *registerManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &registerOptions{}
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeRegister); err != nil {
		return nil, false, err
	}
	config := cfg.Register
	option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.Port = constants.RegisterPort

	return m.newServiceConfigMap(v1alpha1.RegisterComponentType, "", oc, opt), false, nil
}

func (m *registerManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.RegisterComponentType, oc, constants.RegisterPort, int32(cfg.Register.Port))}
}

func (m *registerManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.RegisterComponentType, "", oc, &oc.Spec.Register.DeploymentSpec, constants.RegisterPort, true, false)
}

func (m *registerManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Register
}
