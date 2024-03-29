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

package rbacutils

import (
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/util/tagutils"
)

type SPolicy struct {
	// policy rules
	Rules TPolicy
	// tags for domains
	DomainTags tagutils.TTagSetList
	// tags for projects
	ProjectTags tagutils.TTagSetList
	// tags for resources
	ObjectTags tagutils.TTagSetList
}

func (policy SPolicy) GetMatchRule(service string, resource string, action string, extra ...string) *SPolicyMatch {
	rule := GetMatchRule(policy.Rules, service, resource, action, extra...)
	if rule == nil {
		return nil
	}
	return &SPolicyMatch{
		Rule:        *rule,
		DomainTags:  policy.DomainTags,
		ProjectTags: policy.ProjectTags,
		ObjectTags:  policy.ObjectTags,
	}
}

func DecodePolicy(policyJson jsonutils.JSONObject) (*SPolicy, error) {
	tags := []tagutils.TTagSetList{
		{}, // domain
		{}, // project
		{}, // resource
	}
	for i, key := range []string{
		DomainTagsKey,
		ProjectTagsKey,
		ObjectTagsKey,
	} {
		if policyJson.Contains(key) {
			err := policyJson.Unmarshal(&tags[i], key)
			if err != nil {
				tmpTagSet := make(tagutils.TTagSet, 0)
				err2 := policyJson.Unmarshal(&tmpTagSet, key)
				if err2 == nil {
					tags[i] = append(tags[i], tmpTagSet)
				} else {
					return nil, errors.Wrapf(errors.NewAggregate([]error{err, err2}), "Unmarshal TTagSetList %s", key)
				}
			}
		}
	}
	rules, err := decodePolicy(policyJson.(*jsonutils.JSONDict).CopyExcludes(DomainTagsKey, ProjectTagsKey, ObjectTagsKey))
	if err != nil {
		return nil, errors.Wrap(err, "decodePolicy")
	}
	return &SPolicy{
		Rules:       rules,
		DomainTags:  tags[0],
		ProjectTags: tags[1],
		ObjectTags:  tags[2],
	}, nil
}

func DecodePolicyData(domainTags, projectTags, objectTags tagutils.TTagSetList, input jsonutils.JSONObject) (*SPolicy, error) {
	rules, err := DecodeRawPolicyData(input)
	if err != nil {
		return nil, errors.Wrap(err, "decodePolicyData")
	}
	return &SPolicy{
		Rules:       rules,
		DomainTags:  domainTags,
		ProjectTags: projectTags,
		ObjectTags:  objectTags,
	}, nil
}

func (policy SPolicy) Encode() jsonutils.JSONObject {
	ret := rules2Json(policy.Rules)
	if dict, ok := ret.(*jsonutils.JSONDict); ok {
		// In order to make compatible with old policy client that supports TTagset
		if len(policy.DomainTags) > 1 {
			dict.Add(jsonutils.Marshal(policy.DomainTags), DomainTagsKey)
		} else if len(policy.DomainTags) == 1 {
			dict.Add(jsonutils.Marshal(policy.DomainTags[0]), DomainTagsKey)
		}
		if len(policy.ProjectTags) > 1 {
			dict.Add(jsonutils.Marshal(policy.ProjectTags), ProjectTagsKey)
		} else if len(policy.ProjectTags) == 1 {
			dict.Add(jsonutils.Marshal(policy.ProjectTags[0]), ProjectTagsKey)
		}
		if len(policy.ObjectTags) > 1 {
			dict.Add(jsonutils.Marshal(policy.ObjectTags), ObjectTagsKey)
		} else if len(policy.ObjectTags) == 1 {
			dict.Add(jsonutils.Marshal(policy.ObjectTags[0]), ObjectTagsKey)
		}
	} else {
		log.Fatalf("rule2Json output a NonJSON???")
	}
	return ret
}

// policy1 contains policy2 means
//  1. any action allow in policy2 is allowed in policy1
//  2. policy tags of policy1 contains of policy tags of policy2
func (policy1 SPolicy) Contains(policy2 SPolicy) bool {
	if !policy1.Rules.Contains(policy2.Rules) {
		return false
	}
	if !policy1.DomainTags.ContainsAll(policy2.DomainTags) {
		return false
	}
	if !policy1.ProjectTags.ContainsAll(policy2.ProjectTags) {
		return false
	}
	if !policy1.ObjectTags.ContainsAll(policy2.ObjectTags) {
		return false
	}
	return true
}
