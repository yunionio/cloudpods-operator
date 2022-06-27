package component

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

const YUNION_HOST_ROOT = "/yunion-host-root"

type hostImageManager struct {
	*ComponentManager
}

func newHostImageManager(man *ComponentManager) manager.Manager {
	return &hostImageManager{man}
}

func (m *hostImageManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *hostImageManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.HostImageComponentType
}

func (m *hostImageManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.HostImage.Disable, "")
}

func (m *hostImageManager) getDaemonSet(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.DaemonSet, error) {
	return m.newHostPrivilegedDaemonSet(v1alpha1.HostImageComponentType, oc, cfg)
}

func (m *hostImageManager) newHostPrivilegedDaemonSet(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*apps.DaemonSet, error) {
	var (
		privileged  = true
		dsSpec      = oc.Spec.HostImage
		configMap   = controller.ComponentConfigMapName(oc, v1alpha1.HostComponentType)
		containersF = func(volMounts []corev1.VolumeMount) []corev1.Container {
			return []corev1.Container{
				{
					Name:  cType.String(),
					Image: oc.Spec.HostImage.Image,
					Command: []string{
						"sh", "-ce", fmt.Sprintf(`
mkdir -p /etc/resolvconf/run && cp /etc/resolv.conf /etc/resolvconf/run
mount --bind -o ro /etc/hosts %s/etc/hosts
mount --bind -o ro /etc/resolv.conf %s/etc/resolv.conf
test -d %s/etc/resolvconf && mount --rbind /etc/resolvconf %s/etc/resolvconf
mkdir -p %s/etc/yunion/common
mount --bind /etc/yunion/common %s/etc/yunion/common
mkdir -p %s/etc/yunion/pki
mount --bind /etc/yunion/pki %s/etc/yunion/pki
mkdir -p %s/opt/yunion/bin
mount --bind /opt/yunion/bin %s/opt/yunion/bin
chroot %s /opt/yunion/bin/%s --config /etc/yunion/%s.conf --common-config-file /etc/yunion/common/common.conf`,
							YUNION_HOST_ROOT,
							YUNION_HOST_ROOT,
							YUNION_HOST_ROOT, YUNION_HOST_ROOT,
							YUNION_HOST_ROOT,
							YUNION_HOST_ROOT,
							YUNION_HOST_ROOT,
							YUNION_HOST_ROOT,
							YUNION_HOST_ROOT,
							YUNION_HOST_ROOT,
							YUNION_HOST_ROOT, cType.String(), v1alpha1.HostComponentType.String()),
					},
					ImagePullPolicy: dsSpec.ImagePullPolicy,
					VolumeMounts:    volMounts,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &privileged,
					},
				},
			}
		}
	)
	if dsSpec.NodeSelector == nil {
		dsSpec.NodeSelector = make(map[string]string)
	}
	dsSpec.NodeSelector[constants.OnecloudEnableHostLabelKey] = "enable"
	ds, err := m.newDaemonSet(cType, oc, cfg,
		NewHostImageVolumeHelper(cType, oc, configMap), dsSpec, "", nil, containersF)
	if err != nil {
		return nil, err
	}

	return ds, nil
}
