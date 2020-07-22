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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type serviceOperatorManager struct {
	*ComponentManager
}

func newServiceOperatorManager(man *ComponentManager) manager.Manager {
	return &serviceOperatorManager{man}
}

func (m *serviceOperatorManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.ServiceOperator.Disable)
}

func (m *serviceOperatorManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.ServiceOperator.CloudUser
}

type OnecloudResourceOperatorOption struct {
	WebhookPort          int
	EnableLeaderElection bool
	EnableWebhooks       bool
	SyncPeriod           int
	Region               string
	AuthURL              string
	AdminUsername        string
	AdminPassword        string
	AdminDomain          string
	AdminProject         string
	APIntervalPending    *int `json:"ap_interval_pending"`
	APIntervalWaiting    *int `json:"ap_interval_waiting"`
	APDense              bool `json:"ap_dense"`
	VMIntervalPending    *int `json:"vm_interval_pending"`
}

func (m *serviceOperatorManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	opt := &OnecloudResourceOperatorOption{}

	config := cfg.ServiceOperator
	opt.EnableWebhooks = false
	opt.APDense = true
	opt.SyncPeriod = 15
	// init auth info
	opt.AuthURL = controller.GetAuthURL(oc)
	opt.AdminUsername = config.Username
	opt.AdminPassword = config.Password
	opt.AdminProject = constants.SysAdminProject
	opt.AdminDomain = constants.DefaultDomain
	return m.newServiceConfigMap(v1alpha1.ServiceOperatorComponentType, oc, opt), nil
}

func (m *serviceOperatorManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	spec := oc.Spec.ServiceOperator
	ns := oc.GetNamespace()
	ocName := oc.GetName()
	deployName := controller.NewClusterComponentName(ocName, v1alpha1.ServiceOperatorComponentType)
	appLable := m.getComponentLabel(oc, constants.ServiceOperatorAdminUser)
	appLable["control-plane"] = "controller-manager"
	cfgName := controller.ComponentConfigMapName(oc, v1alpha1.ServiceOperatorComponentType)
	configName := "oro.conf"

	// init volumes
	volumes := []corev1.Volume{
		{
			Name: constants.VolumeConfigName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cfgName,
					},
					Items: []corev1.KeyToPath{
						{Key: constants.VolumeConfigName, Path: configName},
					},
				},
			},
		},
	}
	volumesMounts := []corev1.VolumeMount{
		{
			Name:      constants.VolumeConfigName,
			ReadOnly:  true,
			MountPath: constants.ConfigDir,
		},
	}

	containers := []corev1.Container{
		{
			Args: []string{
				fmt.Sprintf("--secure-listen-address=0.0.0.0:%d", cfg.ServiceOperator.Port),
				"--upstream=http://127.0.0.1:8080/",
				"--logtostderr=true",
				"--v=10",
			},
			Image: "registry.cn-beijing.aliyuncs.com/yunionio/kube-rbac-proxy:v0.5.0",
			Name:  "kube-rbac-proxy",
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: int32(cfg.ServiceOperator.Port),
					Name:          "api",
				},
			},
		},
		{
			Command: []string{
				"/manager",
				"--config",
				fmt.Sprintf("%s/%s", constants.ConfigDir, configName),
			},
			Image:           oc.Spec.ServiceOperator.Image,
			ImagePullPolicy: oc.Spec.ServiceOperator.ImagePullPolicy,
			Name:            "manager",
			VolumeMounts:    volumesMounts,
		},
	}
	initContainers := []corev1.Container{
		{
			Command: []string{
				"kubectl",
				"apply",
				"-f",
				"/etc/crds/",
			},
			Image:           oc.Spec.ServiceOperator.Image,
			ImagePullPolicy: oc.Spec.ServiceOperator.ImagePullPolicy,
			Name:            "init",
		},
	}

	deploy := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deployName,
			Namespace:       ns,
			Labels:          appLable.Labels(),
			OwnerReferences: []metav1.OwnerReference{controller.GetOwnerRef(oc)},
		},
		Spec: apps.DeploymentSpec{
			Replicas: &spec.Replicas,
			Selector: appLable.LabelSelector(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: appLable.Labels(),
				},
				Spec: corev1.PodSpec{
					NodeSelector:       spec.NodeSelector,
					RestartPolicy:      corev1.RestartPolicyAlways,
					Tolerations:        spec.Tolerations,
					Volumes:            volumes,
					HostNetwork:        false,
					ServiceAccountName: "onecloud-operator",
					Containers:         containers,
					InitContainers:     initContainers,
				},
			},
		},
	}
	return deploy, nil
}

func (n *serviceOperatorManager) getService(oc *v1alpha1.OnecloudCluster) []*corev1.Service {
	service := m.newSingleNodePortService(v1alpha1.ServiceOperatorComponentType, oc, constants.ServiceOperatorPort)
	// diy
	service.ObjectMeta.Labels["control-plane"] = "controller-manager"
	service.Spec.Selector["control-plane"] = "controller-manager"
	return []*corev1.Service{service}
}
