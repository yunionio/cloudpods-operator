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

package v1alpha1

import (
	"errors"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	StartingDeadlineSeconds int64   = 300
	CronjobMonitorExpand    float64 = 1.2
)

// ComponentType represents component type
type ComponentType string
type EtcdClusterPhase string
type EtcdClusterConditionType string

const (
	OnecloudClusterResourceKind   = "OnecloudCluster"
	OnecloudClusterResourcePlural = "onecloudclusters"

	EtcdClusterPhaseNone     EtcdClusterPhase = ""
	EtcdClusterPhaseCreating                  = "Creating"
	EtcdClusterPhaseRunning                   = "Running"
	EtcdClusterPhaseFailed                    = "Failed"

	EtcdClusterConditionAvailable  EtcdClusterConditionType = "Available"
	EtcdClusterConditionRecovering                          = "Recovering"
	EtcdClusterConditionScaling                             = "Scaling"
	EtcdClusterConditionUpgrading                           = "Upgrading"
)

var (
	OnecloudClusterCRDName = OnecloudClusterResourcePlural + "." + GroupName
)

const (
	// KeystoneComponentType is keystone component type
	KeystoneComponentType ComponentType = "keystone"
	// RegionComponentType is region component type
	RegionComponentType ComponentType = "region"
	// ClimcComponentType is climc component type
	ClimcComponentType ComponentType = "climc"
	// GlanceComponentType is glance component type
	GlanceComponentType ComponentType = "glance"
	// WebconsoleComponentType is webconsole component type
	WebconsoleComponentType ComponentType = "webconsole"
	// SchedulerComponentType is scheduler component type
	SchedulerComponentType ComponentType = "scheduler"
	// LogComponentType is logger service component type
	LoggerComponentType ComponentType = "logger"
	// InfluxdbComponentType is influxdb component type
	InfluxdbComponentType ComponentType = "influxdb"
	// MonitorComponentType is alert monitor component type
	MonitorComponentType ComponentType = "monitor"
	// APIGatewayComponentType is apiGateway component type
	APIGatewayComponentType ComponentType = "apigateway"
	//APIGatewayComponentTypeEE is enterprise edition apiGateway
	APIGatewayComponentTypeEE ComponentType = "apigateway-ee"
	// WebComponentType is web frontent component type
	WebComponentType            ComponentType = "web"
	YunionagentComponentType    ComponentType = "yunionagent"
	YunionconfComponentType     ComponentType = "yunionconf"
	KubeServerComponentType     ComponentType = "kubeserver"
	AnsibleServerComponentType  ComponentType = "ansibleserver"
	CloudnetComponentType       ComponentType = "cloudnet"
	CloudeventComponentType     ComponentType = "cloudevent"
	NotifyComponentType         ComponentType = "notify"
	HostComponentType           ComponentType = "host"
	HostDeployerComponentType   ComponentType = "host-deployer"
	HostImageComponentType      ComponentType = "host-image"
	BaremetalAgentComponentType ComponentType = "baremetal-agent"
	// S3gatewayComponentType is multi-cloud S3 object storage gateway
	S3gatewayComponentType ComponentType = "s3gateway"
	// DevtoolComponentType is devops tool based on ansible
	DevtoolComponentType ComponentType = "devtool"
	// MeterComponentType is meter service
	MeterComponentType ComponentType = "meter"
	// AutoUpdateComponentType is autoupdate service
	AutoUpdateComponentType ComponentType = "autoupdate"
	// CloudmonPing is ping cronjob
	CloudmonPingComponentType ComponentType = "cloudmon-ping"
	// CloudmonReportUsage is report-usage cronjob
	CloudmonReportUsageComponentType ComponentType = "cloudmon-report-usage"
	// CloudmonReportServerAli is report-usage cronjob
	CloudmonReportServerComponentType ComponentType = "cloudmon-report-server"
	// CloudmonReportHost is report-host cronjob
	CloudmonReportHostComponentType ComponentType = "cloudmon-report-host"
	// Cloudmon is report monitor data deployment
	CloudmonComponentType ComponentType = "cloudmon"
	// Esxi Agent
	EsxiAgentComponentType ComponentType = "esxi-agent"
	// Onecloud Reource Operator
	ServiceOperatorComponentType ComponentType = "onecloud-service-operator"

	OvnNorthComponentType ComponentType = "ovn-north"
	OvnHostComponentType  ComponentType = "ovn-host"
	VpcAgentComponentType ComponentType = "vpcagent"

	RegionDNSComponentType ComponentType = "region-dns"

	// Etcd component types
	EtcdComponentType       ComponentType = "etcd"
	EtcdClientComponentType ComponentType = "etcd-client"

	ItsmComponentType ComponentType = "itsm"
	// Telegraf is monitor agent component type
	TelegrafComponentType ComponentType = "telegraf"
	CloudIdComponentType  ComponentType = "cloudid"
)

