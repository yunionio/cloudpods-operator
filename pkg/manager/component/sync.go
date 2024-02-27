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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/image"
)

type syncManager interface {
	getProductVersions() []v1alpha1.ProductVersion
	getComponentManager() *ComponentManager
	getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType
	getDBConfig(*v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig
	getClickhouseConfig(*v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig
	getCloudUser(*v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser
	getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl
	getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus
}

type serviceFactory interface {
	getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service
}

type ingressFactory interface {
	getIngress(oc *v1alpha1.OnecloudCluster, zone string) *unstructured.Unstructured
	updateIngress(oc *v1alpha1.OnecloudCluster, oldIng *unstructured.Unstructured) *unstructured.Unstructured
}

type configMapFactory interface {
	getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error)
}

type pvcFactory interface {
	getPVC(oc *v1alpha1.OnecloudCluster, zone string) (*corev1.PersistentVolumeClaim, error)
}

type deploymentFactory interface {
	getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error)
}

type daemonSetFactory interface {
	getDaemonSet(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.DaemonSet, error)
}

type cronJobFactory interface {
	getCronJob(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*batchv1.CronJob, error)
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

	getComponentType() v1alpha1.ComponentType
}

func isInProductVersion(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster) bool {
	expected := oc.Spec.ProductVersion
	vs := factory.getProductVersions()
	for _, v := range vs {
		if v == expected {
			return true
		}
	}
	return false
}

func isValidProductVersion(oc *v1alpha1.OnecloudCluster) error {
	v := oc.Spec.ProductVersion
	for _, pv := range []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	} {
		if v == pv {
			return nil
		}
	}
	return errors.Errorf("Invalid productVersion %q", v)
}

func syncComponent(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster, isDisable bool, zone string) error {
	cType := factory.getComponentType()
	if isDisable {
		klog.Infof("component %q is disable, skip sync", cType)
		return nil
	}
	if err := isValidProductVersion(oc); err != nil {
		return errors.Wrapf(err, "check productVersion of %q", cType)
	}
	m := factory.getComponentManager()
	if err := m.syncConfigMap(oc, factory, zone); err != nil {
		return errors.Wrapf(err, "sync configmap of %q", cType)
	}
	if err := m.syncPVC(oc, factory, zone); err != nil {
		return errors.Wrapf(err, "sync pvc of %q", cType)
	}
	if err := m.syncDeployment(oc, factory, newPostSyncComponent(factory), zone); err != nil {
		return errors.Wrapf(err, "sync deployment %q", cType)
	}
	if err := m.syncDaemonSet(oc, factory, zone); err != nil {
		return errors.Wrapf(err, "sync daemonset %q", cType)
	}
	if err := m.syncCronJob(oc, factory, zone); err != nil {
		return errors.Wrapf(err, "sync cronjob %q", cType)
	}
	if err := m.syncService(oc, factory, zone); err != nil {
		return errors.Wrapf(err, "sync service of %q", cType)
	}
	if !controller.DisableSyncIngress {
		if err := m.syncIngress(oc, factory, zone); err != nil {
			return errors.Wrapf(err, "sync ingress of %q", cType)
		}
	}
	if err := m.syncPhase(oc, factory, zone); err != nil {
		return errors.Wrapf(err, "sync phase control %q", cType)
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

func newPostSyncComponent(f cloudComponentFactory) func(*v1alpha1.OnecloudCluster, *apps.Deployment, string) error {
	return func(oc *v1alpha1.OnecloudCluster, deploy *apps.Deployment, zone string) error {
		m := f.getComponentManager()

		deployStatus := f.getDeploymentStatus(oc, zone)
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
