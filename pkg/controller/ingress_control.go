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
	extensions "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

// IngressControlInterface defines the interface that uses to create, update and delete Ingress
type IngressControlInterface interface {
	CreateIngress(*v1alpha1.OnecloudCluster, *extensions.Ingress) error
	UpdateIngress(*v1alpha1.OnecloudCluster, *extensions.Ingress) (*extensions.Ingress, error)
	DeleteIngress(*v1alpha1.OnecloudCluster, *extensions.Ingress) error
}

type realIngressControl struct {
	*baseControl
	kubeCli       kubernetes.Interface
	ingressLister listers.IngressLister
}

func NewIngressControl(kubeCli kubernetes.Interface, ingressLister listers.IngressLister, recorder record.EventRecorder) IngressControlInterface {
	return &realIngressControl{newBaseControl("Ingress", recorder), kubeCli, ingressLister}
}

func (c *realIngressControl) CreateIngress(oc *v1alpha1.OnecloudCluster, ing *extensions.Ingress) error {
	_, err := c.kubeCli.ExtensionsV1beta1().Ingresses(oc.Namespace).Create(ing)
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	c.RecordCreateEvent(oc, ing, err)
	return err
}

func (c *realIngressControl) UpdateIngress(oc *v1alpha1.OnecloudCluster, ing *extensions.Ingress) (*extensions.Ingress, error) {
	ns := oc.GetNamespace()
	ocName := oc.GetName()
	ingName := ing.GetName()
	ingSpec := ing.Spec.DeepCopy()
	var updatedIng *extensions.Ingress
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var updateErr error
		updatedIng, updateErr = c.kubeCli.ExtensionsV1beta1().Ingresses(ns).Update(ing)
		if updateErr == nil {
			klog.Infof("OnecloudCluster: [%s/%s]'s Ingress: [%s/%s] updated successfully", ns, ocName, ns, ingName)
			return nil
		}
		klog.Errorf("failed to update OnecloudCluster: [%s/%s]'s Ingress: [%s/%s], error: %v", ns, ocName, ns, ingName, updateErr)

		if updated, err := c.ingressLister.Ingresses(ns).Get(ingName); err == nil {
			ing = updated.DeepCopy()
			ing.Spec = *ingSpec
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated Ingress %s/%s from lister: %v", ns, ingName, err))
		}
		return updateErr
	})

	c.RecordUpdateEvent(oc, ing, err)
	return updatedIng, err
}

func (c *realIngressControl) DeleteIngress(oc *v1alpha1.OnecloudCluster, ing *extensions.Ingress) error {
	err := c.kubeCli.ExtensionsV1beta1().Ingresses(oc.GetNamespace()).Delete(ing.Name, nil)
	c.RecordDeleteEvent(oc, ing, err)
	return err
}
