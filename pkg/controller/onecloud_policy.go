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

package controller

import (
	"golang.org/x/sync/errgroup"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
)

const (
	PolicyDomainAdmin = `
# project owner, allow do any with her project resources
roles:
  - domainadmin
scope: domain
policy:
  '*': allow
`
	PolicyMember = `
# rbac for normal user, not allow for delete
scope: project
roles:
  - member
policy:
  '*':
    '*':
      '*': allow
      delete: deny
  compute:
    networks:
      create: deny
      perform: deny
      delete: deny
      update: deny
`
	PolicyProjectFA = `
# project finance administrator, allow any operation in meter
roles:
  - fa
scope: project
policy:
  meter: allow
`
	PolicyProjectOwner = `
# project owner, allow do any with her project resources
roles:
  - project_owner
  - admin
scope: project
policy:
  '*': allow
  compute:
    networks:
      create: deny
      perform: deny
      delete: deny
      update: deny
`
	PolicyProjectSA = `
# project system administrator, allow any operation in compute, image, k8s
roles:
  - sa
scope: project
policy:
  compute: allow
  image: allow
  k8s: owner
  compute:
    networks:
      create: deny
      perform: deny
      delete: deny
      update: deny
`
	PolicySysAdmin = `
# system wide administrator, root of the platform, can do anything
projects:
  - system
roles:
  - admin
scope: system
policy:
  '*': allow
`
	PolicySysFA = `
# system wide financial administrator, can do anything wrt billing&metering
projects:
  - system
roles:
  - fa
scope: system
policy:
  meter: allow
`
	PolicySysSA = `
# system wide ops administrator, can do anything wrt compute/image/k8s
projects:
  - system
roles:
  - sa
scope: system
policy:
  compute: allow
  image: allow
  k8s: allow
`
)

type Policies map[string]string

var DefaultPolicies Policies
var DefaultRoles map[string]string

func init() {
	DefaultPolicies = map[string]string{
		constants.PolicyTypeDomainAdmin:  PolicyDomainAdmin,
		constants.PolicyTypeMember:       PolicyMember,
		constants.PolicyTypeProjectFA:    PolicyProjectFA,
		constants.PolicyTypeProjectOwner: PolicyProjectOwner,
		constants.PolicyTypeProjectSA:    PolicyProjectSA,
		constants.PolicyTypeSysAdmin:     PolicySysAdmin,
		constants.PolicyTypeSysFA:        PolicySysFA,
		constants.PolicyTypeSysSA:        PolicySysSA,
	}

	DefaultRoles = map[string]string{
		constants.RoleAdmin:        "系统管理员",
		constants.RoleFA:           "财务管理员",
		constants.RoleSA:           "运维管理员",
		constants.RoleProjectOwner: "项目主管",
		constants.RoleMember:       "普通成员",
		constants.RoleDomainAdmin:  "域管理员",
	}
}

func PolicyCreate(s *mcclient.ClientSession, policyType string, content string, enable bool) (jsonutils.JSONObject, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(policyType), "type")
	params.Add(jsonutils.NewString(content), "policy")
	if enable {
		params.Add(jsonutils.JSONTrue, "enabled")
	} else {
		params.Add(jsonutils.JSONFalse, "enabled")
	}
	return modules.Policies.Create(s, params)
}

func PoliciesPublic(s *mcclient.ClientSession, types []string) error {
	var errgrp errgroup.Group
	for _, t := range types {
		pt := t
		errgrp.Go(func() error {
			policyJson, err := modules.Policies.Get(s, pt, nil)
			if err != nil {
				return errors.Wrapf(err, "modules.Policies.Get %s", pt)
			}
			if !jsonutils.QueryBoolean(policyJson, "is_public", false) {
				_, err = modules.Policies.PerformAction(s, pt, "public", nil)
				if err != nil {
					return errors.Wrap(err, "Policies.PerformAction")
				}
			}
			return nil
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	return nil
}

func RolesPublic(s *mcclient.ClientSession, roles []string) error {
	var errgrp errgroup.Group
	for _, t := range roles {
		pt := t
		errgrp.Go(func() error {
			roleJson, err := modules.RolesV3.Get(s, pt, nil)
			if err != nil {
				return errors.Wrapf(err, "modules.RolesV3.Get %s", pt)
			}
			if !jsonutils.QueryBoolean(roleJson, "is_public", false) {
				_, err := modules.RolesV3.PerformAction(s, pt, "public", nil)
				if err != nil {
					return errors.Wrap(err, "RolesV3.PerformAction")
				}
			}
			return nil
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	return nil
}
