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

package component

import (
	"fmt"

	errorswrap "github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	jobbatchv1 "k8s.io/api/batch/v1"
	batchv1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appv1 "k8s.io/client-go/listers/apps/v1"
	batchlisters "k8s.io/client-go/listers/batch/v1beta1"
	corelisters "k8s.io/client-go/listers/core/v1"
	extensionlisters "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/klog"

	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud-operator/pkg/label"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type ComponentManager struct {
	kubeCli kubernetes.Interface

	deployControl controller.DeploymentControlInterface
	deployLister  appv1.DeploymentLister
	svcControl    controller.ServiceControlInterface
	svcLister     corelisters.ServiceLister
	pvcControl    controller.PVCControlInterface
	pvcLister     corelisters.PersistentVolumeClaimLister
	ingControl    controller.IngressControlInterface
	ingLister     extensionlisters.IngressLister
	dsControl     controller.DaemonSetControlInterface
	dsLister      appv1.DaemonSetLister
	cronControl   controller.CronJobControlInterface
	cronLister    batchlisters.CronJobLister
	nodeLister    corelisters.NodeLister

	configer        Configer
	onecloudControl *controller.OnecloudControl
}

// NewComponentManager return *BaseComponentManager
func NewComponentManager(
	kubeCli kubernetes.Interface,
	deployCtrol controller.DeploymentControlInterface,
	deployLister appv1.DeploymentLister,
	svcControl controller.ServiceControlInterface,
	svcLister corelisters.ServiceLister,
	pvcControl controller.PVCControlInterface,
	pvcLister corelisters.PersistentVolumeClaimLister,
	ingControl controller.IngressControlInterface,
	ingLister extensionlisters.IngressLister,
	dsControl controller.DaemonSetControlInterface,
	dsLister appv1.DaemonSetLister,
	cronControl controller.CronJobControlInterface,
	cronLister batchlisters.CronJobLister,
	nodeLister corelisters.NodeLister,
	configer Configer,
	onecloudControl *controller.OnecloudControl,
) *ComponentManager {
	return &ComponentManager{
		kubeCli:         kubeCli,
		deployControl:   deployCtrol,
		deployLister:    deployLister,
		svcControl:      svcControl,
		svcLister:       svcLister,
		pvcControl:      pvcControl,
		pvcLister:       pvcLister,
		ingControl:      ingControl,
		ingLister:       ingLister,
		dsControl:       dsControl,
		dsLister:        dsLister,
		cronControl:     cronControl,
		cronLister:      cronLister,
		nodeLister:      nodeLister,
		configer:        configer,
		onecloudControl: onecloudControl,
	}
}

func (m *ComponentManager) syncService(
	oc *v1alpha1.OnecloudCluster,
	svcFactory func(*v1alpha1.OnecloudCluster) *corev1.Service,
) error {
	ns := oc.GetNamespace()
	newSvc := svcFactory(oc)
	if newSvc == nil {
		return nil
	}
	oldSvcTmp, err := m.svcLister.Services(ns).Get(newSvc.GetName())
	if errors.IsNotFound(err) {
		err = SetServiceLastAppliedConfigAnnotation(newSvc)
		if err != nil {
			return err
		}
		return m.svcControl.CreateService(oc, newSvc)
	}
	if err != nil {
		return err
	}

	oldSvc := oldSvcTmp.DeepCopy()

	equal, err := serviceEqual(newSvc, oldSvc)
	if err != nil {
		return err
	}
	if !equal {
		svc := *oldSvc
		svc.Spec = newSvc.Spec
		svc.Spec.ClusterIP = oldSvc.Spec.ClusterIP
		err = SetServiceLastAppliedConfigAnnotation(&svc)
		if err != nil {
			return err
		}
		_, err = m.svcControl.UpdateService(oc, &svc)
		return err
	}

	return nil
}

