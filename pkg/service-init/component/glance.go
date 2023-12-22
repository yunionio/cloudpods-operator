package component

import (
	"yunion.io/x/onecloud/pkg/image/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewGlance())
}

type glance struct {
	*baseService
}

func NewGlance() Component {
	return &glance{
		baseService: newBaseService(v1alpha1.GlanceComponentType, new(options.SImageOptions)),
	}
}

func (r glance) BuildClusterConfigDB(clsCfg *v1alpha1.OnecloudClusterConfig, db v1alpha1.DBConfig) error {
	clsCfg.Glance.DB = db
	return nil
}

func (r glance) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.Glance.CloudUser = user
	return nil
}

func (r glance) GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Glance.DB
}

func (r glance) GetDefaultClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Glance.ClickhouseConf
}

func (r glance) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Glance.CloudUser
}

func (g glance) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeGlance); err != nil {
		return nil, err
	}
	config := cfg.Glance
	option.SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	option.SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceDBCommonOptions.ServiceCommonOptions)

	opt.FilesystemStoreDatadir = constants.GlanceFileStoreDir
	//opt.TorrentStoreDir = constants.GlanceTorrentStoreDir
	opt.EnableTorrentService = false
	// TODO: fix this
	opt.AutoSyncTable = true
	opt.Port = config.Port

	if oc.Spec.ProductVersion == v1alpha1.ProductVersionCMP {
		opt.EnableRemoteExecutor = false
		opt.TargetImageFormats = []string{"qcow2", "vmdk"}
	} else {
		opt.EnableRemoteExecutor = true
	}
	return opt, nil
}

func (g glance) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return man.Glance()
}
