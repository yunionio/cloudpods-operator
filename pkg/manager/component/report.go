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

	"yunion.io/x/onecloud/pkg/ansibleserver/options"
	"yunion.io/x/onecloud/pkg/mcclient"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type reportManager struct {
	*ComponentManager
}

func newReportManager(man *ComponentManager) manager.ServiceManager {
	return &reportManager{man}
}

func (m *reportManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *reportManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.ReportComponentType
}

func (m *reportManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Report.Disable || !IsEnterpriseEdition(oc)
}

func (m *reportManager) GetServiceName() string {
	return constants.ServiceNameReport
}

func (m *reportManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *reportManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Report.DB
}

func (m *reportManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Report.DbEngine)
}

func (m *reportManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Report.CloudUser
}

func (m *reportManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.ReportComponentType,
		constants.ServiceNameReport, constants.ServiceTypeReport,
		man.GetCluster().Spec.Report.Service.NodePort, "")
}

func (m *reportManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeReport); err != nil {
		return nil, false, err
	}
	config := cfg.Report

	switch oc.Spec.GetDbEngine(oc.Spec.Report.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = config.Port
	return m.newServiceConfigMap(v1alpha1.ReportComponentType, "", oc, opt), false, nil
}

func (m *reportManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return m.newSinglePortService(v1alpha1.ReportComponentType, oc, oc.Spec.Report.Service.InternalOnly, int32(oc.Spec.Report.Service.NodePort), int32(cfg.Report.Port), oc.Spec.Report.SlaveReplicas > 0)
}

func (m *reportManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "report",
				Image:           oc.Spec.Report.Image,
				ImagePullPolicy: oc.Spec.Report.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/report", "--config", "/etc/yunion/report.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.ReportComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.ReportComponentType), v1alpha1.ReportComponentType), &oc.Spec.Report.DeploymentSpec, cf)
}

func (m *reportManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Report
}

func (m *reportManager) supportsReadOnlyService() bool {
	return false
}

func (m *reportManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return nil
}

func (m *reportManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return func(s *mcclient.ClientSession) error {
		if m.IsDisabled(oc) {
			return onecloud.EnsureDisableService(s, m.GetServiceName())
		} else {
			return onecloud.EnsureEnableService(s, m.GetServiceName(), m.supportsReadOnlyService() && oc.Spec.Report.SlaveReplicas > 0)
		}
	}
}