func (m *ComponentManager) syncIngress(
	oc *v1alpha1.OnecloudCluster,
	ingFactory func(*v1alpha1.OnecloudCluster) *extensions.Ingress,
) error {
	ns := oc.GetNamespace()
	newIng := ingFactory(oc)
	if newIng == nil {
		return nil
	}
	oldIngTmp, err := m.ingLister.Ingresses(ns).Get(newIng.GetName())
	if errors.IsNotFound(err) {
		err = SetIngressLastAppliedConfigAnnotation(newIng)
		if err != nil {
			return err
		}
		return m.ingControl.CreateIngress(oc, newIng)
	}
	if err != nil {
		return err
	}

	oldIng := oldIngTmp.DeepCopy()

	equal, err := ingressEqual(newIng, oldIng)
	if err != nil {
		return err
	}
	if !equal {
		ing := *oldIng
		ing.Spec = newIng.Spec
		err = SetIngressLastAppliedConfigAnnotation(&ing)
		if err != nil {
			return err
		}
		_, err = m.ingControl.UpdateIngress(oc, &ing)
		return err
	}
	return nil
}

func (m *ComponentManager) syncConfigMap(
	oc *v1alpha1.OnecloudCluster,
	dbConfigFactory func(*v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig,
	svcAccountFactory func(*v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser,
	cfgMapFactory func(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error),
) error {
	clustercfg, err := m.configer.GetClusterConfig(oc)
	if err != nil {
		return errorswrap.Wrap(err, "get cluster config")
	}
	if dbConfigFactory != nil {
		dbConfig := dbConfigFactory(clustercfg)
		if dbConfig != nil {
			if err := EnsureClusterDBUser(oc, *dbConfig); err != nil {
				return errorswrap.Wrap(err, "ensure cluster db user")
			}
		}
	}
	if svcAccountFactory != nil {
		account := svcAccountFactory(clustercfg)
		if account != nil {
			s, err := m.onecloudControl.GetSession(oc)
			if err != nil {
				return errorswrap.Wrap(err, "get cloud session")
			}
			if err := EnsureServiceAccount(s, *account); err != nil {
				return errorswrap.Wrapf(err, "ensure service account %#v", *account)
			}
		}
	}
	cfgMap, err := cfgMapFactory(oc, clustercfg)
	if err != nil {
		return err
	}
	if cfgMap == nil {
		return nil
	}
	if err := SetConfigMapLastAppliedConfigAnnotation(cfgMap); err != nil {
		return err
	}
	oldCfgMap, _ := m.configer.Lister().ConfigMaps(oc.GetNamespace()).Get(cfgMap.GetName())
	if oldCfgMap != nil {
		if equal, err := configMapEqual(cfgMap, oldCfgMap); err != nil {
			return err
		} else if equal {
			return nil
		}
	}
	return m.configer.CreateOrUpdateConfigMap(oc, cfgMap)
}

func (m *ComponentManager) syncDaemonSet(
	oc *v1alpha1.OnecloudCluster,
	dsFactory func(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*apps.DaemonSet, error),
) error {
	ns := oc.GetNamespace()
	cfg, err := m.configer.GetClusterConfig(oc)
	if err != nil {
		return err
	}
	newDs, err := dsFactory(oc, cfg)
	if err != nil {
		return err
	}
	if newDs == nil {
		return nil
	}
	oldDsTmp, err := m.dsLister.DaemonSets(ns).Get(newDs.GetName())
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if errors.IsNotFound(err) {
		return m.dsControl.CreateDaemonSet(oc, newDs)
	}
	oldDs := oldDsTmp.DeepCopy()
	if err = m.updateDaemonSet(oc, newDs, oldDs); err != nil {
		return err
	}
	return nil
}

func (m *ComponentManager) updateDaemonSet(oc *v1alpha1.OnecloudCluster, newDs, oldDs *apps.DaemonSet) error {
	if !daemonSetEqual(newDs, oldDs) {
		ds := *oldDs
		ds.Spec.Template = newDs.Spec.Template
		ds.Spec.UpdateStrategy = newDs.Spec.UpdateStrategy
		err := SetDaemonSetLastAppliedConfigAnnotation(&ds)
		if err != nil {
			return err
		}
		_, err = m.dsControl.UpdateDaemonSet(oc, &ds)
		return err
	}
	return nil
}

func (m *ComponentManager) syncCronJob(
	oc *v1alpha1.OnecloudCluster,
	cronFactory func(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*batchv1.CronJob, error),
) error {
	cfg, err := m.configer.GetClusterConfig(oc)
	if err != nil {
		return err
	}
	newCronJob, err := cronFactory(oc, cfg)
	if err != nil {
		return err
	}
	if newCronJob == nil {
		return nil
	}
	oldCronJob, err := m.cronLister.CronJobs(oc.GetNamespace()).Get(newCronJob.GetName())
	if err != nil && !errors.IsNotFound(err) {
		return errorswrap.Wrap(err, "sync cronjob on list")
	}
	if errors.IsNotFound(err) {
		return m.cronControl.CreateCronJob(oc, newCronJob)
	}
	_oldCronJob := oldCronJob.DeepCopy()
	if err = m.updateCronJob(oc, newCronJob, _oldCronJob); err != nil {
		return err
	}
	return nil
}

func (m *ComponentManager) updateCronJob(oc *v1alpha1.OnecloudCluster, newCronJob, oldCronJob *batchv1.CronJob) error {
	if !cronJobEqual(newCronJob, oldCronJob) {
		klog.Infof("start update cron job %s", newCronJob.GetName())
		cronJob := *oldCronJob
		cronJob.Spec = newCronJob.Spec
		err := SetCronJobLastAppliedConfigAnnotation(&cronJob)
		if err != nil {
			return err
		}
		_, err = m.cronControl.UpdateCronJob(oc, &cronJob)
		if err != nil {
			return errorswrap.Wrap(err, "update cron job")
		}
		return nil
	}
	return nil
}

func (m *ComponentManager) syncDeployment(
	oc *v1alpha1.OnecloudCluster,
	deploymentFactory func(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error),
	postSyncFunc func(*v1alpha1.OnecloudCluster, *apps.Deployment) error,
) error {
	ns := oc.GetNamespace()
	cfg, err := m.configer.GetClusterConfig(oc)
	if err != nil {
		return err
	}
	newDeploy, err := deploymentFactory(oc, cfg)
	if err != nil {
		return err
	}
	if newDeploy == nil {
		return nil
	}

	oldDeployTmp, err := m.deployLister.Deployments(ns).Get(newDeploy.GetName())
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	postFunc := func(deploy *apps.Deployment) error {
		if postSyncFunc != nil {
			deploy, err := m.kubeCli.AppsV1().Deployments(deploy.GetNamespace()).Get(deploy.GetName(), metav1.GetOptions{})
			if err != nil {
				return errorswrap.Wrapf(err, "get deployment %s", deploy.GetName())
			}
			return postSyncFunc(oc, deploy)
		}
		return nil
	}

	if errors.IsNotFound(err) {
		if err = SetDeploymentLastAppliedConfigAnnotation(newDeploy); err != nil {
			return err
		}
		if err = m.deployControl.CreateDeployment(oc, newDeploy); err != nil {
			return err
		}
		if err := postFunc(newDeploy); err != nil {
			return errorswrap.Wrapf(err, "post create deployment %s", newDeploy.GetName())
		}
		return controller.RequeueErrorf("OnecloudCluster: [%s/%s], waiting for %s deployment running", ns, oc.GetName(), newDeploy.GetName())
	}

	oldDeploy := oldDeployTmp.DeepCopy()

	if err = m.updateDeployment(oc, newDeploy, oldDeploy); err != nil {
		return err
	}
	return postFunc(oldDeploy)
}

func (m *ComponentManager) updateDeployment(oc *v1alpha1.OnecloudCluster, newDeploy, oldDeploy *apps.Deployment) error {
	if !deploymentEqual(*newDeploy, *oldDeploy) {
		deploy := *oldDeploy
		deploy.Spec.Template = newDeploy.Spec.Template
		*deploy.Spec.Replicas = *newDeploy.Spec.Replicas
		deploy.Spec.Strategy = newDeploy.Spec.Strategy
		err := SetDeploymentLastAppliedConfigAnnotation(&deploy)
		if err != nil {
			return err
		}
		_, err = m.deployControl.UpdateDeployment(oc, &deploy)
		return err
	}
	return nil
}

func (m *ComponentManager) newDefaultDeployment(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	volHelper *VolumeHelper,
	spec v1alpha1.DeploymentSpec,
	initContainersFactory func([]corev1.VolumeMount) []corev1.Container,
	containersFactory func([]corev1.VolumeMount) []corev1.Container,
) (*apps.Deployment, error) {
	return m.newDefaultDeploymentWithCloudAffinity(componentType, oc, volHelper, spec, initContainersFactory, containersFactory)
}

func (m *ComponentManager) newDefaultDeploymentWithoutCloudAffinity(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	volHelper *VolumeHelper,
	spec v1alpha1.DeploymentSpec,
	initContainersFactory func([]corev1.VolumeMount) []corev1.Container,
	containersFactory func([]corev1.VolumeMount) []corev1.Container,
) (*apps.Deployment, error) {
	return m.newDeployment(componentType, oc, volHelper, spec, initContainersFactory, containersFactory, false, corev1.DNSClusterFirst)
}

func (m *ComponentManager) newDefaultDeploymentWithCloudAffinity(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	volHelper *VolumeHelper,
	spec v1alpha1.DeploymentSpec,
	initContainersFactory func([]corev1.VolumeMount) []corev1.Container,
	containersFactory func([]corev1.VolumeMount) []corev1.Container,
) (*apps.Deployment, error) {
	if spec.Affinity == nil {
		spec.Affinity = &corev1.Affinity{}
	}
	if spec.Affinity.NodeAffinity == nil {
		spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
	}
	spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = []corev1.PreferredSchedulingTerm{
		corev1.PreferredSchedulingTerm{
			Weight: 1,
			Preference: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					corev1.NodeSelectorRequirement{
						Key:      constants.OnecloudControllerLabelKey,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"enable"},
					},
				},
			},
		},
	}
	return m.newDeployment(componentType, oc, volHelper, spec, initContainersFactory, containersFactory, false, corev1.DNSClusterFirst)
}

