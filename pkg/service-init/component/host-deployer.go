package component

import (
	"yunion.io/x/onecloud/pkg/hostman/options"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewHostDeployer())
}

type hostDeployer struct {
	*baseService
}

func NewHostDeployer() Component {
	return &hostDeployer{
		baseService: newBaseService(v1alpha1.HostDeployerComponentType, nil),
	}
}

func (hd hostDeployer) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.HostAgent.CloudUser = user
	return nil
}

func (hd hostDeployer) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.HostAgent.CloudUser
}

func (hd hostDeployer) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := new(options.SHostBaseOptions)
	if err := option.SetOptionsDefault(opt, ""); err != nil {
		return nil, err
	}
	if oc.Spec.ProductVersion == v1alpha1.ProductVersionCMP || oc.Spec.ProductVersion == v1alpha1.ProductVersionBaremetal {
		opt.EnableRemoteExecutor = false
	} else {
		opt.EnableRemoteExecutor = true
	}
	config := cfg.HostAgent
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)
	return opt, nil
}
