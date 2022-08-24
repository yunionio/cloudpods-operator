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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"yunion.io/x/log"
	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/k8sutil"
)

var (
	// controllerKind contains the schema.GroupVersionKind for this controller type
	controllerKind = v1alpha1.SchemeGroupVersion.WithKind(constants.OnecloudClusterKind)
)

// RequeueError is used to requeue the item, this error type should't be considered as a real error
type RequeueError struct {
	s string
}

func (re *RequeueError) Error() string {
	return re.s
}

// RequeueErrorf returns a RequeueError
func RequeueErrorf(format string, a ...interface{}) error {
	return &RequeueError{fmt.Sprintf(format, a...)}
}

// IsRequeueError returns whether err is a RequeueError
func IsRequeueError(err error) bool {
	_, ok := err.(*RequeueError)
	return ok
}

// GetOwnerRef returns OnecloudCluster's OwnerReference
func GetOwnerRef(oc *v1alpha1.OnecloudCluster) metav1.OwnerReference {
	controller := true
	blockOwnerDeletion := true
	return metav1.OwnerReference{
		APIVersion:         controllerKind.GroupVersion().String(),
		Kind:               controllerKind.Kind,
		Name:               oc.GetName(),
		UID:                oc.GetUID(),
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
}

// GetServiceType returns component's service type
func GetServiceType(services []v1alpha1.Service, serviceName string) corev1.ServiceType {
	for _, svc := range services {
		if svc.Name == serviceName {
			switch svc.Type {
			case "NodePort":
				return corev1.ServiceTypeNodePort
			case "LoadBalancer":
				return corev1.ServiceTypeLoadBalancer
			default:
				return corev1.ServiceTypeClusterIP
			}
		}
	}
	return corev1.ServiceTypeClusterIP
}

func NewClusterComponentName(clusterName string, componentName v1alpha1.ComponentType) string {
	return fmt.Sprintf("%s-%s", clusterName, componentName.String())
}

// KeystoneComponentName return keystone component name
func KeystoneComponentName(clusterName string) string {
	return NewClusterComponentName(clusterName, v1alpha1.KeystoneComponentType)
}

// AnnProm adds annotations for prometheus scraping metrics
func AnnProm(port int32) map[string]string {
	return map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/path":   "/metrics",
		"prometheus.io/port":   fmt.Sprintf("%d", port),
	}
}

// ComponentConfigMapName returns the default ConfigMap name of the specified component type
func ComponentConfigMapName(oc *v1alpha1.OnecloudCluster, component v1alpha1.ComponentType) string {
	nameKey := fmt.Sprintf("%s-%s", oc.Name, component)
	return nameKey + getConfigMapSuffix(oc, component.String(), nameKey)
}

// ClusterConfigMapName returns the default OnecloudClusterConfig ConfigMap name
func ClusterConfigMapName(oc *v1alpha1.OnecloudCluster) string {
	return fmt.Sprintf("%s-%s", oc.Name, "cluster-config")
}

// ClusterCertSecretName returns the default cluster certs name
func ClustercertSecretName(oc *v1alpha1.OnecloudCluster) string {
	return fmt.Sprintf("%s-%s", oc.Name, "certs")
}

// getConfigMapSuffix return the ConfigMap name suffix
func getConfigMapSuffix(oc *v1alpha1.OnecloudCluster, component string, name string) string {
	if oc.Annotations == nil {
		return ""
	}
	sha := oc.Annotations[fmt.Sprintf("onecloud.yunion.io/%s.%s.sha", component, name)]
	if len(sha) == 0 {
		return ""
	}
	return "-" + sha
}

type Object interface {
	GetName() string
}

func recordCreateEvent(recorder record.EventRecorder, oc *v1alpha1.OnecloudCluster, kind string, object Object, err error) {
	recordResourceEvent(recorder, "create", oc, kind, object, err)
}

func recordUpdateEvent(recorder record.EventRecorder, oc *v1alpha1.OnecloudCluster, kind string, object Object, err error) {
	recordResourceEvent(recorder, "update", oc, kind, object, err)
}

func recordDeleteEvent(recorder record.EventRecorder, oc *v1alpha1.OnecloudCluster, kind string, object Object, err error) {
	recordResourceEvent(recorder, "delete", oc, kind, object, err)
}

func recordResourceEvent(recorder record.EventRecorder, verb string, oc *v1alpha1.OnecloudCluster, kind string, object Object, err error) {
	ocName := oc.Name
	resourceName := object.GetName()
	if err == nil {
		reason := fmt.Sprintf("Successful%s", strings.Title(verb))
		message := fmt.Sprintf("%s %s %s in OnecloudCluster %s successful",
			strings.ToLower(verb), kind, resourceName, ocName)
		if !k8sutil.GetClusterVersion().IsGreatOrEqualThanV120() {
			recorder.Event(oc, corev1.EventTypeNormal, reason, message)
		}
		log.Infof(message)
	} else {
		reason := fmt.Sprintf("Failed%s", strings.Title(verb))
		message := fmt.Sprintf("%s %s %s in OnecloudCluster %s failed error: %s", strings.ToLower(verb), kind, resourceName, ocName, err)
		if !k8sutil.GetClusterVersion().IsGreatOrEqualThanV120() {
			recorder.Event(oc, corev1.EventTypeWarning, reason, message)
		}
		log.Errorf(message)
	}
}
