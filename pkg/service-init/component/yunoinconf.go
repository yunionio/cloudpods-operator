package component

import (
	"fmt"
	"sort"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules/yunionconf"
	"yunion.io/x/onecloud/pkg/yunionconf/options"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/util/sets"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewYunionconf())
}

const (
	FEAT_VOLCENGINE   = "volcengine"
	FEAT_ORACLE_CLOUD = "oraclecloud"
	FEAT_KSYUN        = "ksyun"

	FEAT_VMWARE  = "vmware"
	FEAT_PROXMOX = "proxmox"
)

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

	switch oc.Spec.GetDbEngine(oc.Spec.Yunionconf.DbEngine) {
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

type GlobalSettingsValue struct {
	SetupKeys                []string        `json:"setupKeys"`
	SetupKeysVersion         string          `json:"setupKeysVersion"`
	SetupOneStackInitialized bool            `json:"setupOneStackInitialized"`
	ProductVersion           string          `json:"productVersion"`
	UserDefinedKeys          map[string]bool `json:"userDefinedKeys"`
}

func NewGlobalSettingsValue(setupKeys []string, oneStackInited bool, productVersion v1alpha1.ProductVersion) *GlobalSettingsValue {
	inputValue := &GlobalSettingsValue{
		SetupKeys:                setupKeys,
		SetupKeysVersion:         "3.0",
		SetupOneStackInitialized: oneStackInited,
		ProductVersion:           string(productVersion),
		UserDefinedKeys:          make(map[string]bool),
	}
	for _, key := range setupKeys {
		inputValue.UserDefinedKeys[key] = true
	}
	return inputValue
}

func (v GlobalSettingsValue) Equal(o GlobalSettingsValue) bool {
	vk := sets.NewString(v.SetupKeys...)
	ok := sets.NewString(o.SetupKeys...)
	if !vk.Equal(ok) {
		return false
	}
	if v.SetupKeysVersion != o.SetupKeysVersion {
		return false
	}
	if v.SetupOneStackInitialized != o.SetupOneStackInitialized {
		return false
	}
	if v.ProductVersion != o.ProductVersion {
		return false
	}
	return true
}

func (v *GlobalSettingsValue) CalculateSetupKeys(oldSettings GlobalSettingsValue) {
	// 旧配置打开的功能
	for _, pKey := range oldSettings.SetupKeys {
		v.UserDefinedKeys[pKey] = true
	}
	if len(oldSettings.UserDefinedKeys) == 0 {
		oldSettings.UserDefinedKeys = make(map[string]bool)
	}
	oldSs := sets.NewString(oldSettings.SetupKeys...)
	// 根据旧配置，关闭或打开现有功能
	if len(oldSettings.UserDefinedKeys) == 0 {
		// 关闭不需要的新功能
		for _, sK := range v.SetupKeys {
			if len(oldSettings.UserDefinedKeys) == 0 && !oldSs.Has(sK) {
				v.UserDefinedKeys[sK] = false
			}
		}
		// 打开必要的新功能(这个只需要在就配置升级上来的时候启用)
		newFeatures := sets.NewString()
		if v.ProductVersion == string(v1alpha1.ProductVersionCMP) || v.ProductVersion == string(v1alpha1.ProductVersionFullStack) {
			newFeatures = sets.NewString(FEAT_VOLCENGINE, FEAT_ORACLE_CLOUD, FEAT_KSYUN)
		} else if v.ProductVersion == string(v1alpha1.ProductVersionEdge) {
			newFeatures = sets.NewString(FEAT_VMWARE, FEAT_PROXMOX)
		}
		for _, nf := range newFeatures.List() {
			if _, ok := oldSettings.UserDefinedKeys[nf]; !ok {
				v.UserDefinedKeys[nf] = true
			}
		}
	}

	// 旧配置用户定义开关的功能
	for pKey, pKeyOn := range oldSettings.UserDefinedKeys {
		v.UserDefinedKeys[pKey] = pKeyOn
	}
	calSetupKeys := []string{}
	for k, isOn := range v.UserDefinedKeys {
		if isOn {
			calSetupKeys = append(calSetupKeys, k)
		}
	}
	sort.Strings(calSetupKeys)
	v.SetupKeys = calSetupKeys
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
		needUpdate := false
		needCreate := true
		var currentValue *GlobalSettingsValue = nil
		var currentId int64
		if len(items.Data) != 0 {
			curConfig := items.Data[0]
			if id, err := curConfig.Int("id"); err != nil {
				return errors.Wrapf(err, "get id from %s", curConfig.PrettyString())
			} else {
				currentId = id
			}
			currentValue = new(GlobalSettingsValue)
			if err := curConfig.Unmarshal(currentValue, "value"); err != nil {
				return errors.Wrapf(err, "unmarshal %s to GlobalSettingsValue", curConfig.PrettyString())
			}
			needCreate = false
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
			FEAT_VOLCENGINE,
			// "ucloud",
			// "ecloud",
			// "jdcloud",
			FEAT_VMWARE,
			"openstack",
			// "dstack",
			// "zstack",
			// "apsara",
			"cloudpods",
			// "hcso",
			"nutanix",
			// "bingocloud",
			// "incloudsphere",
			"s3",
			"ceph",
			"xsky",
			FEAT_PROXMOX,
			FEAT_ORACLE_CLOUD,
			FEAT_KSYUN,
		}
		setupKeysEdge := []string{
			"onecloud",
			"onestack",
			"baremetal",
			"lb",
			FEAT_VMWARE,
			FEAT_PROXMOX,
		}
		setupKeysBaremetal := []string{
			"onecloud",
			"onestack",
			"baremetal",
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
		case v1alpha1.ProductVersionBaremetal:
			setupKeys = setupKeysBaremetal
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

		// construct default GlobalSettingsValue
		inputValue := NewGlobalSettingsValue(setupKeys, oneStackInited, oc.Spec.ProductVersion)

		if currentValue != nil {
			inputValue.CalculateSetupKeys(*currentValue)
			needUpdate = !currentValue.Equal(*inputValue)
		}

		input := map[string]interface{}{
			"name":       gsName,
			"service_id": constants.ServiceNameYunionAgent,
			"value":      inputValue,
		}
		params := jsonutils.Marshal(input)
		if needCreate {
			if _, err := yunionconf.Parameters.Create(s, params); err != nil {
				return errors.Wrapf(err, "create global-settings with %s", params)
			}
			log.Infof("create global-settings with: %s", params.String())
			return nil
		}
		if needUpdate {
			if _, err := yunionconf.Parameters.Update(s, fmt.Sprintf("%d", currentId), params); err != nil {
				return errors.Wrapf(err, "update global-settings with id %d: %s", currentId, params)
			}
			log.Infof("update global-settings with: %s", params.String())
			return nil
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "pc.man.RunWithSession")
	}
	return nil
}
