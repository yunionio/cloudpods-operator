package component

import (
	"path"

	"yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewKubeserver())
}

type kubeServer struct {
	*baseService
}

type KubeOptions struct {
	options.CommonOptions
	options.DBOptions

	HttpsPort         int
	TlsCertFile       string
	TlsPrivateKeyFile string
}

func NewKubeserver() Component {
	return &kubeServer{
		baseService: newBaseService(v1alpha1.KubeServerComponentType, new(KubeOptions)),
	}
}

func (r kubeServer) BuildClusterConfigDB(clsCfg *v1alpha1.OnecloudClusterConfig, db v1alpha1.DBConfig) error {
	clsCfg.KubeServer.DB = db
	return nil
}

func (r kubeServer) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.KubeServer.CloudUser = user
	return nil
}

func (r kubeServer) GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.KubeServer.DB
}

func (r kubeServer) GetDefaultClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.KubeServer.ClickhouseConf
}

func (r kubeServer) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.KubeServer.CloudUser
}

func (r kubeServer) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &KubeOptions{}
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeKubeServer); err != nil {
		return nil, err
	}
	config := cfg.KubeServer

	switch oc.Spec.GetDbEngine(oc.Spec.KubeServer.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)
	opt.AutoSyncTable = true
	opt.TlsCertFile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.TlsPrivateKeyFile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.HttpsPort = config.Port

	return opt, nil
}
func (r kubeServer) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return man.KubeServer(nil)
}
