package component

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud/pkg/mcclient"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

const (
	ItsmTemplate = `
debug=false
trace=false

# BANNER
banner.charset=UTF-8

# LOGGING
logging.level.com.yunion=INFO

# OUTPUT
spring.output.ansi.enabled=ALWAYS
spring.security.basic.enabled=false

# ----------------------------------------
# WEB PROPERTIES
# ----------------------------------------

# EMBEDDED SERVER CONFIGURATION (ServerProperties)
server.port={{.Port}}
server.session-timeout=180
server.context-path=/
server.tomcat.basedir=.
server.tomcat.uri-encoding=UTF-8
server.tomcat.accesslog.enabled=true
server.tomcat.accesslog.dir=access_logs
server.tomcat.accesslog.file-date-format=.yyyy-MM-dd
server.tomcat.accesslog.prefix=access_log
server.tomcat.accesslog.suffix=.log
server.tomcat.accesslog.rotate=true

# ----------------------------------------
# DATA PROPERTIES
# ----------------------------------------

# DATASOURCE (DataSourceAutoConfiguration & DataSourceProperties)
datasource.primary.jdbc-url=jdbc:mysql://{{.DBHost}}:{{.DBPort}}/{{.DB}}?useUnicode=true&characterEncoding=utf8&zeroDateTimeBehavior=convertToNull&useSSL=false&createDatabaseIfNotExist=true
datasource.primary.username={{.DBUser}}
datasource.primary.password={{.DBPassowrd}}
datasource.primary.driver-class-name=com.mysql.cj.jdbc.Driver
datasource.primary.initialize=false
datasource.primary.continue-on-error=false
datasource.primary.sql-script-encoding=utf-8
datasource.primary.schema=classpath:sql/schema.sql
datasource.primary.initialization-mode=always

datasource.secondary.jdbc-url=jdbc:mysql://{{.DBHost}}:{{.DBPort}}/{{.DB2nd}}?useUnicode=true&characterEncoding=utf8&zeroDateTimeBehavior=convertToNull&useSSL=false&createDatabaseIfNotExist=true&useTimezone=true&serverTimezone=UTC
datasource.secondary.username={{.DBUser}}
datasource.secondary.password={{.DBPassowrd}}
datasource.secondary.driver-class-name=com.mysql.cj.jdbc.Driver
datasource.secondary.initialize=false
datasource.secondary.continue-on-error=false
datasource.secondary.sql-script-encoding=utf-8

# ----------------------------------------
# PROCESS SERVICE PROPERTIES
# ----------------------------------------

# Workflow
camunda.bpm.history-level=FULL
camunda.bpm.filter.create=All tasks
camunda.bpm.database.type=mysql


# ----------------------------------------
# Custom PROPERTIES
# ----------------------------------------

# OneCloud Authentication
yunion.rc.auth.url={{.AuthURL}}
yunion.rc.auth.domain={{.AuthDomain}}
yunion.rc.auth.username={{.AuthUsername}}
yunion.rc.auth.password={{.AuthPassword}}
yunion.rc.auth.project={{.AuthProject}}
yunion.rc.auth.region={{.Region}}
yunion.rc.auth.cache-size=500
yunion.rc.auth.timeout=1000
yunion.rc.auth.debug=true
yunion.rc.auth.insecure=true
yunion.rc.auth.refresh-interval=300000

# email notify.
yunion.rc.email.link.parameter.name=mail-call-back-address
yunion.rc.email.link.parameter.key=address
yunion.rc.email.link.encryption.key={{.EncryptionKey}}

#
non_default_domain_projects=false`
)

const (
	JAVA_APP_JAR         = "JAVA_APP_JAR"
	JAVA_APP_WORKING_DIR = "/deployments"
	JAVA_OPTIONS         = "JAVA_OPTIONS"
)

var (
	GetOwnerRef = controller.GetOwnerRef
)

type ItsmConfigOption struct {
	JavaDBConfig
	DB2nd         string
	EncryptionKey string
}

type JavaDBConfig struct {
	JavaBaseConfig
	DBHost     string
	DBPort     int32
	DB         string
	DBUser     string
	DBPassowrd string
}

type JavaBaseConfig struct {
	Port         int
	AuthURL      string
	AuthDomain   string
	AuthUsername string
	AuthPassword string
	AuthProject  string
	Region       string
}

type itsmManager struct {
	*ComponentManager
}

func newItsmManager(man *ComponentManager) manager.Manager {
	return &itsmManager{man}
}

func (m *itsmManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *itsmManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.ItsmComponentType
}

