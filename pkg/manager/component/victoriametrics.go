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
	"strings"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
)

type vmManager struct {
	*ComponentManager
}

// TODO: use an abstract layer to remove following duplicated code from influxdb
func newVictoriaMetricsManager(man *ComponentManager) manager.ServiceManager {
	return &vmManager{man}
}

func (m *vmManager) getContainerPort() int32 {
	return constants.VictoriaMetricsPort
}

func (m *vmManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *vmManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.VictoriaMetricsComponentType
}

func (m *vmManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.VictoriaMetrics.Disable || !isInProductVersion(m, oc)
}

func (m *vmManager) GetServiceName() string {
	return constants.ServiceNameVictoriaMetrics
}

// Sync implements manager.Manager.
func (m *vmManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *vmManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewVictoriaMetrics().GetPhaseControl(man)
}

func (m *vmManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{
		m.newSinglePortService(v1alpha1.VictoriaMetricsComponentType, oc, oc.Spec.VictoriaMetrics.Service.InternalOnly, int32(oc.Spec.VictoriaMetrics.Service.NodePort), m.getContainerPort()),
	}
}

func (m *vmManager) getPVC(oc *v1alpha1.OnecloudCluster, zone string) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.VictoriaMetrics
	return m.ComponentManager.newPVC(v1alpha1.VictoriaMetricsComponentType, oc, cfg.StatefulDeploymentSpec)
}

func (m *vmManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	dataVolName := "server-volume"
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		volMounts = append(volMounts, corev1.VolumeMount{
			Name:      dataVolName,
			MountPath: constants.VictoriaMetricsDataStore,
		})
		rd := oc.Spec.VictoriaMetrics.RententionPeriodDays
		if rd == 0 {
			rd = 90
		}
		fakeInfluxDBs := []string{"telegraf", "meter_db", "monitor", "system", "mysql_metrics"}
		return []corev1.Container{
			{
				Name:            v1alpha1.VictoriaMetricsComponentType.String(),
				Image:           oc.Spec.VictoriaMetrics.Image,
				ImagePullPolicy: oc.Spec.VictoriaMetrics.ImagePullPolicy,
				Args: []string{
					fmt.Sprintf("--httpListenAddr=:%d", m.getContainerPort()),
					"--tls",
					"--tlsCipherSuites=TLS_RSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_CHACHA20_POLY1305_SHA256",
					"--tlsMinVersion=TLS12",
					fmt.Sprintf("--tlsCertFile=%s", path.Join(constants.CertDir, constants.ServiceCertName)),
					fmt.Sprintf("--tlsKeyFile=%s", path.Join(constants.CertDir, constants.ServiceKeyName)),
					// https://docs.victoriametrics.com/#retention
					fmt.Sprintf("--retentionPeriod=%dd", rd),
					fmt.Sprintf("--storageDataPath=%s", constants.VictoriaMetricsDataStore),
					"--envflag.enable=true",
					"--envflag.prefix=VM_",
					"--loggerFormat=json",
					fmt.Sprintf("--influx.databaseNames=%s", strings.Join(fakeInfluxDBs, ",")),
					fmt.Sprintf("--maxLabelsPerTimeseries=%d", 60),
					fmt.Sprintf("--pprofAuthKey=%s", "pprof@AuthKey"),
				},
				VolumeMounts: volMounts,
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: m.getContainerPort(),
						Name:          "http",
						Protocol:      corev1.ProtocolTCP,
					},
				},
				ReadinessProbe: &corev1.Probe{
					FailureThreshold: 3,
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/health",
							Port:   intstr.FromInt(int(m.getContainerPort())),
							Scheme: corev1.URISchemeHTTPS,
						},
					},
					InitialDelaySeconds: 5,
					PeriodSeconds:       15,
					SuccessThreshold:    1,
					TimeoutSeconds:      5,
				},
				LivenessProbe: &corev1.Probe{
					FailureThreshold: 10,
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/health",
							Port:   intstr.FromInt(int(m.getContainerPort())),
							Scheme: corev1.URISchemeHTTPS,
						},
					},
					InitialDelaySeconds: 30,
					PeriodSeconds:       30,
					SuccessThreshold:    1,
					TimeoutSeconds:      5,
				},
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.VictoriaMetricsComponentType, "", oc, NewVolumeHelper(oc, "", v1alpha1.VictoriaMetricsComponentType), &oc.Spec.VictoriaMetrics.DeploymentSpec, containersF)
	if err != nil {
		return nil, err
	}
	if oc.Spec.VictoriaMetrics.StorageClassName == v1alpha1.DefaultStorageClass {
		// if use local path storage, remove cloud affinity
		deploy = m.removeDeploymentAffinity(deploy)
	}

	pod := &deploy.Spec.Template.Spec
	pod.Volumes = append(pod.Volumes, corev1.Volume{
		Name: dataVolName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: m.newPvcName(oc.GetName(), oc.Spec.VictoriaMetrics.StorageClassName, v1alpha1.VictoriaMetricsComponentType),
				ReadOnly:  false,
			},
		},
	})
	if oc.Spec.VictoriaMetrics.StorageClassName != v1alpha1.DefaultStorageClass {
		deploy.Spec.Strategy.Type = apps.RecreateDeploymentStrategyType
	}
	trueVar := true
	tempSpec := &deploy.Spec.Template.Spec
	tempSpec.AutomountServiceAccountToken = &trueVar
	deploy.Spec.Strategy = apps.DeploymentStrategy{
		Type: apps.RecreateDeploymentStrategyType,
	}

	return deploy, nil
}

func (m *vmManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.VictoriaMetrics
}
