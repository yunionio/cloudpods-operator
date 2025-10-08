// Copyright 2025 Yunion
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
	"yunion.io/x/onecloud/pkg/llm/options"
)

type llmManager struct {
	*ComponentManager
}

func newLLMManager(man *ComponentManager) manager.ServiceManager {
	return &llmManager{man}
}

func (m *llmManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *llmManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.LLMComponentType
}

func (m *llmManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.LLM.Disable
}

func (m *llmManager) GetServiceName() string {
	return constants.ServiceNameLLM
}

func (m *llmManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *llmManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.LLM.DB
}

func (m *llmManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.LLM.DbEngine)
}

func (m *llmManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.LLM.CloudUser
}

func (m *llmManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.LLM()
}

func (m *llmManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeLLM); err != nil {
		return nil, false, err
	}
	config := cfg.LLM

	switch oc.Spec.GetDbEngine(oc.Spec.LLM.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)
	opt.Port = config.Port
	opt.AutoSyncTable = true

	return m.newServiceConfigMap(v1alpha1.LLMComponentType, "", oc, opt), false, nil
}

func (m *llmManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSinglePortService(v1alpha1.LLMComponentType, oc, oc.Spec.LLM.Service.InternalOnly, int32(oc.Spec.LLM.Service.NodePort), int32(cfg.LLM.Port))}
}

func (m *llmManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.LLMComponentType, "", oc, &oc.Spec.LLM.DeploymentSpec, int32(cfg.LLM.Port), false, false)
}

func (m *llmManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.LLM
}
