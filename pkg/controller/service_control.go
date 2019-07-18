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

package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

// ServiceControlInterface manages Services used in OnecloudCluster
type ServiceControlInterface interface {
	CreateService(*v1alpha1.OnecloudCluster, *corev1.Service) error
	UpdateService(*v1alpha1.OnecloudCluster, *corev1.Service) (*corev1.Service, error)
	DeleteService(*v1alpha1.OnecloudCluster, *corev1.Service) error
}

type realServiceControl struct {
	*baseControl
	kubeCli   kubernetes.Interface
	svcLister corelisters.ServiceLister
}

// NewServiceControl creates a new ServiceControlInterface
func NewServiceControl(kubeCli kubernetes.Interface, svcLister corelisters.ServiceLister, recorder record.EventRecorder) ServiceControlInterface {
	return &realServiceControl{
		newBaseControl("Service", recorder),
		kubeCli,
		svcLister,
	}
}

func (c *realServiceControl) CreateService(oc *v1alpha1.OnecloudCluster, svc *corev1.Service) error {
	_, err := c.kubeCli.CoreV1().Services(oc.Namespace).Create(svc)
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	c.RecordCreateEvent(oc, svc, err)
	return err
}

func (c *realServiceControl) UpdateService(oc *v1alpha1.OnecloudCluster, svc *corev1.Service) (*corev1.Service, error) {
	ns := oc.GetNamespace()
	ocName := oc.GetName()
	svcName := svc.GetName()
	svcSpec := svc.Spec.DeepCopy()

	var updateSvc *corev1.Service
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var updateErr error
		updateSvc, updateErr = c.kubeCli.CoreV1().Services(ns).Update(svc)
		if updateErr == nil {
			klog.Infof("update Service: [%s/%s] successfully, cluster: %s", ns, svcName, ocName)
			return nil
		}

		if updated, err := c.svcLister.Services(oc.Namespace).Get(svcName); err != nil {
			svc = updated.DeepCopy()
			svc.Spec = *svcSpec
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated Service %s/%s from lister: %v", ns, svcName, err))
		}

		return updateErr
	})
	c.RecordUpdateEvent(oc, svc, err)
	return updateSvc, err
}

func (c *realServiceControl) DeleteService(oc *v1alpha1.OnecloudCluster, svc *corev1.Service) error {
	err := c.kubeCli.CoreV1().Services(oc.Namespace).Delete(svc.Name, nil)
	c.RecordDeleteEvent(oc, svc, err)
	return err
}
