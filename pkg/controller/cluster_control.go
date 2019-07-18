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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/client/clientset/versioned"
	listers "yunion.io/x/onecloud-operator/pkg/client/listers/onecloud/v1alpha1"
)

// ClusterControlInterface manages Onecloud clusters
type ClusterControlInterface interface {
	UpdateCluster(cluster *v1alpha1.OnecloudCluster, newStatus *v1alpha1.OnecloudClusterStatus, oldStatus *v1alpha1.OnecloudClusterStatus) (*v1alpha1.OnecloudCluster, error)
}

type clusterControl struct {
	cli      versioned.Interface
	ocLister listers.OnecloudClusterLister
	recorder record.EventRecorder
}

// NewClusterControl creates a new ClsuterControlInterface
func NewClusterControl(
	cli versioned.Interface,
	ocLister listers.OnecloudClusterLister,
	recorder record.EventRecorder) ClusterControlInterface {
	return &clusterControl{
		cli:      cli,
		ocLister: ocLister,
		recorder: recorder,
	}
}

func (c *clusterControl) UpdateCluster(oc *v1alpha1.OnecloudCluster, newStatus, oldStatus *v1alpha1.OnecloudClusterStatus) (*v1alpha1.OnecloudCluster, error) {
	ns := oc.GetNamespace()
	ocName := oc.GetName()

	status := oc.Status.DeepCopy()
	var updateOC *v1alpha1.OnecloudCluster

	// don't wait due to limited number of clients, but backoff after the default number of steps
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var updateErr error
		updateOC, updateErr = c.cli.OnecloudV1alpha1().OnecloudClusters(ns).Update(oc)
		if updateErr == nil {
			klog.Infof("OnecloudCluster: [%s/%s] updated successfully", ns, ocName)
			return nil
		}
		klog.Errorf("failed to update OnecloudCluster: [%s/%s], error: %v", ns, ocName, updateErr)

		if updated, err := c.ocLister.OnecloudClusters(ns).Get(ocName); err != nil {
			// make a copy so we don't mutate the shared cache
			oc = updated.DeepCopy()
			oc.Status = *status
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated OnecloudCluster %s/%s from lister: %v", ns, ocName, err))
		}
		return updateErr
	})
	if !apiequality.Semantic.DeepEqual(newStatus, oldStatus) {
		c.recordClusterEvent("update", oc, err)
	}
	return updateOC, err
}

func (c *clusterControl) recordClusterEvent(verb string, oc *v1alpha1.OnecloudCluster, err error) {
	ocName := oc.GetName()
	if err == nil {
		reason := fmt.Sprintf("Successful%s", strings.Title(verb))
		msg := fmt.Sprintf("%s OnecloudCluster %s sucessful",
			strings.ToLower(verb), ocName)
		c.recorder.Event(oc, corev1.EventTypeNormal, reason, msg)
	} else {
		reason := fmt.Sprintf("Failed%s", strings.Title(verb))
		msg := fmt.Sprintf("%s OnecloudCluster %s failed error: %s", strings.ToLower(verb), ocName, err)
		c.recorder.Event(oc, corev1.EventTypeWarning, reason, msg)
	}
}
