package component

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

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

func (m *baremetalManager) getConfigMap(
	oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig,
) (*corev1.ConfigMap, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeBaremetal); err != nil {
		return nil, err
	}
	config := cfg.BaremetalAgent
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.Port = constants.BaremetalPort
	return m.newServiceConfigMap(v1alpha1.BaremetalAgentComponentType, oc, opt), nil
}

func (m *baremetalManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.BaremetalAgent
}

func (m *baremetalManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig,
) (*apps.Deployment, error) {
	if err := ensureOptCloudExist(); err != nil {
		return nil, err
	}
	cType := v1alpha1.BaremetalAgentComponentType
	dmSpec := oc.Spec.BaremetalAgent
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
	dm, err := m.newDefaultDeploymentNoInit(cType, oc,
		newBaremetalVolHelper(
			oc, controller.ComponentConfigMapName(oc, v1alpha1.BaremetalAgentComponentType),
			v1alpha1.BaremetalAgentComponentType,
		),
		dmSpec, containersF,
	)
	if err != nil {
		return nil, err
	}
	// set braemetal use host network
	dm.Spec.Template.Spec.HostNetwork = true
	dm.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	return dm, nil
}

func newBaremetalVolHelper(oc *v1alpha1.OnecloudCluster, optCfgMap string, component v1alpha1.ComponentType) *VolumeHelper {
	volHelper := NewVolumeHelper(oc, optCfgMap, component)
	var hostPathDirectory = corev1.HostPathDirectory
	volHelper.volumeMounts = append(volHelper.volumeMounts,
		corev1.VolumeMount{
			Name:      "opt",
			ReadOnly:  false,
			MountPath: "/opt/cloud/workspace",
		},
	)
	volHelper.volumes = append(volHelper.volumes,
		corev1.Volume{
			Name: "opt",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/opt/cloud/workspace",
					Type: &hostPathDirectory,
				},
			},
		},
	)
	return volHelper
}
