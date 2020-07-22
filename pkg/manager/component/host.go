package component

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud/pkg/cloudcommon/options"
)

type hostManager struct {
	*ComponentManager
}

func newHostManager(man *ComponentManager) manager.Manager {
	return &hostManager{man}
}

func (m *hostManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.HostAgent.Disable)
}

func (m *hostManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.HostAgent.CloudUser
}

func (m *hostManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterServiceComponent(man, constants.ServiceNameHost, constants.ServiceTypeHost)
}

func (m *hostManager) getConfigMap(
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*corev1.ConfigMap, error) {
	commonOpt := new(options.CommonOptions)
	// opt := &options.HostOptions
	if err := SetOptionsDefault(commonOpt, ""); err != nil {
		return nil, err
	}
	config := cfg.HostAgent
	SetOptionsServiceTLS(&commonOpt.BaseOptions)
	SetServiceCommonOptions(commonOpt, oc, config.ServiceCommonOptions)
	commonOpt.Port = constants.HostPort

	return m.newServiceConfigMap(v1alpha1.HostComponentType, oc, commonOpt), nil
}

func (m *hostManager) getDaemonSet(
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*apps.DaemonSet, error) {
	return m.newHostPrivilegedDaemonSet(v1alpha1.HostComponentType, oc, cfg)
}

func (m *hostManager) newHostPrivilegedDaemonSet(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*apps.DaemonSet, error) {
	var (
		privileged  = true
		dsSpec      = oc.Spec.HostAgent
		configMap   = controller.ComponentConfigMapName(oc, cType)
		containersF = func(volMounts []corev1.VolumeMount) []corev1.Container {
			return []corev1.Container{
				{
					Name:            cType.String(),
					Image:           dsSpec.Image,
					ImagePullPolicy: dsSpec.ImagePullPolicy,
					Env: []corev1.EnvVar{
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
				{
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
				},
				{
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
				},
			}
		}
	)
	if dsSpec.NodeSelector == nil {
		dsSpec.NodeSelector = make(map[string]string)
	}
	dsSpec.NodeSelector[constants.OnecloudEnableHostLabelKey] = "enable"
	ds, err := m.newDaemonSet(cType, oc, cfg,
		NewHostVolume(cType, oc, configMap), dsSpec.DaemonSetSpec, "", nil, containersF)
	if err != nil {
		return nil, err
	}

	// add pod label for pod affinity
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
