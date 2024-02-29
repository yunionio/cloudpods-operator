package component

import (
	"fmt"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
	"yunion.io/x/onecloud/pkg/webconsole/options"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

func init() {
	RegisterComponent(NewWebconsole())
}

type webconsole struct {
	*baseService
}

func NewWebconsole() Component {
	return &webconsole{newBaseService(v1alpha1.WebconsoleComponentType, new(options.WebConsoleOptions))}
}

func (r webconsole) BuildClusterConfigDB(clsCfg *v1alpha1.OnecloudClusterConfig, db v1alpha1.DBConfig) error {
	clsCfg.Webconsole.DB = db
	return nil
}

func (r webconsole) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.Webconsole.CloudUser = user
	return nil
}

func (r webconsole) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeWebconsole); err != nil {
		return nil, err
	}
	config := cfg.Webconsole

	switch oc.Spec.GetDbEngine(oc.Spec.Webconsole.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	opt.AutoSyncTable = true
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)

	opt.IpmitoolPath = "/usr/sbin/ipmitool"
	opt.EnableAutoLogin = true
	address := oc.Spec.LoadBalancerEndpoint
	opt.Port = constants.WebconsolePort
	// opt.ApiServer = fmt.Sprintf("https://%s:%d", address, constants.WebconsolePort)
	opt.ApiServer = fmt.Sprintf("https://%s", address)

	return opt, nil
}

func (w webconsole) GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Webconsole.DB
}

func (w webconsole) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Webconsole.CloudUser
}

func (w webconsole) GetDefaultClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Webconsole.ClickhouseConf
}

func (w webconsole) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.WebconsoleComponentType,
		constants.ServiceNameWebconsole, constants.ServiceTypeWebconsole,
		man.GetCluster().Spec.Webconsole.Service.NodePort, "")
}
