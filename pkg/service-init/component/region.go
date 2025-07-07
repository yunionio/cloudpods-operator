package component

import (
	"fmt"

	"yunion.io/x/onecloud/pkg/compute/options"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/dbutil"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewRegion())
}

type region struct {
	*baseService
}

func NewRegion() Component {
	return &region{
		baseService: newBaseService(v1alpha1.RegionComponentType, new(options.ComputeOptions)),
	}
}

func (r region) BuildClusterConfigDB(clsCfg *v1alpha1.OnecloudClusterConfig, db v1alpha1.DBConfig) error {
	clsCfg.RegionServer.DB = db
	return nil
}

func (r region) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.RegionServer.CloudUser = user
	return nil
}

func (r region) GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.RegionServer.DB
}

func (r region) GetDefaultClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.RegionServer.ClickhouseConf
}

func (r region) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.RegionServer.CloudUser
}

func (r region) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeComputeV2); err != nil {
		return nil, err
	}
	config := cfg.RegionServer
	spec := oc.Spec.RegionServer

	switch oc.Spec.GetDbEngine(oc.Spec.RegionServer.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceDBCommonOptions.ServiceCommonOptions, cfg.CommonConfig)
	// TODO: fix this, currently init container can't sync table
	opt.AutoSyncTable = true

	opt.DNSDomain = spec.DNSDomain
	if spec.DNSServer == "" {
		spec.DNSServer = oc.Spec.LoadBalancerEndpoint
	}
	oc.Spec.RegionServer = spec
	opt.DNSServer = spec.DNSServer

	opt.PortV2 = config.Port
	if err := r.setBaremetalPrepareConfigure(oc, cfg, opt); err != nil {
		return nil, errors.Wrap(err, "setBaremetalPrepareConfiguration")
	}
	return opt, nil
}

func (r region) setBaremetalPrepareConfigure(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, opt *options.ComputeOptions) error {
	masterAddress := oc.Spec.LoadBalancerEndpoint
	opt.BaremetalPreparePackageUrl = fmt.Sprintf("https://%s/baremetal-prepare/baremetal_prepare.tar.gz", dbutil.FormatHost(masterAddress))
	return nil
}

func (r region) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return man.Region()
}
