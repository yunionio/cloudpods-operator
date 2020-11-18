package controller

import (
	"yunion.io/x/onecloud/pkg/util/rbacutils"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
)

type sPolicyDefinition struct {
	Name     string
	Desc     string
	Scope    rbacutils.TRbacScope
	Services map[string][]string
	Extra    map[string]map[string][]string
}

type sRoleDefiniton struct {
	Name        string
	Description string
	Policy      string
	Project     string
}

var (
	policyDefinitons = []sPolicyDefinition{
		{
			Name:  "",
			Desc:  "任意资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"*": nil,
			},
		},
		{
			Name:  "dashboard",
			Desc:  "控制面板查看相关资源",
			Scope: rbacutils.ScopeProject,
			Extra: map[string]map[string][]string{
				"compute": {
					"capabilities": {
						"list",
					},
					"usages": {
						"list",
						"get",
					},
				},
				"image": {
					"usages": {
						"list",
						"get",
					},
				},
				"identity": {
					"usages": {
						"list",
						"get",
					},
				},
				"meter": {
					"bill_conditions": {
						"list",
					},
				},
				"monitor": {
					"alertresources": {
						"list",
					},
					"unifiedmonitors": {
						"perform",
					},
				},
				"log": {
					"actions": {
						"list",
					},
				},
			},
		},
		{
			Name:  "compute",
			Desc:  "计算服务(云主机与容器)相关资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"compute": nil,
				"image":   nil,
				"k8s":     nil,
			},
		},
		{
			Name:  "server",
			Desc:  "云主机相关资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"compute": {
					"servers",
					"servertemplates",
					"instancegroups",
					"scalinggroups",
					"scalingactivities",
					"scalingpolicies",
					"disks",
					"networks",
					"eips",
					"snapshotpolicies",
					"snapshotpolicycaches",
					"snapshotpolicydisks",
					"snapshots",
					"instance_snapshots",
					"snapshotpolicies",
					"secgroupcaches",
					"secgrouprules",
					"secgroups",
				},
				"image": nil,
			},
			Extra: map[string]map[string][]string{
				"compute": {
					"isolated_devices": {
						"get",
						"list",
					},
				},
			},
		},
		{
			Name:  "host",
			Desc:  "宿主机和物理机相关资源",
			Scope: rbacutils.ScopeDomain,
			Services: map[string][]string{
				"compute": {
					"hosts",
					"isolated_devices",
					"hostwires",
					"hoststorages",
					"baremetalagents",
					"baremetalnetworks",
					"baremetalevents",
				},
			},
		},
		{
			Name:  "storage",
			Desc:  "云硬盘存储相关资源",
			Scope: rbacutils.ScopeDomain,
			Services: map[string][]string{
				"compute": {
					"storages",
				},
			},
		},
		{
			Name:  "loadbalancer",
			Desc:  "负载均衡相关资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"compute": {
					"loadbalanceracls",
					"loadbalanceragents",
					"loadbalancerbackendgroups",
					"loadbalancerbackends",
					"loadbalancercertificates",
					"loadbalancerclusters",
					"loadbalancerlistenerrules",
					"loadbalancerlisteners",
					"loadbalancernetworks",
					"loadbalancers",
				},
			},
			Extra: map[string]map[string][]string{
				"compute": {
					"networks": {
						"get",
						"list",
					},
				},
			},
		},
		{
			Name:  "oss",
			Desc:  "对象存储相关资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"compute": {
					"buckets",
				},
			},
		},
		{
			Name:  "dbinstance",
			Desc:  "关系型数据库(MySQL等)相关资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"compute": {
					"dbinstance_skus",
					"dbinstanceaccounts",
					"dbinstancebackups",
					"dbinstancedatabases",
					"dbinstancenetworks",
					"dbinstanceparameters",
					"dbinstanceprivileges",
					"dbinstances",
				},
			},
		},
		{
			Name:  "elasticcache",
			Desc:  "弹性缓存(Redis等)相关资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"compute": {
					"elasticcacheaccounts",
					"elasticcacheacls",
					"elasticcachebackups",
					"elasticcacheparameters",
					"elasticcaches",
					"elasticcacheskus",
				},
			},
		},
		{
			Name:  "network",
			Desc:  "网络相关资源",
			Scope: rbacutils.ScopeDomain,
			Services: map[string][]string{
				"compute": {
					"vpcs",
					"wires",
					"natdentries",
					"natgateways",
					"natsentries",
					"networkinterfacenetworks",
					"networkinterfaces",
					"networks",
					"reservedips",
					"route_tables",
					"globalvpcs",
					"vpc_peering_connections",
					"eips",
					"dns_recordsets",
					"dns_trafficpolicies",
					"dns_zonecaches",
					"dns_zones",
					"dnsrecords",
				},
			},
		},
		{
			Name:  "meter",
			Desc:  "计费计量分析服务相关资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"meter": nil,
			},
		},
		{
			Name:  "identity",
			Desc:  "身份认证(IAM)服务相关资源",
			Scope: rbacutils.ScopeDomain,
			Services: map[string][]string{
				"identity": nil,
			},
		},
		{
			Name:  "image",
			Desc:  "镜像服务相关资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"image": nil,
			},
		},
		{
			Name:  "monitor",
			Desc:  "监控服务相关资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"monitor": nil,
			},
		},
		{
			Name:  "container",
			Desc:  "容器服务相关资源",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"k8s": nil,
			},
		},
		{
			Name:  "cloudid",
			Desc:  "云用户及权限管理相关资源",
			Scope: rbacutils.ScopeDomain,
			Services: map[string][]string{
				"compute": {
					"cloudaccounts",
					"cloudproviders",
				},
				"identity": {
					"users",
					"projects",
					"roles",
				},
				"cloudid": nil,
			},
		},
		{
			Name:  "cloudaccount",
			Desc:  "云账号管理相关资源",
			Scope: rbacutils.ScopeDomain,
			Services: map[string][]string{
				"compute": {
					"cloudaccounts",
					"cloudproviderquotas",
					"cloudproviderregions",
					"cloudproviders",
				},
			},
		},
		{
			Name:  "projectresource",
			Desc:  "项目管理相关资源",
			Scope: rbacutils.ScopeDomain,
			Services: map[string][]string{
				"compute": {
					"project_quotas",
					"quotas",
					"region_quotas",
					"zone_quotas",
				},
				"image": {
					"image_quotas",
				},
				"identity": {
					"projects",
					"roles",
					"policies",
				},
			},
		},
		{
			Name:  "domainresource",
			Desc:  "域管理相关资源",
			Scope: rbacutils.ScopeSystem,
			Services: map[string][]string{
				"compute": {
					"domain_quotas",
					"infras_quotas",
				},
				"identity": {
					"domains",
					"identity_quotas",
					"projects",
					"roles",
					"policies",
					"users",
					"groups",
				},
			},
		},
		{
			Name:  "notify",
			Desc:  "通知服务相关资源",
			Scope: rbacutils.ScopeSystem,
			Services: map[string][]string{
				"notify": nil,
			},
		},
	}

	roleDefinitions = []sRoleDefiniton{
		{
			Name:        constants.RoleAdmin,
			Description: "系统管理员",
			Policy:      "sysadmin",
			Project:     "system",
		},
		{
			Name:        constants.RoleDomainAdmin,
			Description: "域管理员",
			Policy:      "domainadmin",
		},
		{
			Name:        constants.RoleProjectOwner,
			Description: "项目主管",
			Policy:      "projectadmin",
		},
		{
			Name:        constants.RoleFA,
			Description: "财务管理员",
			Policy:      "sysmeteradmin",
		},
	}
)
