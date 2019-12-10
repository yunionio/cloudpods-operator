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

package cluster

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	eventv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"yunion.io/x/onecloud-operator/pkg/apis/crds"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/scheme"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/client/clientset/versioned"
	informers "yunion.io/x/onecloud-operator/pkg/client/informers/externalversions"
	listers "yunion.io/x/onecloud-operator/pkg/client/listers/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager/certs"
	"yunion.io/x/onecloud-operator/pkg/manager/component"
	"yunion.io/x/onecloud-operator/pkg/manager/config"
	k8sutil "yunion.io/x/onecloud-operator/pkg/util/k8s"
)

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = v1alpha1.SchemeGroupVersion.WithKind("OnecloudCluster")

// Controller controls onecloudclusters.
type Controller struct {
	// kubernetes client interface
	kubeClient kubernetes.Interface
	// kubernetes extension client interface
	kubeExtCli apiextensionsclient.Interface
	// operator client interface
	cli versioned.Interface
	// control returns an interface capable of syncing a onecloud cluster.
	control ControlInterface
	// ocLister is able to list/get onecloud clusters from a shared informer's store
	ocLister listers.OnecloudClusterLister
	// ocListerSynced returns true if the onecloud cluster shared informer has synced at least once
	ocListerSynced cache.InformerSynced
	// deploymentLister is able to list/get deployment sets from a shared informer's store
	deploymentLister appslisters.DeploymentLister
	// deploymentListerSynced returns true if the deployment shared informer has synced at least once
	deploymentListerSynced cache.InformerSynced
	// clusters that need to be synced
	queue workqueue.RateLimitingInterface
}

// NewController creates a onecloudcluster controller
func NewController(
	kubeCli kubernetes.Interface,
	kubeExtCli apiextensionsclient.Interface,
	cli versioned.Interface,
	informerFactory informers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
) *Controller {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&eventv1.EventSinkImpl{
		Interface: eventv1.New(kubeCli.CoreV1().RESTClient()).Events("")})
	recorder := eventBroadcaster.NewRecorder(v1alpha1.Scheme, corev1.EventSource{Component: "onecloudcluster"})

	ocInformer := informerFactory.Onecloud().V1alpha1().OnecloudClusters()
	deployInformer := kubeInformerFactory.Apps().V1().Deployments()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	secretInformer := kubeInformerFactory.Core().V1().Secrets()
	//epsInformer := kubeInformerFactory.Core().V1().Endpoints()
	//podInformer := kubeInformerFactory.Core().V1().Pods()
	//nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	cfgInformer := kubeInformerFactory.Core().V1().ConfigMaps()
	pvcInformer := kubeInformerFactory.Core().V1().PersistentVolumeClaims()
	ingInformer := kubeInformerFactory.Extensions().V1beta1().Ingresses()
	dsInformer := kubeInformerFactory.Apps().V1().DaemonSets()

	ocControl := controller.NewClusterControl(cli, ocInformer.Lister(), recorder)
	deployControl := controller.NewDeploymentControl(kubeCli, deployInformer.Lister(), recorder)
	svcControl := controller.NewServiceControl(kubeCli, svcInformer.Lister(), recorder)
	cfgControl := controller.NewConfigMapControl(kubeCli, cfgInformer.Lister(), recorder)
	ingControl := controller.NewIngressControl(kubeCli, ingInformer.Lister(), recorder)
	dsControl := controller.NewDaemonSetControl(kubeCli, dsInformer.Lister(), recorder)

	configer := config.NewConfigManager(cfgControl, cfgInformer.Lister())
	certControl := controller.NewOnecloudCertControl(kubeCli, secretInformer.Lister(), recorder)
	onecloudControl := controller.NewOnecloudControl(kubeCli)
	pvcControl := controller.NewPVCControl(kubeCli, pvcInformer.Lister(), recorder)

	componentsMan := component.NewComponentManager(
		deployControl, deployInformer.Lister(),
		svcControl, svcInformer.Lister(),
		pvcControl, pvcInformer.Lister(),
		ingControl, ingInformer.Lister(),
		dsControl, dsInformer.Lister(),
		configer, onecloudControl)

	c := &Controller{
		kubeClient: kubeCli,
		kubeExtCli: kubeExtCli,
		cli:        cli,
		control: NewDefaultOnecloudClusterControl(
			ocControl,
			configer,
			certs.NewCertsManager(certControl, secretInformer.Lister()),
			componentsMan,
			recorder,
		),
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"onecloudcluster",
		),
	}

	ocInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueCluster,
		UpdateFunc: func(old, cur interface{}) {
			c.enqueueCluster(cur)
		},
		DeleteFunc: c.enqueueCluster,
	})
	c.ocLister = ocInformer.Lister()
	c.ocListerSynced = ocInformer.Informer().HasSynced

	return c
}

// Run runs the onecloud cluster controller.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting onecloud cluster controller")
	defer klog.Info("Shutting down onecloud cluster controller")

	if !cache.WaitForCacheSync(stopCh, c.ocListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

// worker runs a worker goroutine that invokes processNextWorkItem until the controller's queue is closed
func (c *Controller) worker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem dequeues items, processes them, and marks them done. It enforces that the syncHandler is never
// invoked concurrently with the same key.
func (c *Controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	if err := c.sync(key.(string)); err != nil {
		utilruntime.HandleError(fmt.Errorf("OnecloudCluster: %v, sync failed %v, requeuing", key.(string), err))
		c.queue.AddRateLimited(key)
	} else {
		c.queue.Forget(key)
	}
	return true
}

// sync syncs the given onecloud cluster
func (c *Controller) sync(key string) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing OnecloudCluster %q (%v)", key, time.Since(startTime))
	}()

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	oc, err := c.ocLister.OnecloudClusters(ns).Get(name)
	if errors.IsNotFound(err) {
		klog.Infof("OnecloudCluster has been deleted %v", key)
		return nil
	}
	if err != nil {
		return err
	}

	tmpOc := oc.DeepCopy()
	scheme.Scheme.Default(tmpOc)

	return c.syncCluster(tmpOc)
}

func (c *Controller) syncCluster(oc *v1alpha1.OnecloudCluster) error {
	return c.control.UpdateOnecloudCluster(oc)
}

// enqueueCluster enqueues the given onecloud cluster in the work queue.
func (c *Controller) enqueueCluster(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", obj, err))
		return
	}
	c.queue.Add(key)
}

func (c *Controller) InitCRDResource() error {
	err := k8sutil.CreateOrUpdateCRD(c.kubeExtCli, crds.OnecloudClusterCRD)
	if err != nil {
		return fmt.Errorf("failed to create CRD: %v", err)
	}
	return k8sutil.WaitCRDReady(c.kubeExtCli, v1alpha1.OnecloudClusterCRDName)
}
