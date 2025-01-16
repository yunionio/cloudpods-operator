package component

import (
	"yunion.io/x/onecloud/pkg/esxi/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewEsxiAgent())
}

type esxiAgent struct {
	*baseService
}

func NewEsxiAgent() Component {
	return &esxiAgent{
		baseService: newBaseService(v1alpha1.EsxiAgentComponentType, new(options.EsxiOptions)),
	}
}

func (ea esxiAgent) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.EsxiAgent.CloudUser = user
	return nil
}

func (ea esxiAgent) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.EsxiAgent.CloudUser
}

func (ea esxiAgent) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, ""); err != nil {
		return nil, err
	}
	zoneId := oc.GetZone("")
	opt.Zone = zoneId
	opt.ListenInterface = "eth0"
	config := cfg.EsxiAgent
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)
	opt.Port = constants.EsxiAgentPort
	return opt, nil
}
