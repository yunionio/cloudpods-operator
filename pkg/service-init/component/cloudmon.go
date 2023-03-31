package component

import (
	"path"

	"yunion.io/x/onecloud/pkg/cloudmon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewCloudmon())
}

type cloudmon struct {
	*baseService
}

func NewCloudmon() Component {
	return &cloudmon{
		baseService: newBaseService(v1alpha1.CloudmonComponentType, new(options.CloudMonOptions)),
	}
}

func (r cloudmon) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.Cloudmon.CloudUser = user
	return nil
}

func (r cloudmon) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Cloudmon.CloudUser
}

func (r cloudmon) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeCloudmon); err != nil {
		return nil, err
	}
	config := cfg.Cloudmon
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config)
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = config.Port

	return opt, nil
}
func (r cloudmon) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(man,
		v1alpha1.CloudmonComponentType,
		constants.ServiceNameCloudmon,
		constants.ServiceTypeCloudmon,
		man.GetCluster().Spec.Cloudmon.Service.NodePort, "")
}
