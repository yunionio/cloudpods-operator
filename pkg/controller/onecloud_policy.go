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
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/onecloud/pkg/util/httputils"
	"yunion.io/x/onecloud/pkg/util/rbacutils"
)

var (
	allowResult = jsonutils.NewString("allow")
	denyResult  = jsonutils.NewString("deny")

	editorAction = getEditActionPolicy()
	viewerAction = getViewerActionPolicy()
)

func getAdminPolicy(services map[string][]string) jsonutils.JSONObject {
	policy := jsonutils.NewDict()
	for k, resList := range services {
		if len(resList) == 0 {
			policy.Add(allowResult, k)
		} else {
			resPolicy := jsonutils.NewDict()
			for i := range resList {
				resPolicy.Add(allowResult, resList[i])
			}
			policy.Add(resPolicy, k)
		}
	}
	return policy
}

func getEditActionPolicy() jsonutils.JSONObject {
	p := jsonutils.NewDict()
	p.Add(denyResult, "create")
	p.Add(denyResult, "delete")
	perform := jsonutils.NewDict()
	perform.Add(denyResult, "purge")
	perform.Add(denyResult, "clone")
	perform.Add(allowResult, "*")
	p.Add(perform, "perform")
	p.Add(allowResult, "*")
	return p
}

func getViewerActionPolicy() jsonutils.JSONObject {
	p := jsonutils.NewDict()
	p.Add(allowResult, "get")
	p.Add(allowResult, "list")
	p.Add(denyResult, "*")
	return p
}

func getEditorPolicy(services map[string][]string) jsonutils.JSONObject {
	policy := jsonutils.NewDict()
	for k, resList := range services {
		if len(resList) == 0 {
			resList = []string{"*"}
		}
		resPolicy := jsonutils.NewDict()
		for i := range resList {
			resPolicy.Add(editorAction, resList[i])
		}
		policy.Add(resPolicy, k)
	}
	return policy
}

func getViewerPolicy(services map[string][]string) jsonutils.JSONObject {
	policy := jsonutils.NewDict()
	for k, resList := range services {
		if len(resList) == 0 {
			resList = []string{"*"}
		}
		resPolicy := jsonutils.NewDict()
		for i := range resList {
			resPolicy.Add(viewerAction, resList[i])
		}
		policy.Add(resPolicy, k)
	}
	return policy
}

func addExtraPolicy(policy *jsonutils.JSONDict, extra map[string]map[string][]string) jsonutils.JSONObject {
	for s, resources := range extra {
		resourcePolicy := jsonutils.NewDict()
		for r, actions := range resources {
			actionPolicy := jsonutils.NewDict()
			for i := range actions {
				actionPolicy.Add(allowResult, actions[i])
			}
			actionPolicy.Add(denyResult, "*")
			resourcePolicy.Add(actionPolicy, r)
		}
		policy.Add(resourcePolicy, s)
	}
	return policy
}

func generateAllPolicies() []sPolicyData {
	ret := make([]sPolicyData, 0)
	for i := range policyDefinitons {
		def := policyDefinitons[i]
		for _, scope := range []rbacutils.TRbacScope{
			rbacutils.ScopeSystem,
			rbacutils.ScopeDomain,
			rbacutils.ScopeProject,
		} {
			if scope.HigherEqual(def.Scope) {
				ps := generatePolicies(scope, def)
				ret = append(ret, ps...)
			}
		}
	}
	return ret
}

type sPolicyData struct {
	name        string
	scope       rbacutils.TRbacScope
	policy      jsonutils.JSONObject
	description string
}

