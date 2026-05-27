package component

import (
	"yunion.io/x/onecloud/pkg/cloudcommon/options"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewAiProxy())
}

type aiProxy struct {
	*baseService
}

type AiProxyOptions struct {
	options.CommonOptions
	options.DBOptions
}

func NewAiProxy() Component {
	return &aiProxy{
		baseService: newBaseService(v1alpha1.AiProxyComponentType, new(AiProxyOptions)),
	}
}

func (r aiProxy) BuildClusterConfigDB(clsCfg *v1alpha1.OnecloudClusterConfig, db v1alpha1.DBConfig) error {
	clsCfg.AiProxy.DB = db
	return nil
}

func (r aiProxy) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.AiProxy.CloudUser = user
	return nil
}

func (r aiProxy) GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.AiProxy.DB
}

func (r aiProxy) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.AiProxy.CloudUser
}

func (r aiProxy) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &AiProxyOptions{}
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeAiProxy); err != nil {
		return nil, errors.Wrap(err, "set aiproxy option")
	}
	config := cfg.AiProxy

	switch oc.Spec.GetDbEngine(oc.Spec.AiProxy.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)
	opt.Port = config.Port
	opt.AutoSyncTable = true

	return opt, nil
}

func (r aiProxy) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponentWithSSL(man, v1alpha1.AiProxyComponentType,
		constants.ServiceNameAiProxy, constants.ServiceTypeAiProxy,
		man.GetCluster().Spec.AiProxy.Service.NodePort, "", false)
}
