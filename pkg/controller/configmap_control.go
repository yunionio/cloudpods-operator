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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

type ConfigMapControlInterface interface {
	CreateConfigMap(*v1alpha1.OnecloudCluster, *corev1.ConfigMap) error
	UpdateConfigMap(*v1alpha1.OnecloudCluster, *corev1.ConfigMap) (*corev1.ConfigMap, error)
	DeleteConfigMap(*v1alpha1.OnecloudCluster, *corev1.ConfigMap) error
}

type realConfigMapControl struct {
	*baseControl
	kubeCli   kubernetes.Interface
	cfgLister corelisters.ConfigMapLister
}

func NewConfigMapControl(kubeCli kubernetes.Interface, cfgLister corelisters.ConfigMapLister, recorder record.EventRecorder) ConfigMapControlInterface {
	return &realConfigMapControl{
		newBaseControl("ConfigMap", recorder),
		kubeCli,
		cfgLister,
	}
}

func (c *realConfigMapControl) CreateConfigMap(oc *v1alpha1.OnecloudCluster, cfg *corev1.ConfigMap) error {
	_, err := c.kubeCli.CoreV1().ConfigMaps(oc.Namespace).Create(context.Background(), cfg, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	c.RecordCreateEvent(oc, cfg, err)
	return err
}

func (c *realConfigMapControl) UpdateConfigMap(oc *v1alpha1.OnecloudCluster, cfg *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	ns := oc.GetNamespace()
	ocName := oc.GetName()
	cfgName := cfg.GetName()

	var updateCfg *corev1.ConfigMap
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var updateErr error
		updateCfg, updateErr = c.kubeCli.CoreV1().ConfigMaps(ns).Update(context.Background(), cfg, metav1.UpdateOptions{})
		if updateErr == nil {
			klog.Infof("update ConfigMap: [%s/%s] successfully, cluster: %s", ns, cfgName, ocName)
			return nil
		}

		if updated, err := c.cfgLister.ConfigMaps(ns).Get(cfgName); err != nil {
			cfg = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated ConfigMap %s/%s from lister: %v", ns, cfgName, err))
		}

		return updateErr
	})
	c.RecordUpdateEvent(oc, cfg, err)
	return updateCfg, err
}

func (c *realConfigMapControl) DeleteConfigMap(oc *v1alpha1.OnecloudCluster, cfg *corev1.ConfigMap) error {
	err := c.kubeCli.CoreV1().ConfigMaps(oc.Namespace).Delete(context.Background(), cfg.Name, metav1.DeleteOptions{})
	c.RecordDeleteEvent(oc, cfg, err)
	return err
}
