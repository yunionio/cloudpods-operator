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

	apps "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

// DeploymentControlInterface defines the interface that uses to create, update, and delete Deployment
type DeploymentControlInterface interface {
	// CreateDeployment creates deployment in a OnecloudCluster.
	CreateDeployment(*v1alpha1.OnecloudCluster, *apps.Deployment) error
	// UpdateDeployment updates a deployment in a OnecloudCluster.
	UpdateDeployment(*v1alpha1.OnecloudCluster, *apps.Deployment) (*apps.Deployment, error)
	// DeleteDeployment deletes a deployment in a OnecloudCluster.
	DeleteDeployment(*v1alpha1.OnecloudCluster, string) error
}

type realDeploymentControl struct {
	*baseControl
	kubeCli      kubernetes.Interface
	deployLister appslisters.DeploymentLister
}

// NewDeploymentControl returns a DeploymentControlInterface
func NewDeploymentControl(kubeCli kubernetes.Interface, deployLister appslisters.DeploymentLister, recorder record.EventRecorder) DeploymentControlInterface {
	return &realDeploymentControl{newBaseControl("Deployment", recorder), kubeCli, deployLister}
}

func (c *realDeploymentControl) CreateDeployment(oc *v1alpha1.OnecloudCluster, deploy *apps.Deployment) error {
	_, err := c.kubeCli.AppsV1().Deployments(oc.Namespace).Create(context.Background(), deploy, v1.CreateOptions{})
	// sink already exists errors
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	c.RecordCreateEvent(oc, deploy, err)
	return err
}

func (c *realDeploymentControl) UpdateDeployment(oc *v1alpha1.OnecloudCluster, deploy *apps.Deployment) (*apps.Deployment, error) {
	ns := oc.GetNamespace()
	ocName := oc.GetName()
	deployName := deploy.GetName()
	deploySpec := deploy.Spec.DeepCopy()
	var updatedDeploy *apps.Deployment

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var updateErr error
		updatedDeploy, updateErr = c.kubeCli.AppsV1().Deployments(ns).Update(context.Background(), deploy, v1.UpdateOptions{})
		if updateErr == nil {
			klog.Infof("OnecloudCluster: [%s/%s]'s Deployment: [%s/%s] updated successfully", ns, ocName, ns, deployName)
			return nil
		}
		klog.Errorf("failed to update OnecloudCluster: [%s/%s]'s Deployment: [%s/%s], error: %v", ns, ocName, ns, deployName, updateErr)

		if updated, err := c.deployLister.Deployments(ns).Get(deployName); err == nil {
			// make a copy so we don't mutate the shared cache
			deploy = updated.DeepCopy()
			deploy.Spec = *deploySpec
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated Deployment %s/%s from lister: %v", ns, deployName, err))
		}
		return updateErr
	})

	c.RecordUpdateEvent(oc, deploy, err)
	return updatedDeploy, err
}

func (c *realDeploymentControl) DeleteDeployment(oc *v1alpha1.OnecloudCluster, deploymentName string) error {
	err := c.kubeCli.AppsV1().Deployments(oc.Namespace).Delete(context.Background(), deploymentName, v1.DeleteOptions{})
	c.RecordDeleteEvent(oc, newFakeObject(deploymentName), err)
	return err
}
