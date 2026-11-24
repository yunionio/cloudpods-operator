package controller

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	certutil "k8s.io/client-go/util/cert"

	"yunion.io/x/log"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/pkiutil"
)

func NewEtcdClusterCACert() *OnecloudCert {
	return &OnecloudCert{
		Name:     "ca",
		LongName: "self-signed onecloud cA to provision identities for etcd components",
		BaseName: constants.EtcdServerCACertName,
		CAName:   "ca",
		config: certutil.Config{
			CommonName: "onecloud",
		},
	}
}

func NewEtcdServerCert(caName string, serviceName string, certName string) *OnecloudCert {
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
				getServerAltNames,
				serviceName,
				certName,
			),
		},
	}
}

func NewEtcdClientCert(caName string, serviceName string, certName string) *OnecloudCert {
	return &OnecloudCert{
		Name:     serviceName,
		LongName: fmt.Sprintf("certificate for serving the %s service", serviceName),
		BaseName: serviceName,
		CAName:   caName,
		config: certutil.Config{
			CommonName: serviceName,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		configMutators: []configMutatorsFunc{
			makeAltNamesMutator(
				getClientAltNames,
				serviceName,
				certName,
			),
		},
	}
}

func NewEtcdPeerCert(caName string, serviceName string, certName string) *OnecloudCert {
	return &OnecloudCert{
		Name:     serviceName,
		LongName: fmt.Sprintf("certificate for serving the %s service", serviceName),
		BaseName: serviceName,
		CAName:   caName,
		config: certutil.Config{
			CommonName: serviceName,
			Usages: []x509.ExtKeyUsage{
				x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth,
			},
		},
		configMutators: []configMutatorsFunc{
			makeAltNamesMutator(
				getPeerAltNames,
				serviceName,
				certName,
			),
		},
	}
}

func getServerAltNames(
	oc *v1alpha1.OnecloudCluster, serviceName string, certName string,
) (*certutil.AltNames, error) {
	ns := oc.GetNamespace()
	altNames := &certutil.AltNames{
		DNSNames: []string{
			fmt.Sprintf("*.%s-etcd.%s.svc", oc.Name, ns),
			fmt.Sprintf("%s-etcd-client.%s.svc", oc.Name, ns),
			constants.Localhost,
		},
		IPs: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
		},
	}

	if oc.Spec.LoadBalancerEndpoint != "" {
		altNames.IPs = append(altNames.IPs, net.ParseIP(oc.Spec.LoadBalancerEndpoint))
	}

	return altNames, nil
}

func getPeerAltNames(oc *v1alpha1.OnecloudCluster, serviceName string, certName string) (*certutil.AltNames, error) {
	ns := oc.GetNamespace()
	altNames := &certutil.AltNames{
		DNSNames: []string{
			fmt.Sprintf("*.%s-etcd.%s.svc", oc.Name, ns),
			fmt.Sprintf("*.%s-etcd.%s.svc.cluster.local", oc.Name, ns),
			constants.Localhost,
		},
		IPs: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
		},
	}

	return altNames, nil
}

func getClientAltNames(oc *v1alpha1.OnecloudCluster, serviceName string, certName string) (*certutil.AltNames, error) {
	altNames := &certutil.AltNames{
		DNSNames: []string{
			"",
		},
	}

	return altNames, nil
}

func (c *realOnecloudCertControl) CreateEtcdCert(oc *v1alpha1.OnecloudCluster) error {
	for _, secretName := range []string{constants.EtcdServerSecret, constants.EtcdClientSecret, constants.EtcdPeerSecret} {
		err := c.kubeCli.CoreV1().Secrets(oc.GetNamespace()).Delete(context.Background(), secretName, metav1.DeleteOptions{})
		if err != nil {
			log.Errorf("Delete secret %s failed: %s", secretName, err)
		}
	}
	return c.applyEtcdCerts(oc, true)
}

