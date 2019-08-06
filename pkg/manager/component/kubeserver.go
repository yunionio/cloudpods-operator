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
	"path"

	"yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type kubeManager struct {
	*ComponentManager
}

func newKubeManager(man *ComponentManager) manager.Manager {
	return &kubeManager{man}
}

type kubeOptions struct {
	options.CommonOptions
	options.DBOptions

	HttpsPort         int
	TlsCertFile       string
	TlsPrivateKeyFile string
}

func (m *kubeManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc)
}

func (m *kubeManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.KubeServer.DB
}

func (m *kubeManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.KubeServer.CloudUser
}

func (m *kubeManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.KubeServerComponentType,
		constants.ServiceNameKubeServer, constants.ServiceTypeKubeServer,
		constants.KubeServerPort, "api")
}

func (m *kubeManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	opt := &kubeOptions{}
	if err := SetOptionsDefault(opt, constants.ServiceTypeKubeServer); err != nil {
		return nil, err
	}
	config := cfg.KubeServer
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.TlsCertFile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.TlsPrivateKeyFile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.HttpsPort = constants.KubeServerPort
	return m.newServiceConfigMap(v1alpha1.KubeServerComponentType, oc, opt), nil
}

func (m *kubeManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service {
	return m.newSingleNodePortService(v1alpha1.KubeServerComponentType, oc, constants.KubeServerPort)
}

func (m *kubeManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:         "server",
				Image:        oc.Spec.KubeServer.Image,
				Command:      []string{"/opt/yunion/bin/kube-server", "--config", "/etc/yunion/kubeserver.conf"},
				VolumeMounts: volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(
		v1alpha1.KubeServerComponentType, oc,
		NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.KubeServerComponentType), v1alpha1.KubeServerComponentType),
		oc.Spec.KubeServer, cf)
}

func (m *kubeManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.KubeServer
}
