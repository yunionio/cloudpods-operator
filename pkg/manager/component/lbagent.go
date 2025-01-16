package component

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/lbagent"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type lbagentManager struct {
	*ComponentManager
}

func newLbagentManager(man *ComponentManager) manager.Manager {
	return &lbagentManager{man}
}

func (m *lbagentManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *lbagentManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.LbagentComponentType
}

func (m *lbagentManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Lbagent.Disable
}

func (m *lbagentManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *lbagentManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Lbagent.CloudUser
}

func (m *lbagentManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterServiceComponent(man, constants.ServiceNameLbagent, constants.ServiceTypeLbagent)
}

func (m *lbagentManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	commonOpt := new(lbagent.LbagentCommonOptions)
	if err := option.SetOptionsDefault(commonOpt, ""); err != nil {
		return nil, false, err
	}
	config := cfg.Lbagent
	option.SetOptionsServiceTLS(&commonOpt.BaseOptions, false)
	option.SetServiceCommonOptions(&commonOpt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)
	commonOpt.Port = constants.LbagentPort
	commonOpt.BaseDataDir = "/opt/cloud/workspace/lbagent"
	commonOpt.EnableRemoteExecutor = true
	commonOpt.DisableLocalVpc = oc.Spec.DisableLocalVpc

	return m.shouldSyncConfigmap(oc, v1alpha1.LbagentComponentType, commonOpt, func(oldOpt string) bool {
		return false
	})
}

func (m *lbagentManager) getDaemonSet(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.DaemonSet, error) {
	return m.newLbagentPrivilegedDaemonSet(v1alpha1.LbagentComponentType, oc, cfg)
}

func (m *lbagentManager) newLbagentPrivilegedDaemonSet(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*apps.DaemonSet, error) {
	var (
		privileged = true
		dsSpec     = oc.Spec.Lbagent
		configMap  = controller.ComponentConfigMapName(oc, cType)
	)
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		containers := []corev1.Container{
			{
				Name:            cType.String(),
				Image:           dsSpec.Image,
				ImagePullPolicy: dsSpec.ImagePullPolicy,
				Env: []corev1.EnvVar{
					{
						Name:  "HOST_OVN_ENCAP_IP_DETECTION_METHOD",
						Value: oc.Spec.HostAgent.OvnEncapIpDetectionMethod,
					},
					{
						Name: "HOST_OVN_SOUTH_DATABASE",
						Value: fmt.Sprintf("tcp:%s:%d",
							controller.NewClusterComponentName(
								oc.GetName(),
								v1alpha1.OvnNorthComponentType,
							),
							constants.OvnSouthDbPort,
						),
					},
					{
						Name:  "OVN_CONTAINER_IMAGE_TAG",
						Value: v1alpha1.DefaultOvnImageTag,
					},
				},
				Command: []string{
					fmt.Sprintf("/opt/yunion/bin/%s", cType.String()),
					"--config",
					"/etc/yunion/lbagent.conf",
					"--common-config-file",
					"/etc/yunion/common/common.conf",
				},
				VolumeMounts: volMounts,
				SecurityContext: &corev1.SecurityContext{
					Privileged: &privileged,
				},
				WorkingDir: "/opt/cloud",
			},
		}
		if !oc.Spec.DisableLocalVpc {
			containers = append(containers, corev1.Container{
				Name:            "ovn-controller",
				Image:           dsSpec.OvnController.Image,
				ImagePullPolicy: dsSpec.OvnController.ImagePullPolicy,
				Command:         []string{"/start.sh", "controller"},
				VolumeMounts:    NewOvsVolumeHelper(cType, oc, configMap).GetVolumeMounts(),
				SecurityContext: &corev1.SecurityContext{
					Privileged: &privileged,
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{
							corev1.Capability("SYS_NICE"),
						},
					},
				},
			})
		}
		return containers
	}

	if dsSpec.NodeSelector == nil {
		dsSpec.NodeSelector = make(map[string]string)
	}
	dsSpec.NodeSelector[constants.OnecloudEnableLbagentLabelKey] = "enable"
	ds, err := m.newDaemonSet(cType, oc, cfg,
		NewHostVolume(cType, oc, configMap), dsSpec.DaemonSetSpec, "", nil, containersF)
	if err != nil {
		return nil, errors.Wrap(err, "newDaemonSet")
	}

	/* set lbagent pod TerminationGracePeriodSeconds, default 30s */
	var terminationGracePeriodSecond int64 = 60 * 5
	ds.Spec.Template.Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSecond

	/* set lbagent pod max maxUnavailable count, default 1 */
	if ds.Spec.UpdateStrategy.RollingUpdate == nil {
		ds.Spec.UpdateStrategy.RollingUpdate = new(apps.RollingUpdateDaemonSet)
	}
	var maxUnavailableCount = intstr.FromInt(3)
	ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = &maxUnavailableCount

	return ds, nil
}
