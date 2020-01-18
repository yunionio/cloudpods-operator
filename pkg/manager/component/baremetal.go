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
)

type baremetalManager struct {
	*ComponentManager
}

func newBaremetalManager(m *ComponentManager) manager.Manager {
	return &baremetalManager{m}
}

func (m *baremetalManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.BaremetalAgent.Disable)
}

func (m *baremetalManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.BaremetalAgent.CloudUser
}

func (m *baremetalManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterServiceComponent(man, constants.ServiceNameBaremetal, constants.ServiceTypeBaremetal)
}

func (m *baremetalManager) getConfigMap(
	oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig,
) (*corev1.ConfigMap, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, ""); err != nil {
		return nil, err
	}
	config := cfg.BaremetalAgent
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.Port = constants.BaremetalPort
	return m.newServiceConfigMap(v1alpha1.BaremetalAgentComponentType, oc, opt), nil
}

func (m *baremetalManager) getPVC(oc *v1alpha1.OnecloudCluster) (*corev1.PersistentVolumeClaim, error) {
	cfg := oc.Spec.BaremetalAgent
	return m.ComponentManager.newPVC(v1alpha1.BaremetalAgentComponentType, oc, cfg)
}

func (m *baremetalManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.BaremetalAgent
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

func (m *baremetalManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig,
) (*apps.Deployment, error) {
	if err := m.updateBaremetalSpecByNodeInfo(&oc.Spec.BaremetalAgent); err != nil {
		return nil, err
	}

	cType := v1alpha1.BaremetalAgentComponentType
	dmSpec := oc.Spec.BaremetalAgent.DeploymentSpec
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
	dm, err := m.newDefaultDeploymentNoInitWithoutCloudAffinity(cType, oc,
		newBaremetalVolHelper(
			oc, controller.ComponentConfigMapName(oc, v1alpha1.BaremetalAgentComponentType),
			v1alpha1.BaremetalAgentComponentType,
		),
		dmSpec, hostNetwork, containersF,
	)
	if err != nil {
		return nil, err
	}
	if dm.Spec.Template.Spec.NodeSelector == nil {
		dm.Spec.Template.Spec.NodeSelector = make(map[string]string)
	}
	dm.Spec.Template.Spec.NodeSelector[constants.OnecloudEanbleBaremetalLabelKey] = "enable"
	return dm, nil
}

func newBaremetalVolHelper(oc *v1alpha1.OnecloudCluster, optCfgMap string, component v1alpha1.ComponentType) *VolumeHelper {
	volHelper := NewVolumeHelper(oc, optCfgMap, component)
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
				ClaimName: controller.NewClusterComponentName(oc.GetName(), v1alpha1.BaremetalAgentComponentType),
				ReadOnly:  false,
			},
		},
	})
	return volHelper
}
