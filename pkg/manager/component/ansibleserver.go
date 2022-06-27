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

	"yunion.io/x/onecloud/pkg/ansibleserver/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type ansibleManager struct {
	*ComponentManager
}

func newAnsibleManager(man *ComponentManager) manager.Manager {
	return &ansibleManager{man}
}

func (m *ansibleManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *ansibleManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.AnsibleServerComponentType
}

func (m *ansibleManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.AnsibleServer.Disable, "")
}

func (m *ansibleManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.AnsibleServer.DB
}

func (m *ansibleManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.AnsibleServer.CloudUser
}

func (m *ansibleManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.AnsibleServerComponentType,
		constants.ServiceNameAnsibleServer, constants.ServiceTypeAnsibleServer,
		constants.AnsibleServerPort, "")
}

func (m *ansibleManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeAnsibleServer); err != nil {
		return nil, false, err
	}
	config := cfg.AnsibleServer
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = constants.AnsibleServerPort
	return m.newServiceConfigMap(v1alpha1.AnsibleServerComponentType, "", oc, opt), false, nil
}

func (m *ansibleManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.AnsibleServerComponentType, oc, constants.AnsibleServerPort)}
}

func (m *ansibleManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "server",
				Image:           oc.Spec.AnsibleServer.Image,
				ImagePullPolicy: oc.Spec.AnsibleServer.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/ansibleserver", "--config", "/etc/yunion/ansibleserver.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.AnsibleServerComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.AnsibleServerComponentType), v1alpha1.AnsibleServerComponentType), &oc.Spec.AnsibleServer, cf)
}

func (m *ansibleManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.AnsibleServer
}
