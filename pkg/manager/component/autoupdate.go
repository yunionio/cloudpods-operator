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
	"strings"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type autoUpdateManager struct {
	*ComponentManager
}

func newAutoUpdateManager(man *ComponentManager) manager.ServiceManager {
	return &autoUpdateManager{man}
}

func (m *autoUpdateManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *autoUpdateManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.AutoUpdateComponentType
}

func (m *autoUpdateManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.AutoUpdate.Disable
}

func (m *autoUpdateManager) GetServiceName() string {
	return constants.ServiceNameAutoUpdate
}

func (m *autoUpdateManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if !IsEnterpriseEdition(oc) {
		return nil
	}
	return syncComponent(m, oc, "")
}

func (m *autoUpdateManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.AutoUpdate.DB
}

func (m *autoUpdateManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.AutoUpdate.DbEngine)
}

func (m *autoUpdateManager) getClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.AutoUpdate.ClickhouseConf
}

func (m *autoUpdateManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.AutoUpdate.CloudUser
}

func (m *autoUpdateManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	oc := man.GetCluster()
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.AutoUpdateComponentType,
		constants.ServiceNameAutoUpdate, constants.ServiceTypeAutoUpdate,
		oc.Spec.AutoUpdate.Service.NodePort, "")
}

type autoUpdateOptions struct {
	common_options.CommonOptions
	common_options.DBOptions

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
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeAutoUpdate); err != nil {
		return nil, false, err
	}
	config := cfg.AutoUpdate
	switch oc.Spec.GetDbEngine(oc.Spec.AutoUpdate.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}
	option.SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)

	opt.Port = config.Port
	opt.UpdateServer = "https://iso.yunion.cn"
	opt.UpdateCheckIntervalHours = 1.0
	opt.HostUpdateStartHour = 2 // like 02:00 in the morning. 0200=> 2
	opt.HostUpdateStopHour = 4
	opt.HostUpdateTimeZone = "Asia/Shanghai"
	opt.KubernetesMode = true
	opt.Channel = "stable"

	return m.shouldSyncConfigmap(oc, v1alpha1.AutoUpdateComponentType, opt, func(optStr string) bool {
		if !strings.Contains(optStr, "sql_connection") {
			// hack: force update old configmap if not contains sql_connection option
			return true
		}
		return false
	})
}

func (m *autoUpdateManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{
		m.newSinglePortService(v1alpha1.AutoUpdateComponentType, oc, oc.Spec.AutoUpdate.Service.InternalOnly, int32(oc.Spec.AutoUpdate.Service.NodePort), int32(cfg.AutoUpdate.Port)),
	}
}

func (m *autoUpdateManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.AutoUpdateComponentType, "", oc, &oc.Spec.AutoUpdate.DeploymentSpec, int32(cfg.AutoUpdate.Port), true, false)
	if err != nil {
		return nil, err
	}
	// TODO: support add serviceAccount for component
	// use operator serviceAccount currently
	deploy.Spec.Template.Spec.ServiceAccountName = constants.ServiceAccountOnecloudOperator
	return deploy, nil
}

func (m *autoUpdateManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.AutoUpdate
}
