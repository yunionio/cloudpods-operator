// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package constants

import (
	"path"
	"time"
)

const (
	// The following labels are recommended by kubernetes https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/

	// ManagedByLabelKey is Kubernetes recommended label key, it represents the tool being used to manage the operation of an application
	// For resources managed by OneCloud Operator, its value is always onecloud-operator
	ManagedByLabelKey string = "app.kubernetes.io/managed-by"
	// ComponentLabelKey is Kubernetes recommended label key, it represents the component within the architecture
	ComponentLabelKey string = "app.kubernetes.io/component"
	// NameLabelKey is Kubernetes recommended label key, it represents the name of the application
	// It should always be onecloud-cluster in our case.
	NameLabelKey string = "app.kubernetes.io/name"
	// InstanceLabelKey is Kubernetes recommended label key, it represents a unique name identifying the instance of an application
	// It's set by helm when installing a release
	InstanceLabelKey string = "app.kubernetes.io/instance"
	AppLabelKey      string = "app"
	ZoneLabelKey     string = "zone"

	// LabelNodeRoleMaster specifies that a node is a control-plane
	// This is a duplicate definition of the constant in pkg/controller/service/service_controller.go
	LabelNodeRoleMaster string = "node-role.kubernetes.io/master"

	ServiceAccountOnecloudOperator string = "onecloud-operator"
)

const (
	OnecloudEditionAnnotationKey     string = "onecloud.yunion.io/edition"
	OnecloudEnableHostLabelKey       string = "onecloud.yunion.io/host"
	OnecloudEanbleBaremetalLabelKey  string = "onecloud.yunion.io/baremetal"
	OnecloudControllerLabelKey       string = "onecloud.yunion.io/controller"
	OnecloudHostDeployerLabelKey     string = "onecloud.yunion.io/host-deployer"
	OnecloudEnableLbagentLabelKey    string = "onecloud.yunion.io/lbagent"
	OnecloudCommunityEdition         string = "ce"
	OnecloudEnterpriseSupportEdition string = "ce/support"
	OnecloudEnterpriseEdition        string = "ee"

	WebCEImageName        = "web"
	APIGatewayCEImageName = "apigateway"
	WebEEImageName        = "web-ee"
	APIGatewayEEImageName = "apigateway-ee"

	RegionCEImageName = "region"
	RegionEEImageName = "region-ee"
)

const (
	OnecloudClusterKind               = "OnecloudCluster"
	OnecloudClusterConfigKind         = "OnecloudClusterConfig"
	OnecloudClusterConfigConfigMapKey = OnecloudClusterConfigKind
)

const (
	OnecloudMinioNamespace = "onecloud-minio"
	OnecloudMinioSecret    = "minio"
	OnecloudMinioSvc       = "minio"
)

const (
	MonitorStackNamespace = "onecloud-monitoring"

	MonitorMinioName         = "monitor-minio"
	MonitorThanosQuery       = "thanos-query"
	MonitorGrafana           = "monitor-grafana"
	MonitorStackGrafanaDB    = "grafana"
	MonitorStackGrafanaDBUer = "grafana"
	MonitorLoki              = "monitor-loki"
	MonitorPrometheus        = "prometheus-monitor-monitor-stack-prometheus"

	MonitorBucketThanos = "thanos"
	MonitorBucketLoki   = "loki"
)

