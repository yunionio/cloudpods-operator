package component

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/option"
	//"yunion.io/x/onecloud-operator/pkg/service-init/component"
)

type fsgatewayManager struct {
	*ComponentManager
}

func newFsGatewayManager(man *ComponentManager) manager.Manager {
	return &fsgatewayManager{man}
}

func (m *fsgatewayManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
	}
}

func (m *fsgatewayManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.FsgatewayComponentType
}

func (m *fsgatewayManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.FsGateway.Disable, "")
}

func (m *fsgatewayManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.FsGateway.CloudUser
}

func (m *fsgatewayManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &options.CommonOptions{}
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeFsGateway); err != nil {
		return nil, false, err
	}
	config := cfg.FsGateway
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(opt, oc, config)
	opt.Port = oc.Spec.FsGateway.Service.NodePort
	return m.newServiceConfigMap(v1alpha1.FsgatewayComponentType, "", oc, opt), false, nil
}

func (m *fsgatewayManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return man.FsGateway()
}

func (m *fsgatewayManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.FsgatewayComponentType, oc, int32(oc.Spec.FsGateway.Service.NodePort), int32(cfg.FsGateway.Port))}
}

func (m *fsgatewayManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeploymentWithReadinessProbePath(
		v1alpha1.FsgatewayComponentType, "", oc, &oc.Spec.FsGateway.DeploymentSpec,
		constants.FsGatewayPort, false, false, "/ping")
	if err != nil {
		return nil, err
	}
	pod := &deploy.Spec.Template.Spec
	podVols := pod.Volumes
	volMounts := pod.Containers[0].VolumeMounts
	var privileged = true
	pod.Containers[0].SecurityContext = &corev1.SecurityContext{
		Privileged: &privileged,
	}
	var (
		hostPathDirOrCreate = corev1.HostPathDirectoryOrCreate
		mountMode           = corev1.MountPropagationBidirectional
	)
	podVols = append(podVols, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: constants.FsGatwayMountPath,
				Type: &hostPathDirOrCreate,
			},
		},
	})
	volMounts = append(volMounts, corev1.VolumeMount{
		Name:             "data",
		MountPath:        constants.FsGatwayMountPath,
		MountPropagation: &mountMode,
	})
	if oc.Spec.Glance.StorageClassName != v1alpha1.DefaultStorageClass {
		deploy.Spec.Strategy.Type = apps.RecreateDeploymentStrategyType
	}

	pod.Containers[0].VolumeMounts = volMounts
	pod.Volumes = podVols
	return deploy, err
}

func (m *fsgatewayManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.FsGateway
}
