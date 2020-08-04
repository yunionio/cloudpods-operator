package k8sutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"yunion.io/x/onecloud-operator/pkg/util/etcdutil"
)

type TLSData struct {
	CertData []byte
	KeyData  []byte
	CAData   []byte
}

// GetTLSDataFromSecret retrives the kubernete secret that contain etcd tls certs and put them into TLSData.
func GetTLSDataFromSecret(kubecli kubernetes.Interface, ns, se string) (*TLSData, error) {
	secret, err := kubecli.CoreV1().Secrets(ns).Get(se, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return &TLSData{
		CertData: secret.Data[etcdutil.CliCertFile],
		KeyData:  secret.Data[etcdutil.CliKeyFile],
		CAData:   secret.Data[etcdutil.CliCAFile],
	}, nil
}
