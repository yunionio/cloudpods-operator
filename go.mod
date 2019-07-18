module yunion.io/x/onecloud-operator

go 1.12

require (
	github.com/go-sql-driver/mysql v1.4.1
	github.com/google/gofuzz v1.0.0 // indirect
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/json-iterator/go v1.1.6 // indirect
	github.com/onsi/ginkgo v1.8.0 // indirect
	github.com/onsi/gomega v1.5.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/ugorji/go/codec v1.1.7 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/sync v0.0.0-20181221193216-37e7f081c4d4
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	k8s.io/api v0.0.0-20190708174958-539a33f6e817
	k8s.io/apiextensions-apiserver v0.0.0-20190606210616-f848dc7be4a4
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/apiserver v0.0.0-20190606205144-71ebb8303503 // indirect
	k8s.io/client-go v9.0.0+incompatible
	k8s.io/cloud-provider v0.0.0-20190606212257-347f17c60af0 // indirect
	k8s.io/cluster-bootstrap v0.0.0-20190606212113-a4a4ceb6dbd9 // indirect
	k8s.io/code-generator v0.0.0-20190311093542-50b561225d70 // indirect
	k8s.io/component-base v0.0.0-20190606204607-bb6a29a90c31 // indirect
	k8s.io/klog v0.3.3
	k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208 // indirect
	k8s.io/kubernetes v1.14.3
	k8s.io/utils v0.0.0-20190607212802-c55fbcfc754a // indirect
	yunion.io/x/jsonutils v0.0.0-20190625054549-a964e1e8a051
	yunion.io/x/log v0.0.0-20190629062853-9f6483a7103d
	yunion.io/x/onecloud v0.0.0-20190725062408-c88eae5261a2
	yunion.io/x/pkg v0.0.0-20190628082551-f4033ba2ea30
	yunion.io/x/structarg v0.0.0-20190717142057-5caf182cbb4d
)

replace (
	github.com/Sirupsen/logrus v1.4.2 => github.com/sirupsen/logrus v1.4.2
	github.com/ugorji/go => github.com/ugorji/go v0.0.0-20181204163529-d75b2dcb6bc8
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190620073620-d55040311883
	yunion.io/x/onecloud => yunion.io/x/onecloud v0.0.0-20190725062408-c88eae5261a2
)
