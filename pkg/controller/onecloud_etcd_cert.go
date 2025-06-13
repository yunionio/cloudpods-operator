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
	caCert := NewEtcdClusterCACert()
	config, err := caCert.GetConfig(oc)
	if err != nil {
		return err
	}
	cert, key, err := NewCACertAndKey(config)
	if err != nil {
		return err
	}

	// server
	store := newCertsStore()
	if err := store.WriteCert(constants.EtcdServerCACertName, cert); err != nil {
		return err
	}
	svcCerts := NewEtcdServerCert(caCert.BaseName, constants.EtcdServerName, constants.EtcdServerCertName)
	svcCert, svcKey, err := svcCerts.CreateFromCA(oc, cert, key)
	if err != nil {
		return err
	}
	if err := store.WriteCertAndKey(svcCerts.BaseName, svcCert, svcKey); err != nil {
		return err
	}
	certSecret := newEtcdSecretFromStore(oc, constants.EtcdServerSecret, store)
	_, err = c.kubeCli.CoreV1().Secrets(oc.GetNamespace()).Create(context.Background(), certSecret, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// client
	store = newCertsStore()
	if err := store.WriteCert(constants.EtcdClientCACertName, cert); err != nil {
		return err
	}
	cliCerts := NewEtcdClientCert(caCert.BaseName, constants.EtcdClientName, constants.EtcdClientCertName)
	cliCert, cliKey, err := cliCerts.CreateFromCA(oc, cert, key)
	if err != nil {
		return err
	}
	if err := store.WriteCertAndKey(cliCerts.BaseName, cliCert, cliKey); err != nil {
		return err
	}
	certSecret = newEtcdSecretFromStore(oc, constants.EtcdClientSecret, store)
	_, err = c.kubeCli.CoreV1().Secrets(oc.GetNamespace()).Create(context.Background(), certSecret, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// peer
	store = newCertsStore()
	if err := store.WriteCert(constants.EtcdPeerCACertName, cert); err != nil {
		return err
	}
	peerCerts := NewEtcdPeerCert(caCert.BaseName, constants.EtcdPeerName, constants.EtcdPeerCertName)
	peerCert, peerKey, err := peerCerts.CreateFromCA(oc, cert, key)
	if err != nil {
		return err
	}
	if err := store.WriteCertAndKey(peerCerts.BaseName, peerCert, peerKey); err != nil {
		return err
	}
	certSecret = newEtcdSecretFromStore(oc, constants.EtcdPeerSecret, store)
	_, err = c.kubeCli.CoreV1().Secrets(oc.GetNamespace()).Create(context.Background(), certSecret, metav1.CreateOptions{})
	if err != nil {
		return err
	}

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
