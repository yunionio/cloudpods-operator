package component

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog"

	"yunion.io/x/onecloud/pkg/baremetal/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
)

type baremetalManager struct {
	*ComponentManager
}

func newBaremetalManager(m *ComponentManager) manager.ServiceManager {
	return &baremetalManager{m}
}

func (m *baremetalManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *baremetalManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.BaremetalAgentComponentType
}

func (m *baremetalManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.BaremetalAgent.Disable || !isInProductVersion(m, oc)
}

func (b *baremetalManager) GetServiceName() string {
	return constants.ServiceNameBaremetal
}

func (m *baremetalManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return m.multiZoneSync(oc, oc.Spec.BaremetalAgent.Zones, m)
}

func (m *baremetalManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.BaremetalAgent.CloudUser
}

func (m *baremetalManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterServiceComponent(man, constants.ServiceNameBaremetal, constants.ServiceTypeBaremetal)
}

func (m *baremetalManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	zoneId := oc.GetZone(zone)

	optObj, err := component.NewBaremetal().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}
	opt := optObj.(*options.BaremetalOptions)
	opt.Zone = zoneId
	return m.newServiceConfigMap(v1alpha1.BaremetalAgentComponentType, zone, oc, opt), false, nil
}

func (m *baremetalManager) getPVC(oc *v1alpha1.OnecloudCluster, zone string) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.BaremetalAgent.StatefulDeploymentSpec
	return m.ComponentManager.newPVC(m.getZoneComponent(v1alpha1.BaremetalAgentComponentType, zone), oc, cfg)
}

func (m *baremetalManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	if len(zone) == 0 {
		return &oc.Status.BaremetalAgent.DeploymentStatus
	} else {
		if oc.Status.BaremetalAgent.ZoneBaremetalAgent == nil {
			oc.Status.BaremetalAgent.ZoneBaremetalAgent = make(map[string]*v1alpha1.DeploymentStatus)
		}
		_, ok := oc.Status.BaremetalAgent.ZoneBaremetalAgent[zone]
		if !ok {
			oc.Status.BaremetalAgent.ZoneBaremetalAgent[zone] = new(v1alpha1.DeploymentStatus)
		}
		return oc.Status.BaremetalAgent.ZoneBaremetalAgent[zone]
	}
}

func (m *baremetalManager) updateBaremetalSpecByNodeInfo(spec *v1alpha1.StatefulDeploymentSpec) error {
	// list nodes by baremetal node selector
	baremetalNodeSelector := labels.NewSelector()
	r, err := labels.NewRequirement(
		constants.OnecloudEanbleBaremetalLabelKey, selection.Equals, []string{"enable"})
	if err != nil {
		return err
	}
	baremetalNodeSelector = baremetalNodeSelector.Add(*r)
	nodes, err := m.nodeLister.List(baremetalNodeSelector)
	if err != nil {
		return err
	}
	klog.Infof("List nodes by baremetal selector, node count %v", len(nodes))
	if len(nodes) == 0 {
		spec.Replicas = 0
	}
	return nil
}

func (m *baremetalManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	if err := m.updateBaremetalSpecByNodeInfo(&oc.Spec.BaremetalAgent.StatefulDeploymentSpec); err != nil {
		return nil, err
	}

	cType := v1alpha1.BaremetalAgentComponentType
	zoneComponentType := m.getZoneComponent(cType, zone)
	dmSpec := &oc.Spec.BaremetalAgent.DeploymentSpec
	privileged := true
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:  cType.String(),
				Image: dmSpec.Image,
				Command: []string{
					fmt.Sprintf("/opt/yunion/bin/%s", cType.String()),
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", cType.String()),
				},
				ImagePullPolicy: dmSpec.ImagePullPolicy,
				VolumeMounts:    volMounts,
				SecurityContext: &corev1.SecurityContext{
					Privileged: &privileged,
				},
			},
		}
	}
	hostNetwork := true
	dm, err := m.newDefaultDeploymentNoInitWithoutCloudAffinity(cType, zoneComponentType, oc,
		m.newBaremetalVolHelper(
			oc, controller.ComponentConfigMapName(oc, zoneComponentType),
			zoneComponentType,
		),
		dmSpec, hostNetwork, containersF,
	)
	if err != nil {
		return nil, err
	}
	if oc.Spec.BaremetalAgent.StorageClassName != v1alpha1.DefaultStorageClass {
		dm.Spec.Strategy.Type = apps.RecreateDeploymentStrategyType
	}

	if dm.Spec.Template.Spec.NodeSelector == nil {
		dm.Spec.Template.Spec.NodeSelector = make(map[string]string)
	}
	dm.Spec.Template.Spec.NodeSelector[constants.OnecloudEanbleBaremetalLabelKey] = "enable"
	return setSelfAntiAffnity(dm, cType), nil
}

func (m *baremetalManager) newBaremetalVolHelper(oc *v1alpha1.OnecloudCluster, optCfgMap string, component v1alpha1.ComponentType) *VolumeHelper {
	volHelper := NewVolumeHelper(oc, optCfgMap, v1alpha1.BaremetalAgentComponentType)
	volHelper.volumeMounts = append(volHelper.volumeMounts,
		corev1.VolumeMount{
			Name:      "opt",
			ReadOnly:  false,
			MountPath: constants.BaremetalDataStore,
		},
	)
	volHelper.volumes = append(volHelper.volumes, corev1.Volume{
		Name: "opt",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: m.newPvcName(oc.GetName(), oc.Spec.BaremetalAgent.StorageClassName, component),
				ReadOnly:  false,
			},
		},
	})
	return volHelper
}
