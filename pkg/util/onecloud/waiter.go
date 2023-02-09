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

package onecloud

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"yunion.io/x/onecloud/pkg/mcclient"
	compute_modules "yunion.io/x/onecloud/pkg/mcclient/modules/compute"
	identity_modules "yunion.io/x/onecloud/pkg/mcclient/modules/identity"
	image_modules "yunion.io/x/onecloud/pkg/mcclient/modules/image"
	scheduler_modules "yunion.io/x/onecloud/pkg/mcclient/modules/scheduler"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/util/apiclient"
)

type Waiter interface {
	apiclient.Waiter

	WaitForServicePods(serviceName string) error
	WaitForKeystone() error
	WaitForRegion() error
	WaitForScheduler() error
	WaitForGlance() error
}

type OCWaiter struct {
	apiclient.Waiter
	kubeClient clientset.Interface

	sessionFactory func() (*mcclient.ClientSession, error)
	timeout        time.Duration
	writer         io.Writer
}

// NewOCWaiter returns a new Onecloud waiter object that check service healthy
func NewOCWaiter(
	kubeClient clientset.Interface,
	sessionFactory func() (*mcclient.ClientSession, error),
	timeout time.Duration,
	writer io.Writer,
) Waiter {
	return &OCWaiter{
		Waiter:         apiclient.NewKubeWaiter(kubeClient, timeout, writer),
		kubeClient:     kubeClient,
		sessionFactory: sessionFactory,
		timeout:        timeout,
		writer:         writer,
	}
}

func (w *OCWaiter) getSession() (*mcclient.ClientSession, error) {
	return w.sessionFactory()
}

func (w *OCWaiter) WaitForServicePods(serviceName string) error {
	if err := w.WaitForPodsWithLabel("component=" + serviceName); err != nil {
		return errors.Wrapf(err, "wait %s pod running", serviceName)
	}
	return nil
}

// WaitForPodsWithLabel will lookup pods with the given label and wait until they are all
// reporting status as running.
func (w *OCWaiter) WaitForPodsWithLabel(kvLabel string) error {

	lastKnownPodNumber := -1
	return wait.PollImmediate(constants.APICallRetryInterval, w.timeout, func() (bool, error) {
		listOpts := metav1.ListOptions{LabelSelector: kvLabel}
		pods, err := w.kubeClient.CoreV1().Pods(metav1.NamespaceSystem).List(context.Background(), listOpts)
		if err != nil {
			fmt.Fprintf(w.writer, "[apiclient] Error getting Pods with label selector %q [%v]\n", kvLabel, err)
			return false, nil
		}

		if lastKnownPodNumber != len(pods.Items) {
			fmt.Fprintf(w.writer, "[apiclient] Found %d Pods for label selector %s\n", len(pods.Items), kvLabel)
			lastKnownPodNumber = len(pods.Items)
		}

		if len(pods.Items) == 0 {
			return false, nil
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase != v1.PodRunning {
				return false, nil
			}
		}

		return true, nil
	})
}

func (w *OCWaiter) WaitForKeystone() error {
	start := time.Now()
	return wait.PollImmediate(constants.APICallRetryInterval, w.timeout, func() (bool, error) {
		session, err := w.getSession()
		w.timeout.Seconds()
		if err != nil {
			duration := time.Since(start).Seconds()
			if (duration + float64(10*time.Second)) > w.timeout.Seconds() {
				fmt.Fprintf(w.writer, "[keystone] Error get auth session: %v", err)
			}
			return false, nil
		}
		if _, err := identity_modules.Policies.List(session, nil); err != nil {
			return false, errors.Wrap(err, "Failed to get policy")
		}
		fmt.Printf("[keystone] healthy after %f seconds\n", time.Since(start).Seconds())
		return true, nil
	})
}

func (w *OCWaiter) waitForServiceHealthy(serviceName string, checkFunc func(*mcclient.ClientSession) (bool, error)) error {
	start := time.Now()
	return wait.PollImmediate(constants.APICallRetryInterval, w.timeout, func() (bool, error) {
		session, err := w.getSession()
		if err != nil {
			return false, errors.Errorf("Failed to get onecloud session: %v", session)
		}
		ok, err := checkFunc(session)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		fmt.Printf("[%s] healthy after %f seconds\n", serviceName, time.Since(start).Seconds())
		return true, nil
	})
}

func (w *OCWaiter) WaitForRegion() error {
	return w.waitForServiceHealthy(constants.ServiceNameRegionV2, func(s *mcclient.ClientSession) (bool, error) {
		_, err := compute_modules.Servers.List(s, nil)
		if err == nil {
			return true, nil
		}
		klog.V(1).Infof("region list servers error %v", err)
		return false, nil
	})
}

func (w *OCWaiter) WaitForScheduler() error {
	return w.waitForServiceHealthy(constants.ServiceNameScheduler, func(s *mcclient.ClientSession) (bool, error) {
		_, err := scheduler_modules.SchedManager.HistoryList(s, nil)
		if err == nil {
			return true, nil
		}
		klog.V(1).Infof("scheduler list history error %v", err)
		return false, nil
	})
}

func (w *OCWaiter) WaitForGlance() error {
	return w.waitForServiceHealthy(constants.ServiceNameGlance, func(s *mcclient.ClientSession) (bool, error) {
		_, err := image_modules.Images.List(s, nil)
		if err == nil {
			return true, nil
		}
		klog.V(1).Infof("image list servers error %v", err)
		return false, nil
	})
}
