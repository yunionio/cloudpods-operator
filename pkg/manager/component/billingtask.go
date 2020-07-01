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
	return syncComponent(m, oc, oc.Spec.BillingTask.Disable)
}

func (m *billingTaskManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return nil
}

func (m *billingTaskManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.BillingTask.CloudUser
}

func (m *billingTaskManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.BillingTaskComponentType,
		constants.ServiceNameBillingTask, constants.ServiceTypeBillingTask,
		constants.BillingTaskPort, "")
}

type aliPayOptions struct {
	AlipayAppId       string `help:"APPID"`
	AlipayCertPath    string `help:"支付宝证书文件所在路径" default:"/etc/yunion/alipay"`
	AlipayNotifyUrl   string `help:"支付成功回调地址.支付宝服务器主动通知商户服务器里指定的页面http/https路径。" default:"https://api.yunion.cn/api/v2/paymentnotify/alipay"`
	AlipayReturnUrl   string `help:"支付成功后跳回指定页面http/https路径" default:""`
	AlipayExpiredTime int    `help:"支付超时时间(分钟)" default:"5"`
	AlipayDebug       bool   `help:"Debug" default:"false"`
}

type billingTaskOptions struct {
	common_options.CommonOptions
	aliPayOptions

	BillingPaymentStatusSyncIntervals int `help:"billing payment status sync intervals(seconds)" default:"60"`
}

func (m *billingTaskManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	opt := &billingTaskOptions{}
	if err := SetOptionsDefault(opt, constants.ServiceTypeBillingTask); err != nil {
		return nil, err
	}
	config := cfg.BillingTask
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config)
	opt.Port = constants.BillingTaskPort

	return m.newServiceConfigMap(v1alpha1.BillingTaskComponentType, oc, opt), nil
}

func (m *billingTaskManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service {
	return m.newSingleNodePortService(v1alpha1.BillingTaskComponentType, oc, constants.BillingTaskPort)
}

func (m *billingTaskManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.BillingTaskComponentType, oc, oc.Spec.Billing, constants.BillingTaskPort, true)
}

func (m *billingTaskManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster) *v1alpha1.DeploymentStatus {
	return &oc.Status.BillingTask
}
