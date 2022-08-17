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

package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/client/clientset/versioned"
	informers "yunion.io/x/onecloud-operator/pkg/client/informers/externalversions"
	"yunion.io/x/onecloud-operator/pkg/controller"
	occluster "yunion.io/x/onecloud-operator/pkg/controller/cluster"
	k8sutil "yunion.io/x/onecloud-operator/pkg/util/k8s"
	k8sutil2 "yunion.io/x/onecloud-operator/pkg/util/k8sutil"
	"yunion.io/x/onecloud-operator/pkg/version"
)

var (
	printVersion         bool
	enableLeaderElection bool
	workers              int
	leaseDuration        = 15 * time.Second
	renewDuration        = 5 * time.Second
	retryPeriod          = 3 * time.Second
	resyncDuration       = 30 * time.Second
	waitDuration         = 5 * time.Second
)

func init() {
	flag.IntVar(&workers, "workers", 5, "The number of workers that are allowed to sync concurrently.")
	flag.BoolVar(&printVersion, "V", false, "Show version and quit")
	flag.BoolVar(&printVersion, "version", false, "Show version and quit")
	flag.BoolVar(&controller.SessionDebug, "debug", false, "Onecloud session debug")
	flag.BoolVar(&controller.SyncUser, "sync-user", false, "Operator sync onecloud user password if changed")
	flag.BoolVar(&controller.DisableInitCRD, "disable-init-crd", false, "Disable CRD initialization")
	flag.BoolVar(&controller.DisableNodeSelectorController, "disable-node-selector-controller", false, "Ignore onecloud.yunion.io/controller node selector")
	flag.BoolVar(&controller.DisableSyncIngress, "disable-sync-ingress", false, "Disable ingress resource syncing")
	flag.BoolVar(&controller.UseRandomServicePort, "use-random-service-port", false, "Use random service node port")
	flag.BoolVar(&controller.EtcdKeepFailedPods, "etcd-keep-failed-pods", false, "Keep the failed etcd pods")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for multiple instances")

	flag.Parse()
}

func main() {
	if printVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	rand.Seed(time.Now().UnixNano())
	klog.InitFlags(nil)

	hostName, err := os.Hostname()
	if err != nil {
		klog.Fatalf("failed to get hostname: %v", err)
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("failed to get config: %v", err)
	}

	ns := os.Getenv("NAMESPACE")
	if ns == "" {
		klog.Fatal("NAMESPACE environment variable not set")
	}

	cli, err := versioned.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("failed to create Clientset: %v", err)
	}
	kubeCli, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("failed to create kubernetes Clientset: %v", err)
	}
	kubeExtCli := k8sutil.MustNewKubeExtClient()

	dynamicCli, err := dynamic.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("failed to create kubernetes dynamic client: %v", err)
	}

	var informerFactory informers.SharedInformerFactory
	var kubeInformerFactory kubeinformers.SharedInformerFactory

	options := []informers.SharedInformerOption{
		/* Operator need to care about other namespaces
		 * informers.WithNamespace(ns),
		 */
	}
	informerFactory = informers.NewSharedInformerFactoryWithOptions(cli, resyncDuration, options...)

	kubeOptions := []kubeinformers.SharedInformerOption{
		/* Operator need to care about other namespaces
		 * kubeinformers.WithNamespace(ns),
		 */
	}
	kubeInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(kubeCli, resyncDuration, kubeOptions...)

	dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicCli, resyncDuration)

	rl := resourcelock.EndpointsLock{
		EndpointsMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "onecloud-controller-manager",
		},
		Client: kubeCli.CoreV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      hostName,
			EventRecorder: &record.FakeRecorder{},
		},
	}

	cv, err := k8sutil2.NewClusterVersion(kubeCli)
	if err != nil {
		klog.Fatalf("NewClusterVersion: %v", err)
	}

	ocController, err := occluster.NewController(kubeCli, kubeExtCli, dynamicCli, cli, informerFactory, kubeInformerFactory, dynamicInformerFactory, cv)
	if err != nil {
		klog.Fatalf("NewController: %v", err)
	}

	if !controller.DisableInitCRD {
		if err := ocController.InitCRDResource(); err != nil {
			klog.Fatalf("init CRD resources: %v", err)
		}
	}

	controllerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go informerFactory.Start(controllerCtx.Done())
	go kubeInformerFactory.Start(controllerCtx.Done())
	go dynamicInformerFactory.Start(controllerCtx.Done())

	if enableLeaderElection {
		// leader election for multiple onecloud-controller-manager
		onStarted := func(ctx context.Context) {
			ocController.Run(workers, ctx.Done())
		}
		onStopped := func() {
			klog.Fatalf("leader election lost")
		}
		go wait.Forever(func() {
			leaderelection.RunOrDie(controllerCtx, leaderelection.LeaderElectionConfig{
				Lock:          &rl,
				LeaseDuration: leaseDuration,
				RenewDeadline: renewDuration,
				RetryPeriod:   retryPeriod,
				Callbacks: leaderelection.LeaderCallbacks{
					OnStartedLeading: onStarted,
					OnStoppedLeading: onStopped,
				},
			})
		}, waitDuration)

	} else {
		go func() {
			ch := make(chan struct{})
			ocController.Run(workers, ch)
		}()
	}

	klog.Fatal(http.ListenAndServe(":6060", nil))
}
