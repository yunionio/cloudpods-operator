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
)

const (
	OnecloudEditionAnnotationKey string = "onecloud.yunion.io/edition"
	OnecloudCommunityEdition     string = "ce"
	OnecloudEnterpriseEdition    string = "ee"
)

const (
	OnecloudClusterKind               = "OnecloudCluster"
	OnecloudClusterConfigKind         = "OnecloudClusterConfig"
	OnecloudClusterConfigConfigMapKey = OnecloudClusterConfigKind
)

const (
	SysAdminUsername = "sysadmin"
	SysAdminProject  = "system"
	DefaultDomain    = "Default"

	KeystoneDB         = "keystone"
	KeystoneDBUser     = "keystone"
	KeystonePublicPort = 5000
	KeystoneAdminPort  = 35357

	GlanceDB           = "glance"
	GlanceDBUser       = "glance"
	GlanceAdminUser    = "glance"
	GlanceAdminProject = SysAdminProject
	GlanceRegistryPort = 9191
	GlanceAPIPort      = 9292
	ServiceNameGlance  = "glance"
	ServiceTypeGlance  = "image"
	GlanceDataStore    = "/opt/cloud/workspace/data/glance"
	QemuPath           = "/usr/local/qemu-2.12.1"
	KernelPath         = "/lib/modules"

	BaremetalsPath    = "/opt/cloud/workspace/baremetals"
	BaremetalTFTPRoot = "/opt/cloud/yunion/baremetal"

	RegionAdminUser    = "regionadmin"
	RegionAdminProject = SysAdminProject
	RegionPort         = 8889
	SchedulerPort      = 8897
	RegionDB           = "yunioncloud"
	RegionDBUser       = "yunioncloud"

	BaremetalAdminUser    = "baremetal"
	BaremetalAdminProject = SysAdminProject
	BaremetalPort         = 8879

	KubeServerAdminUser = "kubeserver"
	KubeServerPort      = 8443
	KubeServerDB        = "kubeserver"
	KubeServerDBUser    = "kubeserver"

	WebconsoleAdminUser    = "webconsole"
	WebconsoleAdminProject = SysAdminProject
	WebconsolePort         = 8899

	LoggerAdminUser = "loggeradmin"
	LoggerPort      = 9999
	LoggerDB        = "yunionlogger"
	LoggerDBUser    = "yunionlogger"

	APIGatewayAdminUser = "yunionapi"
	APIGatewayPort      = 9300
	APIWebsocketPort    = 10443

	YunionAgentAdminUser = "yunionagent"
	YunionAgentPort      = 9899
	YunionAgentDB        = "yunionagent"
	YunionAgentDBUser    = "yunionagent"

	YunionConfAdminUser = "yunionconf"
	YunionConfPort      = 9889
	YunionConfDB        = "yunionconf"
	YunionConfDBUser    = "yunionconf"

	NotifyAdminUser = "notify"
	NotifyPort      = 7777
	NotifyDB        = "notify"
	NotifyDBUser    = "notify"

	InfluxdbPort      = 8086
	InfluxdbDataStore = "/var/lib/influxdb"

	AnsibleServerAdminUser    = "ansibleadmin"
	AnsibleServerAdminProject = SysAdminProject
	AnsibleServerPort         = 8890
	AnsibleServerDB           = "yunionansible"
	AnsibleServerDBUser       = "yunionansible"

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

	ServiceNameKubeServer = "k8s"
	ServiceTypeKubeServer = "k8s"

	ServiceNameAnsibleServer = "ansible"
	ServiceTypeAnsibleServer = "ansible"

	ServiceNameNotify = "notify"
	ServiceTypeNotify = "notify"

	ServiceURLCloudmeta  = "https://meta.yunion.cn"
	ServiceNameCloudmeta = "cloudmeta"
	ServiceTypeCloudmeta = "cloudmeta"

	ServiceURLTorrentTracker  = "https://tracker.yunion.cn"
	ServiceNameTorrentTracker = "torrent-tracker"
	ServiceTypeTorrentTracker = "torrent-tracker"

	NetworkTypeBaremetal = "baremetal"
	NetworkTypeServer    = "server"
)

const (
	RoleAdmin        = "admin"
	RoleFA           = "fa"
	RoleSA           = "sa"
	RoleProjectOwner = "project_owner"
	RoleMember       = "member"
	RoleDomainAdmin  = "domainadmin"

	PolicyTypeDomainAdmin  = "domainadmin"
	PolicyTypeMember       = "member"
	PolicyTypeProjectFA    = "projectfa"
	PolicyTypeProjectOwner = "projectowner"
	PolicyTypeProjectSA    = "projectsa"
	PolicyTypeSysAdmin     = "sysadmin"
	PolicyTypeSysFA        = "sysfa"
	PolicyTypeSysSA        = "syssa"
)

var (
	PublicRoles = []string{
		RoleFA,
		RoleSA,
		RoleProjectOwner,
		RoleMember,
		RoleDomainAdmin,
	}
	PublicPolicies = []string{
		PolicyTypeDomainAdmin, PolicyTypeProjectOwner,
		PolicyTypeProjectSA, PolicyTypeProjectFA,
		PolicyTypeMember,
	}

	GlanceFileStoreDir    = path.Join(GlanceDataStore, "images")
	GlanceTorrentStoreDir = path.Join(GlanceDataStore, "torrents")
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
)
