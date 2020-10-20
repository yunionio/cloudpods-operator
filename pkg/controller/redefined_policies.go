package controller

import (
	"yunion.io/x/onecloud/pkg/util/rbacutils"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
)

type sPolicyDefinition struct {
	Name     string
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
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"*": nil,
			},
		},
		{
			Name:  "dashboard",
			Scope: rbacutils.ScopeProject,
			Extra: map[string]map[string][]string{
				"meter": {
					"bill_conditions": {
						"list",
					},
				},
				"monitor": {
					"alertresources": {
						"list",
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
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"compute": nil,
				"image":   nil,
				"k8s":     nil,
			},
		},
		{
			Name:  "server",
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
			Scope: rbacutils.ScopeDomain,
			Services: map[string][]string{
				"compute": {
					"storages",
				},
			},
		},
		{
			Name:  "loadbalancer",
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
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"compute": {
					"buckets",
				},
			},
		},
		{
			Name:  "dbinstance",
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
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"meter": nil,
			},
		},
		{
			Name:  "identity",
			Scope: rbacutils.ScopeDomain,
			Services: map[string][]string{
				"identity": nil,
			},
		},
		{
			Name:  "image",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"image": nil,
			},
		},
		{
			Name:  "monitor",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"monitor": nil,
			},
		},
		{
			Name:  "container",
			Scope: rbacutils.ScopeProject,
			Services: map[string][]string{
				"k8s": nil,
			},
		},
		{
			Name:  "cloudid",
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
			Name:  "project",
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
			Name:  "domain",
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
