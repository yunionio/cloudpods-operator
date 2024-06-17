package cluster

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/scheme"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

func NewDockerComposeCluster(
	dbConfig v1alpha1.Mysql,
	accessIp string,
	defaultRegion string,
	version string,
	productVersion v1alpha1.ProductVersion,
) *v1alpha1.OnecloudCluster {
	cluster := &v1alpha1.OnecloudCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.DockerComposeClusterName,
		},
		Spec: v1alpha1.OnecloudClusterSpec{
			Mysql:                dbConfig,
			LoadBalancerEndpoint: accessIp,
			Region:               defaultRegion,
			Version:              version,
			ProductVersion:       productVersion,
		},
	}
	scheme.Scheme.Default(cluster)
	cluster.Spec.Minio.Enable = false
	return cluster
}
