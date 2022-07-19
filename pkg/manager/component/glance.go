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
	"context"
	"fmt"
	"strings"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"

	"yunion.io/x/onecloud/pkg/image/options"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	identity_modules "yunion.io/x/onecloud/pkg/mcclient/modules/identity"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

var s3ConfigSynced bool

type glanceManager struct {
	*ComponentManager
}

func newGlanceManager(man *ComponentManager) manager.Manager {
	return &glanceManager{man}
}

func (m *glanceManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *glanceManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.GlanceComponentType
}

func (m *glanceManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Glance.Disable, "")
}

func (m *glanceManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Glance.DB
}

func (m *glanceManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Glance.CloudUser
}

func (m *glanceManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.Glance()
}

func (m *glanceManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.GlanceComponentType, oc, constants.GlanceAPIPort)}
}

func (m *glanceManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeGlance); err != nil {
		return nil, false, err
	}
	config := cfg.Glance
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions, false)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceDBCommonOptions.ServiceCommonOptions)

	opt.FilesystemStoreDatadir = constants.GlanceFileStoreDir
	//opt.TorrentStoreDir = constants.GlanceTorrentStoreDir
	opt.EnableTorrentService = false
	// TODO: fix this
	opt.AutoSyncTable = true
	opt.Port = constants.GlanceAPIPort

	opt.EnableRemoteExecutor = true
	if oc.Spec.Glance.SwitchToS3 {
		if err := m.setS3Config(oc); err != nil {
			return nil, false, err
		}
		opt.StorageDriver = "s3"
	}

	newCfg := m.newServiceConfigMap(v1alpha1.GlanceComponentType, "", oc, opt)
	return m.customConfig(oc, newCfg)
}

func (m *glanceManager) setS3Config(oc *v1alpha1.OnecloudCluster) error {
	if s3ConfigSynced {
		return nil
	}
	// fetch s3 config
	s := auth.GetAdminSession(context.Background(), oc.Spec.Region, "")
	conf, err := identity_modules.ServicesV3.GetSpecific(s, "glance", "config", nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Warningf("Glance service config not found yet")
			return nil
		}
		return err
	}
	confJson, err := conf.Get("config", "default")
	if err != nil {
		return fmt.Errorf("failed get conf %s", err)
	}
	if confJson.Contains("s3_endpoint") {
		s3ConfigSynced = true
		return nil
	}

	sec, err := m.kubeCli.CoreV1().Secrets(constants.OnecloudMinioNamespace).Get(constants.OnecloudMinioSecret, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		log.Infof("namespace %s not found", constants.OnecloudMinioNamespace)
		return fmt.Errorf("namespace %s not found", constants.OnecloudMinioNamespace)
	}

	svc, err := m.kubeCli.CoreV1().Services(constants.OnecloudMinioNamespace).Get(constants.OnecloudMinioSvc, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		log.Infof("service %s not found", constants.OnecloudMinioNamespace)
		return fmt.Errorf("service %s not found", constants.OnecloudMinioSvc)
	}

	//svc.Spec.ClusterIP
	port := svc.Spec.Ports[0].Port
	endpoint := fmt.Sprintf("%s.%s:%d", svc.GetName(), svc.GetNamespace(), port)

	var ak, sk string
	// base64 decode
	if accessKey, ok := sec.Data["accesskey"]; ok {
		ak = string(accessKey)
	} else {
		if oc.Spec.Glance.SwitchToS3 {
			return fmt.Errorf("s3 access key not found")
		}
		return nil
	}
	if secretKey, ok := sec.Data["secretkey"]; ok {
		sk = string(secretKey)
	} else {
		return fmt.Errorf("s3 secret key not found")
	}

	log.Infof("glance s3 config %s %s %s", endpoint, ak, sk)
	confDict := confJson.(*jsonutils.JSONDict)
	confDict.Set("storage_driver", jsonutils.NewString("s3"))
	confDict.Set("s3_endpoint", jsonutils.NewString(endpoint))
	confDict.Set("s3_use_ssl", jsonutils.NewBool(false))
	confDict.Set("s3_access_key", jsonutils.NewString(ak))
	confDict.Set("s3_secret_key", jsonutils.NewString(sk))
	config := jsonutils.NewDict()
	config.Add(confDict, "config", "default")
	out, err := identity_modules.ServicesV3.PerformAction(s, "glance", "config", config)
	if err != nil {
		return err
	}
	log.Infof("sync configmap output %s", out)
	s3ConfigSynced = true
	return nil
}

func (m *glanceManager) customConfig(oc *v1alpha1.OnecloudCluster, newCfg *corev1.ConfigMap) (*corev1.ConfigMap, bool, error) {
	oldCfgMap, err := m.configer.Lister().ConfigMaps(oc.GetNamespace()).
		Get(controller.ComponentConfigMapName(oc, v1alpha1.GlanceComponentType))
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
			return m.newConfigMap(v1alpha1.GlanceComponentType, "", oc, jConfig.YAMLString()), true, nil
		}
	}
	return newCfg, false, nil
}

func (m *glanceManager) getPVC(oc *v1alpha1.OnecloudCluster, zone string) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.Glance.StatefulDeploymentSpec
	pvc, err := m.ComponentManager.newPVC(v1alpha1.GlanceComponentType, oc, cfg)
	if err != nil {
		return nil, err
	}
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Annotations[constants.SpecifiedPresistentVolumePath] = constants.GlanceDataStore
	return pvc, nil
}

func (m *glanceManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.GlanceComponentType, "", oc, &oc.Spec.Glance.DeploymentSpec, constants.GlanceAPIPort, true, false)
	if err != nil {
		return nil, err
	}

	podTemplate := &deploy.Spec.Template.Spec
	podVols := podTemplate.Volumes
	volMounts := podTemplate.Containers[0].VolumeMounts
	var privileged = true
	podTemplate.Containers[0].SecurityContext = &corev1.SecurityContext{
		Privileged: &privileged,
	}

	// if we are not use local path, propagation mount '/opt/cloud' to glance
	// and persistent volume will propagate to host, then host deployer can find it
	// FIX: QIUJIAN always mount /opt/cloud
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
	if oc.Spec.Glance.StorageClassName != v1alpha1.DefaultStorageClass {
		deploy.Spec.Strategy.Type = apps.RecreateDeploymentStrategyType
	}

	if !oc.Spec.Glance.SwitchToS3 { // use pvc as data store
		if oc.Spec.Glance.StorageClassName == v1alpha1.DefaultStorageClass {
			// if use local path storage, remove cloud affinity
			deploy = m.removeDeploymentAffinity(deploy)
		}

		// data store pvc, mount path: /opt/cloud/workspace/data/glance
		podVols = append(podVols, corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: m.newPvcName(oc.GetName(), oc.Spec.Glance.StorageClassName, v1alpha1.GlanceComponentType),
					ReadOnly:  false,
				},
			},
		})
		// mount PVC to /data to bind glance to node
		volMounts = append(volMounts, corev1.VolumeMount{
			Name:      "data",
			MountPath: "/data", // constants.GlanceDataStore,
		})
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

	podVols = append(podVols, corev1.Volume{
		Name: "dev",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/dev",
				Type: &hostPathDirectory,
			},
		},
	})
	volMounts = append(volMounts, corev1.VolumeMount{
		Name:      "dev",
		ReadOnly:  false,
		MountPath: "/dev",
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

func (m *glanceManager) getPodLabels() map[string]string {
	return map[string]string{
		constants.OnecloudHostDeployerLabelKey: "",
	}
}

func (m *glanceManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Glance.DeploymentStatus
}
