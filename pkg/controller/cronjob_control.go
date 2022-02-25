package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/listers/batch/v1beta1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

type CronJobControlInterface interface {
	CreateCronJob(*v1alpha1.OnecloudCluster, *batchv1.CronJob) error
	UpdateCronJob(*v1alpha1.OnecloudCluster, *batchv1.CronJob) (*batchv1.CronJob, error)
	DeleteCronJob(*v1alpha1.OnecloudCluster, string) error
}

type cronJobControl struct {
	*baseControl
	kubeCli       kubernetes.Interface
	cronJobLister v1beta1.CronJobLister
}

func NewCronJobControl(
	kubeCli kubernetes.Interface, cronJobLister v1beta1.CronJobLister, recorder record.EventRecorder,
) CronJobControlInterface {
	return &cronJobControl{newBaseControl("CronJob", recorder), kubeCli, cronJobLister}
}

func (c *cronJobControl) CreateCronJob(
	oc *v1alpha1.OnecloudCluster, cronJob *batchv1.CronJob,
) error {
	_, err := c.kubeCli.BatchV1beta1().CronJobs(oc.Namespace).Create(context.Background(), cronJob, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	c.RecordCreateEvent(oc, cronJob, err)
	return err
}

func (c *cronJobControl) UpdateCronJob(
	oc *v1alpha1.OnecloudCluster, cronJob *batchv1.CronJob,
) (*batchv1.CronJob, error) {
	var ns = oc.GetNamespace()
	var ocName = oc.GetName()
	var cronJobName = cronJob.GetName()
	var newCronJob *batchv1.CronJob
	var cronJobSpec = cronJob.Spec.DeepCopy()
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var updateErr error
		newCronJob, updateErr = c.kubeCli.BatchV1beta1().CronJobs(ns).Update(context.Background(), cronJob, metav1.UpdateOptions{})
		if updateErr == nil {
			klog.Infof("OnecloudCluster: [%s/%s]'s CronJob: [%s/%s] updated successfully", ns, ocName, ns, cronJobName)
			return nil
		}
		klog.Errorf("Failed to update OnecloudCluster: [%s/%s]'s CronJob: [%s/%s], error: %v",
			ns, ocName, ns, cronJobName, updateErr)
		if updated, err := c.cronJobLister.CronJobs(ns).Get(cronJobName); err == nil {
			cronJob = updated.DeepCopy()
			cronJob.Spec = *cronJobSpec
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated CronJob %s/%s from lister: %v", ns, cronJobName, err))
		}

		return updateErr
	})

	c.RecordUpdateEvent(oc, cronJob, err)
	return newCronJob, err
}

func (c *cronJobControl) DeleteCronJob(
	oc *v1alpha1.OnecloudCluster, cronJobName string,
) error {
	err := c.kubeCli.BatchV1beta1().CronJobs(oc.Namespace).Delete(context.Background(), cronJobName, metav1.DeleteOptions{})
	c.RecordDeleteEvent(oc, newFakeObject(cronJobName), err)
	return err
}
