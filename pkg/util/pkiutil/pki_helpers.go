package pkiutil

import (
	"crypto"
	"crypto/rand"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math"
	"math/big"
	"net"
	"time"
	"yunion.io/x/onecloud-operator/pkg/apis/constants"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

const (
	oneday       = time.Hour * 24
	duration365d = oneday * 365
)

var (
	NewPrivateKey       = pkiutil.NewPrivateKey
	EncodeCertPEM       = pkiutil.EncodeCertPEM
	TryLoadCertFromDisk = pkiutil.TryLoadCertFromDisk
	TryLoadKeyFromDisk  = pkiutil.TryLoadKeyFromDisk
)

// NewCertificateAuthority creates new certificate and private key for the certificate authority
func NewCertificateAuthority(config *certutil.Config) (*x509.Certificate, crypto.Signer, error) {
	key, err := NewPrivateKey()
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to create private key")
	}

	cert, err := certutil.NewSelfSignedCACert(*config, key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to create self-signed certificate")
	}

	return cert, key, nil
}

// NewCertAndKey creates new certificate and key by passing the certificate authority certificate and key
func NewCertAndKey(caCert *x509.Certificate, caKey crypto.Signer, config *certutil.Config) (*x509.Certificate, crypto.Signer, error) {
	key, err := NewPrivateKey()
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to create private key")
	}

	cert, err := NewSignedCert(config, key, caCert, caKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to sign certificate")
	}

	return cert, key, nil
}

// NewSignedCert creates a signed certificate using the given CA certificate and key
func NewSignedCert(cfg *certutil.Config, key crypto.Signer, caCert *x509.Certificate, caKey crypto.Signer) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if len(cfg.CommonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}
	if len(cfg.Usages) == 0 {
		return nil, errors.New("must specify at least one ExtKeyUsage")
	}

	// see: https://support.apple.com/en-us/HT210176
	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:     cfg.AltNames.DNSNames,
		IPAddresses:  cfg.AltNames.IPs,
		SerialNumber: serial,
		NotBefore:    caCert.NotBefore,
		NotAfter:     time.Now().Add(oneday * 800).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  cfg.Usages,
	}
	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

// GetServiceAltNames builds an AltNames object to be used when generating service certificate
func GetServiceAltNames(oc *v1alpha1.OnecloudCluster, serviceName string, certName string) (*certutil.AltNames, error) {
	ns := oc.GetNamespace()
	// create AltNames with defaults DNSNames/IPs
	altNames := &certutil.AltNames{
		DNSNames: []string{
			serviceName,
			fmt.Sprintf("%s.%s", serviceName, ns),
			fmt.Sprintf("%s.%s.svc", serviceName, ns),
			constants.Localhost,
		},
	}

	// add cluster controlPlaneEndpoint if present (dns or ip)
	if len(oc.Spec.LoadBalancerEndpoint) > 0 {
		if ip := net.ParseIP(oc.Spec.LoadBalancerEndpoint); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		} else {
			altNames.DNSNames = append(altNames.DNSNames, oc.Spec.LoadBalancerEndpoint)
		}
	}

	appendSANsToAltNames(altNames, oc.Spec.CertSANs, certName)

	return altNames, nil
}

// appendSANsToAltNames parses SANs from as list of strings and adds them to altNames for use on a specific cert
// altNames is passed in with a pointer, and the struct is modified
// valid IP address strings are parsed and added to altNames.IPs as net.IP's
// RFC-1123 compliant DNS strings are added to altNames.DNSNames as strings
// certNames is used to print user facing warnings and should be the name of the cert the altNames will be used for
func appendSANsToAltNames(altNames *certutil.AltNames, SANs []string, certName string) {
	for _, altname := range SANs {
		if ip := net.ParseIP(altname); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		} else if len(validation.IsDNS1123Subdomain(altname)) == 0 {
			altNames.DNSNames = append(altNames.DNSNames, altname)
		} else {
			fmt.Printf(
				"[certificates] WARNING: '%s' was not added to the '%s' SAN, because it is not a valid IP or RFC-1123 compliant DNS entry\n",
				altname,
				certName,
			)
		}
	}
}
