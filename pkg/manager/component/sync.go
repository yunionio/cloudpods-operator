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
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
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

type cloudComponentFactory interface {
	syncManager
	serviceFactory
	ingressFactory
	configMapFactory
	deploymentFactory
	pvcFactory
	daemonSetFactory
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
	return nil
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
		}

		if _, err := m.onecloudControl.GetSession(oc); err != nil {
			return errors.Wrapf(err, "get cluster %s session", oc.GetName())
		}

		phase := f.getPhaseControl(m.onecloudControl.Components(oc))
		if phase == nil {
			return nil
		}
		if err := phase.Setup(); err != nil {
			return err
		}
		if err := phase.SystemInit(); err != nil {
			return err
		}
		return nil
	}
}
