package component

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type cloudmonReportUsageManager struct {
	*ComponentManager
}

func newCloudmonReportUsageManager(man *ComponentManager) manager.Manager {
	return &cloudmonReportUsageManager{man}
}

func (m *cloudmonReportUsageManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if !IsEnterpriseEdition(oc) {
		// TODO: DELETE cronjob
		return nil
	}
	return syncComponent(m, oc, oc.Spec.CloudmonReportUsage.Disable)
}

func (m *cloudmonReportUsageManager) getCronJob(
	oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig,
) (*batchv1.CronJob, error) {
	return m.newCronJob(v1alpha1.CloudmonReportUsageComponentType, oc, cfg)
}

func (m *cloudmonReportUsageManager) newCronJob(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*batchv1.CronJob, error) {
	spec := &oc.Spec.CloudmonReportUsage
	spec.Schedule = "*/15 * * * *"
	configMapType := v1alpha1.APIGatewayComponentType
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:  cType.String(),
				Image: spec.Image,
				Command: []string{
					"/opt/yunion/bin/cloudmon",
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", configMapType),
					"report-usage",
				},
				ImagePullPolicy: oc.Spec.CloudmonReportUsage.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
		}
	}
	volhelper := NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, configMapType), configMapType)
	return m.newDefaultCronJob(cType, oc, volhelper, *spec, nil, containersF)
}
