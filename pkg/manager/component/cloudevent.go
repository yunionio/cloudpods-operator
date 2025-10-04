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
	"path"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud/pkg/cloudevent/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type cloudeventManager struct {
	*ComponentManager
}

func newCloudeventManager(man *ComponentManager) manager.ServiceManager {
	return &cloudeventManager{man}
}

func (m *cloudeventManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
	}
}

func (m *cloudeventManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.CloudeventComponentType
}

func (m *cloudeventManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Cloudevent.Disable || !isInProductVersion(m, oc)
}

func (m *cloudeventManager) GetServiceName() string {
	return constants.ServiceNameCloudevent
}

func (m *cloudeventManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *cloudeventManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Cloudevent.DB
}

func (m *cloudeventManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Cloudevent.DbEngine)
}

func (m *cloudeventManager) getClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Cloudevent.ClickhouseConf
}

func (m *cloudeventManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Cloudevent.CloudUser
}

func (m *cloudeventManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.CloudeventComponentType,
		constants.ServiceNameCloudevent, constants.ServiceTypeCloudevent,
		man.GetCluster().Spec.Cloudevent.Service.NodePort, "")
}

func (m *cloudeventManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeCloudevent); err != nil {
		return nil, false, err
	}
	config := cfg.Cloudevent

	switch oc.Spec.GetDbEngine(oc.Spec.Cloudevent.DbEngine) {
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
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = config.Port
	return m.newServiceConfigMap(v1alpha1.CloudeventComponentType, "", oc, opt), false, nil
}

func (m *cloudeventManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSinglePortService(v1alpha1.CloudeventComponentType, oc, oc.Spec.Cloudevent.Service.InternalOnly, int32(oc.Spec.Cloudevent.Service.NodePort), int32(cfg.Cloudevent.Port))}
}

func (m *cloudeventManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "server",
				Image:           oc.Spec.Cloudevent.Image,
				ImagePullPolicy: oc.Spec.Cloudnet.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/cloudevent", "--config", "/etc/yunion/cloudevent.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.CloudeventComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.CloudeventComponentType), v1alpha1.CloudeventComponentType), &oc.Spec.Cloudevent.DeploymentSpec, cf)
}

func (m *cloudeventManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Cloudevent
}
