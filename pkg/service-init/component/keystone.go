package component

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/keystone/options"
	"yunion.io/x/onecloud/pkg/util/fileutils2"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

func init() {
	RegisterComponent(NewKeystone())
}

type keystone struct {
	*baseService
}

func NewKeystone() Component {
	return &keystone{
		baseService: newBaseService(
			v1alpha1.KeystoneComponentType,
			new(options.SKeystoneOptions)),
	}
}

func (k keystone) BuildCluster(oc *v1alpha1.OnecloudCluster, opt interface{}) error {
	cfg := opt.(*options.SKeystoneOptions)
	bPwd := cfg.BootstrapAdminUserPassword
	if bPwd == "" {
		return errors.Errorf("bootstrap_admin_user_password is empty")
	}
	oc.Spec.Keystone.BootstrapPassword = bPwd
	return nil
}

func (k keystone) BuildClusterConfigDB(clsCfg *v1alpha1.OnecloudClusterConfig, dbCfg v1alpha1.DBConfig) error {
	clsCfg.Keystone.DB = dbCfg
	return nil
}

func (k keystone) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	opt := &options.Options
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeIdentity); err != nil {
		return nil, err
	}
	config := cfg.Keystone

	switch oc.Spec.GetDbEngine(oc.Spec.Keystone.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetClickhouseOptions(&opt.DBOptions, oc.Spec.Clickhouse, config.ClickhouseConf)
	option.SetOptionsServiceTLS(&opt.BaseOptions, oc.Spec.Keystone.DisableTLS)
	option.SetServiceBaseOptions(&opt.BaseOptions, oc.GetRegion(), config.ServiceBaseConfig, cfg.CommonConfig)

	opt.BootstrapAdminUserPassword = oc.Spec.Keystone.BootstrapPassword
	// always reset admin user password to ensure password is correct
	opt.ResetAdminUserPassword = true
	opt.AdminPort = constants.KeystoneAdminPort

	return opt, nil
}

func (k keystone) GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Keystone.DB
}

func (k keystone) GetDefaultClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Keystone.ClickhouseConf
}

func (k keystone) BeforeStart(oc *v1alpha1.OnecloudCluster, targetCfgDir string) error {
	rcFile := GetRCAdminFilePath(targetCfgDir)
	if fileutils2.Exists(rcFile) {
		return nil
	}
	envContent := GetRCAdminContent(oc, true)
	if err := os.WriteFile(rcFile, []byte(envContent), 0644); err != nil {
		return errors.Wrapf(err, "write %q", rcFile)
	}
	log.Infof("generate rcadmin file %q", rcFile)
	return nil
}

func GetRCAdminContent(oc *v1alpha1.OnecloudCluster, withPasswd bool) string {
	envs := GetRCAdminEnv(oc, withPasswd)
	envContent := ""
	for _, env := range envs {
		envContent += fmt.Sprintf("export %s=%s\n", env.Name, env.Value)
	}
	return envContent
}

func GetRCAdminEnv(oc *v1alpha1.OnecloudCluster, withPasswd bool) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name:  "OS_USERNAME",
			Value: constants.SysAdminUsername,
		},
		{
			Name:  "OS_REGION_NAME",
			Value: oc.GetRegion(),
		},
		{
			Name:  "OS_AUTH_URL",
			Value: controller.GetAuthURL(oc),
		},
		{
			Name:  "OS_PROJECT_NAME",
			Value: constants.SysAdminProject,
		},
		{
			Name:  "YUNION_INSECURE",
			Value: "true",
		},
		{
			Name:  "EDITOR",
			Value: "vim",
		},
	}
	if withPasswd {
		envs = append(envs, corev1.EnvVar{
			Name:  "OS_PASSWORD",
			Value: oc.Spec.Keystone.BootstrapPassword,
		})
	}
	return envs
}
func (k keystone) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return man.Keystone()
}
