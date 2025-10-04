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

type extdbManager struct {
	*ComponentManager
}

func newExtdbManager(man *ComponentManager) manager.ServiceManager {
	return &extdbManager{man}
}

func (m *extdbManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *extdbManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.ExtdbComponentType
}

func (m *extdbManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Extdb.Disable || !IsEnterpriseEdition(oc) || !isInProductVersion(m, oc)
}

func (m *extdbManager) GetServiceName() string {
	return constants.ServiceNameExtdb
}

func (m *extdbManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *extdbManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Extdb.DB
}

func (m *extdbManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Extdb.DbEngine)
}

func (m *extdbManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Extdb.CloudUser
}

func (m *extdbManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.ExtdbComponentType,
		constants.ServiceNameExtdb, constants.ServiceTypeExtdb,
		man.GetCluster().Spec.Extdb.Service.NodePort, "")
}

type extdbOptions struct {
	common_options.CommonOptions
	common_options.DBOptions
}

func (m *extdbManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &extdbOptions{}
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeExtdb); err != nil {
		return nil, false, err
	}
	config := cfg.Extdb

	switch oc.Spec.GetDbEngine(oc.Spec.Extdb.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)

	opt.AutoSyncTable = true
	opt.Port = config.Port
	return m.newServiceConfigMap(v1alpha1.ExtdbComponentType, "", oc, opt), false, nil
}

func (m *extdbManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSinglePortService(v1alpha1.ExtdbComponentType, oc, oc.Spec.Extdb.Service.InternalOnly, int32(oc.Spec.Extdb.Service.NodePort), int32(cfg.Extdb.Port))}
}

func (m *extdbManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "extdb",
				Image:           oc.Spec.Extdb.Image,
				ImagePullPolicy: oc.Spec.Extdb.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/extdb", "--config", "/etc/yunion/extdb.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.ExtdbComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.ExtdbComponentType), v1alpha1.ExtdbComponentType), &oc.Spec.Extdb.DeploymentSpec, cf)
}

func (m *extdbManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Extdb
}
