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

package certs

import (
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	corelisters "k8s.io/client-go/listers/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
)

type CertsManager struct {
	certControl  controller.OnecloudCertControlInterface
	secretLister corelisters.SecretLister
}

func NewCertsManager(
	certControl controller.OnecloudCertControlInterface,
	secretLister corelisters.SecretLister,
) *CertsManager {
	return &CertsManager{
		certControl:  certControl,
		secretLister: secretLister,
	}
}

func (c *CertsManager) CreateOrUpdate(oc *v1alpha1.OnecloudCluster) error {
	ns := oc.GetNamespace()
	certSecretName := controller.ClustercertSecretName(oc)
	_, err := c.secretLister.Secrets(ns).Get(certSecretName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := c.certControl.CreateCert(oc); err != nil {
			return errors.Wrap(err, "create cluster cert")
		}
		return nil
	} else {
		// already exists, update it
		// TODO
		//return nil
	}

	if !oc.Spec.Etcd.Disable && oc.Spec.Etcd.EnableTls {
		for _, secretName := range []string{constants.EtcdServerSecret, constants.EtcdClientSecret, constants.EtcdPeerSecret} {
			_, err := c.secretLister.Secrets(ns).Get(secretName)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
				if err := c.certControl.CreateEtcdCert(oc); err != nil {
					return errors.Wrap(err, "create cluster cert")
				}
				return nil
			} else {
				// already exists, update it
				// TODO
				continue
			}
		}
	}
	return nil
}