// ComponentPhase is the current state of component
type ComponentPhase string

const (
	// NormalPhase represents normal state of OneCloud cluster.
	NormalPhase ComponentPhase = "Normal"
	// UpgradePhase represents the upgrade state of Onecloud cluster.
	UpgradePhase ComponentPhase = "Upgrade"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OnecloudCluster defines the cluster
type OnecloudCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec defines the behavior of a onecloud cluster
	Spec OnecloudClusterSpec `json:"spec"`

	// Most recently observed status of the onecloud cluster
	Status OnecloudClusterStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OnecloudClusterList represents a list of Onecloud Clusters
type OnecloudClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OnecloudCluster `json:"items"`
}

// OnecloudClusterSpec describes the attributes that a user creates on a onecloud cluster
type OnecloudClusterSpec struct {
	// Etcd holds configuration for etcd
	Etcd Etcd `json:"etcd,omitempty"`
	// Mysql holds configuration for mysql
	Mysql Mysql `json:"mysql"`
	// Version is onecloud components version
	Version string `json:"version"`
	// CertSANs sets extra Subject Alternative Names for the Cluster signing cert.
	CertSANs []string
	// Services list non-headless services type used in OnecloudCluster
	Services []Service `json:"services,omitempty"`
	// ImageRepository defines default image registry
	ImageRepository string `json:"imageRepository"`
	// Region is cluster region
	Region string `json:"region"`
	// Zone is cluster first zone
	Zone string `json:"zone"`
	// Keystone holds configuration for keystone
	Keystone KeystoneSpec `json:"keystone"`
	// RegionServer holds configuration for region
	RegionServer RegionSpec `json:"regionServer"`
	// RegionDNS holds configuration for region-dns
	RegionDNS RegionDNSSpec `json:"regionDNS"`
	// Scheduler holds configuration for scheduler
	Scheduler DeploymentSpec `json:"scheduler"`
	// Glance holds configuration for glance
	Glance StatefulDeploymentSpec `json:"glance"`
	// Climc holds configuration for climc
	Climc DeploymentSpec `json:"climc"`
	// Webconsole holds configuration for webconsole
	Webconsole DeploymentSpec `json:"webconsole"`
	// Logger holds configuration for log service
	Logger DeploymentSpec `json:"logger"`
	// Yunionconf holds configuration for yunionconf service
	Yunionconf DeploymentSpec `json:"yunionconf"`
	// Yunionagent holds configuration for yunionagent service
	Yunionagent DaemonSetSpec `json:"yunionagent"`
	// Influxdb holds configuration for influxdb
	Influxdb StatefulDeploymentSpec `json:"influxdb"`
	// Telegraf holds configuration for telegraf
	Telegraf TelegrafSpec `json:"telegraf"`
	// Monitor holds configuration for monitor service
	Monitor DeploymentSpec `json:"monitor"`
	// LoadBalancerEndpoint is upstream loadbalancer virtual ip address or DNS domain
	LoadBalancerEndpoint string `json:"loadBalancerEndpoint"`
	// APIGateway holds configuration for yunoinapi
	APIGateway DeploymentSpec `json:"apiGateway"`
	// Web holds configuration for web
	Web DeploymentSpec `json:"web"`
	// KubeServer holds configuration for kube-server service
	KubeServer DeploymentSpec `json:"kubeserver"`
	// AnsibleServer holds configuration for ansibleserver service
	AnsibleServer DeploymentSpec `json:"ansibleserver"`
	// Cloudnet holds configuration for cloudnet service
	Cloudnet DeploymentSpec `json:"cloudnet"`
	// Cloudevent holds configuration for cloudevent service
	Cloudevent DeploymentSpec `json:"cloudevent"`
	// Notify holds configuration for notify service
	Notify StatefulDeploymentSpec `json:"notify"`
	// HostAgent holds configuration for host
	HostAgent HostAgentSpec `json:"hostagent"`
	// HostDeployer holds configuration for host-deployer
	HostDeployer DaemonSetSpec `json:"hostdeployer"`
	// HostImage holds configration for host-image
	HostImage DaemonSetSpec `json:"hostimage"`
	// BaremetalAgent holds configuration for baremetal agent
	BaremetalAgent StatefulDeploymentSpec `json:"baremetalagent"`
	// S3gateway holds configuration for s3gateway service
	S3gateway DeploymentSpec `json:"s3gateway"`
	// Devtool holds configuration for devtool service
	Devtool DeploymentSpec `json:"devtool"`
	// Meter holds configuration for meter
	Meter StatefulDeploymentSpec `json:"meter"`
	// AutoUpdate holds configuration for autoupdate
	AutoUpdate DeploymentSpec `json:"autoupdate"`
	// Cloudmon holds configuration for report monitor data
	Cloudmon CloudmonSpec `json:"cloudmon"`
	// EsxiAgent hols configuration for esxi agent
	EsxiAgent StatefulDeploymentSpec `json:"esxiagent"`
	// Itsm holds configuration for itsm service
	Itsm DeploymentSpec `json:"itsm"`

	// ServiceOperator hols configuration for service-operator
	ServiceOperator DeploymentSpec `json:"onecloudServiceOperator"`

	OvnNorth DeploymentSpec `json:"ovnNorth"`
	VpcAgent DeploymentSpec `json:"vpcAgent"`

	// Cloudid holds configuration for cloudid service
	CloudId DeploymentSpec `json:"cloudid"`
}

