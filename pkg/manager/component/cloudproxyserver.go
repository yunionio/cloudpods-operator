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

	"yunion.io/x/onecloud/pkg/cloudproxy/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type cloudproxyManager struct {
	*ComponentManager
}

func newCloudproxyManager(man *ComponentManager) manager.Manager {
	return &cloudproxyManager{man}
}

func (m *cloudproxyManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
	}
}

func (m *cloudproxyManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.CloudproxyComponentType
}

func (m *cloudproxyManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Cloudproxy.Disable, "")
}

func (m *cloudproxyManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Cloudproxy.DB
}

func (m *cloudproxyManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Cloudproxy.CloudUser
}

func (m *cloudproxyManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.Cloudproxy()
}

func (m *cloudproxyManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options{}
	if err := SetOptionsDefault(opt, constants.ServiceTypeCloudproxy); err != nil {
		return nil, false, err
	}
	config := cfg.Cloudproxy
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.CommonOptions.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)

	opt.EnableAPIServer = true
	opt.EnableProxyAgent = true
	opt.ProxyAgentId = controller.GetDefaultProxyAgentName()

	opt.Port = config.Port
	return m.newServiceConfigMap(v1alpha1.CloudproxyComponentType, "", oc, opt), false, nil
}

func (m *cloudproxyManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.CloudproxyComponentType, oc, int32(oc.Spec.Cloudproxy.Service.NodePort), int32(cfg.Cloudproxy.Port))}
}

func (m *cloudproxyManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "server",
				Image:           oc.Spec.Cloudproxy.Image,
				ImagePullPolicy: oc.Spec.Cloudproxy.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/cloudproxy", "--config", "/etc/yunion/cloudproxy.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.CloudproxyComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.CloudproxyComponentType), v1alpha1.CloudproxyComponentType), &oc.Spec.Cloudproxy.DeploymentSpec, cf)
}

func (m *cloudproxyManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Cloudproxy
}
