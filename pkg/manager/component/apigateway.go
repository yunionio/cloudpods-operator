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

type apiGatewayManager struct {
	*ComponentManager
}

func newAPIGatewayManager(man *ComponentManager) manager.Manager {
	return &apiGatewayManager{man}
}

func (m *apiGatewayManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc)
}

type apiOptions struct {
	options.CommonOptions
	WsPort      int  `default:"10443"`
	ShowCaptcha bool `default:"true"`
	EnableTotp  bool `default:"false"`
}

func (m *apiGatewayManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.APIGateway.CloudUser
}

func (m *apiGatewayManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	opt := &apiOptions{}
	if err := SetOptionsDefault(opt, "apigateway"); err != nil {
		return nil, err
	}
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, cfg.APIGateway)
	opt.WsPort = constants.APIWebsocketPort

	return m.newServiceConfigMap(v1alpha1.APIGatewayComponentType, oc, opt), nil
}

func (m *apiGatewayManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service {
	ports := []corev1.ServicePort{
		NewServiceNodePort("api", constants.APIGatewayPort),
		NewServiceNodePort("ws", constants.APIWebsocketPort),
	}
	return m.newNodePortService(v1alpha1.APIGatewayComponentType, oc, ports)
}

func (m *apiGatewayManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	spec := &oc.Spec.APIGateway
	isEE := IsEnterpriseEdition(spec)
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		cmd := "/opt/yunion/bin/apigateway"
		if isEE {
			cmd = "/opt/yunion/bin/yunionapi"
		}
		cs := []corev1.Container{
			{
				Name:         "api",
				Image:        oc.Spec.APIGateway.Image,
				Command:      []string{cmd, "--config", "/etc/yunion/apigateway.conf"},
				VolumeMounts: volMounts,
			},
		}
		if isEE {
			cs = append(cs, corev1.Container{
				Name:         "ws",
				Image:        oc.Spec.APIGateway.Image,
				Command:      []string{"/opt/yunion/bin/ws", "--config", "/etc/yunion/apigateway.conf"},
				VolumeMounts: volMounts,
			})
		}
		return cs
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.APIGatewayComponentType, oc,
		NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.APIGatewayComponentType), v1alpha1.APIGatewayComponentType),
		oc.Spec.APIGateway, cf)
	if err != nil {
		return nil, err
	}
	podSpec := &deploy.Spec.Template.Spec
	apiContainer := &podSpec.Containers[0]
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	apiContainer.VolumeMounts = append(apiContainer.VolumeMounts, corev1.VolumeMount{
		Name:      "data",
		ReadOnly:  false,
		MountPath: "/etc/yunion/data/",
	})
	if isEE {
		wsContainer := &podSpec.Containers[1]
		wsContainer.VolumeMounts = append(wsContainer.VolumeMounts, corev1.VolumeMount{
			Name:      "data",
			ReadOnly:  false,
			MountPath: "/etc/yunion/data/",
		})
	}
	return deploy, nil
}

func (m *apiGatewayManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.APIGateway
}
