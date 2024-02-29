package component

import (
	"yunion.io/x/onecloud/pkg/scheduler/options"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewScheduler())
}

type scheduler struct {
	*baseService
}

func NewScheduler() Component {
	return &scheduler{newBaseService(v1alpha1.SchedulerComponentType, new(options.SchedulerOptions))}
}

func (s *scheduler) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeScheduler); err != nil {
		return nil, errors.Wrap(err, "scheduler: SetOptionsDefault")
	}
	// scheduler use region config directly
	config := cfg.RegionServer

	switch oc.Spec.GetDbEngine(oc.Spec.Scheduler.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceDBCommonOptions.ServiceCommonOptions)

	opt.SchedulerPort = constants.SchedulerPort
	return opt, nil
}

func (s *scheduler) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.SchedulerComponentType,
		constants.ServiceNameScheduler, constants.ServiceTypeScheduler, man.GetCluster().Spec.Scheduler.Service.NodePort, "")
}
