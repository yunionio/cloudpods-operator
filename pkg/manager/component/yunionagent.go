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
	return syncComponent(m, oc, oc.Spec.Yunionagent.Disable)
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
	opt.Port = constants.YunionAgentPort

	return m.newServiceConfigMap(v1alpha1.YunionagentComponentType, oc, opt), nil
}

func (m *yunionagentManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service {
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
	return svc
}

// use local volume pvc avoid pod migrate
func (m *yunionagentManager) getPVC(oc *v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.Yunionagent
	return m.newPVC(v1alpha1.YunionagentComponentType, oc, cfg)
}

func (m *yunionagentManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.YunionagentComponentType, oc, oc.Spec.Yunionagent.DeploymentSpec, constants.YunionAgentPort, false)
	if err != nil {
		return nil, err
	}
	podSpec := &deploy.Spec.Template.Spec
	podSpec.HostNetwork = true
	podSpec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: controller.NewClusterComponentName(oc.GetName(), v1alpha1.YunionagentComponentType),
				ReadOnly:  false,
			},
		},
	})
	volMounts := podSpec.Containers[0].VolumeMounts
	volMounts = append(volMounts, corev1.VolumeMount{
		Name:      "data",
		MountPath: "/mnt",
	})
	podSpec.Containers[0].VolumeMounts = volMounts
	return deploy, nil
}

func (m *yunionagentManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.Yunionagent
}
