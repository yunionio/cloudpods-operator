package component

import (
	"yunion.io/x/onecloud/pkg/baremetal/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewBaremetal())
}

type baremetal struct {
	*baseService
}

func NewBaremetal() Component {
	return &baremetal{
		baseService: newBaseService(v1alpha1.BaremetalAgentComponentType, new(options.BaremetalOptions)),
	}
}

func (b baremetal) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.BaremetalAgent.CloudUser = user
	return nil
}

func (b baremetal) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.BaremetalAgent.CloudUser
}

func (b baremetal) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, ""); err != nil {
		return nil, err
	}
	zoneId := oc.GetZone("")
	opt.Zone = zoneId

	config := cfg.BaremetalAgent
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.Port = constants.BaremetalPort
	opt.AutoRegisterBaremetal = false
	opt.LinuxDefaultRootUser = true
	opt.DefaultIpmiPassword = "YunionDev@123"
	if oc.Spec.ProductVersion == v1alpha1.ProductVersionBaremetal {
		opt.ListenInterface = "eth0"
	}
	return opt, nil
}