const (
	SysAdminUsername = "sysadmin"
	SysAdminProject  = "system"
	DefaultDomain    = "Default"

	// note: service node port in range 30000-32767
	KeystoneDB         = "keystone"
	KeystoneDBUser     = "keystone"
	KeystonePublicPort = 30500
	KeystoneAdminPort  = 30357

	GlanceDB           = "glance"
	GlanceDBUser       = "glance"
	GlanceAdminUser    = "glance"
	GlanceAdminProject = SysAdminProject
	// GlanceRegistryPort = 30191
	GlanceAPIPort     = 30292
	ServiceNameGlance = "glance"
	ServiceTypeGlance = "image"
	GlanceDataStore   = "/opt/cloud/workspace/data/glance"
	QemuPath          = "/usr/local/qemu-2.12.1"
	KernelPath        = "/lib/modules"

	BaremetalsPath    = "/opt/cloud/workspace/baremetals"
	BaremetalTFTPRoot = "/opt/cloud/yunion/baremetal"

	RegionAdminUser    = "regionadmin"
	RegionAdminProject = SysAdminProject
	RegionPort         = 30888
	SchedulerPort      = 30887
	RegionDB           = "yunioncloud"
	RegionDBUser       = "yunioncloud"

	ServiceNameHost  = "host"
	ServiceTypeHost  = "host"
	HostAdminUser    = "hostadmin"
	HostAdminProject = SysAdminProject
	// Host not use node port
	HostPort = 8885

	ServiceNameBaremetal  = "baremetal"
	ServiceTypeBaremetal  = "baremetal"
	BaremetalAdminUser    = "baremetal"
	BaremetalAdminProject = SysAdminProject
	// Baremetal not use node port
	BaremetalPort = 8879

	KubeServerAdminUser = "kubeserver"
	KubeServerPort      = 30442
	KubeServerDB        = "kubeserver"
	KubeServerDBUser    = "kubeserver"

	WebconsoleAdminUser    = "webconsole"
	WebconsoleAdminProject = SysAdminProject
	WebconsolePort         = 30899
	WebconsoleDB           = "webconsole"
	WebconsoleDBUser       = "webconsole"

	LoggerAdminUser = "loggeradmin"
	LoggerPort      = 30999
	LoggerDB        = "yunionlogger"
	LoggerDBUser    = "yunionlogger"

	ServiceNameAPIGateway = "yunionapi"
	ServiceTypeAPIGateway = "yunionapi"
	ServiceNameWebsocket  = "websocket"
	ServiceTypeWebsocket  = "websocket"
	APIGatewayAdminUser   = "yunionapi"
	APIGatewayPort        = 30300
	APIWebsocketPort      = 30443

	YunionAgentAdminUser = "yunionagent"
	YunionAgentPort      = 30898
	YunionAgentDB        = "yunionagent"
	YunionAgentDBUser    = "yunionagent"

	YunionConfAdminUser = "yunionconf"
	YunionConfPort      = 30889
	YunionConfDB        = "yunionconf"
	YunionConfDBUser    = "yunionconf"

	NotifyAdminUser = "notify"
	NotifyPort      = 30777
	NotifyDB        = "notify"
	NotifyDBUser    = "notify"

	InfluxdbPort      = 30086
	InfluxdbDataStore = "/var/lib/influxdb"

	VictoriaMetricsPort                    = 30428
	VictoriaMetricsDefaultRententionPeriod = 93
	VictoriaMetricsDataStore               = "/storage"

	OvnNorthDbPort = 32241
	OvnSouthDbPort = 32242

	MonitorAdminUser    = "monitoradmin"
	MonitorAdminProject = SysAdminProject
	MonitorPort         = 30093
	MonitorDB           = "monitor"
	MonitorDBUser       = "monitor"

	AnsibleServerAdminUser    = "ansibleadmin"
	AnsibleServerAdminProject = SysAdminProject
	AnsibleServerPort         = 30890
	AnsibleServerDB           = "yunionansible"
	AnsibleServerDBUser       = "yunionansible"

	CloudnetAdminUser    = "cloudnetadmin"
	CloudnetAdminProject = SysAdminProject
	CloudnetPort         = 30891
	CloudnetDB           = "yunioncloudnet"
	CloudnetDBUser       = "yunioncloudnet"

	CloudproxyAdminUser    = "cloudproxyadmin"
	CloudproxyAdminProject = SysAdminProject
	CloudproxyPort         = 30882
	CloudproxyDB           = "yunioncloudproxy"
	CloudproxyDBUser       = "yunioncloudproxy"

	CloudeventAdminUser    = "cloudeventadmin"
	CloudeventAdminProject = SysAdminProject
	CloudeventPort         = 30892
	CloudeventDB           = "yunioncloudevent"
	CloudeventDBUser       = "yunioncloudevent"

	CloudIdAdminUser    = "cloudidadmin"
	CloudIdAdminProject = SysAdminProject
	CloudIdPort         = 30893
	CloudIdDB           = "yunioncloudid"
	CloudIdDBUser       = "yunioncloudid"

	EndpointTypeInternal = "internal"
	EndpointTypePublic   = "public"
	EndpointTypeAdmin    = "admin"
	EndpointTypeConsole  = "console"

	// define service constants
	ServiceNameKeystone = "keystone"
	ServiceTypeIdentity = "identity"

	ServiceNameRegion    = "region"
	ServiceNameRegionV2  = "region2"
	ServiceTypeCompute   = "compute"
	ServiceTypeComputeV2 = "compute_v2"

	ServiceNameScheduler = "scheduler"
	ServiceTypeScheduler = "scheduler"

	ServiceNameWebconsole = "webconsole"
	ServiceTypeWebconsole = "webconsole"

	ServiceNameLogger = "log"
	ServiceTypeLogger = "log"

	ServiceNameYunionConf = "yunionconf"
	ServiceTypeYunionConf = "yunionconf"

	ServiceNameYunionAgent = "yunionagent"
	ServiceTypeYunionAgent = "yunionagent"

	ServiceNameInfluxdb = "influxdb"
	ServiceTypeInfluxdb = "influxdb"

	ServiceNameVictoriaMetrics = "victoria-metrics"
	ServiceTypeVictoriaMetrics = "victoria-metrics"

	ServiceNameMonitor = "monitor"
	ServiceTypeMonitor = "monitor"

	ServiceNameOvnNorthDb = "ovn-north-db"
	ServiceTypeOvnNorthDb = "ovn-north-db"
	ServiceNameOvnSouthDb = "ovn-south-db"
	ServiceTypeOvnSouthDb = "ovn-south-db"

	ServiceNameKubeServer = "k8s"
	ServiceTypeKubeServer = "k8s"

	ServiceNameAnsibleServer = "ansible"
	ServiceTypeAnsibleServer = "ansible"

	ServiceNameCloudnet = "cloudnet"
	ServiceTypeCloudnet = "cloudnet"

	ServiceNameCloudproxy = "cloudproxy"
	ServiceTypeCloudproxy = "cloudproxy"

	ServiceNameCloudevent = "cloudevent"
	ServiceTypeCloudevent = "cloudevent"

	ServiceNameCloudId = "cloudid"
	ServiceTypeCloudId = "cloudid"

	ServiceNameCloudmon  = "cloudmon"
	ServiceTypeCloudmon  = "cloudmon"
	CloudmonPort         = 30931
	CloudmonAdminUser    = "cloudmon"
	CloudmonAdminProject = SysAdminProject

	ServiceNameNotify = "notify"
	ServiceTypeNotify = "notify"

	ServiceNameVpcagent  = "vpcagent"
	ServiceTypeVpcagent  = "vpcagent"
	VpcAgentAdminUser    = "vpcagentadmin"
	VpcAgentAdminProject = SysAdminProject
	VpcAgentPort         = 30932

	BaremetalDataStore = "/opt/cloud/workspace"

	ServiceURLCloudmeta  = "https://meta.yunion.cn"
	ServiceNameCloudmeta = "cloudmeta"
	ServiceTypeCloudmeta = "cloudmeta"

	ServiceURLTorrentTracker  = "https://tracker.yunion.cn"
	ServiceNameTorrentTracker = "torrent-tracker"
	ServiceTypeTorrentTracker = "torrent-tracker"

	ServiceNameAutoUpdate  = "autoupdate"
	ServiceTypeAutoUpdate  = "autoupdate"
	AutoUpdateAdminUser    = "autoupdate"
	AutoUpdateAdminProject = SysAdminProject
	AutoUpdatePort         = 30981
	AutoUpdateDB           = "autoupdate"
	AutoUpdateDBUser       = "autoupdate"

	NetworkTypeBaremetal = "baremetal"
	NetworkTypeServer    = "server"

	ServiceNameExternal = "external-service"
	ServiceTypeExternal = ServiceNameExternal

	ServiceNameCommon = "common"
	ServiceTypeCommon = ServiceNameCommon

	ServiceNameOfflineCloudmeta = "offlinecloudmeta"
	ServiceTypeOfflineCloudmeta = "offlinecloudmeta"
	ServiceURLOfflineCloudmeta  = "https://yunionmeta.oss-cn-beijing.aliyuncs.com"

	ServiceNameS3gateway  = "s3gateway"
	ServiceTypeS3gateway  = "s3gateway"
	S3gatewayPort         = 30884
	S3gatewayAdminUser    = "s3gatewayadm"
	S3gatewayAdminProject = SysAdminProject

	ServiceNameDevtool  = "devtool"
	ServiceTypeDevtool  = "devtool"
	DevtoolPort         = 30997
	DevtoolAdminUser    = "devtooladmin"
	DevtoolAdminProject = SysAdminProject
	DevtoolDB           = "devtool"
	DevtoolDBUser       = "devtool"

	ServiceNameMeter  = "meter"
	ServiceTypeMeter  = "meter"
	MeterPort         = 30909
	MeterAdminUser    = "meterdocker"
	MeterAdminProject = SysAdminProject
	MeterDB           = "yunionmeter"
	MeterDBUser       = "yunionmeter"

	MeterDataStore         = "/opt/yunion/meter"
	MeterBillingDataDir    = "billing"
	MeterRatesDataDir      = "rates"
	MeterInfluxDB          = "meter_db"
	MeterMonthlyBill       = true
	MeterAwsRiPlanIdHandle = "true"

	ServiceNameBilling  = "billing"
	ServiceTypeBilling  = "billing"
	BillingPort         = 30404
	BillingAdminUser    = "billingadmin"
	BillingAdminProject = SysAdminProject
	BillingDB           = "yunionbilling"
	BillingDBUser       = "yunionbilling"

	EsxiAgentAdminUser = "esxiagent"
	EsxiAgentPort      = 30883
	EsxiAgentDataStore = "/opt/cloud/workspace"

	ServiceOperatorAdminUser = "osOperator"
	ServiceOperatorPort      = 30885

	EtcdClientPort            = 2379
	EtcdClientNodePort        = 30379
	EtcdPeerPort              = 2380
	EtcdImageName             = "etcd"
	EtcdDefaultClusterSize    = 3
	EtcdImageVersion          = "3.4.6"
	EtcdDefaultRequestTimeout = 5 * time.Second
	EtcdDefaultDialTimeout    = 3 * time.Second
	BusyboxImageName          = "busybox"
	BusyboxImageVersion       = "1.28.0-glibc"
	ServiceNameEtcd           = "etcd"
	ServiceTypeEtcd           = ServiceNameEtcd
	ServiceCertEtcdName       = ServiceNameEtcd

	ItsmAdminUser   = "itsm"
	ItsmPort        = 30595
	ItsmDB          = "itsm"
	ItsmDBUser      = "itsm"
	ServiceNameItsm = "itsm"
	ServiceTypeItsm = "itsm"

	ServiceNameSuggestion = "suggestion"
	ServiceTypeSuggestion = "suggestion"
	SuggestionPort        = 30987

	ServiceNameScheduledtask = "scheduledtask"
	ServiceTypeScheduledtask = "scheduledtask"
	ScheduledtaskPort        = 30978

	ServiceNameAPIMap = "apimap"
	ServiceTypeAPIMap = "apimap"
	APIMapPort        = 31999

	ServiceNameReport  = "report"
	ServiceTypeReport  = "report"
	ReportAdminUser    = "report"
	ReportAdminProject = SysAdminProject
	ReportPort         = 30967
	ReportDB           = "report"
	ReportDBUser       = "report"

	ServiceNameLbagent  = "lbagent"
	ServiceTypeLbagent  = "lbagent"
	LbagentAdminUser    = "lbagentadm"
	LbagentAdminProject = SysAdminProject
	LbagentPort         = 8895

	ServiceNameEChartsSSR = "echarts-ssr"
	ServiceTypeEChartsSSR = "echarts-ssr"
	EChartsSSRPort        = 8081

	ServiceNameBastionHost  = "bastionhost"
	ServiceTypeBastionHost  = "bastionhost"
	BastionHostAdminUser    = "bastionhost"
	BastionHostAdminProject = SysAdminProject
	BastionHostPort         = 30983
	BastionHostDB           = "bastionhost"
	BastionHostDBUser       = "bastionhost"

	ServiceNameExtdb  = "extdb"
	ServiceTypeExtdb  = "extdb"
	ExtdbAdminUser    = "extdb"
	ExtdbAdminProject = SysAdminProject
	ExtdbPort         = 30991
	ExtdbDB           = "extdb"
	ExtdbDBUser       = "extdb"
)