// OnecloudClusterStatus describes cluster status
type OnecloudClusterStatus struct {
	ClusterID      string           `json:"clusterID,omitempty"`
	Keystone       KeystoneStatus   `json:"keystone,omitempty"`
	RegionServer   RegionStatus     `json:"region,omitempty"`
	Glance         GlanceStatus     `json:"glance,omitempty"`
	Scheduler      DeploymentStatus `json:"scheduler,omitempty"`
	Webconsole     DeploymentStatus `json:"webconsole,omitempty"`
	Influxdb       DeploymentStatus `json:"influxdb,omitempty"`
	Monitor        DeploymentStatus `json:"monitor,omitempty"`
	Logger         DeploymentStatus `json:"logger,omitempty"`
	APIGateway     DeploymentStatus `json:"apiGateway,omitempty"`
	Web            DeploymentStatus `json:"web,omitempty"`
	Yunionconf     DeploymentStatus `json:"yunionconf,omitempty"`
	KubeServer     DeploymentStatus `json:"kubeserver,omitempty"`
	AnsibleServer  DeploymentStatus `json:"ansibleserver,omitempty"`
	Cloudnet       DeploymentStatus `json:"cloudnet,omitempty"`
	Cloudevent     DeploymentStatus `json:"cloudevent,omitempty"`
	Notify         DeploymentStatus `json:"notify,omitempty"`
	BaremetalAgent DeploymentStatus `json:"baremetalagent,omitempty"`
	S3gateway      DeploymentStatus `json:"s3gateway,omitempty"`
	Devtool        DeploymentStatus `json:"devtool,omitempty"`
	Meter          MeterStatus      `json:"meter,omitempty"`
	AutoUpdate     DeploymentStatus `json:"autoupdate,omitempty"`
	EsxiAgent      DeploymentStatus `json:"esxiagent,omitempty"`
	OvnNorth       DeploymentStatus `json:"ovnNorth,omitempty"`
	VpcAgent       DeploymentStatus `json:"vpcAgent,omitempty"`
	Etcd           EctdStatus       `json:"etcd,omitempty"`
	Itsm           DeploymentStatus `json:"itsm,omitempty"`
	CloudId        DeploymentStatus `json:"cloudid,omitempty"`
}

