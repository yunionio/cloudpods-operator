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

	"yunion.io/x/onecloud/pkg/mcclient"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

type influxdbManager struct {
	*ComponentManager
}

func newInfluxdbManager(man *ComponentManager) manager.ServiceManager {
	return &influxdbManager{man}
}

func (m *influxdbManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *influxdbManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.InfluxdbComponentType
}

func (m *influxdbManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Influxdb.Disable || !isInProductVersion(m, oc)
}

func (m *influxdbManager) GetServiceName() string {
	return constants.ServiceNameInfluxdb
}

func (m *influxdbManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *influxdbManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewInfluxdb().GetPhaseControl(man)
}

func (m *influxdbManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return m.newSinglePortService(v1alpha1.InfluxdbComponentType, oc, oc.Spec.Influxdb.Service.InternalOnly, int32(oc.Spec.Influxdb.Service.NodePort), constants.InfluxdbPort, oc.Spec.Influxdb.SlaveReplicas > 0)
}

func (m *influxdbManager) getPVC(oc *v1alpha1.OnecloudCluster, zone string) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.Influxdb
	return m.ComponentManager.newPVC(v1alpha1.InfluxdbComponentType, oc, cfg.StatefulDeploymentSpec)
}

func (m *influxdbManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	content, err := component.NewInfluxdb().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}
	return m.newConfigMap(v1alpha1.InfluxdbComponentType, "", oc, content.(string)), false, nil
}

func (m *influxdbManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	configMap := controller.ComponentConfigMapName(oc, v1alpha1.InfluxdbComponentType)
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		volMounts = append(volMounts, corev1.VolumeMount{
			Name:      "data",
			MountPath: constants.InfluxdbDataStore,
		})
		return []corev1.Container{
			{
				Name:            v1alpha1.InfluxdbComponentType.String(),
				Image:           oc.Spec.Influxdb.Image,
				ImagePullPolicy: oc.Spec.Influxdb.ImagePullPolicy,
				Command:         []string{"influxd", "-config", "/etc/yunion/influxdb.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.InfluxdbComponentType, "", oc, NewVolumeHelper(oc, configMap, v1alpha1.InfluxdbComponentType), &oc.Spec.Influxdb.DeploymentSpec, containersF)
	if err != nil {
		return nil, err
	}
	if oc.Spec.Influxdb.StorageClassName == v1alpha1.DefaultStorageClass {
		// if use local path storage, remove cloud affinity
		deploy = m.removeDeploymentAffinity(deploy)
	}
	pod := &deploy.Spec.Template.Spec
	pod.Volumes = append(pod.Volumes, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: m.newPvcName(oc.GetName(), oc.Spec.Influxdb.StorageClassName, v1alpha1.InfluxdbComponentType),
				ReadOnly:  false,
			},
		},
	})
	if oc.Spec.Influxdb.StorageClassName != v1alpha1.DefaultStorageClass {
		deploy.Spec.Strategy.Type = apps.RecreateDeploymentStrategyType
	}
	return deploy, nil
}

func (m *influxdbManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Influxdb
}

func getInfluxDBInternalURL(oc *v1alpha1.OnecloudCluster) string {
	influxdb := oc.Spec.Influxdb
	vm := oc.Spec.VictoriaMetrics
	cType := v1alpha1.InfluxdbComponentType
	port := constants.InfluxdbPort
	if influxdb.Disable && !vm.Disable {
		cType = v1alpha1.VictoriaMetricsComponentType
		port = constants.VictoriaMetricsPort
	}
	internalAddress := controller.NewClusterComponentName(oc.GetName(), cType)
	return fmt.Sprintf("https://%s:%d", internalAddress, port)
}

func (m *influxdbManager) supportsReadOnlyService() bool {
	return false
}

func (m *influxdbManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return nil
}

func (m *influxdbManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return func(s *mcclient.ClientSession) error {
		if m.IsDisabled(oc) {
			return onecloud.EnsureDisableService(s, m.GetServiceName())
		} else {
			return onecloud.EnsureEnableService(s, m.GetServiceName(), m.supportsReadOnlyService() && oc.Spec.Influxdb.SlaveReplicas > 0)
		}
	}
}
