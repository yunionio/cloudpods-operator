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

package k8s

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

var files = map[string][]byte{
	"foo": []byte(`
kind: Foo
apiVersion: foo.k8s.io/v1
fooField: foo
`),
	"bar": []byte(`
apiVersion: bar.k8s.io/v2
barField: bar
kind: Bar
`),
	"baz": []byte(`
apiVersion: baz.k8s.io/v1
kind: Baz
baz:
    foo: bar
`),
	"nokind": []byte(`
apiVersion: baz.k8s.io/v1
foo: foo
bar: bar
`),
	"noapiversion": []byte(`
kind: Bar
foo: foo
bar: bar
`),
}

func TestMarshalUnmarshalYaml(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "someName",
			Namespace: "testNamespace",
			Labels: map[string]string{
				"test": "yes",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
	bytes, err := MarshalToYaml(pod, corev1.SchemeGroupVersion)
	if err != nil {
		t.Fatalf("unexpected error marshalling: %v", err)
	}
	t.Logf("\n%s", bytes)
	obj2, err := UnmarshalFromYaml(bytes, corev1.SchemeGroupVersion)
	if err != nil {
		t.Fatalf("unexpected error marshalling: %v", err)
	}
	pod2, ok := obj2.(*corev1.Pod)
	if !ok {
		t.Fatal("did not get a Pod")
	}

	if pod2.Name != pod.Name {
		t.Errorf("expected %q, got %q", pod.Name, pod2.Name)
	}

	if pod2.Namespace != pod.Namespace {
		t.Errorf("expected %q, got %q", pod.Namespace, pod2.Namespace)
	}

	if !reflect.DeepEqual(pod2.Labels, pod.Labels) {
		t.Errorf("expected %v, got %v", pod.Labels, pod2.Labels)
	}

	if pod2.Spec.RestartPolicy != pod.Spec.RestartPolicy {
		t.Errorf("expected %q, got %q", pod.Spec.RestartPolicy, pod2.Spec.RestartPolicy)
	}
}

func TestMarshalToYamlForCodecs(t *testing.T) {
	cfg := &v1alpha1.OnecloudClusterConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OnecloudClusterConfig",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
	}
	v1alpha1.SetDefaults_OnecloudClusterConfig(cfg)
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	codes := serializer.NewCodecFactory(scheme)

	bytes, err := MarshalToYamlForCodecs(cfg, v1alpha1.SchemeGroupVersion, codes)
	if err != nil {
		t.Fatalf("unexpected error marshalling OnecloudClusterconfig: %v", err)
	}
	t.Logf("\n%s", bytes)

	obj, err := UnmarshalFromYamlForCodecs(bytes, v1alpha1.SchemeGroupVersion, codes)
	if err != nil {
		t.Fatalf("unexpected err unmarshalling OnecloudClusterconfig: %v", err)
	}
	cfg2, ok := obj.(*v1alpha1.OnecloudClusterConfig)
	if !ok || cfg2 == nil {
		t.Fatal("dit not get OnecloudClusterconfig back")
	}
	if !reflect.DeepEqual(*cfg, *cfg2) {
		t.Errorf("expected %v, got %v", *cfg, *cfg2)
	}
}