func (m *ComponentManager) newDefaultDeploymentNoInit(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	volHelper *VolumeHelper,
	spec v1alpha1.DeploymentSpec,
	containersFactory func([]corev1.VolumeMount) []corev1.Container,
) (*apps.Deployment, error) {
	return m.newDefaultDeploymentWithCloudAffinity(componentType, oc, volHelper, spec, nil, containersFactory)
}

func (m *ComponentManager) newDefaultDeploymentNoInitWithoutCloudAffinity(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	volHelper *VolumeHelper,
	spec v1alpha1.DeploymentSpec,
	hostNetwork bool,
	containersFactory func([]corev1.VolumeMount) []corev1.Container,
) (*apps.Deployment, error) {
	dnsPolicy := corev1.DNSClusterFirst
	if hostNetwork {
		dnsPolicy = corev1.DNSClusterFirstWithHostNet
	}
	return m.newDeployment(componentType, oc, volHelper, spec, nil, containersFactory, hostNetwork, dnsPolicy)
}

func (m *ComponentManager) getObjectMeta(oc *v1alpha1.OnecloudCluster, name string, labels map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:            name,
		Namespace:       oc.GetNamespace(),
		Labels:          labels,
		OwnerReferences: []metav1.OwnerReference{controller.GetOwnerRef(oc)},
	}
}

