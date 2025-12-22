package component

import (
	corev1 "k8s.io/api/core/v1"
	"yunion.io/x/log"
	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type hostOvsVswitchdManager struct {
}

func (m *hostOvsVswitchdManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.HostOvsVswitchdComponentType
}

func isDsReady(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster, componentType v1alpha1.ComponentType, expectImage string) bool {
	ns := oc.GetNamespace()
	cm := factory.getComponentManager()
	ocName := oc.GetName()
	dsName := controller.NewClusterComponentName(ocName, componentType)
	dsOvsdbServer, err := cm.dsLister.DaemonSets(ns).Get(dsName)
	if err != nil {
		log.Errorf("get daemonset %s error: %v", dsName, err)
		return false
	}
	if dsOvsdbServer == nil {
		log.Errorf("daemonset %s not found", dsName)
		return false
	}
	// if damonset image is not synchronized, giveup
	if dsOvsdbServer.Spec.Template.Spec.Containers[0].Image != expectImage {
		return false
	}
	/*
	  currentNumberScheduled: 1
	  desiredNumberScheduled: 1
	  numberMisscheduled: 0
	  numberReady: 1
	  numberUnavailable: 0
	  observedGeneration: 22
	  updatedNumberScheduled: 1
	*/
	if dsOvsdbServer.Status.NumberAvailable != dsOvsdbServer.Status.DesiredNumberScheduled || dsOvsdbServer.Status.NumberReady != dsOvsdbServer.Status.DesiredNumberScheduled || dsOvsdbServer.Status.DesiredNumberScheduled != dsOvsdbServer.Status.UpdatedNumberScheduled || dsOvsdbServer.Generation != dsOvsdbServer.Status.ObservedGeneration {
		log.Errorf("daemonset %s is not ready", dsName)
		return false
	}
	log.Infof("daemonset %s is ready", dsName)
	return true
}

func (m *hostOvsVswitchdManager) shouldSync(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster) bool {
	return isDsReady(factory, oc, v1alpha1.HostOvsdbServerComponentType, oc.Spec.HostAgent.OvnController.Image)
}

func (m *hostOvsVswitchdManager) isDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.HostAgent.Disable
}

func (m *hostOvsVswitchdManager) getContainers(oc *v1alpha1.OnecloudCluster, volMounts []corev1.VolumeMount) []corev1.Container {
	privileged := true
	image := oc.Spec.HostAgent.OvnController.Image
	imagePullPolicy := oc.Spec.HostAgent.OvnController.ImagePullPolicy
	if len(image) == 0 {
		image = oc.Spec.Lbagent.OvnController.Image
		imagePullPolicy = oc.Spec.Lbagent.OvnController.ImagePullPolicy
	}
	containers := []corev1.Container{}
	containers = append(containers, corev1.Container{
		Name:            "ovs-vswitchd",
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Command:         []string{"/start.sh", "vswitchd"},
		VolumeMounts:    volMounts,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					corev1.Capability("NET_ADMIN"),
					corev1.Capability("SYS_MODULE"),
					corev1.Capability("SYS_NICE"),
				},
			},
		},
	})
	return containers
}

func (m *hostOvsVswitchdManager) getVolumeHelper(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, configMap string) *VolumeHelper {
	return NewOvsVolumeHelper(cType, oc, configMap)
}

func (m *hostOvsVswitchdManager) getNodeSelector(oc *v1alpha1.OnecloudCluster) map[string]string {
	selector := map[string]string{
		constants.OnecloudEnableHostLabelKey: "enable",
	}
	if !oc.Spec.DisableLocalVpc {
		selector[constants.OnecloudEnableLbagentLabelKey] = "enable"
	}
	return selector
}

func newHostOvsVswitchdManager(man *ComponentManager) manager.Manager {
	return newhostBasedDsManager(man, &hostOvsVswitchdManager{})
}
