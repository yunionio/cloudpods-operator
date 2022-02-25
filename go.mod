module yunion.io/x/onecloud-operator

go 1.12

require (
	github.com/docker/distribution v0.0.0-20170726174610-edc3ab29cdff
	github.com/go-sql-driver/mysql v1.5.0
	github.com/minio/minio-go/v7 v7.0.6
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pborman/uuid v1.2.0
	github.com/pkg/errors v0.9.1
	go.etcd.io/etcd/api/v3 v3.5.0
	go.etcd.io/etcd/client/pkg/v3 v3.5.0
	go.etcd.io/etcd/client/v3 v3.5.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	k8s.io/api v0.23.4
	k8s.io/apiextensions-apiserver v0.23.4
	k8s.io/apimachinery v0.23.4
	k8s.io/client-go v0.23.4
	k8s.io/klog v1.0.0
	yunion.io/x/jsonutils v0.0.0-20220106020632-953b71a4c3a8
	yunion.io/x/log v0.0.0-20201210064738-43181789dc74
	yunion.io/x/onecloud v0.0.0-20211230024531-348442d65f38
	yunion.io/x/pkg v0.0.0-20211116020154-6a76ba2f7e97
	yunion.io/x/structarg v0.0.0-20220224030024-02b7582b2546
)

replace (
	github.com/Sirupsen/logrus v1.4.2 => github.com/sirupsen/logrus v1.4.2
	github.com/ugorji/go => github.com/ugorji/go v0.0.0-20181204163529-d75b2dcb6bc8

	go.etcd.io/bbolt => go.etcd.io/bbolt v1.3.6
	go.etcd.io/etcd/api/v3 => go.etcd.io/etcd/api/v3 v3.5.0
	go.etcd.io/etcd/client/pkg/v3 => go.etcd.io/etcd/client/pkg/v3 v3.5.0
	go.etcd.io/etcd/client/v2 => go.etcd.io/etcd/client/v2 v2.305.0
	go.etcd.io/etcd/client/v3 => go.etcd.io/etcd/client/v3 v3.5.0
	go.etcd.io/etcd/pkg/v3 => go.etcd.io/etcd/pkg/v3 v3.5.0
	go.etcd.io/etcd/raft/v3 => go.etcd.io/etcd/raft/v3 v3.5.0
	go.etcd.io/etcd/server/v3 => go.etcd.io/etcd/server/v3 v3.5.0

	google.golang.org/grpc => google.golang.org/grpc v1.29.0

	k8s.io/api => k8s.io/api v0.23.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.23.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.23.4
	k8s.io/client-go => k8s.io/client-go v0.23.4
	k8s.io/code-generator => k8s.io/code-generator v0.23.4 // indirect
	yunion.io/x/onecloud => ../onecloud

)