func (m *ComponentManager) getComponentLabel(oc *v1alpha1.OnecloudCluster, componentType v1alpha1.ComponentType) label.Label {
	instanceName := oc.GetLabels()[label.InstanceLabelKey]
	return label.New().Instance(instanceName).Component(componentType.String())
}

func (m *ComponentManager) newConfigMap(componentType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, config string) *corev1.ConfigMap {
	name := controller.ComponentConfigMapName(oc, componentType)
	return &corev1.ConfigMap{
		ObjectMeta: m.getObjectMeta(oc, name, m.getComponentLabel(oc, componentType).Labels()),
		Data: map[string]string{
			"config": config,
		},
	}
}

func (m *ComponentManager) newServiceConfigMap(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, opt interface{}) *corev1.ConfigMap {
	configYaml := jsonutils.Marshal(opt).YAMLString()
	return m.newConfigMap(cType, oc, configYaml)
}

func (m *ComponentManager) newService(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	serviceType corev1.ServiceType,
	ports []corev1.ServicePort,
) *corev1.Service {
	ocName := oc.GetName()
	svcName := controller.NewClusterComponentName(ocName, componentType)
	appLabel := m.getComponentLabel(oc, componentType)

	svc := &corev1.Service{
		ObjectMeta: m.getObjectMeta(oc, svcName, appLabel),
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: appLabel,
			Ports:    ports,
		},
	}
	return svc
}

