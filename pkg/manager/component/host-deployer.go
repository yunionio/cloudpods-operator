package component

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type hostDeployerManager struct {
	*ComponentManager
}

func newHostDeployerManger(man *ComponentManager) manager.Manager {
	return &hostDeployerManager{man}
}

func (m *hostDeployerManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *hostDeployerManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.HostDeployerComponentType
}

func (m *hostDeployerManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.HostDeployer.Disable || !isInProductVersion(m, oc)
}

func (m *hostDeployerManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *hostDeployerManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.HostAgent.CloudUser
}

func (m *hostDeployerManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	if oc.Spec.ProductVersion == v1alpha1.ProductVersionCMP {
		return newHostManager(m.ComponentManager).(*hostManager).getConfigMap(oc, cfg, zone)
	}
	return nil, false, nil
}

func (m *hostDeployerManager) getDaemonSet(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.DaemonSet, error) {
	return m.newHostPrivilegedDaemonSet(v1alpha1.HostDeployerComponentType, oc, cfg)
}

func (m *hostDeployerManager) newHostPrivilegedDaemonSet(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*apps.DaemonSet, error) {
	var (
		privileged  = true
		dsSpec      = oc.Spec.HostDeployer
		configMap   = controller.ComponentConfigMapName(oc, v1alpha1.HostComponentType)
		containersF = func(volMounts []corev1.VolumeMount) []corev1.Container {
			return []corev1.Container{
				{
					Name:  cType.String(),
					Image: dsSpec.Image,
					Command: []string{
						fmt.Sprintf("/opt/yunion/bin/%s", cType.String()),
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
	)
	ds, err := m.newDaemonSet(cType, oc, cfg,
		NewHostDeployerVolume(cType, oc, configMap), dsSpec, "", nil, containersF)
	if err != nil {
		return nil, err
	}
	// set inter pod affinity
	if ds.Spec.Template.Spec.Affinity == nil {
		ds.Spec.Template.Spec.Affinity = &corev1.Affinity{}
	}
	if ds.Spec.Template.Spec.Affinity.PodAffinity == nil {
		ds.Spec.Template.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
	}
	ds.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = []corev1.PodAffinityTerm{
		{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{constants.OnecloudHostDeployerLabelKey: ""},
			},
			TopologyKey: "kubernetes.io/hostname",
		},
	}
	if ds.Spec.UpdateStrategy.RollingUpdate == nil {
		ds.Spec.UpdateStrategy.RollingUpdate = &apps.RollingUpdateDaemonSet{}
	}
	// ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable, err = m.getMaxUnavailablePodCount()
	// if err != nil {
	// 	return nil, err
	// }
	maxUnavailable := intstr.FromInt(999)
	ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = &maxUnavailable
	ds.Spec.Template.Spec.HostNetwork = true
	return ds, nil
}

/*func (m *hostDeployerManager) getMaxUnavailablePodCount() (*intstr.IntOrString, error) {
	masterNodeSelector := labels.NewSelector()
	r, err := labels.NewRequirement(
		constants.OnecloudControllerLabelKey, selection.Exists, nil,
	)
	if err != nil {
		return nil, err
	}
	masterNodeSelector = masterNodeSelector.Add(*r)
	nodes, err := m.nodeLister.List(masterNodeSelector)
	if err != nil {
		return nil, err
	}
	klog.Infof("List nodes by controller selector, node count %v", len(nodes))
	if len(nodes) == 0 {
		return nil, nil
	}
	count := intstr.FromInt(len(nodes) + 1)
	return &count, nil
}*/
