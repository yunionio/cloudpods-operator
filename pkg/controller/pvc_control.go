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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

type PVCControlInterface interface {
	CreatePVC(*v1alpha1.OnecloudCluster, *corev1.PersistentVolumeClaim) error
	UpdatePVC(*v1alpha1.OnecloudCluster, *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error)
	DeletePVC(*v1alpha1.OnecloudCluster, *corev1.PersistentVolumeClaim) error
}

type realPVCControl struct {
	*baseControl
	kubeCli   kubernetes.Interface
	pvcLister corelisters.PersistentVolumeClaimLister
}

func NewPVCControl(kubeCli kubernetes.Interface, pvcLister corelisters.PersistentVolumeClaimLister, recorder record.EventRecorder) PVCControlInterface {
	return &realPVCControl{
		baseControl: newBaseControl("PersistentVolumeClaim", recorder),
		kubeCli:     kubeCli,
		pvcLister:   pvcLister,
	}
}

func (c *realPVCControl) CreatePVC(oc *v1alpha1.OnecloudCluster, pvc *corev1.PersistentVolumeClaim) error {
	_, err := c.kubeCli.CoreV1().PersistentVolumeClaims(oc.Namespace).Create(context.Background(), pvc, v1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	c.RecordCreateEvent(oc, pvc, err)
	return err
}

func (c *realPVCControl) DeletePVC(oc *v1alpha1.OnecloudCluster, pvc *corev1.PersistentVolumeClaim) error {
	// TODO
	return nil
}

func (c *realPVCControl) UpdatePVC(oc *v1alpha1.OnecloudCluster, pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	// TODO
	return nil, nil
}
