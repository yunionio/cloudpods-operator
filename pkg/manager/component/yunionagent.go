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
	if !IsEnterpriseEdition(oc) {
		return nil
	}
	return syncComponent(m, oc, oc.Spec.Yunionagent.Disable, "")
}

func (m *yunionagentManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Yunionagent.DB
}

func (m *yunionagentManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Yunionagent.CloudUser
}

func (m *yunionagentManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.YunionAgent()
}

func (m *yunionagentManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, error) {
	opt := &yunionagentOptions{}
	if err := SetOptionsDefault(opt, constants.ServiceTypeYunionAgent); err != nil {
		return nil, err
	}
	config := cfg.Yunionagent
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.Port = constants.YunionAgentPort

	return m.newServiceConfigMap(v1alpha1.YunionagentComponentType, "", oc, opt), nil
}

func (m *yunionagentManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
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
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:  cType.String(),
				Image: dsSpec.Image,
				Command: []string{
					fmt.Sprintf("/opt/yunion/bin/%s", cType.String()),
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", cType.String()),
				},
				ImagePullPolicy: dsSpec.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
		}
	}
	if dsSpec.NodeSelector == nil {
		dsSpec.NodeSelector = make(map[string]string)
	}
	dsSpec.NodeSelector[constants.OnecloudControllerLabelKey] = "enable"
	ds, err := m.newDaemonSet(
		cType, oc, cfg,
		NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, cType), cType),
		dsSpec, "", nil, cf)
	if err != nil {
		return nil, err
	}
	return ds, nil
}
