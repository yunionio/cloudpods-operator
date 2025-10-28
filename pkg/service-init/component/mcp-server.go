package component

import (
	"yunion.io/x/onecloud/pkg/mcp-server/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewMcpServer())
}

type mcpServer struct {
	*baseService
}

func NewMcpServer() Component {
	return &mcpServer{
		baseService: newBaseService(v1alpha1.McpServerComponentType, new(options.MCPServerOptions)),
	}
}

func (r mcpServer) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeMcpServer); err != nil {
		return nil, err
	}

	config := cfg.McpServer

	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)
	opt.Port = config.Port
	return opt, nil
}

func (r mcpServer) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponentWithSSL(man, v1alpha1.McpServerComponentType,
		constants.ServiceNameMcpServer, constants.ServiceTypeMcpServer,
		man.GetCluster().Spec.McpServer.Service.NodePort, "", false)
}
