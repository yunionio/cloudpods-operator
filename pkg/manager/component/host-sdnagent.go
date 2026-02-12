package component

import (
	corev1 "k8s.io/api/core/v1"
	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type hostSdnagentManager struct {
}

func (m *hostSdnagentManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.HostSdnagentComponentType
}

func (m *hostSdnagentManager) shouldSync(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster) bool {
	return true
}

func (m *hostSdnagentManager) isDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.HostAgent.Disable
}

func (m *hostSdnagentManager) getContainers(oc *v1alpha1.OnecloudCluster, volMounts []corev1.VolumeMount) []corev1.Container {
	privileged := true
	dsSpec := oc.Spec.HostAgent
	containers := []corev1.Container{}
	containers = append(containers, corev1.Container{
		Name:            "sdnagent",
		Image:           dsSpec.SdnAgent.Image,
		ImagePullPolicy: dsSpec.SdnAgent.ImagePullPolicy,
		Command: []string{
			"/opt/yunion/bin/sdnagent",
			"--common-config-file",
			"/etc/yunion/common/common.conf",
		},
		VolumeMounts: volMounts,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
		},
	})
	return containers
}

func (m *hostSdnagentManager) getVolumeHelper(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, configMap string) *VolumeHelper {
	return NewHostVolume(cType, oc, configMap)
}

func (m *hostSdnagentManager) getNodeSelector(oc *v1alpha1.OnecloudCluster) []string {
	// return map[string]string{
	// 	constants.OnecloudEnableHostLabelKey: "enable",
	// }
	return []string{
		constants.OnecloudEnableHostLabelKey,
	}
}

func newHostSdnagentManager(man *ComponentManager) manager.Manager {
	return newhostBasedDsManager(man, &hostSdnagentManager{})
}
