package component

import (
	"fmt"

	corelisters "k8s.io/client-go/listers/core/v1"
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
)

type Component interface {
	GetConfigFilePath(targetDir string) string
	GetCertsDirPath(targetDir string) string

	GetType() v1alpha1.ComponentType

	BuildCluster(oc *v1alpha1.OnecloudCluster, opts interface{}) error
	BuildClusterConfigDB(clsCfg *v1alpha1.OnecloudClusterConfig, dbCfg v1alpha1.DBConfig) error
	BuildClusterConfigCloudUser(clsCfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error

	GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error)
	GetOptions() interface{}
	GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig
	GetDefaultClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig
	GetDefaultCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser

	GetPhaseControl(man controller.ComponentManager) controller.PhaseControl
	BeforeStart(oc *v1alpha1.OnecloudCluster, targetCfgDir string) error
}

var globalComponents = make(map[v1alpha1.ComponentType]Component, 0)

func RegisterComponent(comp Component) {
	_, ok := globalComponents[comp.GetType()]
	if ok {
		panic(fmt.Sprintf("%s component already registerd!!!", comp.GetType()))
	}
	globalComponents[comp.GetType()] = comp
}

func GetComponents() map[v1alpha1.ComponentType]Component {
	return globalComponents
}

func GetComponent(cType v1alpha1.ComponentType) Component {
	return globalComponents[cType]
}

type composeComponentManager struct {
	oc *v1alpha1.OnecloudCluster
}

func NewComposeComponentManager(oc *v1alpha1.OnecloudCluster) controller.ComponentManager {
	return &composeComponentManager{
		oc: oc,
	}
}

func (c composeComponentManager) RunWithSession(oc *v1alpha1.OnecloudCluster, f func(s *mcclient.ClientSession) error) error {
	return controller.RunWithSession(oc, f)
}

func (c composeComponentManager) GetController() *controller.OnecloudControl {
	return nil
}

func (c composeComponentManager) GetCluster() *v1alpha1.OnecloudCluster {
	return c.oc
}

func (c composeComponentManager) Keystone() controller.PhaseControl {
	return controller.NewKeystonePhaseControl(c)
}

func (c composeComponentManager) KubeServer(nodeLister corelisters.NodeLister) controller.PhaseControl {
	return controller.NewKubeServerPhaseControl(c, nodeLister)
}

func (c composeComponentManager) Region() controller.PhaseControl {
	return controller.NewRegionPhaseControl(c)
}

func (c composeComponentManager) Glance() controller.PhaseControl {
	return controller.NewGlancePhaseControl(c)
}

func (c composeComponentManager) YunionAgent() controller.PhaseControl {
	return controller.NewYunionAgentPhaseControl(c)
}

func (c composeComponentManager) Devtool() controller.PhaseControl {
	return controller.NewDevtoolPhaseControl(c)
}

func (c composeComponentManager) Monitor() controller.PhaseControl {
	return controller.NewMonitorPhaseControl(c)
}

func (c composeComponentManager) Cloudproxy() controller.PhaseControl {
	return controller.NewCloudproxyPhaseControl(c)
}

func (c composeComponentManager) EChartsSSR() controller.PhaseControl {
	//TODO implement me
	panic("implement me")
}

func (c composeComponentManager) Apigateway() controller.PhaseControl {
	return controller.NewApigatewayPhaseControl(c)
}

func (c composeComponentManager) LLM() controller.PhaseControl {
	return controller.NewLLMPhaseControl(c)
}

func GetComponentDBConfig(cmpt Component, cfg *v1alpha1.OnecloudClusterConfig, existOpt jsonutils.JSONObject) (*v1alpha1.DBConfig, error) {
	dbCfg := cmpt.GetDefaultDBConfig(cfg)
	if dbCfg == nil {
		return nil, nil
	}
	if existOpt == nil {
		return dbCfg, nil
	}
	sqlStr, _ := existOpt.GetString("sql_connection")
	if sqlStr == "" {
		return nil, errors.Errorf("not found 'sql_connection'")
	}
	return ParseSQLAchemyURL(sqlStr)
}

func GetComponentCloudUser(cmpt Component, cfg *v1alpha1.OnecloudClusterConfig, existOpt jsonutils.JSONObject) (*v1alpha1.CloudUser, error) {
	cloudUser := cmpt.GetDefaultCloudUser(cfg)
	if cloudUser == nil {
		return nil, nil
	}
	if existOpt == nil {
		return cloudUser, nil
	}
	adminUser, _ := existOpt.GetString("admin_user")
	if adminUser == "" {
		return nil, errors.Errorf("not found 'admin_user'")
	}
	adminPwd, _ := existOpt.GetString("admin_password")
	if adminPwd == "" {
		return nil, errors.Errorf("not found 'admin_password'")
	}
	return &v1alpha1.CloudUser{
		Username: adminUser,
		Password: adminPwd,
	}, nil
}
