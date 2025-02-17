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
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type yunionagentManager struct {
	*ComponentManager
}

func newYunionagentManager(man *ComponentManager) manager.ServiceManager {
	return &yunionagentManager{man}
}

type yunionagentOptions struct {
	options.CommonOptions
	options.DBOptions
}

func (m *yunionagentManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *yunionagentManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.YunionagentComponentType
}

func (m *yunionagentManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Yunionagent.Disable || !IsEEOrESEEdition(oc)
}

func (m *yunionagentManager) GetServiceName() string {
	return constants.ServiceNameYunionAgent
}

func (m *yunionagentManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if !IsEEOrESEEdition(oc) {
		return nil
	}
	return syncComponent(m, oc, "")
}

func (m *yunionagentManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Yunionagent.DB
}

func (m *yunionagentManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Yunionagent.DbEngine)
}

func (m *yunionagentManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Yunionagent.CloudUser
}

func (m *yunionagentManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.YunionAgent()
}

func (m *yunionagentManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &yunionagentOptions{}
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeYunionAgent); err != nil {
		return nil, false, err
	}
	config := cfg.Yunionagent

	switch oc.Spec.GetDbEngine(oc.Spec.Yunionagent.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)
	opt.AutoSyncTable = true
	// yunionagent use hostNetwork
	opt.Port = oc.Spec.Yunionagent.Service.NodePort

	return m.newServiceConfigMap(v1alpha1.YunionagentComponentType, "", oc, opt), false, nil
}

func (m *yunionagentManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	// use headless service
	svcName := controller.NewClusterComponentName(oc.GetName(), v1alpha1.YunionagentComponentType)
	appLabel := m.getComponentLabel(oc, v1alpha1.YunionagentComponentType)
	svc := &corev1.Service{
		ObjectMeta: m.getObjectMeta(oc, svcName, appLabel),
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  appLabel,
		},
	}
	return []*corev1.Service{svc}
}

func (m *yunionagentManager) getDaemonSet(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.DaemonSet, error) {
	cType := v1alpha1.YunionagentComponentType
	dsSpec := oc.Spec.Yunionagent
	edition := v1alpha1.GetEdition(oc)
	command := []string{
		fmt.Sprintf("/opt/yunion/bin/%s", cType.String()),
		"--config",
		fmt.Sprintf("/etc/yunion/%s.conf", cType.String()),
	}
	if edition == constants.OnecloudEnterpriseSupportEdition {
		command = append(command, "--edition", edition)
	}
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            cType.String(),
				Image:           dsSpec.Image,
				Command:         command,
				ImagePullPolicy: dsSpec.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
		}
	}
	if dsSpec.NodeSelector == nil {
		dsSpec.NodeSelector = make(map[string]string)
	}
	if !controller.DisableNodeSelectorController {
		dsSpec.NodeSelector[constants.OnecloudControllerLabelKey] = "enable"
	}
	ds, err := m.newDaemonSet(
		cType, oc, cfg,
		NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, cType), cType),
		dsSpec.DaemonSetSpec, "", nil, cf)
	if err != nil {
		return nil, err
	}
	ds.Spec.Template.Spec.ServiceAccountName = constants.ServiceAccountOnecloudOperator
	return ds, nil
}
