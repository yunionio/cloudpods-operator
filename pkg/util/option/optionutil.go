package option

import (
	"fmt"
	"path"

	"yunion.io/x/pkg/util/reflectutils"
	"yunion.io/x/structarg"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud/pkg/cloudcommon/options"
)

func SetOptionsDefault(opt interface{}, serviceType string) error {
	parser, err := structarg.NewArgumentParser(opt, serviceType, "", "")
	if err != nil {
		return err
	}
	parser.SetDefault()

	var optionsRef *options.BaseOptions
	if err := reflectutils.FindAnonymouStructPointer(opt, &optionsRef); err != nil {
		return err
	}
	if len(optionsRef.ApplicationID) == 0 {
		optionsRef.ApplicationID = serviceType
	}
	return nil
}

func SetOptionsServiceTLS(config *options.BaseOptions, disableTLS bool) {
	enableConfigTLS(disableTLS, config, constants.CertDir, constants.CACertName, constants.ServiceCertName, constants.ServiceKeyName)
}

func enableConfigTLS(disableTLS bool, config *options.BaseOptions, certDir string, ca string, cert string, key string) {
	config.EnableSsl = !disableTLS
	config.SslCaCerts = path.Join(certDir, ca)
	config.SslCertfile = path.Join(certDir, cert)
	config.SslKeyfile = path.Join(certDir, key)
}

func SetServiceBaseOptions(opt *options.BaseOptions, region string, input v1alpha1.ServiceBaseConfig) {
	opt.Region = region
	opt.Port = input.Port
}

func SetServiceCommonOptions(opt *options.CommonOptions, oc *v1alpha1.OnecloudCluster, input v1alpha1.ServiceCommonOptions) {
	SetServiceBaseOptions(&opt.BaseOptions, oc.GetRegion(), input.ServiceBaseConfig)
	opt.AuthURL = controller.GetAuthURL(oc)
	opt.AdminUser = input.CloudUser.Username
	opt.AdminDomain = constants.DefaultDomain
	opt.AdminPassword = input.CloudUser.Password
	opt.AdminProject = constants.SysAdminProject
}

func SetMysqlOptions(opt *options.DBOptions, mysql v1alpha1.Mysql, input v1alpha1.DBConfig) {
	opt.SqlConnection = fmt.Sprintf("mysql+pymysql://%s:%s@%s:%d/%s?charset=utf8&parseTime=true&interpolateParams=true", input.Username, input.Password, mysql.Host, mysql.Port, input.Database)
}

func SetDamengOptions(opt *options.DBOptions, dameng v1alpha1.Dameng, input v1alpha1.DBConfig) {
	opt.SqlConnection = fmt.Sprintf("dm://%s:%s@%s:%d/%s", input.Username, input.Password, dameng.Host, dameng.Port, input.Database)
}

func SetClickhouseOptions(opt *options.DBOptions, clickhouse v1alpha1.Clickhouse, input v1alpha1.DBConfig) {
	if len(clickhouse.Host) > 0 && len(input.Database) > 0 {
		opt.Clickhouse = fmt.Sprintf("tcp://%s:%d?database=%s&read_timeout=10&write_timeout=20&username=%s&password=%s", clickhouse.Host, clickhouse.Port, input.Database, input.Username, input.Password)
		opt.OpsLogWithClickhouse = true
	}
}
