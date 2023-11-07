package component

import (
	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
)

func init() {
	RegisterComponent(NewVictoriaMetrics())
}

type vm struct {
	*baseService
}

func NewVictoriaMetrics() Component {
	return &vm{
		baseService: newBaseService(v1alpha1.VictoriaMetricsComponentType, nil),
	}
}

func (v vm) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	return "", nil
}

func (v vm) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewTSDBEndpointComponent(
		man, v1alpha1.VictoriaMetricsComponentType,
		constants.ServiceNameVictoriaMetrics, constants.ServiceTypeVictoriaMetrics,
		constants.VictoriaMetricsPort, "")
}
