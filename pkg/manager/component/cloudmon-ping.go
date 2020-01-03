package component

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type cloudmonPingManager struct {
	*ComponentManager
}

func newCloudmonPingManager(man *ComponentManager) manager.Manager {
	return &cloudmonPingManager{man}
}

func (m *cloudmonPingManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.CloudmonPing.Disable)
}

func (m *cloudmonPingManager) getCronJob(
	oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig,
) (*batchv1.CronJob, error) {
	return m.newPingCronJob(v1alpha1.CloudmonPingComponentType, oc, cfg)
}

func (m *cloudmonPingManager) newPingCronJob(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	cfg *v1alpha1.OnecloudClusterConfig,
) (*batchv1.CronJob, error) {
	spec := &oc.Spec.CloudmonPing
	spec.Schedule = "*/5 * * * *"
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
					"ping-probe",
				},
				ImagePullPolicy: oc.Spec.CloudmonPing.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
		}
	}
	volhelper := NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, configMapType), configMapType)
	return m.newDefaultCronJob(cType, oc, volhelper, *spec, nil, containersF)
}
