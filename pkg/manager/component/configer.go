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

package component

import (
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

// Configer implements the logic to get cluster config.
type Configer interface {
	GetClusterConfig(cluster *v1alpha1.OnecloudCluster) (*v1alpha1.OnecloudClusterConfig, error)
	CreateOrUpdateConfigMap(cluster *v1alpha1.OnecloudCluster, newCfgMap *corev1.ConfigMap) error
}
