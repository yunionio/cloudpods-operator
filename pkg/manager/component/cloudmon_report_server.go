package component

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type cloudmonReportServerManager struct {
	*ComponentManager
}

func (m *cloudmonReportServerManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if !IsEnterpriseEdition(oc) {
		// TODO: DELETE cronjob
		return nil
	}
	return syncComponent(m, oc, oc.Spec.CloudmonReportServer.Disable)
}

func newCloudmonReportServerManager(man *ComponentManager) manager.Manager {
	return &cloudmonReportServerManager{man}
}

func (m *cloudmonReportServerManager) getCronJob(
	oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig,
) (*batchv1.CronJob, error) {
	return m.newCronJob(v1alpha1.CloudmonReportServerComponentType, oc, cfg)
}

func (m *cloudmonReportServerManager) newCronJob(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*batchv1.CronJob, error) {
	spec := &oc.Spec.CloudmonReportServer
	spec.Schedule = "*/15 * * * *"
	configMapType := v1alpha1.APIGatewayComponentType
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:  cType.String() + "-aliyun",
				Image: spec.Image,
				Command: []string{
					"/opt/yunion/bin/cloudmon",
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", configMapType),
					"report-server",
					"--interval",
					"15",
					"--provider",
					"Aliyun",
				},
				ImagePullPolicy: oc.Spec.CloudmonReportServer.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
			{
				Name:  cType.String() + "-huawei",
				Image: spec.Image,
				Command: []string{
					"/opt/yunion/bin/cloudmon",
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", configMapType),
					"report-server",
					"--interval",
					"15",
					"--provider",
					"Huawei",
				},
				ImagePullPolicy: oc.Spec.CloudmonReportServer.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
			{
				Name:  cType.String() + "-qcloud",
				Image: spec.Image,
				Command: []string{
					"/opt/yunion/bin/cloudmon",
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", configMapType),
					"report-server",
					"--interval",
					"15",
					"--provider",
					"Qcloud",
				},
				ImagePullPolicy: oc.Spec.CloudmonReportServer.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
			{
				Name:  cType.String() + "-google",
				Image: spec.Image,
				Command: []string{
					"/opt/yunion/bin/cloudmon",
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", configMapType),
					"report-server",
					"--interval",
					"15",
					"--provider",
					"Google",
				},
				ImagePullPolicy: oc.Spec.CloudmonReportServer.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
			{
				Name:  cType.String() + "-aws",
				Image: spec.Image,
				Command: []string{
					"/opt/yunion/bin/cloudmon",
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", configMapType),
					"report-server",
					"--interval",
					"15",
					"--provider",
					"Aws",
				},
				ImagePullPolicy: oc.Spec.CloudmonReportServer.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
			{
				Name:  cType.String() + "-azure",
				Image: spec.Image,
				Command: []string{
					"/opt/yunion/bin/cloudmon",
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", configMapType),
					"report-server",
					"--interval",
					"15",
					"--provider",
					"Azure",
				},
				ImagePullPolicy: oc.Spec.CloudmonReportServer.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
			{
				Name:  cType.String() + "-vmware",
				Image: spec.Image,
				Command: []string{
					"/opt/yunion/bin/cloudmon",
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", configMapType),
					"report-server",
					"--interval",
					"15",
					"--provider",
					"VMware",
				},
				ImagePullPolicy: oc.Spec.CloudmonReportServer.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
			{
				Name:  cType.String() + "-zstack",
				Image: spec.Image,
				Command: []string{
					"/opt/yunion/bin/cloudmon",
					"--config",
					fmt.Sprintf("/etc/yunion/%s.conf", configMapType),
					"report-server",
					"--interval",
					"15",
					"--provider",
					"ZStack",
				},
				ImagePullPolicy: oc.Spec.CloudmonReportServer.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
		}
	}
	volhelper := NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, configMapType), configMapType)
	return m.newDefaultCronJob(cType, oc, volhelper, *spec, nil, containersF)
}
