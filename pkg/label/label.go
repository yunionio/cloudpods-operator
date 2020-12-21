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

package label

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
)

const (
	// The following labels are recommended by kubernetes https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/

	// ManagedByLabelKey is Kubernetes recommended label key, it represents the tool being used to manage the operation of an application
	// For resources managed by OneCloud Operator, its value is always onecloud-operator
	ManagedByLabelKey string = constants.ManagedByLabelKey
	// ComponentLabelKey is Kubernetes recommended label key, it represents the component within the architecture
	ComponentLabelKey string = constants.ComponentLabelKey
	// NameLabelKey is Kubernetes recommended label key, it represents the name of the application
	// It should always be onecloud-cluster in our case.
	NameLabelKey string = constants.NameLabelKey
	// InstanceLabelKey is Kubernetes recommended label key, it represents a unique name identifying the instance of an application
	// It's set by helm when installing a release
	InstanceLabelKey string = constants.InstanceLabelKey
	AppLabelKey      string = constants.AppLabelKey
	ZoneLabelKey     string = constants.ZoneLabelKey
)

// Label is the label field in metadata
type Label map[string]string

// New initialize a new Label
func New() Label {
	return Label{
		NameLabelKey:      "onecloud-cluster",
		ManagedByLabelKey: "onecloud-operator",
	}
}

// Instance adds instance kv pair to label
func (l Label) Instance(name string) Label {
	l[InstanceLabelKey] = name
	return l
}

// Zone adds zone kvm pair to lable
func (l Label) Zone(zone string) Label {
	l[ZoneLabelKey] = zone
	return l
}

// Component adds component kv pair to label
func (l Label) Component(name string) Label {
	l[ComponentLabelKey] = name
	l[AppLabelKey] = name
	return l
}

// ComponentType returns component type
func (l Label) ComponentType() string {
	return l[ComponentLabelKey]
}

// Selector gets labels.Selector from label
func (l Label) Selector() (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(l.LabelSelector())
}

// LabelSelector gets LabelSelector from label
func (l Label) LabelSelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: l}
}

// Labels converts label to map[string]string
func (l Label) Labels() map[string]string {
	return l
}

// String converts label to a string
func (l Label) String() string {
	var arr []string

	for k, v := range l {
		arr = append(arr, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(arr, ",")
}