type EtcdClusterSpec struct {
	// Size is the expected size of the etcd cluster.
	// The etcd-operator will eventually make the size of the running
	// cluster equal to the expected size.
	// The vaild range of the size is from 1 to 7.
	Size int `json:"size"`
	// Repository is the name of the repository that hosts
	// etcd container images. It should be direct clone of the repository in official
	// release:
	//   https://github.com/coreos/etcd/releases
	// That means, it should have exact same tags and the same meaning for the tags.
	//
	// By default, it is `quay.io/coreos/etcd`.
	Repository string `json:"repository,omitempty"`

	// Version is the expected version of the etcd cluster.
	// The etcd-operator will eventually make the etcd cluster version
	// equal to the expected version.
	//
	// The version must follow the [semver]( http://semver.org) format, for example "3.2.13".
	// Only etcd released versions are supported: https://github.com/coreos/etcd/releases
	//
	// If version is not set, default is "3.2.13".
	Version string `json:"version,omitempty"`

	// Paused is to pause the control of the operator for the etcd cluster.
	Paused bool `json:"paused,omitempty"`

	// Pod defines the policy to create pod for the etcd pod.
	//
	// Updating Pod does not take effect on any existing etcd pods.
	Pod *PodPolicy `json:"pod,omitempty"`

	// etcd cluster TLS configuration
	TLS *TLSPolicy `json:"TLS,omitempty"`
}

// TLSPolicy defines the TLS policy of an etcd cluster
type TLSPolicy struct {
	// StaticTLS enables user to generate static x509 certificates and keys,
	// put them into Kubernetes secrets, and specify them into here.
	Static *StaticTLS `json:"static,omitempty"`
}

type StaticTLS struct {
	// Member contains secrets containing TLS certs used by each etcd member pod.
	Member *MemberSecret `json:"member,omitempty"`
	// OperatorSecret is the secret containing TLS certs used by operator to
	// talk securely to this cluster.
	OperatorSecret string `json:"operatorSecret,omitempty"`
}

type MemberSecret struct {
	// PeerSecret is the secret containing TLS certs used by each etcd member pod
	// for the communication between etcd peers.
	PeerSecret string `json:"peerSecret,omitempty"`
	// ServerSecret is the secret containing TLS certs used by each etcd member pod
	// for the communication between etcd server and its clients.
	ServerSecret string `json:"serverSecret,omitempty"`
}

func (tp *TLSPolicy) Validate() error {
	if tp.Static == nil {
		return nil
	}
	st := tp.Static

	if len(st.OperatorSecret) != 0 {
		if len(st.Member.ServerSecret) == 0 {
			return errors.New("operator secret set but member serverSecret not set")
		}
	} else if st.Member != nil && len(st.Member.ServerSecret) != 0 {
		return errors.New("member serverSecret set but operator secret not set")
	}
	return nil
}

func (tp *TLSPolicy) IsSecureClient() bool {
	if tp == nil || tp.Static == nil {
		return false
	}
	return len(tp.Static.OperatorSecret) != 0
}

func (tp *TLSPolicy) IsSecurePeer() bool {
	if tp == nil || tp.Static == nil || tp.Static.Member == nil {
		return false
	}
	return len(tp.Static.Member.PeerSecret) != 0
}

