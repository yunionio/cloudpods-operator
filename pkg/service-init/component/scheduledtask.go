package component

import (
	"path"

	"yunion.io/x/onecloud/pkg/scheduledtask/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewScheduledTask())
}

type scheduledTask struct {
	*baseService
}

func NewScheduledTask() Component {
	return &scheduledTask{
		baseService: newBaseService(v1alpha1.ScheduledtaskComponentType, new(options.SOption)),
	}
}
func (s *scheduledTask) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeScheduledtask); err != nil {
		return nil, err
	}
	// schedtask reuse region's database settings
	config := cfg.RegionServer

	switch oc.Spec.GetDbEngine(oc.Spec.Scheduledtask.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = constants.ScheduledtaskPort

	return opt, nil
}

func (s *scheduledTask) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.ScheduledtaskComponentType,
		constants.ServiceNameScheduledtask, constants.ServiceTypeScheduledtask,
		man.GetCluster().Spec.Scheduledtask.Service.NodePort, "",
	)
}