var (
	GlanceFileStoreDir            = path.Join(GlanceDataStore, "images")
	GlanceTorrentStoreDir         = path.Join(GlanceDataStore, "torrents")
	SpecifiedPresistentVolumePath = "pvc.onecloud.yunion.io/pv-path"
)

var (
	APICallRetryInterval = 1 * time.Second

	// CACertAndKeyBaseName defines certificate authority base name
	CACertAndKeyBaseName = "ca"
	// CACertName defines certificate name
	CACertName = "ca.crt"
	// CAKeyName defines certificate name
	CAKeyName = "ca.key"

	CertDir                   = "/etc/yunion/pki"
	ServiceCertAndKeyBaseName = "service"
	ServiceCertName           = "service.crt"
	ServiceKeyName            = "service.key"

	ConfigDir        = "/etc/yunion"
	VolumeConfigName = "config"
	VolumeCertsName  = "certs"

	Localhost = "localhost"

	EtcdServerSecret = "etcd-server"
	EtcdClientSecret = "etcd-client"
	EtcdPeerSecret   = "etcd-peer"
	EtcdClientTLSDir = "/etc/etcdtls/operator/etcd-tls"

	EtcdServerName       = "server"
	EtcdServerCACertName = "server-ca"
	EtcdServerCertName   = "server.crt"
	EtcdServerKeyName    = "server.key"
	EtcdClientName       = "etcd-client"
	EtcdClientCACertName = "etcd-client-ca"
	EtcdClientCertName   = "etcd-client.crt"
	EtcdClientKeyName    = "etcd-client.key"
	EtcdPeerName         = "peer"
	EtcdPeerCACertName   = "peer-ca"
	EtcdPeerCertName     = "peer.crt"
	EtcdPeerKeyName      = "peer.key"
)

const (
	DefaultDialTimeout    = 5 * time.Second
	DefaultRequestTimeout = 5 * time.Second
	// DefaultBackupTimeout is the default maximal allowed time of the entire backup process.
	DefaultBackupTimeout    = 1 * time.Minute
	DefaultSnapshotInterval = 1800 * time.Second

	DefaultBackupPodHTTPPort = 19999

	OperatorRoot   = "/var/tmp/etcd-operator"
	BackupMountDir = "/var/etcd-backup"

	EnvOperatorPodName      = "MY_POD_NAME"
	EnvOperatorPodNamespace = "MY_POD_NAMESPACE"
)

const (
	RoleAdmin         = "admin"
	RoleFA            = "fa"
	RoleSA            = "sa"
	RoleProjectOwner  = "project_owner"
	RoleMember        = "member"
	RoleDomainAdmin   = "domainadmin"
	RoleProjectEditor = "project_editor"
)

const (
	DockerComposeClusterName = "docker-compose-cluster"
)