// PodPolicy defines the policy to create pod for the etcd container.
type PodPolicy struct {
	// Labels specifies the labels to attach to pods the operator creates for the
	// etcd cluster.
	// "app" and "etcd_*" labels are reserved for the internal use of the etcd operator.
	// Do not overwrite them.
	Labels map[string]string `json:"labels,omitempty"`

	// NodeSelector specifies a map of key-value pairs. For the pod to be eligible
	// to run on a node, the node must have each of the indicated key-value pairs as
	// labels.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// The scheduling constraints on etcd pods.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// **DEPRECATED**. Use Affinity instead.
	AntiAffinity bool `json:"antiAffinity,omitempty"`

	// Resources is the resource requirements for the etcd container.
	// This field cannot be updated once the cluster is created.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Tolerations specifies the pod's tolerations.
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// List of environment variables to set in the etcd container.
	// This is used to configure etcd process. etcd cluster cannot be created, when
	// bad environement variables are provided. Do not overwrite any flags used to
	// bootstrap the cluster (for example `--initial-cluster` flag).
	// This field cannot be updated.
	EtcdEnv []corev1.EnvVar `json:"etcdEnv,omitempty"`

	// PersistentVolumeClaimSpec is the spec to describe PVC for the etcd container
	// This field is optional. If no PVC spec, etcd container will use emptyDir as volume
	// Note. This feature is in alpha stage. It is currently only used as non-stable storage,
	// not the stable storage. Future work need to make it used as stable storage.
	PersistentVolumeClaimSpec *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimSpec,omitempty"`

	// Annotations specifies the annotations to attach to pods the operator creates for the
	// etcd cluster.
	// The "etcd.version" annotation is reserved for the internal use of the etcd operator.
	Annotations map[string]string `json:"annotations,omitempty"`

	// busybox init container image. default is busybox:1.28.0-glibc
	// busybox:latest uses uclibc which contains a bug that sometimes prevents name resolution
	// More info: https://github.com/docker-library/busybox/issues/27
	BusyboxImage string `json:"busyboxImage,omitempty"`

	// SecurityContext specifies the security context for the entire pod
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// DNSTimeoutInSecond is the maximum allowed time for the init container of the etcd pod to
	// reverse DNS lookup its IP given the hostname.
	// The default is to wait indefinitely and has a vaule of 0.
	DNSTimeoutInSecond int64 `json:"DNSTimeoutInSecond,omitempty"`
}

type Etcd struct {
	// etcd operator cluster spec
	EtcdClusterSpec
	// disable etcd
	Disable bool `json:"disable"`
	// enable tls
	EnableTls bool `json:"enableTls"`
}

// Mysql describes an mysql server
type Mysql struct {
	// Host is mysql ip address of hostname
	Host string `json:"host"`
	// Port is mysql listen port
	Port int32 `json:"port"`
	// Username is mysql username
	Username string `json:"username"`
	// Password is mysql user password
	Password string `json:"password"`
}

// DeploymentSpec constains defails of deployment resource service
type DeploymentSpec struct {
	ContainerSpec
	Disable      bool                `json:"disable"`
	Replicas     int32               `json:"replicas"`
	Affinity     *corev1.Affinity    `json:"affinity,omitempty"`
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
	Annotations  map[string]string   `json:"annotations,omitempty"`
}

type CronJobSpec struct {
	ContainerSpec
	Disable      bool                `json:"disable"`
	Schedule     string              `json:"schedule"`
	Affinity     *corev1.Affinity    `json:"affinity,omitempty"`
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
	Annotations  map[string]string   `json:"annotations,omitempty"`
}

type DaemonSetSpec struct {
	ContainerSpec
	Disable      bool                `json:"disable"`
	Affinity     *corev1.Affinity    `json:"affinity,omitempty"`
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
	Annotations  map[string]string   `json:"annotations,omitempty"`
}

