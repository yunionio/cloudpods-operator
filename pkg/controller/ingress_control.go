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
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	cache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/k8sutil"
)

// IngressControlInterface defines the interface that uses to create, update and delete Ingress
type IngressControlInterface interface {
	CreateIngress(*v1alpha1.OnecloudCluster, *unstructured.Unstructured) error
	UpdateIngress(*v1alpha1.OnecloudCluster, *unstructured.Unstructured) (*unstructured.Unstructured, error)
	DeleteIngress(*v1alpha1.OnecloudCluster, *unstructured.Unstructured) error
}

type realIngressControl struct {
	*baseControl
	dynamicCli     dynamic.Interface
	ingressLister  cache.GenericLister
	clusterVersion k8sutil.ClusterVersion
}

func NewIngressControl(
	dCli dynamic.Interface,
	ingressLister cache.GenericLister,
	recorder record.EventRecorder,
	cv k8sutil.ClusterVersion,
) IngressControlInterface {
	return &realIngressControl{
		newBaseControl("Ingress", recorder),
		dCli,
		ingressLister,
		cv,
	}
}

func (c *realIngressControl) Client() dynamic.NamespaceableResourceInterface {
	return c.dynamicCli.Resource(c.clusterVersion.GetIngressGVR())
}

func (c *realIngressControl) CreateIngress(oc *v1alpha1.OnecloudCluster, ing *unstructured.Unstructured) error {
	_, err := c.Client().Namespace(oc.GetNamespace()).Create(context.Background(), ing, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	c.RecordCreateEvent(oc, ing, err)
	return err
}

func (c *realIngressControl) UpdateIngress(oc *v1alpha1.OnecloudCluster, ing *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	ns := oc.GetNamespace()
	ocName := oc.GetName()
	ingName := ing.GetName()
	ingSpec, _, _ := unstructured.NestedMap(ing.DeepCopy().Object, "spec")
	var updatedIng *unstructured.Unstructured
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var updateErr error
		updatedIng, updateErr = c.Client().Namespace(oc.GetNamespace()).Update(context.Background(), ing, metav1.UpdateOptions{})
		if updateErr == nil {
			klog.Infof("OnecloudCluster: [%s/%s]'s Ingress: [%s/%s] updated successfully", ns, ocName, ns, ingName)
			return nil
		}
		klog.Errorf("failed to update OnecloudCluster: [%s/%s]'s Ingress: [%s/%s], error: %v", ns, ocName, ns, ingName, updateErr)

		if updated, err := c.Client().Namespace(ns).Get(context.Background(), ingName, metav1.GetOptions{}); err == nil {
			ing = updated.DeepCopy()
			unstructured.SetNestedField(ing.Object, ingSpec, "spec")
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated Ingress %s/%s from lister: %v", ns, ingName, err))
		}
		return updateErr
	})

	c.RecordUpdateEvent(oc, ing, err)
	return updatedIng, err
}

func (c *realIngressControl) DeleteIngress(oc *v1alpha1.OnecloudCluster, ing *unstructured.Unstructured) error {
	err := c.Client().Namespace(oc.GetNamespace()).Delete(context.Background(), ing.GetName(), metav1.DeleteOptions{})
	c.RecordDeleteEvent(oc, ing, err)
	return err
}
