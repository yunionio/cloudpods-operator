package k8sutil

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"k8s.io/apimachinery/pkg/version"
)

func Test_clusterVersion_GetSemVerion(t *testing.T) {
	cv := clusterVersion{
		version: &version.Info{
			GitVersion: "v1.23.0",
		},
	}
	sv := cv.GetSemVerion()
	if !sv.LessThan(*semver.New("1.24.0")) {
		t.Errorf("v1.23.0 should less than 1.24.0")
	}

	if !cv.IsLessThan("v1.23.1") {
		t.Errorf("v1.23.0 should less than 1.23.1")
	}

	if !cv.IsGreatOrEqualThan("v1.23.0") {
		t.Errorf("v1.23.0 should equal 1.23.0")
	}
}
