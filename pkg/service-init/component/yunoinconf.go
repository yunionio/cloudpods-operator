package component

import (
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules/yunionconf"
	"yunion.io/x/onecloud/pkg/yunionconf/options"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewYunionconf())
}

type yunionconfSvc struct {
	*baseService
}

func NewYunionconf() Component {
	return &yunionconfSvc{newBaseService(v1alpha1.YunionconfComponentType, new(options.YunionConfOptions))}
}

func (r yunionconfSvc) BuildClusterConfigDB(clsCfg *v1alpha1.OnecloudClusterConfig, db v1alpha1.DBConfig) error {
	clsCfg.Yunionconf.DB = db
	return nil
}

func (r yunionconfSvc) BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	clsCfg.Yunionconf.CloudUser = user
	return nil
}

func (r yunionconfSvc) GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Yunionconf.DB
}

func (r yunionconfSvc) GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Yunionconf.CloudUser
}

func (r yunionconfSvc) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeYunionConf); err != nil {
		return nil, err
	}
	config := cfg.Yunionconf
	option.SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.Port = config.Port

	return opt, nil
}

func (r yunionconfSvc) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return newYunionconfPC(man)
}

// yunionconfPC implements controller.PhaseControl
type yunionconfPC struct {
	controller.PhaseControl
	man controller.ComponentManager
}

func newYunionconfPC(man controller.ComponentManager) controller.PhaseControl {
	return &yunionconfPC{
		PhaseControl: controller.NewRegisterEndpointComponent(man, v1alpha1.YunionconfComponentType,
			constants.ServiceNameYunionConf, constants.ServiceTypeYunionConf,
			man.GetCluster().Spec.Yunionconf.Service.NodePort, ""),
		man: man,
	}
}

func (pc *yunionconfPC) Setup() error {
	if err := pc.PhaseControl.Setup(); err != nil {
		return errors.Wrap(err, "endpoint for yunionconfSvc setup")
	}
	// hack: init fake yunionagent service and endpoints
	if err := pc.man.YunionAgent().Setup(); err != nil {
		return errors.Wrap(err, "setup yunionagent for yunionconfSvc")
	}
	return nil
}

func (pc *yunionconfPC) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	// register parameter of services
	// 1. init global-settings parameter if not created
	isEE := v1alpha1.IsEnterpriseEdition(oc)
	gsName := "global-settings"
	if err := pc.man.RunWithSession(pc.man.GetCluster(), func(s *mcclient.ClientSession) error {
		items, err := yunionconf.Parameters.List(s, jsonutils.Marshal(map[string]string{
			"name":  gsName,
			"scope": "system"}))
		if err != nil {
			return errors.Wrapf(err, "search %s", gsName)
		}
		if len(items.Data) != 0 {
			return nil
		}
		var setupKeys []string
		setupKeysCmp := []string{
			"public",
			"private",
			"storage",
			"aliyun",
			"aws",
			"azure",
			"ctyun",
			"google",
			"huawei",
			"qcloud",
			// "ucloud",
			// "ecloud",
			// "jdcloud",
			"vmware",
			"openstack",
			// "dstack",
			// "zstack",
			"apsara",
			"cloudpods",
			// "hcso",
			"nutanix",
			// "bingocloud",
			// "incloudsphere",
			"s3",
			"ceph",
			"xsky",
			"proxmox",
		}
		setupKeysEdge := []string{
			"onecloud",
			"onestack",
			"baremetal",
			"lb",
		}
		setupKeysFull := []string{}
		setupKeysFull = append(setupKeysFull, setupKeysCmp...)
		setupKeysFull = append(setupKeysFull, setupKeysEdge...)
		oneStackInited := false
		switch oc.Spec.ProductVersion {
		case v1alpha1.ProductVersionCMP:
			setupKeys = setupKeysCmp
			oneStackInited = true
		case v1alpha1.ProductVersionEdge:
			setupKeys = setupKeysEdge
		default:
			setupKeys = setupKeysFull
		}
		setupKeys = append(setupKeys, "monitor", "auth")
		if isEE {
			switch oc.Spec.ProductVersion {
			case v1alpha1.ProductVersionEdge:
			default:
				setupKeys = append(setupKeys,
					"ucloud",
					"ecloud",
					"jdcloud",
					"zstack",
					"hcso",
					"bingocloud",
					"incloudsphere",
				)
			}
			setupKeys = append(setupKeys, "k8s", "bill")
		}
		setupKeys = append(setupKeys, "default")
		input := map[string]interface{}{
			"name":       gsName,
			"service_id": constants.ServiceNameYunionAgent,
			"value": map[string]interface{}{
				"setupKeys":                setupKeys,
				"setupKeysVersion":         "3.0",
				"setupOneStackInitialized": oneStackInited,
			},
		}
		params := jsonutils.Marshal(input)
		if _, err := yunionconf.Parameters.Create(s, params); err != nil {
			return errors.Wrap(err, "ensure global-settings")
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "pc.man.RunWithSession")
	}
	return nil
}
