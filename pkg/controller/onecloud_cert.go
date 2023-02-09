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
	"context"
	"crypto"
	"crypto/x509"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/pkiutil"
)

type OnecloudCertControlInterface interface {
	// CreateCert
	CreateCert(oc *v1alpha1.OnecloudCluster) error
	// UpdateCert
	//UpdateCert(oc *v1alpha1.OnecloudCluster)

	// GetCertSecret return certs secret
	// GetCertSecret(oc *v1alpha1.OnecloudCluster) (*corev1.Secret, error)
	CreateEtcdCert(oc *v1alpha1.OnecloudCluster) error
}

type realOnecloudCertControl struct {
	*baseControl
	kubeCli      kubernetes.Interface
	secretLister corelisters.SecretLister
}

func NewOnecloudCertControl(kubeCli kubernetes.Interface, secretLister corelisters.SecretLister, recorder record.EventRecorder) OnecloudCertControlInterface {
	return &realOnecloudCertControl{
		baseControl:  newBaseControl("Secret", recorder),
		kubeCli:      kubeCli,
		secretLister: secretLister,
	}
}

func (c *realOnecloudCertControl) CreateCert(oc *v1alpha1.OnecloudCluster) error {
	caCert := NewClusterCACert()
	config, err := caCert.GetConfig(oc)
	if err != nil {
		return err
	}
	cert, key, err := NewCACertAndKey(config)
	if err != nil {
		return err
	}
	store := newCertsStore()
	if err := store.WriteCertAndKey(caCert.BaseName, cert, key); err != nil {
		return err
	}

	svcCerts := NewServiceCert(caCert.BaseName, constants.ServiceCertAndKeyBaseName, constants.ServiceCertName)
	svcCert, svcKey, err := svcCerts.CreateFromCA(oc, cert, key)
	if err != nil {
		return err
	}
	if err := store.WriteCertAndKey(svcCerts.BaseName, svcCert, svcKey); err != nil {
		return err
	}
	// for web ingress
	if err := store.WriteCertAndKey("tls", svcCert, svcKey); err != nil {
		return err
	}
	certSecret := newSecretFromStore(oc, store)
	_, err = c.kubeCli.CoreV1().Secrets(oc.GetNamespace()).Create(context.Background(), certSecret, metav1.CreateOptions{})
	return err
}

type certsStore map[string][]byte

func newCertsStore() certsStore {
	return make(map[string][]byte)
}

func (s certsStore) WriteCertAndKey(name string, cert *x509.Certificate, key crypto.Signer) error {
	if err := s.WriteKey(name, key); err != nil {
		return errors.Wrapf(err, "cloudn't write %s key", name)
	}
	return s.WriteCert(name, cert)
}

func nameForKey(name string) string {
	return fmt.Sprintf("%s.key", name)
}

func nameForCert(name string) string {
	return fmt.Sprintf("%s.crt", name)
}

func (s certsStore) WriteKey(name string, key crypto.Signer) error {
	if key == nil {
		return errors.New("private key cannot be nil when write")
	}
	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return errors.Wrapf(err, "unable to marshal private key to PEM")
	}
	s[nameForKey(name)] = encoded
	return nil
}

func (s certsStore) WriteCert(name string, cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("certificate cannot be nil when write")
	}

	encoded := pkiutil.EncodeCertPEM(cert)
	s[nameForCert(name)] = encoded
	return nil
}

func newSecretFromStore(oc *v1alpha1.OnecloudCluster, store certsStore) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            ClustercertSecretName(oc),
			Namespace:       oc.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{GetOwnerRef(oc)},
		},
		Data: store,
	}
}

type configMutatorsFunc func(configuration *v1alpha1.OnecloudCluster, config *certutil.Config) error

type OnecloudCert struct {
	Name     string
	LongName string
	BaseName string
	CAName   string
	// Some attributes will depend on the InitConfiguration, only known at runtime.
	// These functions will be run in series, passed both the InitConfiguration and a cert Config.
	configMutators []configMutatorsFunc
	config         certutil.Config
}

func (k *OnecloudCert) GetConfig(oc *v1alpha1.OnecloudCluster) (*certutil.Config, error) {
	for _, f := range k.configMutators {
		if err := f(oc, &k.config); err != nil {
			return nil, err
		}
	}
	return &k.config, nil
}

// CreateFromCA makes and writes a certificate using the given CA cert and key.
func (k *OnecloudCert) CreateFromCA(oc *v1alpha1.OnecloudCluster, caCert *x509.Certificate, caKey crypto.Signer) (*x509.Certificate, crypto.Signer, error) {
	cfg, err := k.GetConfig(oc)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "couldn't create %q certificate", k.Name)
	}
	return pkiutil.NewCertAndKey(caCert, caKey, cfg)
}

func (k *OnecloudCert) CreateAsCA(oc *v1alpha1.OnecloudCluster) (*x509.Certificate, crypto.Signer, error) {
	cfg, err := k.GetConfig(oc)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "couldn't get configuration for %q CA certificate", k.Name)
	}
	return NewCACertAndKey(cfg)
}

// NewCACertAndKey will generate a self signed CA.
func NewCACertAndKey(certSpec *certutil.Config) (*x509.Certificate, crypto.Signer, error) {
	caCert, caKey, err := pkiutil.NewCertificateAuthority(certSpec)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failure while generating CA certificate and key")
	}

	return caCert, caKey, nil
}

func NewClusterCACert() *OnecloudCert {
	return &OnecloudCert{
		Name:     "ca",
		LongName: "self-signed onecloud cA to provision identities for other service components",
		BaseName: constants.CACertAndKeyBaseName,
		CAName:   "ca",
		config: certutil.Config{
			CommonName: "onecloud",
		},
	}
}

func NewServiceCert(caName string, serviceName string, certName string) *OnecloudCert {
	return &OnecloudCert{
		Name:     serviceName,
		LongName: fmt.Sprintf("certificate for serving the %s service", serviceName),
		BaseName: serviceName,
		CAName:   caName,
		config: certutil.Config{
			CommonName: serviceName,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		configMutators: []configMutatorsFunc{
			makeAltNamesMutator(
				pkiutil.GetServiceAltNames,
				serviceName,
				certName,
			),
		},
	}
}

func makeAltNamesMutator(
	f func(*v1alpha1.OnecloudCluster, string, string) (*certutil.AltNames, error),
	serviceName, certName string,
) configMutatorsFunc {
	return func(mc *v1alpha1.OnecloudCluster, cc *certutil.Config) error {
		altNames, err := f(mc, serviceName, certName)
		if err != nil {
			return err
		}
		cc.AltNames = *altNames
		return nil
	}
}

func newClusterCACert(oc *v1alpha1.OnecloudCluster) (*x509.Certificate, crypto.Signer, error) {
	config, err := NewClusterCACert().GetConfig(oc)
	if err != nil {
		return nil, nil, err
	}
	return NewCACertAndKey(config)
}
