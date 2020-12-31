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

	"yunion.io/x/onecloud/pkg/keystone/locale"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/onecloud/pkg/util/httputils"
)

func createOrUpdatePolicy(s *mcclient.ClientSession, policy locale.SPolicyData) error {
	policyJson, err := modules.Policies.GetByName(s, policy.Name, nil)
	if err != nil {
		if httputils.ErrorCode(err) == 404 {
			// not found, to create
			params := jsonutils.NewDict()
			params.Add(jsonutils.NewString(policy.Name), "name")
			params.Add(jsonutils.NewString(policy.Name), "type")
			params.Add(jsonutils.NewString(policy.Name), "description")
			params.Add(policy.Policy, "policy")
			params.Add(jsonutils.NewString(string(policy.Scope)), "scope")
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
		isSystem := jsonutils.QueryBoolean(policyJson, "is_system", false)
		desc, _ := policyJson.GetString("description")
		if remotePolicy == nil || !policy.Policy.Equals(remotePolicy) || desc != policy.Name || !isSystem {
			// need to update policy data
			pid, _ := policyJson.GetString("id")
			params := jsonutils.NewDict()
			params.Add(jsonutils.NewString(policy.Name), "description")
			params.Add(policy.Policy, "policy")
			params.Add(jsonutils.NewString(string(policy.Scope)), "scope")
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

func createOrUpdateRole(s *mcclient.ClientSession, role locale.SRoleDefiniton) error {
	roleJson, err := modules.RolesV3.GetByName(s, role.Name, nil)
	if err != nil {
		if httputils.ErrorCode(err) == 404 {
			// role not exists, create one
			params := jsonutils.NewDict()
			params.Add(jsonutils.NewString(role.Name), "name")
			params.Add(jsonutils.NewString(role.Name), "description")
			params.Add(jsonutils.JSONTrue, "is_public")
			roleJson, err = modules.RolesV3.Create(s, params)
			if err != nil {
				return errors.Wrap(err, "RolesV3.Create")
			}
		} else {
			return errors.Wrap(err, "RolesV3.GetByName")
		}
	} else {
		// role exists, update description
		idstr, _ := roleJson.GetString("id")
		desc, _ := roleJson.GetString("description")
		if desc != role.Name {
			params := jsonutils.NewDict()
			params.Add(jsonutils.NewString(role.Name), "description")
			params.Add(jsonutils.JSONTrue, "is_public")
			roleJson, err = modules.RolesV3.Update(s, idstr, params)
			if err != nil {
				return errors.Wrap(err, "RolesV3.Update")
			}
		}
	}
	roleId, _ := roleJson.GetString("id")
	// perform add policy
	for i := range role.Policies {
		policy := role.Policies[i]
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
