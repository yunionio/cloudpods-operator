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
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/image"
)

type syncManager interface {
	getComponentManager() *ComponentManager
	getDBConfig(*v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig
	getCloudUser(*v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser
	getPhaseControl(controller.ComponentManager) controller.PhaseControl
	getDeploymentStatus(*v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus
}

type serviceFactory interface {
	getService(*v1alpha1.OnecloudCluster) *corev1.Service
}

type ingressFactory interface {
	getIngress(*v1alpha1.OnecloudCluster) *extensions.Ingress
}

type configMapFactory interface {
	getConfigMap(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error)
}

type pvcFactory interface {
	getPVC(*v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error)
}

type deploymentFactory interface {
	getDeployment(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error)
}

type daemonSetFactory interface {
	getDaemonSet(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*apps.DaemonSet, error)
}

type cronJobFactory interface {
	getCronJob(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*batchv1.CronJob, error)
}

type cloudComponentFactory interface {
	syncManager
	serviceFactory
	ingressFactory
	configMapFactory
	deploymentFactory
	pvcFactory
	daemonSetFactory
	cronJobFactory
}

func syncComponent(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster, isDisable bool) error {
	if isDisable {
		klog.Infof("component %#v is disable, skip sync", factory)
		return nil
	}
	m := factory.getComponentManager()
	if err := m.syncService(oc, factory.getService); err != nil {
		return errors.Wrap(err, "sync service")
	}
	if err := m.syncIngress(oc, factory.getIngress); err != nil {
		return errors.Wrap(err, "sync ingress")
	}
	if err := m.syncConfigMap(oc, factory.getDBConfig, factory.getCloudUser, factory.getConfigMap); err != nil {
		return errors.Wrap(err, "sync configmap")
	}
	if err := m.syncPVC(oc, factory.getPVC); err != nil {
		return errors.Wrapf(err, "sync pvc")
	}
	if err := m.syncDeployment(oc, factory.getDeployment, newPostSyncComponent(factory)); err != nil {
		return errors.Wrapf(err, "sync deployment")
	}
	if err := m.syncDaemonSet(oc, factory.getDaemonSet); err != nil {
		return errors.Wrapf(err, "sync daemonset")
	}
	if err := m.syncCronJob(oc, factory.getCronJob); err != nil {
		return errors.Wrapf(err, "sync cronjob")
	}
	if err := m.syncPhase(oc, factory.getPhaseControl); err != nil {
		return errors.Wrapf(err, "sync phase control")
	}
	return nil
}

func getRepoImageName(img string) (string, string, string) {
	ret, err := image.ParseImageReference(img)
	if err != nil {
		klog.Errorf("parse image error: %s", err)
		return "", "", ""
	}

	repo := ret.Repository
	imageName := ret.Image
	tag := ret.Tag

	return repo, imageName, tag
}

func getImageStatusByContainer(container *corev1.Container) *v1alpha1.ImageStatus {
	img := container.Image
	status := &v1alpha1.ImageStatus{
		ImagePullPolicy: container.ImagePullPolicy,
		Image:           container.Image,
	}
	repo, imgName, tag := getRepoImageName(img)
	status.ImageName = imgName
	status.Repository = repo
	status.Tag = tag
	return status
}

func getImageStatus(deploy *apps.Deployment) *v1alpha1.ImageStatus {
	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		return nil
	}
	container := containers[0]
	return getImageStatusByContainer(&container)
}

func newPostSyncComponent(f cloudComponentFactory) func(*v1alpha1.OnecloudCluster, *apps.Deployment) error {
	return func(oc *v1alpha1.OnecloudCluster, deploy *apps.Deployment) error {
		m := f.getComponentManager()

		deployStatus := f.getDeploymentStatus(oc)
		if deployStatus != nil {
			deployStatus.Deployment = &deploy.Status
			upgrading, err := m.deploymentIsUpgrading(deploy, oc)
			if err != nil {
				return err
			}
			if upgrading {
				deployStatus.Phase = v1alpha1.UpgradePhase
			} else {
				deployStatus.Phase = v1alpha1.NormalPhase
			}
			deployStatus.ImageStatus = getImageStatus(deploy)
		}

		return nil
	}
}
