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

type DaemonSetControlInterface interface {
	CreateDaemonSet(*v1alpha1.OnecloudCluster, *apps.DaemonSet) error
	UpdateDaemonSet(*v1alpha1.OnecloudCluster, *apps.DaemonSet) (*apps.DaemonSet, error)
	DeleteDaemonSet(*v1alpha1.OnecloudCluster, *apps.DaemonSet) error
}

type daemonSetControl struct {
	*baseControl
	kubeCli         kubernetes.Interface
	daemonSetLister appslisters.DaemonSetLister
}

func NewDaemonSetControl(
	kubeCli kubernetes.Interface, daemonSetLister appslisters.DaemonSetLister, recorder record.EventRecorder,
) DaemonSetControlInterface {
	return &daemonSetControl{newBaseControl("DaemonSet", recorder), kubeCli, daemonSetLister}
}

func (c *daemonSetControl) CreateDaemonSet(oc *v1alpha1.OnecloudCluster, ds *apps.DaemonSet) error {
	_, err := c.kubeCli.AppsV1().DaemonSets(oc.Namespace).Create(context.Background(), ds, v1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	c.RecordCreateEvent(oc, ds, err)
	return err
}

func (c *daemonSetControl) DeleteDaemonSet(oc *v1alpha1.OnecloudCluster, ds *apps.DaemonSet) error {
	err := c.kubeCli.AppsV1().DaemonSets(oc.Namespace).Delete(context.Background(), ds.Name, v1.DeleteOptions{})
	c.RecordDeleteEvent(oc, ds, err)
	return err
}

func (c *daemonSetControl) UpdateDaemonSet(oc *v1alpha1.OnecloudCluster, ds *apps.DaemonSet) (*apps.DaemonSet, error) {
	var (
		ns       = oc.GetNamespace()
		ocName   = oc.GetName()
		dsName   = ds.GetName()
		dsSpec   = ds.Spec.DeepCopy()
		updateDs *apps.DaemonSet
	)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var updateErr error
		updateDs, updateErr = c.kubeCli.AppsV1().DaemonSets(ns).Update(context.Background(), ds, v1.UpdateOptions{})
		if updateErr == nil {
			klog.Infof("OnecloudCluster: [%s/%s]'s DaemonSet: [%s/%s] updated successfully", ns, ocName, ns, dsName)
			return nil
		}
		klog.Errorf("Failed to update OnecloudCluster: [%s/%s]'s DaemonSet: [%s/%s], error: %v", ns, ocName, ns, dsName, updateErr)
		if updated, err := c.daemonSetLister.DaemonSets(ns).Get(dsName); err == nil {
			ds = updated.DeepCopy()
			ds.Spec = *dsSpec
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated DaemonSet %s/%s from lister: %v", ns, dsName, err))
		}
		return updateErr
	})

	c.RecordUpdateEvent(oc, ds, err)
	return updateDs, err
}
