package component

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

const YUNION_HOST_ROOT = "/yunion-host-root"

type hostImageManager struct {
}

func (m *hostImageManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.HostImageComponentType
}

func (m *hostImageManager) shouldSync(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster) bool {
	return true
}

func (m *hostImageManager) isDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.HostImage.Disable
}

func (m *hostImageManager) getContainers(oc *v1alpha1.OnecloudCluster, volMounts []corev1.VolumeMount) []corev1.Container {
	privileged := true
	dsSpec := oc.Spec.HostImage
	return []corev1.Container{
		{
			Name:  "host-image",
			Image: oc.Spec.HostImage.Image,
			Command: []string{
				fmt.Sprintf("/opt/yunion/bin/host-image"),
				"--common-config-file",
				"/etc/yunion/common/common.conf",
				"--config",
				"/etc/yunion/host.conf",
			},
			ImagePullPolicy: dsSpec.ImagePullPolicy,
			VolumeMounts:    volMounts,
			SecurityContext: &corev1.SecurityContext{
				Privileged: &privileged,
			},
		},
	}
}

func (m *hostImageManager) getVolumeHelper(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, configMap string) *VolumeHelper {
	return NewHostImageVolumeHelper(cType, oc, configMap)
}

func (m *hostImageManager) getNodeSelector(oc *v1alpha1.OnecloudCluster) []string {
	return []string{
		constants.OnecloudEnableHostLabelKey,
	}
}

func newHostImageManager(man *ComponentManager) manager.Manager {
	return newhostBasedDsManager(man, &hostImageManager{})
}
