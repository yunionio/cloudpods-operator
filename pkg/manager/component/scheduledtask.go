package component

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
)

type scheduledtaskManager struct {
	*ComponentManager
}

func newScheduledtaskManager(man *ComponentManager) manager.Manager {
	return &scheduledtaskManager{man}
}

func (m *scheduledtaskManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *scheduledtaskManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.ScheduledtaskComponentType
}

func (m *scheduledtaskManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Scheduledtask.Disable, "")
}

func (m *scheduledtaskManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewScheduledTask().GetPhaseControl(man)
}

func (m *scheduledtaskManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.ScheduledtaskComponentType, oc, int32(oc.Spec.Scheduledtask.Service.NodePort), int32(constants.ScheduledtaskPort))}
}

func (m *scheduledtaskManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewScheduledTask().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}
	return m.newServiceConfigMap(v1alpha1.ScheduledtaskComponentType, "", oc, opt), false, nil
}

func (m *scheduledtaskManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.ScheduledtaskComponentType, "", oc, &oc.Spec.Scheduledtask.DeploymentSpec, constants.ScheduledtaskPort, true, false)
}

func (m *scheduledtaskManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Scheduledtask
}
