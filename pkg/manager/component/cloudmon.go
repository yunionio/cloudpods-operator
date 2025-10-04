package component

import (
	"context"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
)

type cloudmonManager struct {
	*ComponentManager
}

func newCloudMonManager(man *ComponentManager) manager.Manager {
	return &cloudmonManager{man}
}

func (m *cloudmonManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *cloudmonManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.CloudmonComponentType
}

func (m *cloudmonManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Cloudmon.Disable || !isInProductVersion(m, oc)
}

func (m *cloudmonManager) ensureOldCronjobsDeleted(oc *v1alpha1.OnecloudCluster) error {
	for _, componentType := range []v1alpha1.ComponentType{
		v1alpha1.CloudmonPingComponentType, v1alpha1.CloudmonReportHostComponentType,
		v1alpha1.CloudmonReportServerComponentType, v1alpha1.CloudmonReportUsageComponentType,
	} {
		if _, err := m.kubeCli.BatchV1beta1().CronJobs(oc.GetNamespace()).
			Get(context.Background(), controller.NewClusterComponentName(oc.GetName(), componentType), metav1.GetOptions{}); err != nil && !errors.IsNotFound(err) {
			return err
		} else if err == nil {
			err := m.cronControl.DeleteCronJob(oc, controller.NewClusterComponentName(oc.GetName(), componentType))
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (m *cloudmonManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if err := m.ensureOldCronjobsDeleted(oc); err != nil {
		return err
	}
	return syncComponent(m, oc, "")
}

func (m *cloudmonManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Cloudmon.CloudUser
}

func (m *cloudmonManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt, err := component.NewCloudmon().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}
	return m.newServiceConfigMap(v1alpha1.CloudmonComponentType, "", oc, opt), false, nil
}

func (m *cloudmonManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return component.NewCloudmon().GetPhaseControl(man)
}

func (m *cloudmonManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSinglePortService(v1alpha1.CloudmonComponentType, oc, oc.Spec.Cloudmon.Service.InternalOnly, int32(oc.Spec.Cloudmon.Service.NodePort), int32(cfg.Cloudmon.Port))}
}

func (m *cloudmonManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	spec := oc.Spec.Cloudmon.DeploymentSpec
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            v1alpha1.CloudmonComponentType.String(),
				Image:           spec.Image,
				Command:         []string{"/opt/yunion/bin/cloudmon", "--config", "/etc/yunion/cloudmon.conf"},
				ImagePullPolicy: spec.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
		}
	}
	deployment, err := m.newDefaultDeployment(v1alpha1.CloudmonComponentType, "", oc,
		NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.CloudmonComponentType), v1alpha1.CloudmonComponentType), &spec, nil,
		containersF)
	if err != nil {
		return deployment, err
	}
	oc.Spec.Cloudmon.DeploymentSpec = spec
	return deployment, err
}

func (m *cloudmonManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Cloudmon
}
