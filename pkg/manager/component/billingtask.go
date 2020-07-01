package component

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type billingTaskManager struct {
	*ComponentManager
}

func newBillingTaskManager(man *ComponentManager) manager.Manager {
	return &billingTaskManager{man}
}

func (b *billingTaskManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
	}
}

func (b *billingTaskManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.BillingTaskComponentType
}

func (m *billingTaskManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *billingTaskManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.BillingTask.Disable
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
	AlipayCertPath    string `help:"支付宝证书文件所在路径" default:"/etc/yunion/alipay"`
	AlipayNotifyUrl   string `help:"支付成功回调地址.支付宝服务器主动通知商户服务器里指定的页面http/https路径。" default:"https://api.yunion.cn/api/v2/paymentnotify/alipay"`
	AlipayReturnUrl   string `help:"支付成功后跳回指定页面http/https路径" default:""`
	AlipayExpiredTime int    `help:"支付超时时间(分钟)" default:"5"`
	AlipayDebug       bool   `help:"Debug" default:"false"`
}

type billingTaskOptions struct {
	common_options.CommonOptions
	AliPayOptions

	BillingPaymentStatusSyncIntervals int `help:"billing payment status sync intervals(seconds)" default:"60"`
}

func (m *billingTaskManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &billingTaskOptions{}
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeBillingTask); err != nil {
		return nil, false, err
	}
	config := cfg.BillingTask
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config)
	opt.Port = constants.BillingTaskPort

	return m.newServiceConfigMap(v1alpha1.BillingTaskComponentType, "", oc, opt), false, nil
}

func (m *billingTaskManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.BillingTaskComponentType, oc, constants.BillingTaskPort, int32(cfg.BillingTask.Port))}
}

func (m *billingTaskManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	return m.newCloudServiceSinglePortDeployment(v1alpha1.BillingTaskComponentType, "", oc, &oc.Spec.BillingTask.DeploymentSpec, constants.BillingTaskPort, true, false)
}

func (m *billingTaskManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.BillingTask
}