// UpdateEtcdCert renews etcd TLS certificates when any leaf cert is expired or within renew window.
func (c *realOnecloudCertControl) UpdateEtcdCert(oc *v1alpha1.OnecloudCluster) error {
	ns := oc.GetNamespace()
	needRenew := false
	for _, item := range []struct {
		secretName string
		certKey    string
	}{
		{constants.EtcdServerSecret, nameForCert(constants.EtcdServerName)},
		{constants.EtcdClientSecret, nameForCert(constants.EtcdClientName)},
		{constants.EtcdPeerSecret, nameForCert(constants.EtcdPeerName)},
	} {
		secret, err := c.kubeCli.CoreV1().Secrets(ns).Get(context.Background(), item.secretName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		certPEM, ok := secret.Data[item.certKey]
		if !ok || len(certPEM) == 0 {
			needRenew = true
			break
		}
		cert, err := pkiutil.ParseCertPEM(certPEM)
		if err != nil || pkiutil.NeedsRenewal(cert) {
			needRenew = true
			break
		}
	}
	if !needRenew {
		return nil
	}
	log.Infof("renewing etcd TLS certificates for cluster %s/%s", ns, oc.GetName())
	return c.applyEtcdCerts(oc, false)
}

func (c *realOnecloudCertControl) applyEtcdCerts(oc *v1alpha1.OnecloudCluster, createOnly bool) error {
	caCert := NewEtcdClusterCACert()
	config, err := caCert.GetConfig(oc)
	if err != nil {
		return err
	}
	cert, key, err := NewCACertAndKey(config)
	if err != nil {
		return err
	}

	type etcdCertSpec struct {
		secretName string
		caName     string
		build      func(caName string) *OnecloudCert
	}
	specs := []etcdCertSpec{
		{
			secretName: constants.EtcdServerSecret,
			caName:     constants.EtcdServerCACertName,
			build: func(caName string) *OnecloudCert {
				return NewEtcdServerCert(caName, constants.EtcdServerName, constants.EtcdServerCertName)
			},
		},
		{
			secretName: constants.EtcdClientSecret,
			caName:     constants.EtcdClientCACertName,
			build: func(caName string) *OnecloudCert {
				return NewEtcdClientCert(caName, constants.EtcdClientName, constants.EtcdClientCertName)
			},
		},
		{
			secretName: constants.EtcdPeerSecret,
			caName:     constants.EtcdPeerCACertName,
			build: func(caName string) *OnecloudCert {
				return NewEtcdPeerCert(caName, constants.EtcdPeerName, constants.EtcdPeerCertName)
			},
		},
	}

	for _, spec := range specs {
		store := newCertsStore()
		if err := store.WriteCert(spec.caName, cert); err != nil {
			return err
		}
		leaf := spec.build(caCert.BaseName)
		leafCert, leafKey, err := leaf.CreateFromCA(oc, cert, key)
		if err != nil {
			return err
		}
		if err := store.WriteCertAndKey(leaf.BaseName, leafCert, leafKey); err != nil {
			return err
		}
		if err := c.writeEtcdSecret(oc, spec.secretName, store, createOnly); err != nil {
			return err
		}
	}
	return nil
}

func (c *realOnecloudCertControl) writeEtcdSecret(oc *v1alpha1.OnecloudCluster, secretName string, store CertsStore, createOnly bool) error {
	ns := oc.GetNamespace()
	if createOnly {
		certSecret := newEtcdSecretFromStore(oc, secretName, store)
		_, err := c.kubeCli.CoreV1().Secrets(ns).Create(context.Background(), certSecret, metav1.CreateOptions{})
		return err
	}
	secret, err := c.kubeCli.CoreV1().Secrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		certSecret := newEtcdSecretFromStore(oc, secretName, store)
		_, err = c.kubeCli.CoreV1().Secrets(ns).Create(context.Background(), certSecret, metav1.CreateOptions{})
		return err
	}
	secret.Data = store
	_, err = c.kubeCli.CoreV1().Secrets(ns).Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	c.RecordUpdateEvent(oc, secret, nil)
	return nil
}

func newEtcdSecretFromStore(oc *v1alpha1.OnecloudCluster, name string, store CertsStore) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       oc.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{GetOwnerRef(oc)},
		},
		Data: store,
	}
}
