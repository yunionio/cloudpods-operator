package component

import (
	corev1 "k8s.io/api/core/v1"
	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type hostOvnControllerManager struct {
}

func (m *hostOvnControllerManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.HostOvnControllerComponentType
}

func (m *hostOvnControllerManager) shouldSync(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster) bool {
	return isDsReady(factory, oc, v1alpha1.HostOvsVswitchdComponentType, oc.Spec.HostAgent.OvnController.Image)
}

func (m *hostOvnControllerManager) isDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.HostAgent.Disable || oc.Spec.DisableLocalVpc
}

func (m *hostOvnControllerManager) getContainers(oc *v1alpha1.OnecloudCluster, volMounts []corev1.VolumeMount) []corev1.Container {
	privileged := true
	image := oc.Spec.HostAgent.OvnController.Image
	imagePullPolicy := oc.Spec.HostAgent.OvnController.ImagePullPolicy
	if len(image) == 0 {
		image = oc.Spec.Lbagent.OvnController.Image
		imagePullPolicy = oc.Spec.Lbagent.OvnController.ImagePullPolicy
	}
	containers := []corev1.Container{}
	containers = append(containers, corev1.Container{
		Name:            "ovn-controller",
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Command:         []string{"/start.sh", "controller"},
		VolumeMounts:    volMounts,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					corev1.Capability("SYS_NICE"),
				},
			},
		},
	})
	return containers
}

func (m *hostOvnControllerManager) getVolumeHelper(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, configMap string) *VolumeHelper {
	return NewOvsVolumeHelper(cType, oc, configMap)
}

func (m *hostOvnControllerManager) getNodeSelector(oc *v1alpha1.OnecloudCluster) []string {
	// selector := map[string]string{
	// 	constants.OnecloudEnableHostLabelKey: "enable",
	// }
	selector := []string{
		constants.OnecloudEnableHostLabelKey,
	}
	if !oc.Spec.DisableLocalVpc {
		// selector[constants.OnecloudEnableLbagentLabelKey] = "enable"
		selector = append(selector, constants.OnecloudEnableLbagentLabelKey)
	}
	return selector
}

func newHostOvnControllerManager(man *ComponentManager) manager.Manager {
	return newhostBasedDsManager(man, &hostOvnControllerManager{})
}
