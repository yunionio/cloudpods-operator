package component

import (
	"fmt"
	"strings"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud/pkg/hostman/options"
)

type hostManager struct {
	*ComponentManager
}

func newHostManager(man *ComponentManager) manager.Manager {
	return &hostManager{man}
}

func (m *hostManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.HostAgent.Disable, "")
}

func (m *hostManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.HostAgent.CloudUser
}

func (m *hostManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterServiceComponent(man, constants.ServiceNameHost, constants.ServiceTypeHost)
}

func (m *hostManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	commonOpt := new(options.SHostBaseOptions)
	// opt := &options.HostOptions
	if err := SetOptionsDefault(commonOpt, ""); err != nil {
		return nil, false, err
	}
	config := cfg.HostAgent
	SetOptionsServiceTLS(&commonOpt.BaseOptions)
	SetServiceCommonOptions(&commonOpt.CommonOptions, oc, config.ServiceCommonOptions)
	commonOpt.Port = constants.HostPort

	commonOpt.DisableSecurityGroup = oc.Spec.HostAgent.DisableSecurityGroup
	commonOpt.ManageNtpConfiguration = oc.Spec.HostAgent.ManageNtpConfiguration

	return m.shouldSyncConfigmap(oc, v1alpha1.HostComponentType, commonOpt, func(oldOpt string) bool {
		for _, k := range []string{
			"enable_remote_executor",
			"disable_security_group",
			"manage_ntp_configuration",
		} {
			if !strings.Contains(oldOpt, k) {
				// hack: force update old configmap if not contains enable_remote_executor option
				return true
			}
		}
		return false
	})
}

func (m *hostManager) getDaemonSet(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.DaemonSet, error) {
	return m.newHostPrivilegedDaemonSet(v1alpha1.HostComponentType, oc, cfg)
}

func (m *hostManager) newHostPrivilegedDaemonSet(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*apps.DaemonSet, error) {
	var (
		privileged = true
		dsSpec     = oc.Spec.HostAgent
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
						Name:  "HOST_SYSTEM_SERVICES_OFF",
						Value: "host-deployer,host_sdnagent,telegraf",
					},
					{
						Name:  "OVN_CONTAINER_IMAGE_TAG",
						Value: v1alpha1.DefaultOvnImageTag,
					},
				},
				Command: []string{
					fmt.Sprintf("/opt/yunion/bin/%s", cType.String()),
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

	if dsSpec.NodeSelector == nil {
		dsSpec.NodeSelector = make(map[string]string)
	}
	dsSpec.NodeSelector[constants.OnecloudEnableHostLabelKey] = "enable"
	ds, err := m.newDaemonSet(cType, oc, cfg,
		NewHostVolume(cType, oc, configMap), dsSpec.DaemonSetSpec, "", nil, containersF)
	if err != nil {
		return nil, err
	}

	/* set host pod TerminationGracePeriodSeconds, default 30s */
	var terminationGracePeriodSecond int64 = 60 * 5
	ds.Spec.Template.Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSecond

	/* set host pod max maxUnavailable count, default 1 */
	if ds.Spec.UpdateStrategy.RollingUpdate == nil {
		ds.Spec.UpdateStrategy.RollingUpdate = new(apps.RollingUpdateDaemonSet)
	}
	var maxUnavailableCount = intstr.FromInt(3)
	ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = &maxUnavailableCount

	/* add pod label for pod affinity */
	if ds.Spec.Template.ObjectMeta.Labels == nil {
		ds.Spec.Template.ObjectMeta.Labels = make(map[string]string)
	}
	ds.Spec.Template.ObjectMeta.Labels[constants.OnecloudHostDeployerLabelKey] = ""
	if ds.Spec.Selector == nil {
		ds.Spec.Selector = &metav1.LabelSelector{}
	}
	if ds.Spec.Selector.MatchLabels == nil {
		ds.Spec.Selector.MatchLabels = make(map[string]string)
	}
	ds.Spec.Selector.MatchLabels[constants.OnecloudHostDeployerLabelKey] = ""
	return ds, nil
}
