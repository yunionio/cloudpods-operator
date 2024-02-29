package component

import (
	"path"

	"yunion.io/x/onecloud/pkg/ansibleserver/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewAnsibleServer())
}

type ansibleServer struct {
	*baseService
}

func NewAnsibleServer() Component {
	return &ansibleServer{
		baseService: newBaseService(v1alpha1.AnsibleServerComponentType, new(options.AnsibleServerOptions)),
	}
}

func (r ansibleServer) BuildClusterConfigDB(clsCfg *v1alpha1.OnecloudClusterConfig, dbCfg v1alpha1.DBConfig) error {
	clsCfg.AnsibleServer.DB = dbCfg
	return nil
}

func (r ansibleServer) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.AnsibleServer.CloudUser = user
	return nil
}

func (r ansibleServer) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeAnsibleServer); err != nil {
		return nil, err
	}
	config := cfg.AnsibleServer

	switch oc.Spec.GetDbEngine(oc.Spec.AnsibleServer.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)

	return opt, nil
}

func (a ansibleServer) GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.AnsibleServer.DB
}

func (a ansibleServer) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.AnsibleServer.CloudUser
}

func (a ansibleServer) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	cfg := man.GetCluster().Spec.AnsibleServer
	return controller.NewRegisterEndpointComponent(man, v1alpha1.AnsibleServerComponentType,
		constants.ServiceNameAnsibleServer, constants.ServiceTypeAnsibleServer,
		cfg.Service.NodePort, "")
}
