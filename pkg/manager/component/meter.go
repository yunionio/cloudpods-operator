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
	"path/filepath"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type meterManager struct {
	*ComponentManager
}

func newMeterManager(man *ComponentManager) manager.Manager {
	return &meterManager{man}
}

func (m *meterManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *meterManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.MeterComponentType
}

func (m *meterManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if !IsEnterpriseEdition(oc) {
		return nil
	}
	return syncComponent(m, oc, oc.Spec.Meter.Disable, "")
}

func (m *meterManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Meter.DB
}

func (m *meterManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Meter.CloudUser
}

func (m *meterManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man,
		v1alpha1.MeterComponentType,
		constants.ServiceNameMeter,
		constants.ServiceTypeMeter,
		constants.MeterPort, "")
}

func (m *meterManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.MeterComponentType, oc, constants.MeterPort)}
}

type meterOptions struct {
	common_options.CommonOptions
	common_options.DBOptions

	BillingFileDirectory    string
	AwsRiPlanIdHandle       string
	CloudratesFileDirectory string
	InfluxdbMeterName       string
	MonthlyBill             bool
}

func (m *meterManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &meterOptions{}
	if err := SetOptionsDefault(opt, constants.ServiceTypeMeter); err != nil {
		return nil, false, err
	}
	config := cfg.Meter
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceDBCommonOptions.ServiceCommonOptions)

	opt.BillingFileDirectory = filepath.Join(constants.MeterDataStore, constants.MeterBillingDataDir)
	opt.CloudratesFileDirectory = filepath.Join(constants.MeterDataStore, constants.MeterRatesDataDir)
	opt.AwsRiPlanIdHandle = constants.MeterAwsRiPlanIdHandle
	opt.InfluxdbMeterName = constants.MeterInfluxDB
	opt.MonthlyBill = constants.MeterMonthlyBill

	// TODO: fix this
	opt.AutoSyncTable = true
	opt.Port = constants.MeterPort

	return m.newServiceConfigMap(v1alpha1.MeterComponentType, "", oc, opt), false, nil
}

func (m *meterManager) getPVC(oc *v1alpha1.OnecloudCluster, zone string) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.Meter
	return m.ComponentManager.newPVC(v1alpha1.MeterComponentType, oc, cfg)
}

func (m *meterManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.MeterComponentType, "", oc, &oc.Spec.Meter.DeploymentSpec, constants.MeterPort, true, false)
	if err != nil {
		return nil, err
	}
	/*
	 * if oc.Spec.Meter.StorageClassName == v1alpha1.DefaultStorageClass {
	 * 	// if use local path storage, remove cloud affinity
	 * 	deploy = m.removeDeploymentAffinity(deploy)
	 * }
	 */
	podTemplate := &deploy.Spec.Template.Spec
	podVols := podTemplate.Volumes
	podVols = append(podVols, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: m.newPvcName(oc.GetName(), oc.Spec.Meter.StorageClassName, v1alpha1.MeterComponentType),
				ReadOnly:  false,
			},
		},
	})
	volMounts := podTemplate.Containers[0].VolumeMounts
	volMounts = append(volMounts, corev1.VolumeMount{
		Name:      "data",
		MountPath: constants.MeterDataStore,
	})

	if oc.Spec.Meter.StorageClassName != v1alpha1.DefaultStorageClass {
		deploy.Spec.Strategy.Type = apps.RecreateDeploymentStrategyType
	}

	// var run
	var hostPathDirectory = corev1.HostPathDirectory
	podVols = append(podVols, corev1.Volume{
		Name: "run",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/run",
				Type: &hostPathDirectory,
			},
		},
	})
	volMounts = append(volMounts, corev1.VolumeMount{
		Name:      "run",
		ReadOnly:  false,
		MountPath: "/var/run",
	})

	podTemplate.Containers[0].VolumeMounts = volMounts
	podTemplate.Volumes = podVols

	// add pod label for pod affinity
	if deploy.Spec.Template.ObjectMeta.Labels == nil {
		deploy.Spec.Template.ObjectMeta.Labels = make(map[string]string)
	}
	deploy.Spec.Template.ObjectMeta.Labels[constants.OnecloudHostDeployerLabelKey] = ""
	if deploy.Spec.Selector == nil {
		deploy.Spec.Selector = &metav1.LabelSelector{}
	}
	if deploy.Spec.Selector.MatchLabels == nil {
		deploy.Spec.Selector.MatchLabels = make(map[string]string)
	}
	deploy.Spec.Selector.MatchLabels[constants.OnecloudHostDeployerLabelKey] = ""
	return deploy, nil
}

func (m *meterManager) getPodLabels() map[string]string {
	return map[string]string{
		constants.OnecloudHostDeployerLabelKey: "",
	}
}

func (m *meterManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Meter.DeploymentStatus
}
