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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/esxi/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type esxiManager struct {
	*ComponentManager
}

func newEsxiManager(m *ComponentManager) manager.Manager {
	return &esxiManager{m}
}

func (m *esxiManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
	}
}

func (m *esxiManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.EsxiAgentComponentType
}

func (m *esxiManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return m.multiZoneSync(oc, oc.Spec.EsxiAgent.Zones, m, oc.Spec.EsxiAgent.Disable)
}

func (m *esxiManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.EsxiAgent.CloudUser
}

func (m *esxiManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	zoneId := oc.GetZone(zone)

	opt := &options.Options
	if err := option.SetOptionsDefault(opt, ""); err != nil {
		return nil, false, err
	}
	opt.Zone = zoneId
	opt.ListenInterface = "eth0"
	config := cfg.EsxiAgent
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.Port = constants.EsxiAgentPort
	newCfg := m.newServiceConfigMap(v1alpha1.EsxiAgentComponentType, zone, oc, opt)
	return m.customConfig(oc, newCfg, zone)
}

func (m *esxiManager) customConfig(oc *v1alpha1.OnecloudCluster, newCfg *corev1.ConfigMap, zone string) (*corev1.ConfigMap, bool, error) {
	oldCfgMap, err := m.configer.Lister().ConfigMaps(oc.GetNamespace()).
		Get(controller.ComponentConfigMapName(oc, m.getZoneComponent(v1alpha1.EsxiAgentComponentType, zone)))
	if err != nil && !errors.IsNotFound(err) {
		return nil, false, err
	}
	if errors.IsNotFound(err) {
		return newCfg, false, nil
	}
	if oldConfigStr, ok := oldCfgMap.Data["config"]; ok {
		config, err := jsonutils.ParseYAML(oldConfigStr)
		if err != nil {
			return nil, false, err
		}
		jConfig := config.(*jsonutils.JSONDict)

		var updateOldConf bool
		// force update deploy server socket path with new config
		deployPath, err := jConfig.GetString("deploy_server_socket_path")
		if err == nil && deployPath != options.Options.DeployServerSocketPath {
			jConfig.Set("deploy_server_socket_path", jsonutils.NewString(options.Options.DeployServerSocketPath))
			updateOldConf = true
		}

		if updateOldConf {
			return m.newConfigMap(v1alpha1.EsxiAgentComponentType, zone, oc, jConfig.YAMLString()), true, nil
		}
	}
	return newCfg, false, nil
}

func (m *esxiManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	if len(zone) == 0 {
		return &oc.Status.EsxiAgent.DeploymentStatus
	} else {
		if oc.Status.EsxiAgent.ZoneEsxiAgent == nil {
			oc.Status.EsxiAgent.ZoneEsxiAgent = make(map[string]*v1alpha1.DeploymentStatus)
		}
		_, ok := oc.Status.EsxiAgent.ZoneEsxiAgent[zone]
		if !ok {
			oc.Status.EsxiAgent.ZoneEsxiAgent[zone] = new(v1alpha1.DeploymentStatus)
		}
		return oc.Status.EsxiAgent.ZoneEsxiAgent[zone]
	}
}

func (m *esxiManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	dm, err := m.newCloudServiceSinglePortDeployment(
		v1alpha1.EsxiAgentComponentType,
		m.getZoneComponent(v1alpha1.EsxiAgentComponentType, zone),
		oc, &oc.Spec.EsxiAgent.DeploymentSpec, constants.EsxiAgentPort,
		false, false,
	)
	if err != nil {
		return nil, err
	}
	// if oc.Spec.EsxiAgent.StorageClassName == v1alpha1.DefaultStorageClass {
	// 	// if use local path storage, remove cloud affinity
	// 	dm = m.removeDeploymentAffinity(dm)
	// }

	// workspace
	podTemplate := &dm.Spec.Template.Spec
	podVols := podTemplate.Volumes
	volMounts := podTemplate.Containers[0].VolumeMounts
	var privileged = true
	podTemplate.Containers[0].SecurityContext = &corev1.SecurityContext{
		Privileged: &privileged,
	}

	var (
		hostPathDirOrCreate = corev1.HostPathDirectoryOrCreate
		mountMode           = corev1.MountPropagationBidirectional
	)
	podVols = append(podVols, corev1.Volume{
		Name: "opt-cloud",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/opt/cloud",
				Type: &hostPathDirOrCreate,
			},
		},
	})
	volMounts = append(volMounts, corev1.VolumeMount{
		Name:             "opt-cloud",
		MountPath:        "/opt/cloud",
		MountPropagation: &mountMode,
	})

	if oc.Spec.EsxiAgent.StorageClassName != v1alpha1.DefaultStorageClass {
		dm.Spec.Strategy.Type = apps.RecreateDeploymentStrategyType
	}

	// /var/run
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
	if dm.Spec.Template.ObjectMeta.Labels == nil {
		dm.Spec.Template.ObjectMeta.Labels = make(map[string]string)
	}
	dm.Spec.Template.ObjectMeta.Labels[constants.OnecloudHostDeployerLabelKey] = ""
	if dm.Spec.Selector == nil {
		dm.Spec.Selector = &metav1.LabelSelector{}
	}
	if dm.Spec.Selector.MatchLabels == nil {
		dm.Spec.Selector.MatchLabels = make(map[string]string)
	}
	dm.Spec.Selector.MatchLabels[constants.OnecloudHostDeployerLabelKey] = ""
	return setSelfAntiAffnity(dm, v1alpha1.EsxiAgentComponentType), nil
}