func (m *ComponentManager) newNodePortService(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, ports []corev1.ServicePort) *corev1.Service {
	return m.newService(cType, oc, corev1.ServiceTypeNodePort, ports)
}

func (m *ComponentManager) newSingleNodePortService(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, port int32) *corev1.Service {
	ports := []corev1.ServicePort{
		NewServiceNodePort("api", port),
	}
	return m.newNodePortService(cType, oc, ports)
}

func (m *ComponentManager) newDeployment(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	volHelper *VolumeHelper,
	spec v1alpha1.DeploymentSpec,
	initContainersFactory func([]corev1.VolumeMount) []corev1.Container,
	containersFactory func([]corev1.VolumeMount) []corev1.Container,
	hostNetwork bool,
	dnsPolicy corev1.DNSPolicy,
) (*apps.Deployment, error) {
	ns := oc.GetNamespace()
	ocName := oc.GetName()

	appLabel := m.getComponentLabel(oc, componentType)

	vols := volHelper.GetVolumes()
	volMounts := volHelper.GetVolumeMounts()

	podAnnotations := spec.Annotations

	deployName := controller.NewClusterComponentName(ocName, componentType)

	appDeploy := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deployName,
			Namespace:       ns,
			Labels:          appLabel.Labels(),
			OwnerReferences: []metav1.OwnerReference{controller.GetOwnerRef(oc)},
		},
		Spec: apps.DeploymentSpec{
			Replicas: &spec.Replicas,
			Strategy: apps.DeploymentStrategy{Type: apps.RollingUpdateDeploymentStrategyType},
			Selector: appLabel.LabelSelector(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      appLabel.Labels(),
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					Affinity:      spec.Affinity,
					NodeSelector:  spec.NodeSelector,
					Containers:    containersFactory(volMounts),
					RestartPolicy: corev1.RestartPolicyAlways,
					Tolerations:   spec.Tolerations,
					Volumes:       vols,
					HostNetwork:   hostNetwork,
					DNSPolicy:     dnsPolicy,
				},
			},
		},
	}
	templateSpec := &appDeploy.Spec.Template.Spec
	if initContainersFactory != nil {
		templateSpec.InitContainers = initContainersFactory(volMounts)
	}
	if containersFactory != nil {
		templateSpec.Containers = containersFactory(volMounts)
	}
	if hostNetwork {
		appDeploy.Spec.Strategy = apps.DeploymentStrategy{Type: apps.RecreateDeploymentStrategyType}
	}
	return appDeploy, nil
}

func (m *ComponentManager) newCloudServiceDeployment(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	deployCfg v1alpha1.DeploymentSpec,
	initContainersF func([]corev1.VolumeMount) []corev1.Container,
	ports []corev1.ContainerPort,
) (*apps.Deployment, error) {
	configMap := controller.ComponentConfigMapName(oc, cType)
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:  cType.String(),
				Image: deployCfg.Image,
				Command: []string{
					fmt.Sprintf("/opt/yunion/bin/%s", cType.String()),
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", cType.String()),
				},
				ImagePullPolicy: deployCfg.ImagePullPolicy,
				Ports:           ports,
				VolumeMounts:    volMounts,
			},
		}
	}

	return m.newDefaultDeployment(cType, oc, NewVolumeHelper(oc, configMap, cType),
		deployCfg, initContainersF, containersF)
}

