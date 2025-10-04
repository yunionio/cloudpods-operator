package component

import (
	"path"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

const (
	hostRoot           = "/hostfs"
	defaultTelegrafDir = "/etc/telegraf"
)

type telegrafManager struct {
	*ComponentManager
}

func newTelegrafManager(man *ComponentManager) manager.Manager {
	return &telegrafManager{ComponentManager: man}
}

func (m *telegrafManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *telegrafManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.TelegrafComponentType
}

func (m *telegrafManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Telegraf.Disable || !isInProductVersion(m, oc)
}

func (m *telegrafManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *telegrafManager) getDaemonSet(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.DaemonSet, error) {
	return m.newTelegrafDaemonSet(v1alpha1.TelegrafComponentType, oc, cfg)
}

func (m *telegrafManager) newTelegrafDaemonSet(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*apps.DaemonSet, error) {
	dsSpec := oc.Spec.Telegraf
	privileged := true

	telegrafConfigDir := defaultTelegrafDir
	if dsSpec.TelegrafConfigDir != "" {
		telegrafConfigDir = dsSpec.TelegrafConfigDir
	}

	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		cs := []corev1.Container{
			{
				Name:            cType.String(),
				Image:           dsSpec.Image, // TODO: set default_image
				ImagePullPolicy: dsSpec.ImagePullPolicy,
				Env: []corev1.EnvVar{
					{
						Name:  "HOST_ETC",
						Value: path.Join(hostRoot, "/etc"),
					},
					{
						Name:  "HOST_PROC",
						Value: path.Join(hostRoot, "/proc"),
					},
					{
						Name:  "HOST_SYS",
						Value: path.Join(hostRoot, "/sys"),
					},
					{
						Name:  "HOST_VAR",
						Value: path.Join(hostRoot, "/var"),
					},
					{
						Name:  "HOST_RUN",
						Value: path.Join(hostRoot, "/run"),
					},
					{
						Name:  "HOST_MOUNT_PREFIX",
						Value: hostRoot,
					},
				},
				Args: []string{
					"/usr/bin/telegraf",
					"-config", "/etc/telegraf/telegraf.conf",
					"-config-directory", "/etc/telegraf/telegraf.d",
				},
				VolumeMounts: volMounts,
				SecurityContext: &corev1.SecurityContext{
					Privileged: &privileged,
				},
			},
		}
		if oc.Spec.Telegraf.EnableRaidPlugin {
			cs = append(cs, corev1.Container{
				Name:            "telegraf-raid-plugin",
				Image:           dsSpec.TelegrafRaid.Image,
				ImagePullPolicy: dsSpec.TelegrafRaid.ImagePullPolicy,
				Command: []string{
					"/opt/yunion/bin/telegraf-raid-plugin",
				},
				VolumeMounts: volMounts,
				SecurityContext: &corev1.SecurityContext{
					Privileged: &privileged,
				},
				Env: []corev1.EnvVar{
					{
						Name: "NODENAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "spec.nodeName",
							},
						},
					},
				},
			})
		}
		return cs
	}
	initContainers := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            cType.String() + "-init",
				Image:           dsSpec.InitContainerImage,
				ImagePullPolicy: dsSpec.ImagePullPolicy,
				Command:         []string{"/bin/telegraf-init"},
				VolumeMounts:    volMounts,
				Env: []corev1.EnvVar{
					{
						Name: "NODENAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "spec.nodeName",
							},
						},
					},
					{
						Name:  "INFLUXDB_URL",
						Value: getInfluxDBInternalURL(oc),
					},
				},
			},
		}
	}([]corev1.VolumeMount{{
		Name:      "etc-telegraf",
		ReadOnly:  false,
		MountPath: "/etc/telegraf",
	}})
	ds, err := m.newDaemonSet(
		cType, oc, cfg, NewTelegrafVolume(cType, oc, telegrafConfigDir), dsSpec.DaemonSetSpec,
		"", initContainers, containersF,
	)
	if err != nil {
		return nil, errors.Wrap(err, "new daemonset")
	}
	ds.Spec.Template.Spec.HostPID = false
	ds.Spec.Template.Spec.ServiceAccountName = constants.ServiceAccountOnecloudOperator
	/* set host pod max maxUnavailable count, default 1 */
	if ds.Spec.UpdateStrategy.RollingUpdate == nil {
		ds.Spec.UpdateStrategy.RollingUpdate = new(apps.RollingUpdateDaemonSet)
	}
	var maxUnavailableCount = intstr.FromInt(20)
	ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = &maxUnavailableCount
	return ds, nil
}

func NewTelegrafVolume(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	telegrafConfigDir string,
) *VolumeHelper {
	var h = &VolumeHelper{
		cluster:      oc,
		component:    cType,
		volumes:      make([]corev1.Volume, 0),
		volumeMounts: make([]corev1.VolumeMount, 0),
	}
	h.volumeMounts = append(h.volumeMounts,
		corev1.VolumeMount{
			Name:      "etc-telegraf",
			ReadOnly:  false,
			MountPath: "/etc/telegraf",
		},
		corev1.VolumeMount{
			Name:      "root",
			ReadOnly:  true,
			MountPath: hostRoot,
		},
	)

	var volSrcType = corev1.HostPathDirectoryOrCreate
	var hostPathDirectory = corev1.HostPathDirectory
	h.volumes = append(h.volumes,
		corev1.Volume{
			Name: "etc-telegraf",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: telegrafConfigDir,
					Type: &volSrcType,
				},
			},
		},
		corev1.Volume{
			Name: "root",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/",
					Type: &hostPathDirectory,
				},
			},
		},
	)
	return h
}
