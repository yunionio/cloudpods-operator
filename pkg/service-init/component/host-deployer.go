package component

import (
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/option"
	"yunion.io/x/onecloud/pkg/hostman/options"
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

func (hd hostDeployer) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := new(options.SHostBaseOptions)
	if err := option.SetOptionsDefault(opt, ""); err != nil {
		return nil, err
	}
	if oc.Spec.ProductVersion == v1alpha1.ProductVersionCMP {
		opt.EnableRemoteExecutor = false
	} else {
		opt.EnableRemoteExecutor = true
	}
	return opt, nil
}
