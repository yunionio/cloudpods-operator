package component

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"yunion.io/x/onecloud/pkg/mcclient"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type hostBasedService interface {
	getComponentType() v1alpha1.ComponentType
	isDisabled(oc *v1alpha1.OnecloudCluster) bool
	getContainers(oc *v1alpha1.OnecloudCluster, volMounts []corev1.VolumeMount) []corev1.Container
	getVolumeHelper(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, configMap string) *VolumeHelper
	shouldSync(factory cloudComponentFactory, oc *v1alpha1.OnecloudCluster) bool
	getNodeSelector(oc *v1alpha1.OnecloudCluster) []string
}

type hostBasedDsManager struct {
	*ComponentManager

	service hostBasedService
}

func newhostBasedDsManager(man *ComponentManager, service hostBasedService) manager.Manager {
	return &hostBasedDsManager{ComponentManager: man, service: service}
}

func (m *hostBasedDsManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *hostBasedDsManager) GetComponentType() v1alpha1.ComponentType {
	return m.service.getComponentType()
}

func (m *hostBasedDsManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return m.service.isDisabled(oc) || !isInProductVersion(m, oc)
}

func (m *hostBasedDsManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *hostBasedDsManager) getDaemonSet(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.DaemonSet, error) {
	if !m.service.shouldSync(m, oc) {
		return nil, nil
	}
	return m.newHostPrivilegedDaemonSet(m.service.getComponentType(), oc, cfg)
}

func (m *hostBasedDsManager) newHostPrivilegedDaemonSet(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*apps.DaemonSet, error) {
	var (
		dsSpec    = oc.Spec.HostAgent
		configMap = controller.ComponentConfigMapName(oc, v1alpha1.HostComponentType)
	)
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return m.service.getContainers(oc, volMounts)
	}

	nodeSelector := m.service.getNodeSelector(oc)
	if len(nodeSelector) == 1 {
		// use node selector
		if dsSpec.NodeSelector == nil {
			dsSpec.NodeSelector = make(map[string]string)
		}
		for _, v := range nodeSelector {
			dsSpec.NodeSelector[v] = "enable"
		}
	} else if len(nodeSelector) > 1 {
		// use node affinity - any of the key-value pairs should match (OR logic)
		if dsSpec.Affinity == nil {
			dsSpec.Affinity = &corev1.Affinity{}
		}
		if dsSpec.Affinity.NodeAffinity == nil {
			dsSpec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
		}
		// Create one NodeSelectorTerm per key-value pair for OR logic
		terms := make([]corev1.NodeSelectorTerm, 0, len(nodeSelector))
		for k := range nodeSelector {
			terms = append(terms, corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      nodeSelector[k],
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"enable"},
					},
				},
			})
		}
		dsSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
			NodeSelectorTerms: terms,
		}
	}

	volHelper := m.service.getVolumeHelper(cType, oc, configMap)
	ds, err := m.newDaemonSet(cType, oc, cfg, volHelper, dsSpec.DaemonSetSpec, "", nil, containersF)
	if err != nil {
		return nil, err
	}

	/* set pod max maxUnavailable count, default 1 */
	if ds.Spec.UpdateStrategy.RollingUpdate == nil {
		ds.Spec.UpdateStrategy.RollingUpdate = new(apps.RollingUpdateDaemonSet)
	}
	var maxUnavailableCount = intstr.FromInt(3)
	ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = &maxUnavailableCount

	return ds, nil
}

func (m *hostBasedDsManager) supportsReadOnlyService() bool {
	return false
}

func (m *hostBasedDsManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return nil
}

func (m *hostBasedDsManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return nil
}
