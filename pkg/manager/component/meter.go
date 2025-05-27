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
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type meterManager struct {
	*ComponentManager
}

func newMeterManager(man *ComponentManager) manager.ServiceManager {
	return &meterManager{man}
}

func (m *meterManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *meterManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.MeterComponentType
}

func (m *meterManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Meter.Disable || !IsEnterpriseEdition(oc)
}

func (m *meterManager) GetServiceName() string {
	return constants.ServiceNameMeter
}

func (m *meterManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if !IsEnterpriseEdition(oc) {
		return nil
	}
	return syncComponent(m, oc, "")
}

func (m *meterManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Meter.DB
}

func (m *meterManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	// always mysql
	return v1alpha1.DBEngineMySQL
}

func (m *meterManager) getClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Meter.ClickhouseConf
}

func (m *meterManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Meter.CloudUser
}

func (m *meterManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man,
		v1alpha1.MeterComponentType,
		constants.ServiceNameMeter,
		constants.ServiceTypeMeter,
		man.GetCluster().Spec.Meter.Service.NodePort, "")
}

func (m *meterManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSinglePortService(v1alpha1.MeterComponentType, oc, oc.Spec.Meter.Service.InternalOnly, int32(oc.Spec.Meter.Service.NodePort), int32(cfg.Meter.Port))}
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
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeMeter); err != nil {
		return nil, false, err
	}
	config := cfg.Meter

	switch oc.Spec.GetDbEngine(oc.Spec.Meter.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceDBCommonOptions.ServiceCommonOptions)

	opt.BillingFileDirectory = filepath.Join(constants.MeterDataStore, constants.MeterBillingDataDir)
	opt.CloudratesFileDirectory = filepath.Join(constants.MeterDataStore, constants.MeterRatesDataDir)
	opt.AwsRiPlanIdHandle = constants.MeterAwsRiPlanIdHandle
	opt.InfluxdbMeterName = constants.MeterInfluxDB
	opt.MonthlyBill = constants.MeterMonthlyBill

	// TODO: fix this
	opt.AutoSyncTable = true
	opt.Port = config.Port

	return m.newServiceConfigMap(v1alpha1.MeterComponentType, "", oc, opt), false, nil
}

func (m *meterManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeploymentWithReadinessProbePath(
		v1alpha1.MeterComponentType, "", oc, &oc.Spec.Meter.DeploymentSpec,
		constants.MeterPort, true, false,
		"/ping",
	)
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
	volMounts := podTemplate.Containers[0].VolumeMounts

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

/*func (m *meterManager) getPodLabels() map[string]string {
	return map[string]string{
		constants.OnecloudHostDeployerLabelKey: "",
	}
}*/

func (m *meterManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Meter.DeploymentStatus
}
