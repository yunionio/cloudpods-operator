// Copyright 2016 The etcd-operator Authors
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

package k8sutil

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	corelisters "k8s.io/client-go/listers/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
)

// IsNodeReady checks if the Node condition is ready.
func IsNodeReady(n v1.Node) bool {
	for _, cd := range n.Status.Conditions {
		if cd.Type == v1.NodeReady {
			return cd.Status == v1.ConditionTrue
		}
	}

	return false
}

func GetReadyMasterNodes(lister corelisters.NodeLister) ([]*v1.Node, error) {
	// list nodes by master node selector
	masterNodeSelector := labels.NewSelector()
	r, err := labels.NewRequirement(
		constants.LabelNodeRoleMaster, selection.Exists, nil)
	if err != nil {
		return nil, err
	}
	masterNodeSelector = masterNodeSelector.Add(*r)
	nodes, err := lister.List(masterNodeSelector)
	if err != nil {
		return nil, err
	}

	ret := []*v1.Node{}
	for _, n := range nodes {
		if IsNodeReady(*n) {
			ret = append(ret, n)
		}
	}

	return ret, nil
}
