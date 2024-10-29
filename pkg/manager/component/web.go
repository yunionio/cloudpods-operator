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
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
)

type webManager struct {
	*ComponentManager
}

func newWebManager(man *ComponentManager) manager.Manager {
	m := &webManager{man}
	return m
}

func (m *webManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.WebComponentType
}

func (m *webManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *webManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if len(oc.Spec.Web.ImageName) == 0 {
		if IsEEOrESEEdition(oc) {
			oc.Spec.Web.ImageName = constants.WebEEImageName
		} else {
			oc.Spec.Web.ImageName = constants.WebCEImageName
		}
	}
	return syncComponent(m, oc, oc.Spec.Web.Disable, "")
}

func (m *webManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	ports := []corev1.ServicePort{
		{
			Name:       "http",
			Protocol:   corev1.ProtocolTCP,
			Port:       80,
			TargetPort: intstr.FromInt(80),
		},
		{
			Name:       "https",
			Protocol:   corev1.ProtocolTCP,
			Port:       443,
			TargetPort: intstr.FromInt(443),
		},
		{
			Name:       "overview",
			Protocol:   corev1.ProtocolTCP,
			Port:       8080,
			TargetPort: intstr.FromInt(8080),
		},
		{
			Name:       "docs",
			Protocol:   corev1.ProtocolTCP,
			Port:       8081,
			TargetPort: intstr.FromInt(8081),
		},
	}
	return []*corev1.Service{m.newService(v1alpha1.WebComponentType, oc, corev1.ServiceTypeClusterIP, ports)}
}

func (m *webManager) getIngress(oc *v1alpha1.OnecloudCluster, zone string) *unstructured.Unstructured {
	ocName := oc.GetName()
	svcName := controller.NewClusterComponentName(ocName, v1alpha1.WebComponentType)
	appLabel := m.getComponentLabel(oc, v1alpha1.WebComponentType)
	secretName := controller.ClustercertSecretName(oc)

	obj := new(unstructured.Unstructured)
	objMeta := m.getObjectMeta(oc, svcName, appLabel)
	obj.SetAnnotations(objMeta.Annotations)
	obj.SetName(objMeta.Name)
	obj.SetNamespace(objMeta.Namespace)
	obj.SetLabels(objMeta.Labels)
	obj.SetOwnerReferences(objMeta.OwnerReferences)

	// unstructured.SetNestedSlice(obj.Object, []interface{}{
	// 	map[string]interface{}{
	// 		"secretName": secretName,
	// 	},
	// }, "spec", "tls")
	// unstructured.SetNestedSlice(obj.Object, []interface{}{
	// 	map[string]interface{}{
	// 		"http": map[string]interface{}{
	// 			"paths": []interface{}{
	// 				map[string]interface{}{
	// 					"path": "/",
	// 					"backend": map[string]interface{}{
	// 						"serviceName": svcName,
	// 						"servicePort": int64(443),
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }, "spec", "rules")

	useHTTP := oc.Spec.Web.UseHTTP
	port := 443
	if useHTTP {
		port = 80
	}

	unstructured.SetNestedMap(obj.Object, map[string]interface{}{
		"tls": []interface{}{
			map[string]interface{}{
				"secretName": secretName,
			},
		},
		"rules": []interface{}{
			map[string]interface{}{
				"http": map[string]interface{}{
					"paths": []interface{}{
						map[string]interface{}{
							"pathType": "Prefix",
							"path":     "/",
							"backend": map[string]interface{}{
								"serviceName": svcName,
								"servicePort": int64(port),
								"service": map[string]interface{}{
									"name": svcName,
									"port": map[string]interface{}{
										"number": int64(port),
									},
								},
							},
						},
					},
				},
			},
		},
	}, "spec")

	// ing := &extensions.Ingress{
	// 	ObjectMeta: m.getObjectMeta(oc, svcName, appLabel),
	// 	Spec: extensions.IngressSpec{
	// 		TLS: []extensions.IngressTLS{
	// 			{
	// 				SecretName: secretName,
	// 			},
	// 		},
	// 		Rules: []extensions.IngressRule{
	// 			{
	// 				IngressRuleValue: extensions.IngressRuleValue{
	// 					HTTP: &extensions.HTTPIngressRuleValue{
	// 						Paths: []extensions.HTTPIngressPath{
	// 							{
	// 								Path: "/",
	// 								Backend: extensions.IngressBackend{
	// 									ServiceName: svcName,
	// 									ServicePort: intstr.FromInt(443),
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	// for nginx ingress
	anno := obj.GetAnnotations()
	if len(anno) == 0 {
		anno = map[string]string{}
	}
	if useHTTP {
		anno["nginx.ingress.kubernetes.io/backend-protocol"] = "HTTP"
	} else {
		anno["nginx.ingress.kubernetes.io/backend-protocol"] = "HTTPS"
	}
	obj.SetAnnotations(anno)

	return m.addIngressPaths(IsEnterpriseEdition(oc), svcName, obj)
}

