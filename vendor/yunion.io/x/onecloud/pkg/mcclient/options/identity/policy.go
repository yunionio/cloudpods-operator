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

package identity

import (
	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/mcclient/options"
)

type PolicyListOptions struct {
	options.BaseListOptions
	Type          string `help:"filter by type"`
	IsSystem      *bool  `help:"filter by is_system" negative:"is_no_system"`
	Format        string `help:"policy format, default to yaml" default:"yaml" choices:"yaml|json"`
	OrderByDomain string `help:"order by domain name" choices:"asc|desc"`
	Role          string `help:"filter by role"`
}

func (opts *PolicyListOptions) Params() (jsonutils.JSONObject, error) {
	return options.ListStructToParams(opts)
}

type PolicyGetPropertyTagValuePairOptions struct {
	PolicyListOptions
	options.TagValuePairsOptions
}

func (opts *PolicyGetPropertyTagValuePairOptions) Params() (jsonutils.JSONObject, error) {
	params, err := opts.PolicyListOptions.Params()
	if err != nil {
		return nil, errors.Wrap(err, "PolicyListOptions.Params")
	}
	tagParams, _ := opts.TagValuePairsOptions.Params()
	params.(*jsonutils.JSONDict).Update(tagParams)
	return params, nil
}

type PolicyGetPropertyTagValueTreeOptions struct {
	PolicyListOptions
	options.TagValueTreeOptions
}

func (opts *PolicyGetPropertyTagValueTreeOptions) Params() (jsonutils.JSONObject, error) {
	params, err := opts.PolicyListOptions.Params()
	if err != nil {
		return nil, errors.Wrap(err, "PolicyListOptions.Params")
	}
	tagParams, _ := opts.TagValueTreeOptions.Params()
	params.(*jsonutils.JSONDict).Update(tagParams)
	return params, nil
}

type PolicyGetPropertyDomainTagValuePairOptions struct {
	PolicyListOptions
	options.DomainTagValuePairsOptions
}

func (opts *PolicyGetPropertyDomainTagValuePairOptions) Params() (jsonutils.JSONObject, error) {
	params, err := opts.PolicyListOptions.Params()
	if err != nil {
		return nil, errors.Wrap(err, "PolicyListOptions.Params")
	}
	tagParams, _ := opts.DomainTagValuePairsOptions.Params()
	params.(*jsonutils.JSONDict).Update(tagParams)
	return params, nil
}

type PolicyGetPropertyDomainTagValueTreeOptions struct {
	PolicyListOptions
	options.DomainTagValueTreeOptions
}

func (opts *PolicyGetPropertyDomainTagValueTreeOptions) Params() (jsonutils.JSONObject, error) {
	params, err := opts.PolicyListOptions.Params()
	if err != nil {
		return nil, errors.Wrap(err, "PolicyListOptions.Params")
	}
	tagParams, _ := opts.DomainTagValueTreeOptions.Params()
	params.(*jsonutils.JSONDict).Update(tagParams)
	return params, nil
}
