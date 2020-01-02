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
	"path"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

// documentation: check https://hub.docker.com/_/kapacitor
const (
	KapacitorConfigTemplate = `
data_dir = "/var/lib/kapacitor"

[replay]
  dir = "/var/lib/kapacitor/replay"

[storage]
  boltdb = "/var/lib/kapacitor/kapacitor.db"

[load]
  enabled = true
  dir="/opt/yunion/load-directory-service"

[http]
  bind-address = ":{{.Port}}"
  auth-enabled = false
  log-enabled = true
  https-enabled = true
  https-certificate = "{{.CertPath}}"
  https-private-key = "{{.KeyPath}}"
  shutdown-timeout = "10s"

[[influxdb]]
  enabled = true
  name = "default"
  default = true
  urls = ["{{.InfluxdbURL}}"]
  insecure-skip-verify = true
  subscription-protocol = "https"
  subscription-mode = "cluster"
  [influxdb.excluded-subscriptions]
    _kapacitor = ["autogen"]

[logging]
  file = "STDERR"
  level = "DEBUG"
`
)

type KapacitorConfig struct {
	Port        int
	CertPath    string
	KeyPath     string
	InfluxdbURL string
}

func (c KapacitorConfig) GetContent() (string, error) {
	return CompileTemplateFromMap(KapacitorConfigTemplate, c)
}

type kapacitorManager struct {
	*ComponentManager
}

func newKapacitorManager(man *ComponentManager) manager.Manager {
	return &kapacitorManager{man}
}

func (m *kapacitorManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Kapacitor.Disable)
}

func (m *kapacitorManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.KapacitorComponentType,
		constants.ServiceNameKapacitor, constants.ServiceTypeKapacitor,
		constants.KapacitorPort, "")
}

func (m *kapacitorManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service {
	return m.newSingleNodePortService(v1alpha1.KapacitorComponentType, oc, constants.KapacitorPort)
}

func (m *kapacitorManager) getPVC(oc *v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.Kapacitor
	return m.ComponentManager.newPVC(v1alpha1.KapacitorComponentType, oc, cfg)
}

func (m *kapacitorManager) getConfigMap(oc *v1alpha1.OnecloudCluster, clusterCfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	config := KapacitorConfig{
		Port:        constants.KapacitorPort,
		CertPath:    path.Join(constants.CertDir, constants.ServiceCertName),
		KeyPath:     path.Join(constants.CertDir, constants.ServiceKeyName),
		InfluxdbURL: fmt.Sprintf("https://%s:%d", controller.NewClusterComponentName(oc.GetName(), v1alpha1.InfluxdbComponentType), constants.InfluxdbPort),
	}
	content, err := config.GetContent()
	if err != nil {
		return nil, err
	}
	return m.newConfigMap(v1alpha1.KapacitorComponentType, oc, content), nil
}

func (m *kapacitorManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	configMap := controller.ComponentConfigMapName(oc, v1alpha1.KapacitorComponentType)
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		volMounts = append(volMounts, corev1.VolumeMount{
			Name:      "data",
			MountPath: constants.KapacitorDataStore,
		})
		envs := GetRCAdminEnv(oc)
		envs = append(envs, []corev1.EnvVar{
			{
				Name:  "KAPACITOR_UNSAFE_SSL",
				Value: "true",
			},
			{
				Name:  "KAPACITOR_URL",
				Value: fmt.Sprintf("https://%s:%d", controller.NewClusterComponentName(oc.GetName(), v1alpha1.KapacitorComponentType), constants.KapacitorPort),
			},
		}...)
		return []corev1.Container{
			{
				Name:            v1alpha1.KapacitorComponentType.String(),
				Image:           oc.Spec.Kapacitor.Image,
				ImagePullPolicy: oc.Spec.Kapacitor.ImagePullPolicy,
				Env:             envs,
				Command: []string{
					"/usr/bin/kapacitord",
					"run",
					"-config",
					"/etc/yunion/kapacitor.conf",
					"-hostname",
					fmt.Sprintf("%s.%s", controller.NewClusterComponentName(oc.GetName(), v1alpha1.KapacitorComponentType), oc.GetNamespace()),
				},
				VolumeMounts: volMounts,
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.KapacitorComponentType, oc,
		NewVolumeHelper(oc, configMap, v1alpha1.KapacitorComponentType),
		oc.Spec.Kapacitor.DeploymentSpec, cf)
	if err != nil {
		return nil, err
	}
	pod := &deploy.Spec.Template.Spec
	pod.Volumes = append(pod.Volumes, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: controller.NewClusterComponentName(oc.GetName(), v1alpha1.KapacitorComponentType),
				ReadOnly:  false,
			},
		},
	})
	return deploy, nil
}

func (m *kapacitorManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.Kapacitor
}
