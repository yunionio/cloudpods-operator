package component

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type cloudmonManager struct {
	*ComponentManager
}

func newCloudMonManager(man *ComponentManager) manager.Manager {
	return &cloudmonManager{man}
}

func (m *cloudmonManager) ensureOldCronjobsDeleted(oc *v1alpha1.OnecloudCluster) error {
	for _, componentType := range []v1alpha1.ComponentType{
		v1alpha1.CloudmonPingComponentType, v1alpha1.CloudmonReportHostComponentType,
		v1alpha1.CloudmonReportServerComponentType, v1alpha1.CloudmonReportUsageComponentType,
	} {
		if _, err := m.kubeCli.BatchV1beta1().CronJobs(oc.GetNamespace()).
			Get(controller.NewClusterComponentName(oc.GetName(), componentType), metav1.GetOptions{}); err != nil && !errors.IsNotFound(err) {
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
	if !IsEnterpriseEdition(oc) {
		if _, err := m.deployLister.Deployments(oc.GetNamespace()).Get(controller.NewClusterComponentName(oc.GetName(), v1alpha1.CloudmonComponentType)); err != nil && !errors.IsNotFound(err) {
			return err
		} else if err == nil {
			if err := m.deployControl.DeleteDeployment(
				oc, controller.NewClusterComponentName(oc.GetName(), v1alpha1.CloudmonComponentType),
			); err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}
	return syncComponent(m, oc, oc.Spec.Cloudmon.Disable)
}

func (m *cloudmonManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	spec := oc.Spec.Cloudmon.DeploymentSpec
	configMap := controller.ComponentConfigMapName(oc, v1alpha1.APIGatewayComponentType)
	h := NewVolumeHelper(oc, configMap, v1alpha1.APIGatewayComponentType)
	containersF := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:  v1alpha1.CloudmonComponentType.String(),
				Image: spec.Image,
				Command: []string{"/bin/sh", "-c", fmt.Sprintf(`
					# = = = = = = = ping probe = = = = = = =
					echo '*/%d * * * * timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf ping-probe 2>&1' > /etc/crontabs/root
					# = = = = = = = report host = = = = = = =
					echo '*/%d * * * * timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-host --interval %d --provider VMware 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * * timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-host --interval %d --provider ZStack 2>&1' >> /etc/crontabs/root
					# = = = = = = = report server = = = = = = =
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-server --interval %d --provider Aliyun 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-server --interval %d --provider Huawei 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-server --interval %d --provider Qcloud 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-server --interval %d --provider Google 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-server --interval %d --provider Aws 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-server --interval %d --provider Azure 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-server --interval %d --provider VMware 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-server --interval %d --provider ZStack 2>&1' >> /etc/crontabs/root
					# = = = = = = = report usage = = = = = = =
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-usage 2>&1' >> /etc/crontabs/root
					# = = = = = = = report rds = = = = = = =
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-rds --interval %d --provider Aliyun 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-rds --interval %d --provider Huawei 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-rds --interval %d --provider Qcloud 2>&1' >> /etc/crontabs/root
					# = = = = = = = report redis = = = = = = =
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-redis --interval %d --provider Aliyun 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-redis --interval %d --provider Huawei 2>&1' >> /etc/crontabs/root
					# = = = = = = = report oss = = = = = = =
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-oss --interval %d --provider Aliyun 2>&1' >> /etc/crontabs/root
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-oss --interval %d --provider Huawei 2>&1' >> /etc/crontabs/root
					# = = = = = = = report cloudaccount = = = = = = =
					echo '*/%d * * * *  timeout  %d /opt/yunion/bin/cloudmon --config /etc/yunion/%s.conf report-cloudaccount 2>&1' >> /etc/crontabs/root
					crond -f -d 8
					`, oc.Spec.Cloudmon.CloudmonPingDuration, oc.Spec.Cloudmon.CloudmonPingDuration*60, v1alpha1.APIGatewayComponentType,

					oc.Spec.Cloudmon.CloudmonReportHostDuration, oc.Spec.Cloudmon.CloudmonReportHostDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportHostDuration, oc.Spec.Cloudmon.CloudmonReportHostDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,

					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration*3,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,

					oc.Spec.Cloudmon.CloudmonReportUsageDuration, oc.Spec.Cloudmon.CloudmonReportUsageDuration*60, v1alpha1.APIGatewayComponentType,

					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,

					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,

					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,
					oc.Spec.Cloudmon.CloudmonReportServerDuration, oc.Spec.Cloudmon.CloudmonReportServerDuration*60, v1alpha1.APIGatewayComponentType, oc.Spec.Cloudmon.CloudmonReportServerDuration,

					oc.Spec.Cloudmon.CloudmonReportCloudAccountDuration, oc.Spec.Cloudmon.CloudmonReportCloudAccountDuration*60, v1alpha1.APIGatewayComponentType,
				)},
				ImagePullPolicy: spec.ImagePullPolicy,
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeployment(
		v1alpha1.CloudmonComponentType, oc, h,
		spec, nil, containersF,
	)
}
