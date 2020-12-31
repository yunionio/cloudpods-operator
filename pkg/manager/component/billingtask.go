package component

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

type billingTaskManager struct {
	*ComponentManager
}

func newBillingTaskManager(man *ComponentManager) manager.Manager {
	return &billingTaskManager{man}
}

func (m *billingTaskManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.BillingTask.Disable, "")
}

func (m *billingTaskManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return nil
}

func (m *billingTaskManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.BillingTask.CloudUser
}

func (m *billingTaskManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.BillingTaskComponentType,
		constants.ServiceNameBillingTask, constants.ServiceTypeBillingTask,
		constants.BillingTaskPort, "")
}

type AliPayOptions struct {
	AlipayAppId       string `help:"APPID" default:"20002020"`
	AlipayNotifyUrl   string `help:"支付成功回调地址.支付宝服务器主动通知商户服务器里指定的页面http/https路径。" default:"https://api.yunion.cn/api/v2/paymentnotify/alipay"`
	AlipayReturnUrl   string `help:"支付成功后跳回指定页面http/https路径" default:""`
	AlipayExpiredTime int    `help:"支付超时时间(分钟)" default:"5"`
	AlipayDebug       bool   `help:"Debug" default:"false"`
}

type billingTaskOptions struct {
	common_options.CommonOptions
	common_options.DBOptions
	AliPayOptions

	BillingPaymentStatusSyncIntervals int `help:"billing payment status sync intervals(seconds)" default:"60"`
}

func (m *billingTaskManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &billingTaskOptions{}
	if err := SetOptionsDefault(opt, constants.ServiceTypeBillingTask); err != nil {
		return nil, false, err
	}
	config := cfg.BillingTask
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.Port = constants.BillingTaskPort

	return m.newServiceConfigMap(v1alpha1.BillingTaskComponentType, "", oc, opt), false, nil
}

func (m *billingTaskManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.BillingTaskComponentType, oc, constants.BillingTaskPort)}
}

func (m *billingTaskManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.BillingTaskComponentType, "", oc, oc.Spec.BillingTask, constants.BillingTaskPort, false, false)
	if err != nil {
		return nil, err
	}

	// insert AlipayCerts volume
	alipayVol := corev1.Volume{
		Name: "alipay-certs",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: constants.BillingTaskSecret,
				Items: []corev1.KeyToPath{
					{Key: constants.AlipayCertPublicKeyRSA2, Path: constants.AlipayCertPublicKeyRSA2},
					{Key: constants.AlipayRootCert, Path: constants.AlipayRootCert},
					{Key: constants.AlipayAppCertPublicKey, Path: constants.AlipayAppCertPublicKey},
					{Key: constants.YunionCsr, Path: constants.YunionCsr},
					{Key: constants.YunionPublic, Path: constants.YunionPublic},
					{Key: constants.YunionPrivate, Path: constants.YunionPrivate},
				},
			},
		},
	}

	// mount AlipayCerts
	alipayVolMount := corev1.VolumeMount{
		Name:      "alipay-certs",
		ReadOnly:  true,
		MountPath: constants.AlipayCertDir,
	}

	// update Deployment
	spec := &deploy.Spec.Template.Spec
	billingtask := &spec.Containers[0]
	billingtask.VolumeMounts = append(billingtask.VolumeMounts, alipayVolMount)
	spec.Volumes = append(spec.Volumes, alipayVol)
	return deploy, nil
}

func (m *billingTaskManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.BillingTask
}
