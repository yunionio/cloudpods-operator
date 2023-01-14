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

	"yunion.io/x/onecloud/pkg/cloudnet/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type cloudnetManager struct {
	*ComponentManager
}

func newCloudnetManager(man *ComponentManager) manager.Manager {
	return &cloudnetManager{man}
}

func (m *cloudnetManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
	}
}

func (m *cloudnetManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.CloudnetComponentType
}

func (m *cloudnetManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if !oc.Spec.EnableCloudNet && !controller.StopServices {
		return nil
	}
	return syncComponent(m, oc, oc.Spec.Cloudnet.Disable, "")
}

func (m *cloudnetManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Cloudnet.DB
}

func (m *cloudnetManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Cloudnet.CloudUser
}

func (m *cloudnetManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.CloudnetComponentType,
		constants.ServiceNameCloudnet, constants.ServiceTypeCloudnet,
		man.GetCluster().Spec.Cloudnet.Service.NodePort, "")
}

func (m *cloudnetManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeCloudnet); err != nil {
		return nil, false, err
	}
	config := cfg.Cloudnet
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = config.Port
	return m.newServiceConfigMap(v1alpha1.CloudnetComponentType, "", oc, opt), false, nil
}

func (m *cloudnetManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.CloudnetComponentType, oc, int32(oc.Spec.Cloudnet.Service.NodePort), int32(cfg.Cloudnet.Port))}
}

func (m *cloudnetManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "server",
				Image:           oc.Spec.Cloudnet.Image,
				ImagePullPolicy: oc.Spec.Cloudnet.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/cloudnet", "--config", "/etc/yunion/cloudnet.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.CloudnetComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.CloudnetComponentType), v1alpha1.CloudnetComponentType), &oc.Spec.Cloudnet.DeploymentSpec, cf)
}

func (m *cloudnetManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Cloudnet
}
