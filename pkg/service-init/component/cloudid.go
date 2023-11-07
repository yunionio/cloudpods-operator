package component

import (
	"path"

	"yunion.io/x/onecloud/pkg/cloudid/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewCloudId())
}

type cloudid struct {
	*baseService
}

func NewCloudId() Component {
	return &cloudid{
		baseService: newBaseService(v1alpha1.CloudIdComponentType, new(options.SCloudIdOptions)),
	}
}

func (r cloudid) BuildClusterConfigDB(clsCfg *v1alpha1.OnecloudClusterConfig, dbCfg v1alpha1.DBConfig) error {
	clsCfg.CloudId.DB = dbCfg
	return nil
}

func (r cloudid) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.CloudId.CloudUser = user
	return nil
}

func (r cloudid) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeCloudId); err != nil {
		return nil, err
	}
	config := cfg.CloudId
	option.SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = config.Port

	return opt, nil
}

func (a cloudid) GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.CloudId.DB
}

func (a cloudid) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.CloudId.CloudUser
}

func (a cloudid) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	cfg := man.GetCluster().Spec.CloudId
	return controller.NewRegisterEndpointComponent(man, v1alpha1.CloudIdComponentType,
		constants.ServiceNameCloudId, constants.ServiceNameCloudId,
		cfg.Service.NodePort, "")
}