type StatefulDeploymentSpec struct {
	DeploymentSpec
	StorageClassName string `json:"storageClassName,omitempty"`
}

// KeystoneSpec contains details of keystone service
type KeystoneSpec struct {
	DeploymentSpec
	BootstrapPassword string `json:"bootstrapPassword"`
}

// ImageStatus is the image status of a pod
type ImageStatus struct {
	Image           string            `json:"image"`
	Repository      string            `json:"repository"`
	ImageName       string            `json:"imageName"`
	Tag             string            `json:"tag"`
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`
}

type DeploymentStatus struct {
	Phase       ComponentPhase         `json:"phase,omitempty"`
	Deployment  *apps.DeploymentStatus `json:"deployment,omitempty"`
	ImageStatus *ImageStatus           `json:"imageStatus,omitempty"`
}

// KeystoneStatus is Keystone status
type KeystoneStatus struct {
	DeploymentStatus
}

type RegionStatus struct {
	DeploymentStatus
	RegionId     string
	RegionZoneId string
	ZoneId       string
	WireId       string
}

type GlanceStatus struct {
	DeploymentStatus
}

type WebconsoleStatus struct {
	DeploymentStatus
}

type MeterStatus struct {
	DeploymentStatus
}

type EctdStatus struct {
	// Phase is the cluster running phase
	Phase  EtcdClusterPhase `json:"phase"`
	Reason string           `json:"reason,omitempty"`

	// Condition keeps track of all cluster conditions, if they exist.
	Conditions []EtcdClusterCondition `json:"conditions,omitempty"`

	// Size is the current size of the cluster
	Size int `json:"size"`

	// ServiceName is the LB service for accessing etcd nodes.
	ServiceName string `json:"serviceName,omitempty"`

	// ClientPort is the port for etcd client to access.
	// It's the same on client LB service and etcd nodes.
	ClientPort int `json:"clientPort,omitempty"`

	// Members are the etcd members in the cluster
	Members EtcdMembersStatus `json:"members"`
	// CurrentVersion is the current cluster version
	CurrentVersion string `json:"currentVersion"`
	// TargetVersion is the version the cluster upgrading to.
	// If the cluster is not upgrading, TargetVersion is empty.
	TargetVersion string `json:"targetVersion"`
}

// ClusterCondition represents one current condition of an etcd cluster.
// A condition might not show up if it is not happening.
// For example, if a cluster is not upgrading, the Upgrading condition would not show up.
// If a cluster is upgrading and encountered a problem that prevents the upgrade,
// the Upgrading condition's status will would be False and communicate the problem back.
type EtcdClusterCondition struct {
	// Type of cluster condition.
	Type EtcdClusterConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

type EtcdMembersStatus struct {
	// Ready are the etcd members that are ready to serve requests
	// The member names are the same as the etcd pod names
	Ready []string `json:"ready,omitempty"`
	// Unready are the etcd members not ready to serve requests
	Unready []string `json:"unready,omitempty"`
}

type RegionSpec struct {
	DeploymentSpec
	// DNSServer is the address of DNS server
	DNSServer string `json:"dnsServer"`
	// DNSDomain is the global default dns domain suffix for virtual servers
	DNSDomain string `json:"dnsDomain"`
}

type CloudmonSpec struct {
	DeploymentSpec
	CloudmonPingDuration               uint
	CloudmonReportHostDuration         uint
	CloudmonReportServerDuration       uint
	CloudmonReportUsageDuration        uint
	CloudmonReportCloudAccountDuration uint
}

type RegionDNSProxy struct {
	// check: https://coredns.io/plugins/proxy/
	From string   `json:"from"`
	To   []string `json:"to"`
	// Policy string `json:"policy"`
}

type RegionDNSSpec struct {
	DaemonSetSpec
	Proxies []RegionDNSProxy `json:"proxies"`
}

type RegionDNSStatus struct {
	DeploymentStatus
}

type HostAgentSpec struct {
	DaemonSetSpec
	SdnAgent      ContainerSpec
	OvnController ContainerSpec
}

type TelegrafSpec struct {
	DaemonSetSpec
	InitContainerImage string
}

// ContainerSpec is the container spec of a pod
type ContainerSpec struct {
	Image           string               `json:"image"`
	Repository      string               `json:"repository,omitempty"`
	ImageName       string               `json:"imageName,omitempty"`
	Tag             string               `json:"tag,omitempty"`
	ImagePullPolicy corev1.PullPolicy    `json:"imagePullPolicy,omitempty"`
	Requests        *ResourceRequirement `json:"requests,omitempty"`
	Limits          *ResourceRequirement `json:"limits,omitempty"`
}

// Service represent service type used in OnecloudCluster
type Service struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

type ResourceRequirement struct {
	// CPU is how many cores a pod requires
	CPU string `json:"cpu,omitempty"`
	// Memory is how much memory a pod requires
	Memory string `json:"memory,omitempty"`
	// Storage is storage size a pod requires
	Storage string `json:"storage,omitempty"`
}

type DBConfig struct {
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type CloudUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ServiceBaseConfig struct {
	Port int `json:"port"`
}

type ServiceCommonOptions struct {
	ServiceBaseConfig

	CloudUser
}

type KeystoneConfig struct {
	ServiceBaseConfig

	DB DBConfig `json:"db"`
}

type ServiceDBCommonOptions struct {
	ServiceCommonOptions

	DB DBConfig `json:"db"`
}

type RegionConfig struct {
	ServiceDBCommonOptions
}

type GlanceConfig struct {
	ServiceDBCommonOptions
}

type MeterConfig struct {
	ServiceDBCommonOptions
}

type HostConfig struct {
	ServiceCommonOptions
}

type BaremetalConfig struct {
	ServiceCommonOptions
}

type EsxiAgentConfig struct {
	ServiceCommonOptions
}

type VpcAgentConfig struct {
	ServiceCommonOptions
}

type ItsmConfig struct {
	ServiceDBCommonOptions
	SecondDatabase string `json:"secondDatabase"`
	EncryptionKey  string `json:"encryptionKey"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OnecloudClusterConfig struct {
	metav1.TypeMeta

	Keystone        KeystoneConfig         `json:"keystone"`
	RegionServer    RegionConfig           `json:"region"`
	Glance          GlanceConfig           `json:"glance"`
	Webconsole      ServiceCommonOptions   `json:"webconsole"`
	Logger          ServiceDBCommonOptions `json:"logger"`
	Yunionconf      ServiceDBCommonOptions `json:"yunionconf"`
	Yunionagent     ServiceDBCommonOptions `json:"yunionagent"`
	KubeServer      ServiceDBCommonOptions `json:"kubeserver"`
	AnsibleServer   ServiceDBCommonOptions `json:"ansibleserver"`
	Monitor         ServiceDBCommonOptions `json:"monitor"`
	Cloudnet        ServiceDBCommonOptions `json:"cloudnet"`
	Cloudevent      ServiceDBCommonOptions `json:"cloudevent"`
	APIGateway      ServiceCommonOptions   `json:"apiGateway"`
	Notify          ServiceDBCommonOptions `json:"notify"`
	HostAgent       HostConfig             `json:"host"`
	BaremetalAgent  BaremetalConfig        `json:"baremetal"`
	S3gateway       ServiceCommonOptions   `json:"s3gateway"`
	Devtool         ServiceDBCommonOptions `json:"devtool"`
	Meter           MeterConfig            `json:"meter"`
	AutoUpdate      ServiceCommonOptions   `json:"autoupdate"`
	EsxiAgent       EsxiAgentConfig        `json:"esxiagent"`
	VpcAgent        VpcAgentConfig         `json:"vpcagent"`
	ServiceOperator ServiceCommonOptions   `json:"onecloudServiceOperator"`
	Itsm            ItsmConfig             `json:"itsm"`
	CloudId         ServiceDBCommonOptions `json:"cloudid"`
}