func (m *ComponentManager) newCloudServiceDeploymentWithInit(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	deployCfg v1alpha1.DeploymentSpec,
	ports []corev1.ContainerPort,
) (*apps.Deployment, error) {
	initContainersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:  "init",
				Image: deployCfg.Image,
				Command: []string{
					fmt.Sprintf("/opt/yunion/bin/%s", cType.String()),
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", cType.String()),
					"--auto-sync-table",
					"--exit-after-db-init",
				},
				VolumeMounts: volMounts,
			},
		}
	}
	return m.newCloudServiceDeployment(cType, oc, deployCfg, initContainersF, ports)
}

func (m *ComponentManager) newCloudServiceDeploymentNoInit(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	deployCfg v1alpha1.DeploymentSpec,
	ports []corev1.ContainerPort,
) (*apps.Deployment, error) {
	return m.newCloudServiceDeployment(cType, oc, deployCfg, nil, ports)
}

func (m *ComponentManager) newCloudServiceSinglePortDeployment(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	deployCfg v1alpha1.DeploymentSpec,
	port int32,
	doInit bool,
) (*apps.Deployment, error) {
	ports := []corev1.ContainerPort{
		{
			Name:          "api",
			ContainerPort: port,
			Protocol:      corev1.ProtocolTCP,
		},
	}
	f := m.newCloudServiceDeploymentNoInit
	if doInit {
		f = m.newCloudServiceDeploymentWithInit
	}
	return f(cType, oc, deployCfg, ports)
}

func (m *ComponentManager) deploymentIsUpgrading(deploy *apps.Deployment, oc *v1alpha1.OnecloudCluster) (bool, error) {
	if deploymentIsUpgrading(deploy) {
		return true, nil
	}
	return false, nil
}

func (m *ComponentManager) newDaemonSet(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
	volHelper *VolumeHelper,
	spec v1alpha1.DaemonSetSpec, updateStrategy apps.DaemonSetUpdateStrategyType,
	initContainersFactory func() []corev1.Container,
	containersFactory func([]corev1.VolumeMount) []corev1.Container,
) (*apps.DaemonSet, error) {
	ns := oc.GetNamespace()
	ocName := oc.GetName()
	appLabel := m.getComponentLabel(oc, componentType)
	vols := volHelper.GetVolumes()
	volMounts := volHelper.GetVolumeMounts()
	podAnnotations := spec.Annotations
	if len(updateStrategy) == 0 {
		updateStrategy = apps.RollingUpdateDaemonSetStrategyType
	}

	var initContainers []corev1.Container
	if initContainersFactory != nil {
		initContainers = initContainersFactory()
	}

	dsName := controller.NewClusterComponentName(ocName, componentType)
	appDaemonSet := &apps.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            dsName,
			Namespace:       ns,
			Labels:          appLabel.Labels(),
			OwnerReferences: []metav1.OwnerReference{controller.GetOwnerRef(oc)},
		},
		Spec: apps.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      appLabel.Labels(),
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					Affinity:       spec.Affinity,
					NodeSelector:   spec.NodeSelector,
					Containers:     containersFactory(volMounts),
					InitContainers: initContainers,
					RestartPolicy:  corev1.RestartPolicyAlways,
					Tolerations:    spec.Tolerations,
					Volumes:        vols,
					HostNetwork:    true,
					HostPID:        true,
					HostIPC:        true,
					DNSPolicy:      corev1.DNSClusterFirstWithHostNet,
				},
			},
			Selector: appLabel.LabelSelector(),
			UpdateStrategy: apps.DaemonSetUpdateStrategy{
				Type: updateStrategy,
			},
		},
	}
	return appDaemonSet, nil
}

