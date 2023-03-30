package component

import "yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"

func init() {
	RegisterComponent(NewEtcd())
}

type etcd struct {
	*baseService
}

func NewEtcd() Component {
	return &etcd{
		baseService: newBaseService(v1alpha1.EtcdComponentType, nil),
	}
}
