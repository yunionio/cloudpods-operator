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
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/image"
)

const (
	YUNIONAPI_SAAS = "yunionapi-sa"
)

type apiGatewayManager struct {
	*ComponentManager
}

func newAPIGatewayManager(man *ComponentManager) manager.Manager {
	return &apiGatewayManager{man}
}

func (m *apiGatewayManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	oc.Spec.APIGateway.ImageName = constants.APIGatewayEEImageName
	return syncComponent(m, oc, oc.Spec.APIGateway.Disable)
}

type apiOptions struct {
	options.CommonOptions
	WsPort      int  `default:"10443"`
	ShowCaptcha bool `default:"true"`
	EnableTotp  bool `default:"true"`
}

func (m *apiGatewayManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.APIGateway.CloudUser
}

func (m *apiGatewayManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return newAPIGatewaPhaseControl(man)
}

type apiGatewayPhaseControl struct {
	man controller.ComponentManager
	ac  controller.PhaseControl
	wc  controller.PhaseControl
}

func newAPIGatewaPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return &apiGatewayPhaseControl{
		man: man,
		ac:  controller.NewRegisterServiceComponent(man, constants.ServiceNameAPIGateway, constants.ServiceTypeAPIGateway),
		wc: controller.NewRegisterEndpointComponent(man, v1alpha1.APIGatewayComponentType,
			constants.ServiceNameWebsocket, constants.ServiceTypeWebsocket,
			constants.APIWebsocketPort, ""),
	}
}

func (c *apiGatewayPhaseControl) Setup() error {
	if err := c.ac.Setup(); err != nil {
		return err
	}
	if err := c.wc.Setup(); err != nil {
		return err
	}
	return nil
}

func (c *apiGatewayPhaseControl) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	return nil
}

func (m *apiGatewayManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, bool, error) {
	opt := &apiOptions{}
	if err := SetOptionsDefault(opt, "apigateway"); err != nil {
		return nil, false, err
	}
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, cfg.APIGateway)
	opt.Port = constants.APIGatewayPort
	opt.WsPort = constants.APIWebsocketPort
	opt.CorsHosts = []string{"*"}

	return m.newServiceConfigMap(v1alpha1.APIGatewayComponentType, oc, opt), false, nil
}

func (m *apiGatewayManager) getService(oc *v1alpha1.OnecloudCluster) []*corev1.Service {
	ports := []corev1.ServicePort{
		NewServiceNodePort("api", constants.APIGatewayPort),
		NewServiceNodePort("ws", constants.APIWebsocketPort),
	}
	return []*corev1.Service{m.newNodePortService(v1alpha1.APIGatewayComponentType, oc, ports)}
}

func (m *apiGatewayManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	isEE := IsEnterpriseEdition(oc)
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		cmd := fmt.Sprintf("/opt/yunion/bin/%s", YUNIONAPI_SAAS)
		img := oc.Spec.APIGateway.Image
		parts, _ := image.ParseImageReference(img)
		webapigwImage := fmt.Sprintf("%s/%s:%s", parts.Repository, YUNIONAPI_SAAS, parts.Tag)
		cs := []corev1.Container{
			{
				Name:            "api",
				Image:           webapigwImage,
				ImagePullPolicy: oc.Spec.APIGateway.ImagePullPolicy,
				Command:         []string{cmd, "--config", "/etc/yunion/apigateway.conf"},
				VolumeMounts:    volMounts,
			},
		}
		if isEE {
			cs = append(cs, corev1.Container{
				Name:            "ws",
				Image:           oc.Spec.APIGateway.Image,
				ImagePullPolicy: oc.Spec.APIGateway.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/ws", "--config", "/etc/yunion/apigateway.conf"},
				VolumeMounts:    volMounts,
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
