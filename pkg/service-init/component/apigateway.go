package component

import (
	"yunion.io/x/onecloud/pkg/cloudcommon/options"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewApiGateway())
}

type apiGateway struct {
	*baseService
}

type ApiOptions struct {
	options.CommonOptions
	WsPort      int  `default:"10443"`
	ShowCaptcha bool `default:"true"`
	EnableTotp  bool `default:"true"`
}

func NewApiGateway() Component {
	return &apiGateway{newBaseService(v1alpha1.APIGatewayComponentType, new(ApiOptions))}
}

func (a apiGateway) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.APIGateway.CloudUser = user
	return nil
}

func (a apiGateway) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &ApiOptions{}
	if err := option.SetOptionsDefault(opt, "apigateway"); err != nil {
		return nil, err
	}
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, cfg.APIGateway)
	opt.Port = cfg.APIGateway.Port
	opt.WsPort = constants.APIWebsocketPort
	opt.CorsHosts = oc.Spec.APIGateway.CorsHosts

	return opt, nil
}
func (a apiGateway) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.APIGateway.CloudUser
}

func (r apiGateway) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return newAPIGatewaPhaseControl(man)
}

type apiGatewayPhaseControl struct {
	man controller.ComponentManager
	ac  controller.PhaseControl
	wc  controller.PhaseControl
}

func newAPIGatewaPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	oc := man.GetCluster()
	return &apiGatewayPhaseControl{
		man: man,
		ac:  controller.NewRegisterServiceComponent(man, constants.ServiceNameAPIGateway, constants.ServiceTypeAPIGateway),
		wc: controller.NewRegisterEndpointComponent(man, v1alpha1.APIGatewayComponentType,
			constants.ServiceNameWebsocket, constants.ServiceTypeWebsocket,
			oc.Spec.APIGateway.WSService.NodePort, ""),
	}
}

func (c *apiGatewayPhaseControl) Setup() error {
	if err := c.ac.Setup(); err != nil {
		return err
	}
	if err := c.wc.Setup(); err != nil {
		return err
	}
	return nil
}

func (c *apiGatewayPhaseControl) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	return nil
}