func (m *ComponentManager) newCronJob(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	volHelper *VolumeHelper,
	spec v1alpha1.CronJobSpec,
	initContainersFactory func([]corev1.VolumeMount) []corev1.Container,
	containersFactory func([]corev1.VolumeMount) []corev1.Container,
	hostNetwork bool, dnsPolicy corev1.DNSPolicy,
	concurrencyPolicy batchv1.ConcurrencyPolicy,
	startingDeadlineSeconds *int64, suspend *bool,
	successfulJobsHistoryLimit, failedJobsHistoryLimit *int32,
) (*batchv1.CronJob, error) {
	ns := oc.GetNamespace()
	ocName := oc.GetName()
	vols := volHelper.GetVolumes()
	volMounts := volHelper.GetVolumeMounts()
	appLabel := m.getComponentLabel(oc, componentType)
	podAnnotations := spec.Annotations
	cronJobName := controller.NewClusterComponentName(ocName, componentType)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:            cronJobName,
			Namespace:       ns,
			Labels:          appLabel.Labels(),
			OwnerReferences: []metav1.OwnerReference{controller.GetOwnerRef(oc)},
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   spec.Schedule,
			StartingDeadlineSeconds:    startingDeadlineSeconds,
			ConcurrencyPolicy:          concurrencyPolicy,
			Suspend:                    suspend,
			SuccessfulJobsHistoryLimit: successfulJobsHistoryLimit,
			FailedJobsHistoryLimit:     failedJobsHistoryLimit,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      appLabel.Labels(),
					Annotations: podAnnotations,
				},
				Spec: jobbatchv1.JobSpec{
					// Selector: appLabel.LabelSelector(),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      appLabel.Labels(),
							Annotations: podAnnotations,
						},
						Spec: corev1.PodSpec{
							Affinity:      spec.Affinity,
							NodeSelector:  spec.NodeSelector,
							Containers:    containersFactory(volMounts),
							RestartPolicy: corev1.RestartPolicyNever, // never restart
							Tolerations:   spec.Tolerations,
							Volumes:       vols,
							HostNetwork:   hostNetwork,
							DNSPolicy:     dnsPolicy,
						},
					},
				},
			},
		},
	}
	templateSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec
	if initContainersFactory != nil {
		templateSpec.InitContainers = initContainersFactory(volMounts)
	}
	return cronJob, nil
}

func (m *ComponentManager) newDefaultCronJob(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	volHelper *VolumeHelper,
	spec v1alpha1.CronJobSpec,
	initContainersFactory func([]corev1.VolumeMount) []corev1.Container,
	containersFactory func([]corev1.VolumeMount) []corev1.Container,
) (*batchv1.CronJob, error) {
	return m.newCronJob(componentType, oc, volHelper, spec, initContainersFactory,
		containersFactory, false, corev1.DNSClusterFirst, "", nil, nil, nil, nil)
}

func (m *ComponentManager) newPVC(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, spec v1alpha1.StatefulDeploymentSpec) (*corev1.PersistentVolumeClaim, error) {
	ocName := oc.GetName()
	pvcName := controller.NewClusterComponentName(ocName, cType)

	storageClass := spec.StorageClassName
	size := spec.Requests.Storage
	sizeQ, err := resource.ParseQuantity(size)
	if err != nil {
		return nil, err
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: oc.GetNamespace(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: sizeQ,
				},
			},
		},
	}
	if storageClass != "" {
		pvc.Spec.StorageClassName = &storageClass
	}
	return pvc, nil
}

func (m *ComponentManager) syncPVC(oc *v1alpha1.OnecloudCluster,
	pvcFactory func(*v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error)) error {
	ns := oc.GetNamespace()
	newPvc, err := pvcFactory(oc)
	if err != nil {
		return err
	}
	if newPvc == nil {
		return nil
	}
	_, err = m.pvcLister.PersistentVolumeClaims(ns).Get(newPvc.GetName())
	if errors.IsNotFound(err) {
		return m.pvcControl.CreatePVC(oc, newPvc)
	}
	if err != nil {
		return err
	}
	// Not update pvc
	return nil
}

func (m *ComponentManager) syncPhase(oc *v1alpha1.OnecloudCluster,
	phaseFactory func(controller.ComponentManager) controller.PhaseControl) error {
	phase := phaseFactory(m.onecloudControl.Components(oc))
	if _, err := m.onecloudControl.GetSession(oc); err != nil {
		return errorswrap.Wrapf(err, "get cluster %s session", oc.GetName())
	}

	if phase == nil {
		return nil
	}
	if err := phase.Setup(); err != nil {
		return err
	}
	if err := phase.SystemInit(); err != nil {
		return err
	}
	return nil
}

func (m *ComponentManager) getDBConfig(_ *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return nil
}

func (m *ComponentManager) getCloudUser(_ *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return nil
}

