package component

import (
	"yunion.io/x/onecloud/pkg/apimap/options"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type apiMap struct {
	*baseService
}

func NewAPIMap() Component {
	return &apiMap{newBaseService(v1alpha1.APIMapComponentType, new(options.SOptions))}
}

func (am *apiMap) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := options.GetOptions()
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeAPIMap); err != nil {
		return nil, errors.Wrap(err, "apimap: SetOptionsDefault")
	}
	// apimap use region config directly
	config := cfg.RegionServer

	switch oc.Spec.GetDbEngine(oc.Spec.APIMap.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceDBCommonOptions.ServiceCommonOptions)

	opt.Port = constants.APIMapPort
	return opt, nil
}

func (am *apiMap) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man, v1alpha1.APIMapComponentType, constants.ServiceNameAPIMap, constants.ServiceTypeAPIMap, man.GetCluster().Spec.APIMap.Service.NodePort, "")
}
