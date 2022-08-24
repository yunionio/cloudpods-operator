package k8sutil

import (
	"strings"

	"github.com/coreos/go-semver/semver"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"

	"yunion.io/x/pkg/errors"
)

var (
	globalClusterVersion ClusterVersion
)

func InitClusterVersion(cli kubernetes.Interface) error {
	if globalClusterVersion == nil {
		cv, err := newClusterVersion(cli)
		if err != nil {
			return errors.Wrapf(err, "NewClusterVersion")
		}
		globalClusterVersion = cv
		return nil
	}
	return errors.Errorf("Global ClusterVersion already initialized")
}

func GetClusterVersion() ClusterVersion {
	if globalClusterVersion == nil {
		panic("Global ClusterVersion not initialized")
	}
	return globalClusterVersion
}

type ClusterVersion interface {
	GetRawVersion() string
	GetSemVerion() *semver.Version
	IsLessThan(iv string) bool
	IsGreatOrEqualThan(iv string) bool
	IsGreatOrEqualThanV120() bool
	GetIngressGVR() schema.GroupVersionResource
}

type clusterVersion struct {
	client  kubernetes.Interface
	version *version.Info
}

func newClusterVersion(cli kubernetes.Interface) (ClusterVersion, error) {
	ver, err := cli.Discovery().ServerVersion()
	if err != nil {
		return nil, errors.Wrap(err, "Get ServerVersion")
	}
	return &clusterVersion{
		client:  cli,
		version: ver,
	}, nil
}

func (cv clusterVersion) GetRawVersion() string {
	return cv.version.GitVersion
}

func (cv clusterVersion) GetSemVerion() *semver.Version {
	rv := strings.TrimPrefix(cv.GetRawVersion(), "v")
	return semver.New(rv)
}

func (cv clusterVersion) IsLessThan(iv string) bool {
	iv = strings.TrimPrefix(iv, "v")
	return cv.GetSemVerion().LessThan(*semver.New(iv))
}

func (cv clusterVersion) IsGreatOrEqualThan(iv string) bool {
	return !cv.IsLessThan(iv)
}

func (cv clusterVersion) IsGreatOrEqualThanV120() bool {
	// ref issue: https://github.com/kubernetes-sigs/nfs-subdir-external-provisioner/issues/25
	return cv.IsGreatOrEqualThan("1.20.0")
}

func (cv clusterVersion) GetIngressGVR() schema.GroupVersionResource {
	if cv.IsGreatOrEqualThan("1.19.0") {
		return schema.GroupVersionResource{
			Group:    "networking.k8s.io",
			Version:  "v1",
			Resource: "ingresses",
		}
	} else if cv.IsGreatOrEqualThan("1.14.0") {
		return schema.GroupVersionResource{
			Group:    "networking.k8s.io",
			Version:  "v1beta1",
			Resource: "ingresses",
		}
	}
	return schema.GroupVersionResource{
		Group:    "extensions",
		Version:  "v1beta1",
		Resource: "ingresses",
	}
}
