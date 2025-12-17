package component

import (
	corev1 "k8s.io/api/core/v1"
	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type hostHealthManager struct {
}

func (m *hostHealthManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.HostHealthComponentType
}

func (m *hostHealthManager) shouldSync(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster) bool {
	return true
}

func (m *hostHealthManager) isDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.HostAgent.Disable
}

func (m *hostHealthManager) getContainers(oc *v1alpha1.OnecloudCluster, volMounts []corev1.VolumeMount) []corev1.Container {
	privileged := true
	dsSpec := oc.Spec.HostAgent
	containers := []corev1.Container{}
	containers = append(containers, corev1.Container{
		Name:            "host-health",
		Image:           dsSpec.HostHealth.Image,
		ImagePullPolicy: dsSpec.HostHealth.ImagePullPolicy,
		Command: []string{
			"/opt/yunion/bin/host-health",
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

func (m *hostHealthManager) getVolumeHelper(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, configMap string) *VolumeHelper {
	return NewHostVolume(cType, oc, configMap)
}

func (m *hostHealthManager) getNodeSelector(oc *v1alpha1.OnecloudCluster) map[string]string {
	return map[string]string{
		constants.OnecloudEnableHostLabelKey: "enable",
	}
}

func newHostHealthManager(man *ComponentManager) manager.Manager {
	return newhostBasedDsManager(man, &hostHealthManager{})
}
