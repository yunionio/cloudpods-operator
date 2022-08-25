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

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules/yunionconf"
	"yunion.io/x/onecloud/pkg/yunionconf/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type yunoinconfManager struct {
	*ComponentManager
}

func newYunionconfManager(man *ComponentManager) manager.Manager {
	return &yunoinconfManager{man}
}

func (m *yunoinconfManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *yunoinconfManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.YunionconfComponentType
}

func (m *yunoinconfManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Yunionconf.Disable, "")
}

func (m *yunoinconfManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Yunionconf.DB
}

func (m *yunoinconfManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Yunionconf.CloudUser
}

func (m *yunoinconfManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return newYunionconfPC(man)
}

// yunionconfPC implements controller.PhaseControl
type yunionconfPC struct {
	controller.PhaseControl
	man controller.ComponentManager
}

func newYunionconfPC(man controller.ComponentManager) controller.PhaseControl {
	return &yunionconfPC{
		PhaseControl: controller.NewRegisterEndpointComponent(man, v1alpha1.YunionconfComponentType,
			constants.ServiceNameYunionConf, constants.ServiceTypeYunionConf,
			man.GetCluster().Spec.Yunionconf.Service.NodePort, ""),
		man: man,
	}
}

func (pc *yunionconfPC) Setup() error {
	if err := pc.PhaseControl.Setup(); err != nil {
		return errors.Wrap(err, "endpoint for yunionconf setup")
	}
	// hack: init fake yunionagent service and endpoints
	if err := pc.man.YunionAgent().Setup(); err != nil {
		return errors.Wrap(err, "setup yunionagent for yunionconf")
	}
	return nil
}

func (pc *yunionconfPC) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	// register parameter of services
	// 1. init global-settings parameter if not created
	isEE := IsEnterpriseEdition(oc)
	gsName := "global-settings"
	if err := pc.man.RunWithSession(pc.man.GetCluster(), func(s *mcclient.ClientSession) error {
		items, err := yunionconf.Parameters.List(s, jsonutils.Marshal(map[string]string{
			"name":  gsName,
			"scope": "system"}))
		if err != nil {
			return errors.Wrapf(err, "search %s", gsName)
		}
		if len(items.Data) != 0 {
			return nil
		}
		var setupKeys []string
		setupKeysCmp := []string{
			"public",
			"private",
			"storage",
			"aliyun",
			"aws",
			"azure",
			"ctyun",
			"google",
			"huawei",
			"qcloud",
			// "ucloud",
			// "ecloud",
			// "jdcloud",
			"vmware",
			"openstack",
			// "dstack",
			// "zstack",
			"apsara",
			"cloudpods",
			// "hcso",
			"nutanix",
			// "bingocloud",
			// "incloudsphere",
			"s3",
			"ceph",
			"xsky",
		}
		setupKeysEdge := []string{
			"onecloud",
			"onestack",
			"baremetal",
			"lb",
		}
		setupKeysFull := []string{}
		setupKeysFull = append(setupKeysFull, setupKeysCmp...)
		setupKeysFull = append(setupKeysFull, setupKeysEdge...)
		oneStackInited := false
		switch oc.Spec.ProductVersion {
		case v1alpha1.ProductVersionCMP:
			setupKeys = setupKeysCmp
			oneStackInited = true
		case v1alpha1.ProductVersionEdge:
			setupKeys = setupKeysEdge
		default:
			setupKeys = setupKeysFull
		}
		setupKeys = append(setupKeys, "monitor", "auth")
		if isEE {
			switch oc.Spec.ProductVersion {
			case v1alpha1.ProductVersionEdge:
			default:
				setupKeys = append(setupKeys,
					"ucloud",
					"ecloud",
					"jdcloud",
					"zstack",
					"hcso",
					"bingocloud",
					"incloudsphere",
				)
			}
			setupKeys = append(setupKeys, "k8s", "bill")
		}
		setupKeys = append(setupKeys, "default")
		input := map[string]interface{}{
			"name":       gsName,
			"service_id": constants.ServiceNameYunionAgent,
			"value": map[string]interface{}{
				"setupKeys":                setupKeys,
				"setupKeysVersion":         "3.0",
				"setupOneStackInitialized": oneStackInited,
			},
		}
		params := jsonutils.Marshal(input)
		if _, err := yunionconf.Parameters.Create(s, params); err != nil {
			return errors.Wrap(err, "ensure global-settings")
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "pc.man.RunWithSession")
	}
	return nil
}

func (m *yunoinconfManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeYunionConf); err != nil {
		return nil, false, err
	}
	config := cfg.Yunionconf
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.Port = config.Port

	return m.newServiceConfigMap(v1alpha1.YunionconfComponentType, "", oc, opt), false, nil
}

func (m *yunoinconfManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.YunionconfComponentType, oc, int32(oc.Spec.Yunionconf.Service.NodePort), int32(cfg.Yunionconf.Port))}
}

func (m *yunoinconfManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.YunionconfComponentType, "", oc, &oc.Spec.Yunionconf.DeploymentSpec, int32(cfg.Yunionconf.Port), false, false)
}

func (m *yunoinconfManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Yunionconf
}
