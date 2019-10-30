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
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

const (
	CEConfig = `
    location / {
        root /usr/share/nginx/html/web;
		index index.html;
		add_header Cache-Control no-cache;
		expires 1s;
		if (!-e $request_filename) {

		}
    }
`

	EEConfig = `
    location / {
        return 301 https://$host/v1/;
    }

    location ^~/v1 {
        alias /usr/share/nginx/html/web;
        index index.html;
        try_files $uri $uri/ /index.html last;
    }

    location ^~/v2 {
        alias /usr/share/nginx/html/dashboard;
        index index.html;
        try_files $uri $uri/ /index.html last;
    }
`

	WebNginxConfigTemplate = `
server {
{{.EditionConfig}}

    location /static/ {
        # Some basic cache-control for static files to be sent to the browser
        root /usr/share/nginx/html/web;
        expires max;
        add_header Pragma public;
        add_header Cache-Control "public, must-revalidate, proxy-revalidate";
    }

    location /servicetree {
        alias /usr/share/nginx/html/servicetree;
        index index.html;
        add_header Cache-Control no-cache;
        expires 1s;
        if (!-e $request_filename) {
            rewrite ^/(.*) /servicetree/index.html last;
            break;
        }
    }

    location /static-servicetree/ {
        root /usr/share/nginx/html/servicetree;
        expires max;
        add_header Pragma public;
        add_header Cache-Control "public, must-revalidate, proxy-revalidate";
    }

    location /itsm {
        alias /usr/share/nginx/html/itsm;
        index index.html;
        add_header Cache-Control no-cache;
        expires 1s;
        if (!-e $request_filename) {
            rewrite ^/(.*) /itsm/index.html last;
            break;
        }
    }

    location /static-itsm/ {
        root /usr/share/nginx/html/itsm;
        expires max;
        add_header Pragma public;
        add_header Cache-Control "public, must-revalidate, proxy-revalidate";
    }

    location ~* /static/images/favicon.* {
        root /usr/share/nginx/html/login;
        expires max;
        add_header Pragma public;
        add_header Cache-Control "public, must-revalidate, proxy-revalidate";
    }

    location /api {
        proxy_pass {{.APIGatewayURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_buffer_size  16k;
        proxy_buffers   32 16k;
        proxy_busy_buffers_size 16k;
        proxy_temp_file_write_size 16k;
    }

    location /api/v1/imageutils/upload {
        proxy_pass {{.APIGatewayURL}};
        client_max_body_size 0;
        proxy_http_version 1.1;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;
    }

    location /query {
        proxy_pass {{.APIGatewayURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location ~ ^/(vnc|spice|wmks|sol) {
        proxy_pass {{.WebconsoleURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location ~ ^/(websockify|wsproxy|connect) {
        proxy_pass {{.WebconsoleURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 86400;
    }

    location /bi {
        alias /usr/share/nginx/html/bi;
        index index.html;
        if (!-e $request_filename) {
            rewrite ^/(.*) /bi/index.html last;
            break;
        }
    }

    location /static-bi/ {
        root /usr/share/nginx/html/bi;
        expires max;
        add_header Pragma public;
        add_header Cache-Control "public, must-revalidate, proxy-revalidate";
    }

    location /web-console {
        alias /usr/share/nginx/html/web-console;
        index index.html;
        if (!-e $request_filename) {
            rewrite ^/(.*) /web-console/index.html last;
            break;
        }
    }

    location /ws {
        proxy_pass {{.APIGatewayWsURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 86400;
    }
}
`
)

type WebNginxConfig struct {
	EditionConfig   string
	WebconsoleURL   string
	APIGatewayWsURL string
	APIGatewayURL   string
}

func (c WebNginxConfig) GetContent() (string, error) {
	return CompileTemplateFromMap(WebNginxConfigTemplate, c)
}

type webManager struct {
	*ComponentManager
}

func newWebManager(man *ComponentManager) manager.Manager {
	return &webManager{man}
}

func (m *webManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Web.Disable)
}

func (m *webManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service {
	ports := []corev1.ServicePort{
		{
			Name:       "http",
			Protocol:   corev1.ProtocolTCP,
			Port:       80,
			TargetPort: intstr.FromInt(80),
		},
	}
	return m.newService(v1alpha1.WebComponentType, oc, corev1.ServiceTypeClusterIP, ports)
}

func (m *webManager) getIngress(oc *v1alpha1.OnecloudCluster) *extensions.Ingress {
	svc := m.getService(oc)
	ocName := oc.GetName()
	svcName := controller.NewClusterComponentName(ocName, v1alpha1.WebComponentType)
	appLabel := m.getComponentLabel(oc, v1alpha1.WebComponentType)
	secretName := controller.ClustercertSecretName(oc)

	ing := &extensions.Ingress{
		ObjectMeta: m.getObjectMeta(oc, svcName, appLabel),
		Spec: extensions.IngressSpec{
			TLS: []extensions.IngressTLS{
				{
					SecretName: secretName,
				},
			},
			Rules: []extensions.IngressRule{
				{
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path: "/",
									Backend: extensions.IngressBackend{
										ServiceName: svc.GetName(),
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return ing
}

func (m *webManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	urlF := func(ct v1alpha1.ComponentType, port int) string {
		return fmt.Sprintf("https://%s:%d", controller.NewClusterComponentName(oc.GetName(), ct), port)
	}
	conf := CEConfig
	isEE := IsEnterpriseEdition(&oc.Spec.Web)
	if isEE {
		conf = EEConfig
	}
	config := WebNginxConfig{
		EditionConfig:   conf,
		WebconsoleURL:   urlF(v1alpha1.WebconsoleComponentType, constants.WebconsolePort),
		APIGatewayWsURL: urlF(v1alpha1.APIGatewayComponentType, constants.APIWebsocketPort),
		APIGatewayURL:   urlF(v1alpha1.APIGatewayComponentType, constants.APIGatewayPort),
	}
	content, err := config.GetContent()
	if err != nil {
		return nil, err
	}
	return m.newConfigMap(v1alpha1.WebComponentType, oc, content), nil
}

func (m *webManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		confVol := volMounts[len(volMounts)-1]
		confVol.MountPath = "/etc/nginx/conf.d"
		volMounts[len(volMounts)-1] = confVol
		return []corev1.Container{
			{
				Name:  "web",
				Image: oc.Spec.Web.Image,
				Ports: []corev1.ContainerPort{
					{
						Name:          "web",
						ContainerPort: 80,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: volMounts,
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(
		v1alpha1.WebComponentType, oc,
		NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.WebComponentType), v1alpha1.WebComponentType),
		oc.Spec.Web, cf)
	if err != nil {
		return nil, err
	}
	podSpec := &deploy.Spec.Template.Spec
	config := podSpec.Volumes[len(podSpec.Volumes)-1]
	config.ConfigMap.Items[0].Path = "default.conf"
	podSpec.Volumes[len(podSpec.Volumes)-1] = config
	return deploy, nil
}

func (m *webManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.Web
}