func (m *itsmManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if oc.Spec.Itsm.Disable || !IsEnterpriseEdition(oc) {
		controller.RunWithSession(oc, func(s *mcclient.ClientSession) error {
			return onecloud.EnsureDisableService(s, constants.ServiceNameItsm)
		})
	}
	//isEE create itsm DeploymentSpec
	if !IsEnterpriseEdition(oc) {
		return nil
	}
	return syncComponent(m, oc, oc.Spec.Itsm.Disable, "")
}

func (m *itsmManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	tmp := cfg.Itsm.DB
	tmp.Database = cfg.Itsm.SecondDatabase
	return &tmp
}

func (m *itsmManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	// always mysql
	return v1alpha1.DBEngineMySQL
}

func (m *itsmManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Itsm.CloudUser
}

func (m *itsmManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewItsmEndpointComponent(man, v1alpha1.ItsmComponentType,
		constants.ServiceNameItsm, constants.ServiceTypeItsm,
		man.GetCluster().Spec.Itsm.Service.NodePort, "")
}

func (m *itsmManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	cfg_ := cfg.Itsm
	itsmConfigOption := ItsmConfigOption{
		JavaDBConfig:  *NewJavaDBConfig(oc, cfg_.ServiceDBCommonOptions),
		DB2nd:         cfg_.SecondDatabase,
		EncryptionKey: cfg_.EncryptionKey,
	}
	return m.NewConfigMapByTemplate(v1alpha1.ItsmComponentType, oc, ItsmTemplate, itsmConfigOption)
}

func (m *itsmManager) NewConfigMapByTemplate(cType v1alpha1.ComponentType, oc *v1alpha1.OnecloudCluster, template string, config interface{}) (*corev1.ConfigMap, bool, error) {
	data, err := component.CompileTemplateFromMap(template, config)
	if err != nil {
		return nil, false, err
	}
	return m.newConfigMap(cType, "", oc, data), false, nil
}

func NewJavaDBConfig(oc *v1alpha1.OnecloudCluster, cfg v1alpha1.ServiceDBCommonOptions) *JavaDBConfig {
	opt := NewJavaBaseConfig(oc, cfg.Port, cfg.CloudUser.Username, cfg.CloudUser.Password)
	dbCfg := &JavaDBConfig{
		JavaBaseConfig: *opt,
		DBHost:         oc.Spec.Mysql.Host,
		DBPort:         oc.Spec.Mysql.Port,
		DB:             cfg.DB.Database,
		DBUser:         cfg.DB.Username,
		DBPassowrd:     cfg.DB.Password,
	}
	return dbCfg
}

func NewJavaBaseConfig(oc *v1alpha1.OnecloudCluster, port int, user, passwd string) *JavaBaseConfig {
	return &JavaBaseConfig{
		AuthURL:      controller.GetAuthURL(oc),
		AuthProject:  constants.SysAdminProject,
		AuthDomain:   constants.DefaultDomain,
		AuthUsername: user,
		AuthPassword: passwd,
		Region:       oc.Spec.Region,
		Port:         port,
	}
}

func (m *itsmManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.ItsmComponentType, oc, int32(oc.Spec.Itsm.Service.NodePort), int32(cfg.Itsm.Port))}
}

func (m *itsmManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		volMounts = SetJavaConfigVolumeMounts(volMounts)
		return []corev1.Container{
			{
				Name:            "itsm",
				Image:           oc.Spec.Itsm.Image,
				ImagePullPolicy: oc.Spec.Itsm.ImagePullPolicy,
				Env: []corev1.EnvVar{
					{
						Name:  JAVA_APP_JAR,
						Value: "itsm.jar",
					},
					// set max memory to 1G
					{
						Name:  JAVA_OPTIONS,
						Value: "-Xms1024M -Xmx1024M",
					},
				},
				VolumeMounts: volMounts,
			},
		}
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.ItsmComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.ItsmComponentType), v1alpha1.ItsmComponentType), &oc.Spec.Itsm.DeploymentSpec, cf)
	if err != nil {
		return nil, err
	}
	podSpec := &deploy.Spec.Template.Spec
	podSpec.Volumes = SetJavaConfigVolumes(podSpec.Volumes)
	return deploy, nil
}

func (m *itsmManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Itsm
}

func SetJavaConfigVolumeMounts(volMounts []corev1.VolumeMount) []corev1.VolumeMount {
	confVol := volMounts[len(volMounts)-1]
	confVol.MountPath = fmt.Sprintf("%s/config", JAVA_APP_WORKING_DIR)
	volMounts[len(volMounts)-1] = confVol
	return volMounts
}

func SetJavaConfigVolumes(vols []corev1.Volume) []corev1.Volume {
	config := vols[len(vols)-1]
	config.ConfigMap.Items[0].Path = "application.properties"
	vols[len(vols)-1] = config
	return vols
}