func (m *ComponentManager) getPhaseControl(_ controller.ComponentManager) controller.PhaseControl {
	return nil
}

func (m *ComponentManager) getDeploymentStatus(_ *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return nil
}

func (m *ComponentManager) getService(_ *v1alpha1.OnecloudCluster) *corev1.Service {
	return nil
}

func (m *ComponentManager) getIngress(_ *v1alpha1.OnecloudCluster) *extensions.Ingress {
	return nil
}

func (m *ComponentManager) getConfigMap(_ *v1alpha1.OnecloudCluster, _ *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	return nil, nil
}

func (m *ComponentManager) getComponentManager() *ComponentManager {
	return m
}

func (m *ComponentManager) getPVC(_ *v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error) {
	return nil, nil
}
func (m *ComponentManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	return nil, nil
}

func (m *ComponentManager) getDaemonSet(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*apps.DaemonSet, error) {
	return nil, nil
}

func (m *ComponentManager) getCronJob(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*batchv1.CronJob, error) {
	return nil, nil
}

func (m *ComponentManager) Keystone() manager.Manager {
	return newKeystoneComponentManager(m)
}

func (m *ComponentManager) Logger() manager.Manager {
	return newLoggerManager(m)
}

func (m *ComponentManager) Region() manager.Manager {
	return newRegionManager(m)
}

func (m *ComponentManager) Climc() manager.Manager {
	return newClimcComponentManager(m)
}

func (m *ComponentManager) Glance() manager.Manager {
	return newGlanceManager(m)
}

func (m *ComponentManager) Webconsole() manager.Manager {
	return newWebconsoleManager(m)
}

func (m *ComponentManager) Scheduler() manager.Manager {
	return newSchedulerManager(m)
}

func (m *ComponentManager) Influxdb() manager.Manager {
	return newInfluxdbManager(m)
}

func (m *ComponentManager) Kapacitor() manager.Manager {
	return newKapacitorManager(m)
}

func (m *ComponentManager) Yunionagent() manager.Manager {
	return newYunionagentManager(m)
}

func (m *ComponentManager) Yunionconf() manager.Manager {
	return newYunionconfManager(m)
}

func (m *ComponentManager) KubeServer() manager.Manager {
	return newKubeManager(m)
}

func (m *ComponentManager) AnsibleServer() manager.Manager {
	return newAnsibleManager(m)
}

func (m *ComponentManager) Cloudnet() manager.Manager {
	return newCloudnetManager(m)
}

func (m *ComponentManager) APIGateway() manager.Manager {
	return newAPIGatewayManager(m)
}

func (m *ComponentManager) Web() manager.Manager {
	return newWebManager(m)
}

func (m *ComponentManager) Notify() manager.Manager {
	return newNotifyManager(m)
}

func (m *ComponentManager) Baremetal() manager.Manager {
	return newBaremetalManager(m)
}

func (m *ComponentManager) Host() manager.Manager {
	return newHostManager(m)
}

func (m *ComponentManager) HostDeployer() manager.Manager {
	return newHostDeployerManger(m)
}

func (m *ComponentManager) Cloudevent() manager.Manager {
	return newCloudeventManager(m)
}

func (m *ComponentManager) S3gateway() manager.Manager {
	return newS3gatewayManager(m)
}

func (m *ComponentManager) Devtool() manager.Manager {
	return newDevtoolManager(m)
}

func (m *ComponentManager) Meter() manager.Manager {
	return newMeterManager(m)
}

func (m *ComponentManager) AutoUpdate() manager.Manager {
	return newAutoUpdateManager(m)
}

func (m *ComponentManager) CloudmonPing() manager.Manager {
	return newCloudmonPingManager(m)
}

func (m *ComponentManager) CloudmonReportUsage() manager.Manager {
	return newCloudmonReportUsageManager(m)
}

func (m *ComponentManager) EsxiAgent() manager.Manager {
	return newEsxiManager(m)
}

func (m *ComponentManager) Monitor() manager.Manager {
	return newMonitorManager(m)
}

func (m *ComponentManager)CloudmonReportServer() manager.Manager  {
	return newCloudmonReportServerManager(m)
}
