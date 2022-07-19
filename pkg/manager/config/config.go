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

package config

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/scheme"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	k8sutil "yunion.io/x/onecloud-operator/pkg/util/k8s"
)

type ConfigManager struct {
	cfgControl controller.ConfigMapControlInterface
	cfgLister  corelisters.ConfigMapLister
}

func NewConfigManager(
	cfgControl controller.ConfigMapControlInterface,
	cfgLister corelisters.ConfigMapLister,
) *ConfigManager {
	return &ConfigManager{
		cfgControl: cfgControl,
		cfgLister:  cfgLister,
	}
}

func (c *ConfigManager) Lister() corelisters.ConfigMapLister {
	return c.cfgLister
}

func (c *ConfigManager) CreateOrUpdate(oc *v1alpha1.OnecloudCluster) (*v1alpha1.OnecloudClusterConfig, error) {
	ns := oc.GetNamespace()
	cfgMapName := controller.ClusterConfigMapName(oc)
	cfgObj, err := c.cfgLister.ConfigMaps(ns).Get(cfgMapName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		cfg := newClusterConfig()
		// not found cluster config, create new one
		newCfgMap, err := clusterConfigToConfigMap(oc, cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "new cluster configmap")
		}
		if err := c.cfgControl.CreateConfigMap(oc, newCfgMap); err != nil {
			return nil, errors.Wrapf(err, "create cluster config to configmap")
		}
		return cfg, nil
	} else {
		// already exists, update it
		cfg, err := newClusterConfigFromConfigMap(cfgObj)
		if err != nil {
			return nil, errors.Wrapf(err, "new cluster config from configmap %s", cfgObj.GetName())
		}
		newCfgMap, err := clusterConfigToConfigMap(oc, cfg)
		if err != nil {
			return nil, errors.Wrap(err, "renew configmap")
		}
		if _, err := c.cfgControl.UpdateConfigMap(oc, newCfgMap); err != nil {
			return nil, errors.Wrapf(err, "update cluster configmap %s", newCfgMap.GetName())
		}
		return cfg, nil
	}
}

func newClusterConfig() *v1alpha1.OnecloudClusterConfig {
	config := &v1alpha1.OnecloudClusterConfig{}
	return fillClusterConfigDefault(config)
}

func GetClusterConfigByClient(k8sCli kubernetes.Interface, oc *v1alpha1.OnecloudCluster) (*v1alpha1.OnecloudClusterConfig, error) {
	cfgMapName := controller.ClusterConfigMapName(oc)
	ns := oc.GetNamespace()
	obj, err := k8sCli.CoreV1().ConfigMaps(ns).Get(cfgMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return newClusterConfigFromConfigMapNoDefault(obj)
}

func newClusterConfigFromConfigMapNoDefault(cfgMap *corev1.ConfigMap) (*v1alpha1.OnecloudClusterConfig, error) {
	data, ok := cfgMap.Data[constants.OnecloudClusterConfigConfigMapKey]
	if !ok {
		return nil, errors.Errorf("unexpected error when reading %s ConfigMap: %s key value pair missing", cfgMap.Name, constants.OnecloudClusterConfigConfigMapKey)
	}
	cfg := &v1alpha1.OnecloudClusterConfig{}
	if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), []byte(data), cfg); err != nil {
		return nil, errors.Wrapf(err, "failed to decode onecloud cluster config data")
	}
	return cfg, nil
}

func newClusterConfigFromConfigMap(cfgMap *corev1.ConfigMap) (*v1alpha1.OnecloudClusterConfig, error) {
	cfg, err := newClusterConfigFromConfigMapNoDefault(cfgMap)
	if err != nil {
		return nil, err
	}
	return fillClusterConfigDefault(cfg), nil
}

func clusterConfigToConfigMap(oc *v1alpha1.OnecloudCluster, config *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	ns := oc.GetNamespace()
	data, err := k8sutil.MarshalToYamlForCodecs(config, v1alpha1.SchemeGroupVersion, scheme.Codecs)
	if err != nil {
		return nil, err
	}
	cfgMapName := controller.ClusterConfigMapName(oc)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            cfgMapName,
			Namespace:       ns,
			OwnerReferences: []metav1.OwnerReference{controller.GetOwnerRef(oc)},
		},
		Data: map[string]string{
			constants.OnecloudClusterConfigConfigMapKey: string(data),
		},
	}, nil
}

func fillClusterConfigDefault(config *v1alpha1.OnecloudClusterConfig) *v1alpha1.OnecloudClusterConfig {
	scheme.Scheme.Default(config)
	return config
}

func (c *ConfigManager) CreateOrUpdateConfigMap(oc *v1alpha1.OnecloudCluster, newCfgMap *corev1.ConfigMap) error {
	if err := c.cfgControl.CreateConfigMap(oc, newCfgMap); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "unable to create configmap %s", newCfgMap.GetName())
		}

		if _, err := c.cfgControl.UpdateConfigMap(oc, newCfgMap); err != nil {
			return errors.Wrapf(err, "unable to update configmap %s", newCfgMap.GetName())
		}
	}
	return nil
}

func (c *ConfigManager) GetClusterConfig(oc *v1alpha1.OnecloudCluster) (*v1alpha1.OnecloudClusterConfig, error) {
	ns := oc.GetNamespace()
	cfgMapName := controller.ClusterConfigMapName(oc)
	cfgObj, err := c.cfgLister.ConfigMaps(ns).Get(cfgMapName)
	if err != nil {
		return nil, err
	}
	return newClusterConfigFromConfigMapNoDefault(cfgObj)
}
