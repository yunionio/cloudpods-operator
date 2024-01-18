package component

import (
	"path"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

const hostRoot = "/hostfs"

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
	}
}

func (m *telegrafManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.TelegrafComponentType
}

func (m *telegrafManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.Telegraf.Disable, "")
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
		cType, oc, cfg, NewTelegrafVolume(cType, oc), dsSpec.DaemonSetSpec,
		"", initContainers, containersF,
	)
	ds.Spec.Template.Spec.HostPID = false
	ds.Spec.Template.Spec.ServiceAccountName = constants.ServiceAccountOnecloudOperator
	if err != nil {
		return nil, err
	}
	return ds, nil
}

func NewTelegrafVolume(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
) *VolumeHelper {
	var h = &VolumeHelper{
		cluster:      oc,
		component:    cType,
		volumes:      make([]corev1.Volume, 0),
		volumeMounts: make([]corev1.VolumeMount, 0),
	}
	var bidirectional = corev1.MountPropagationBidirectional
	h.volumeMounts = append(h.volumeMounts, []corev1.VolumeMount{
		{
			Name:      "etc-telegraf",
			ReadOnly:  false,
			MountPath: "/etc/telegraf",
		},
		{
			Name:      "proc",
			ReadOnly:  true,
			MountPath: path.Join(hostRoot, "/proc"),
		},
		{
			Name:      "sys",
			ReadOnly:  true,
			MountPath: path.Join(hostRoot, "/sys"),
		},
		{
			Name:             "root",
			ReadOnly:         true,
			MountPath:        hostRoot,
			MountPropagation: &bidirectional,
		},
		{
			Name:             "run",
			ReadOnly:         false,
			MountPath:        "/var/run",
			MountPropagation: &bidirectional,
		},
		{
			Name:      "dev",
			ReadOnly:  false,
			MountPath: "/dev",
		},
		{
			Name:             "cloud",
			ReadOnly:         false,
			MountPath:        "/opt/cloud",
			MountPropagation: &bidirectional,
		},
	}...)

	var volSrcType = corev1.HostPathDirectoryOrCreate
	var hostPathDirectory = corev1.HostPathDirectory
	h.volumes = append(h.volumes, []corev1.Volume{
		{
			Name: "etc-telegraf",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/telegraf",
					Type: &volSrcType,
				},
			},
		},
		{
			Name: "proc",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/proc",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "run",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "sys",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/sys",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "root",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "dev",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/dev",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "cloud",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/opt/cloud",
					Type: &hostPathDirectory,
				},
			},
		},
	}...)
	return h
}
