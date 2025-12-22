package component

import (
	corev1 "k8s.io/api/core/v1"
	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type hostOvsdbServerManager struct {
}

func (m *hostOvsdbServerManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.HostOvsdbServerComponentType
}

func (m *hostOvsdbServerManager) shouldSync(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster) bool {
	return true
}

func (m *hostOvsdbServerManager) isDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.HostAgent.Disable
}

func (m *hostOvsdbServerManager) getContainers(oc *v1alpha1.OnecloudCluster, volMounts []corev1.VolumeMount) []corev1.Container {
	privileged := true
	image := oc.Spec.HostAgent.OvnController.Image
	imagePullPolicy := oc.Spec.HostAgent.OvnController.ImagePullPolicy
	if len(image) == 0 {
		image = oc.Spec.Lbagent.OvnController.Image
		imagePullPolicy = oc.Spec.Lbagent.OvnController.ImagePullPolicy
	}
	containers := []corev1.Container{}
	containers = append(containers, corev1.Container{
		Name:            "ovsdb-server",
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Command:         []string{"/start.sh", "ovsdb"},
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

func (m *hostOvsdbServerManager) getVolumeHelper(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, configMap string) *VolumeHelper {
	return NewOvsVolumeHelper(cType, oc, configMap)
}

func (m *hostOvsdbServerManager) getNodeSelector(oc *v1alpha1.OnecloudCluster) map[string]string {
	selector := map[string]string{
		constants.OnecloudEnableHostLabelKey: "enable",
	}
	if !oc.Spec.DisableLocalVpc {
		selector[constants.OnecloudEnableLbagentLabelKey] = "enable"
	}
	return selector
}

func newHostOvsdbServerManager(man *ComponentManager) manager.Manager {
	return newhostBasedDsManager(man, &hostOvsdbServerManager{})
}
