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
)

type autoUpdateManager struct {
	*ComponentManager
}

func newAutoUpdateManager(man *ComponentManager) manager.Manager {
	return &autoUpdateManager{man}
}

func (m *autoUpdateManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.AutoUpdate.Disable, "")
}

func (m *autoUpdateManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.AutoUpdate.CloudUser
}

func (m *autoUpdateManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.AutoUpdateComponentType,
		constants.ServiceNameAutoUpdate, constants.ServiceTypeAutoUpdate,
		constants.AutoUpdatePort, "")
}

type autoUpdateOptions struct {
	common_options.CommonOptions

	UpdateServer             string
	UpdateCheckIntervalHours float64
	HostUpdateStartHour      int
	HostUpdateStopHour       int
	HostUpdateTimeZone       string
	KubernetesMode           bool
	Channel                  string
}

func (m *autoUpdateManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &autoUpdateOptions{}
	if err := SetOptionsDefault(opt, constants.ServiceTypeAutoUpdate); err != nil {
		return nil, false, err
	}
	config := cfg.AutoUpdate
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config)

	opt.UpdateServer = "https://iso.yunion.cn"
	opt.UpdateCheckIntervalHours = 1.0
	opt.HostUpdateStartHour = 2 // like 02:00 in the morning. 0200=> 2
	opt.HostUpdateStopHour = 4
	opt.HostUpdateTimeZone = "Asia/Shanghai"
	opt.KubernetesMode = true
	opt.Channel = "stable"
	return m.newServiceConfigMap(v1alpha1.AutoUpdateComponentType, "", oc, opt), false, nil
}

func (m *autoUpdateManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.AutoUpdateComponentType, oc, constants.AutoUpdatePort)}
}

func (m *autoUpdateManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.AutoUpdateComponentType, "", oc, &oc.Spec.AutoUpdate, constants.AutoUpdatePort, false, false)
	if err != nil {
		return nil, err
	}
	// TODO: support add serviceAccount for component
	// use operator serviceAccount currently
	deploy.Spec.Template.Spec.ServiceAccountName = "onecloud-operator"
	return deploy, nil
}

func (m *autoUpdateManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.AutoUpdate
}
