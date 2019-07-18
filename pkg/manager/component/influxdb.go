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

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

const (
	InfluxDBConfigTemplate = `
[meta]
  # Where the metadata/raft database is stored
  dir = "/var/lib/influxdb/meta"

  # Automatically create a default retention policy when creating a database.
  retention-autocreate = true

  # If log messages are printed for the meta service
  # logging-enabled = true
  # default-retention-policy-name = "default"
  default-retention-policy-name = "30day_only"

[data]
  # The directory where the TSM storage engine stores TSM files.
  dir = "/var/lib/influxdb/data"

  # The directory where the TSM storage engine stores WAL files.
  wal-dir = "/var/lib/influxdb/wal"

[http]
  https-enabled = true
  https-certificate = "{{.CertPath}}"
  https-private-key = "{{.KeyPath}}"

[subscriber]
  insecure-skip-verify = true
`
)

type InfluxdbConfig struct {
	CertPath string
	KeyPath  string
}

func (c InfluxdbConfig) GetContent() (string, error) {
	return CompileTemplateFromMap(InfluxDBConfigTemplate, c)
}

type influxdbManager struct {
	*ComponentManager
}

func newInfluxdbManager(man *ComponentManager) manager.Manager {
	return &influxdbManager{man}
}

func (m *influxdbManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc)
}

func (m *influxdbManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.InfluxdbComponentType,
		constants.ServiceNameInfluxdb, constants.ServiceTypeInfluxdb,
		constants.InfluxdbPort, "")
}

func (m *influxdbManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service {
	return m.newSingleNodePortService(v1alpha1.InfluxdbComponentType, oc, constants.InfluxdbPort)
}

func (m *influxdbManager) getPVC(oc *v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.Influxdb
	return m.ComponentManager.newPVC(v1alpha1.InfluxdbComponentType, oc, cfg)
}

func (m *influxdbManager) getConfigMap(oc *v1alpha1.OnecloudCluster, clusterCfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	config := InfluxdbConfig{
		CertPath: path.Join(constants.CertDir, constants.ServiceCertName),
		KeyPath:  path.Join(constants.CertDir, constants.ServiceKeyName),
	}
	content, err := config.GetContent()
	if err != nil {
		return nil, err
	}
	return m.newConfigMap(v1alpha1.InfluxdbComponentType, oc, content), nil
}

func (m *influxdbManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	configMap := controller.ComponentConfigMapName(oc, v1alpha1.InfluxdbComponentType)
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		volMounts = append(volMounts, corev1.VolumeMount{
			Name:      "data",
			MountPath: constants.InfluxdbDataStore,
		})
		return []corev1.Container{
			{
				Name:         v1alpha1.InfluxdbComponentType.String(),
				Image:        oc.Spec.Influxdb.Image,
				Command:      []string{"influxd", "-config", "/etc/yunion/influxdb.conf"},
				VolumeMounts: volMounts,
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.InfluxdbComponentType, oc,
		NewVolumeHelper(oc, configMap, v1alpha1.InfluxdbComponentType),
		oc.Spec.Influxdb.DeploymentSpec, containersF)
	if err != nil {
		return nil, err
	}
	pod := &deploy.Spec.Template.Spec
	pod.Volumes = append(pod.Volumes, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: controller.NewClusterComponentName(oc.GetName(), v1alpha1.InfluxdbComponentType),
				ReadOnly:  false,
			},
		},
	})
	return deploy, nil
}

func (m *influxdbManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.Influxdb
}