func generatePolicies(scope rbacutils.TRbacScope, def sPolicyDefinition) []sPolicyData {
	level := ""
	switch scope {
	case rbacutils.ScopeSystem:
		level = "sys"
		if def.Scope == rbacutils.ScopeSystem {
			level = ""
		}
	case rbacutils.ScopeDomain:
		level = "domain"
	case rbacutils.ScopeProject:
		level = "project"
	}

	type sRoleConf struct {
		name       string
		policyFunc func(services map[string][]string) jsonutils.JSONObject
		fullName   string
	}

	var roleConfs []sRoleConf
	if len(def.Services) > 0 {
		roleConfs = []sRoleConf{
			{
				name:       "admin",
				policyFunc: getAdminPolicy,
				fullName:   "管理",
			},
			{
				name:       "editor",
				policyFunc: getEditorPolicy,
				fullName:   "编辑/操作",
			},
			{
				name:       "viewer",
				policyFunc: getViewerPolicy,
				fullName:   "只读",
			},
		}
	} else {
		roleConfs = []sRoleConf{
			{
				name:       "",
				policyFunc: nil,
				fullName:   "",
			},
		}
	}

	ret := make([]sPolicyData, 0)
	for _, role := range roleConfs {
		name := fmt.Sprintf("%s%s%s", level, def.Name, role.name)
		var policy jsonutils.JSONObject
		if def.Services != nil {
			policy = role.policyFunc(def.Services)
		} else {
			policy = jsonutils.NewDict()
		}
		policy = addExtraPolicy(policy.(*jsonutils.JSONDict), def.Extra)
		desc := ""
		switch scope {
		case rbacutils.ScopeSystem:
			desc += "全局"
		case rbacutils.ScopeDomain:
			desc += "本域内"
		case rbacutils.ScopeProject:
			desc += "本项目内"
		}
		if len(def.Desc) > 0 {
			desc += def.Desc
		}
		if len(role.fullName) > 0 {
			desc += role.fullName
		}
		desc += "权限"
		policyJson := jsonutils.NewDict()
		policyJson.Add(policy, "policy")
		ret = append(ret, sPolicyData{
			name:        name,
			scope:       scope,
			policy:      policyJson,
			description: strings.TrimSpace(desc),
		})
	}
	return ret
}

func createOrUpdatePolicy(s *mcclient.ClientSession, policy sPolicyData) error {
	policyJson, err := modules.Policies.GetByName(s, policy.name, nil)
	if err != nil {
		if httputils.ErrorCode(err) == 404 {
			// not found, to create
			params := jsonutils.NewDict()
			params.Add(jsonutils.NewString(policy.name), "name")
			params.Add(jsonutils.NewString(policy.name), "type")
			params.Add(jsonutils.NewString(policy.description), "description")
			params.Add(policy.policy, "policy")
			params.Add(jsonutils.NewString(string(policy.scope)), "scope")
			params.Add(jsonutils.JSONTrue, "enabled")
			params.Add(jsonutils.JSONTrue, "is_system")
			params.Add(jsonutils.JSONTrue, "is_public")
			_, err := modules.Policies.Create(s, params)
			if err != nil {
				return errors.Wrap(err, "Policies.Create")
			}
			return nil
		} else {
			return errors.Wrap(err, "Policies.GetByName")
		}
	} else {
		// find policy, do update
		remotePolicy, _ := policyJson.Get("policy")
		desc, _ := policyJson.GetString("description")
		if remotePolicy == nil || !policy.policy.Equals(remotePolicy) || desc != policy.description {
			// need to update policy data
			pid, _ := policyJson.GetString("id")
			params := jsonutils.NewDict()
			params.Add(jsonutils.NewString(policy.description), "description")
			params.Add(policy.policy, "policy")
			params.Add(jsonutils.NewString(string(policy.scope)), "scope")
			params.Add(jsonutils.JSONTrue, "enabled")
			params.Add(jsonutils.JSONTrue, "is_system")
			_, err := modules.Policies.Update(s, pid, params)
			if err != nil {
				return errors.Wrap(err, "Policies.Update")
			}
			return nil
		} else {
			// no change, no need to update
			return nil
		}
	}
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

func createOrUpdateRole(s *mcclient.ClientSession, role sRoleDefiniton) error {
	roleJson, err := modules.RolesV3.GetByName(s, role.Name, nil)
	if err != nil {
		if httputils.ErrorCode(err) == 404 {
			// role not exists, create one
			params := jsonutils.NewDict()
			params.Add(jsonutils.NewString(role.Name), "name")
			params.Add(jsonutils.NewString(role.Description), "description")
			params.Add(jsonutils.JSONTrue, "is_public")
			roleJson, err = modules.RolesV3.Create(s, params)
			if err != nil {
				return errors.Wrap(err, "RolesV3.Create")
			}
		} else {
			return errors.Wrap(err, "RolesV3.GetByName")
		}
	} else {
		// role exists
	}
	roleId, _ := roleJson.GetString("id")
	// perform add policy
	for _, policy := range role.Policies {
		params := jsonutils.NewDict()
		if len(role.Project) > 0 {
			params.Add(jsonutils.NewString(role.Project), "project_id")
		}
		params.Add(jsonutils.NewString(policy), "policy_id")
		_, err = modules.RolesV3.PerformAction(s, roleId, "add-policy", params)
		if err != nil {
			return errors.Wrap(err, "RolesV3.PerformAction add-policy")
		}
	}
	return nil
}
