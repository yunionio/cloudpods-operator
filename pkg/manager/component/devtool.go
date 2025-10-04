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

	"yunion.io/x/onecloud/pkg/devtool/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type devtoolManager struct {
	*ComponentManager
}

func newDevtoolManager(man *ComponentManager) manager.ServiceManager {
	return &devtoolManager{man}
}

func (m *devtoolManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *devtoolManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.DevtoolComponentType
}

func (m *devtoolManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Devtool.Disable || !isInProductVersion(m, oc)
}

func (m *devtoolManager) GetServiceName() string {
	return constants.ServiceNameDevtool
}

func (m *devtoolManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *devtoolManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Devtool.DB
}

func (m *devtoolManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Devtool.DbEngine)
}

func (m *devtoolManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Devtool.CloudUser
}

func (m *devtoolManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.Devtool()
}

func (m *devtoolManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeDevtool); err != nil {
		return nil, false, err
	}
	config := cfg.Devtool

	switch oc.Spec.GetDbEngine(oc.Spec.Devtool.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.Port = config.Port
	opt.AutoSyncTable = true

	return m.newServiceConfigMap(v1alpha1.DevtoolComponentType, "", oc, opt), false, nil
}

func (m *devtoolManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSinglePortService(v1alpha1.DevtoolComponentType, oc, oc.Spec.Devtool.Service.InternalOnly, int32(oc.Spec.Devtool.Service.NodePort), int32(cfg.Devtool.Port))}
}

func (m *devtoolManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.DevtoolComponentType, "", oc, &oc.Spec.Devtool.DeploymentSpec, int32(cfg.Devtool.Port), false, false)
}

func (m *devtoolManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Devtool
}