func (m *webManager) addIngressPaths(isEE bool, svcName string, ing *unstructured.Unstructured) *unstructured.Unstructured {
	if !isEE {
		return ing
	}
	/*
	 * rule := &ing.Spec.Rules[0]
	 * if !IsPathIngressRule("/overview", rule.HTTP.Paths) {
	 * 	rule.HTTP.Paths = append(rule.HTTP.Paths,
	 * 		extensions.HTTPIngressPath{
	 * 			Path: "/overview",
	 * 			Backend: extensions.IngressBackend{
	 * 				ServiceName: svcName,
	 * 				ServicePort: intstr.FromInt(8080),
	 * 			},
	 * 		},
	 * 	)
	 * }
	 * if !IsPathIngressRule("/docs", rule.HTTP.Paths) {
	 * 	rule.HTTP.Paths = append(rule.HTTP.Paths,
	 * 		extensions.HTTPIngressPath{
	 * 			Path: "/docs",
	 * 			Backend: extensions.IngressBackend{
	 * 				ServiceName: svcName,
	 * 				ServicePort: intstr.FromInt(8081),
	 * 			},
	 * 		},
	 * 	)
	 * }
	 */
	return ing
}

func IsPathIngressRule(path string, paths []interface{}) bool {
	exist := false
	for _, obj := range paths {
		ip := obj.(map[string]interface{})
		if ip["path"].(string) == path {
			exist = true
			break
		}
	}
	return exist
}

func (m *webManager) updateIngress(oc *v1alpha1.OnecloudCluster, oldIng *unstructured.Unstructured) *unstructured.Unstructured {
	newIng := oldIng.DeepCopy()
	spec, _, _ := unstructured.NestedMap(newIng.Object, "spec")
	doUpdate := false
	rules, _, _ := unstructured.NestedSlice(spec, "rules")
	for _, rule := range rules {
		http, _, _ := unstructured.NestedMap(rule.(map[string]interface{}), "http")
		if http == nil {
			continue
		}
		paths, _, _ := unstructured.NestedSlice(http, "paths")
		for _, path := range []string{"/overview", "/docs"} {
			if !IsPathIngressRule(path, paths) {
				doUpdate = true
				break
			}
		}
	}
	if doUpdate {
		svcName := m.getService(oc, nil, "")[0].GetName()
		newIng = m.addIngressPaths(IsEnterpriseEdition(oc), svcName, newIng)
	}
	return newIng
}

func (m *webManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	content, err := component.NewWeb().GetConfig(oc, cfg)
	if err != nil {
		return nil, false, err
	}
	return m.newConfigMap(v1alpha1.WebComponentType, "", oc, content.(string)), false, nil
}

func (m *webManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		confVol := volMounts[len(volMounts)-1]
		confVol.MountPath = "/etc/nginx/conf.d"
		volMounts[len(volMounts)-1] = confVol
		containers := []corev1.Container{
			{
				Name:            "web",
				Image:           oc.Spec.Web.Image,
				ImagePullPolicy: oc.Spec.Web.ImagePullPolicy,
				Command: []string{
					"nginx",
					"-g",
					"daemon off;",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "web",
						ContainerPort: 80,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: volMounts,
			},
		}
		if IsEnterpriseEdition(oc) {
			containers = append(containers,
				corev1.Container{
					Name:            "overview",
					Image:           oc.Spec.Web.Overview.Image,
					ImagePullPolicy: oc.Spec.Web.Overview.ImagePullPolicy,
					Ports: []corev1.ContainerPort{
						{
							Name:          "overview",
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					VolumeMounts: volMounts,
				},
				corev1.Container{
					Name:            "docs",
					Image:           oc.Spec.Web.Docs.Image,
					ImagePullPolicy: oc.Spec.Web.Docs.ImagePullPolicy,
					Ports: []corev1.ContainerPort{
						{
							Name:          "docs",
							ContainerPort: 8081,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					VolumeMounts: volMounts,
				},
			)
		}
		return containers
	}
	deploy, err := m.newDefaultDeploymentNoInit(v1alpha1.WebComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.WebComponentType), v1alpha1.WebComponentType), &oc.Spec.Web.DeploymentSpec, cf)
	if err != nil {
		return nil, err
	}
	podSpec := &deploy.Spec.Template.Spec
	config := podSpec.Volumes[len(podSpec.Volumes)-1]
	config.ConfigMap.Items[0].Path = "default.conf"
	podSpec.Volumes[len(podSpec.Volumes)-1] = config
	return deploy, nil
}

func (m *webManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Web
}
