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
	"k8s.io/client-go/tools/record"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

type baseControl struct {
	kind     string
	recorder record.EventRecorder
}

func newBaseControl(kind string, recorder record.EventRecorder) *baseControl {
	return &baseControl{kind, recorder}
}

func (c *baseControl) Kind() string {
	return c.kind
}

func (c *baseControl) Recorder() record.EventRecorder {
	return c.recorder
}

func (c *baseControl) RecordCreateEvent(oc *v1alpha1.OnecloudCluster, obj Object, err error) {
	recordCreateEvent(c.Recorder(), oc, c.Kind(), obj, err)
}

func (c *baseControl) RecordUpdateEvent(oc *v1alpha1.OnecloudCluster, obj Object, err error) {
	recordUpdateEvent(c.Recorder(), oc, c.Kind(), obj, err)
}

func (c *baseControl) RecordDeleteEvent(oc *v1alpha1.OnecloudCluster, obj Object, err error) {
	recordDeleteEvent(c.Recorder(), oc, c.Kind(), obj, err)
}
