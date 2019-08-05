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

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appv1 "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"

	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud-operator/pkg/label"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type ComponentManager struct {
	deployControl controller.DeploymentControlInterface
	deployLister  appv1.DeploymentLister
	svcControl    controller.ServiceControlInterface
	svcLister     corelisters.ServiceLister
	pvcControl    controller.PVCControlInterface
	pvcLister     corelisters.PersistentVolumeClaimLister

	configer        Configer
	onecloudControl *controller.OnecloudControl
}

// NewComponentManager return *BaseComponentManager
func NewComponentManager(
	deployCtrol controller.DeploymentControlInterface,
	deployLister appv1.DeploymentLister,
	svcControl controller.ServiceControlInterface,
	svcLister corelisters.ServiceLister,
	pvcControl controller.PVCControlInterface,
	pvcLister corelisters.PersistentVolumeClaimLister,
	configer Configer,
	onecloudControl *controller.OnecloudControl,
) *ComponentManager {
	return &ComponentManager{
		deployControl:   deployCtrol,
		deployLister:    deployLister,
		svcControl:      svcControl,
		svcLister:       svcLister,
		pvcControl:      pvcControl,
		pvcLister:       pvcLister,
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

func (m *ComponentManager) syncConfigMap(
	oc *v1alpha1.OnecloudCluster,
	dbConfigFactory func(*v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig,
	svcAccountFactory func(*v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser,
	cfgMapFactory func(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error),
) error {
	clustercfg, err := m.configer.GetClusterConfig(oc)
	if err != nil {
		return err
	}
	if dbConfigFactory != nil {
		dbConfig := dbConfigFactory(clustercfg)
		if dbConfig != nil {
			if err := EnsureClusterDBUser(oc, *dbConfig); err != nil {
				return err
			}
		}
	}
	if svcAccountFactory != nil {
		account := svcAccountFactory(clustercfg)
		if account != nil {
			s, err := m.onecloudControl.GetSession(oc)
			if err != nil {
				return err
			}
			if err := EnsureServiceAccount(s, *account); err != nil {
				return err
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
	return m.configer.CreateOrUpdateConfigMap(oc, cfgMap)
}

func (m *ComponentManager) syncDeployment(
	oc *v1alpha1.OnecloudCluster,
	deploymentFactory func(*v1alpha1.OnecloudCluster, *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error),
	postSyncFunc func(*v1alpha1.OnecloudCluster, *apps.Deployment) error,
) error {
	ocName := oc.GetName()
	ns := oc.GetNamespace()
	cfg, err := m.configer.GetClusterConfig(oc)
	if err != nil {
		return err
	}
	newDeploy, err := deploymentFactory(oc, cfg)
	if err != nil {
		return err
	}

	oldDeployTmp, err := m.deployLister.Deployments(ns).Get(newDeploy.GetName())
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if errors.IsNotFound(err) {
		err = SetDeploymentLastAppliedConfigAnnotation(newDeploy)
		if err != nil {
			return err
		}
		if err := m.deployControl.CreateDeployment(oc, newDeploy); err != nil {
			return err
		}
		oc.Status.Keystone.Deployment = &apps.DeploymentStatus{}
		return controller.RequeueErrorf("OnecloudCluster: [%s/%s], waiting for keystone running", ns, ocName)
	}

	oldDeploy := oldDeployTmp.DeepCopy()

	if !templateEqual(newDeploy.Spec.Template, oldDeploy.Spec.Template) || oc.Status.Keystone.Phase == v1alpha1.UpgradePhase {
		/*		if err := m.ksUpgrader.Upgrade(oc, oldKsDeploy, newKsDeploy); err != nil {
				return err
			}*/
	}

	if err = m.updateDeployment(oc, newDeploy, oldDeploy); err != nil {
		return err
	}
	if postSyncFunc != nil {
		if err := postSyncFunc(oc, oldDeploy); err != nil {
			return err
		}
	}
	return nil
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
	return m.newDeployment(componentType, oc, volHelper, spec, initContainersFactory, containersFactory, false, corev1.DNSClusterFirst)
}

func (m *ComponentManager) newDefaultDeploymentNoInit(
	componentType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	volHelper *VolumeHelper,
	spec v1alpha1.DeploymentSpec,
	containersFactory func([]corev1.VolumeMount) []corev1.Container,
) (*apps.Deployment, error) {
	return m.newDeployment(componentType, oc, volHelper, spec, nil, containersFactory, false, corev1.DNSClusterFirst)
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
			Strategy: apps.DeploymentStrategy{Type: apps.RecreateDeploymentStrategyType},
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
				ImagePullPolicy: oc.Spec.Keystone.ImagePullPolicy,
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

func (m *ComponentManager) getConfigMap(_ *v1alpha1.OnecloudCluster, _ *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	return nil, nil
}

func (m *ComponentManager) getComponentManager() *ComponentManager {
	return m
}

func (m *ComponentManager) getPVC(_ *v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error) {
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

func (m *ComponentManager) Yunionagent() manager.Manager {
	return newYunionagentManager(m)
}

func (m *ComponentManager) Yunionconf() manager.Manager {
	return newYunionconfManager(m)
}

func (m *ComponentManager) APIGateway() manager.Manager {
	return newAPIGatewayManager(m)
}

func (m *ComponentManager) Web() manager.Manager {
	return newWebManager(m)
}
