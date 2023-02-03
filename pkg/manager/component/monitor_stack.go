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

package component

import (
	"context"
	"fmt"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/mcclient"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/k8sutil"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

type monitorStackManager struct {
	*ComponentManager
}

func newMonitorStackManager(man *ComponentManager) manager.Manager {
	return &monitorStackManager{man}
}

func (m *monitorStackManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *monitorStackManager) getComponentType() v1alpha1.ComponentType {
	return v1alpha1.MonitorStackComponentType
}

func (m *monitorStackManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if oc.Spec.MonitorStack.Disable {
		return nil
	}

	clustercfg, err := m.configer.GetClusterConfig(oc)
	if err != nil {
		return errors.Wrap(err, "get cluster config for grafana")
	}

	spec := &oc.Spec.MonitorStack
	if !spec.Grafana.Disable {
		if err := EnsureClusterDBUser(oc, clustercfg.Grafana.DB); err != nil {
			return errors.Wrap(err, "ensure grafana db config")
		}
	}

	if err := m.onecloudControl.RunWithSession(oc, func(s *mcclient.ClientSession) error {
		status := &oc.Status.MonitorStack

		if err := m.syncMinio(s, &oc.Spec, &spec.Minio, &status.MinioStatus); err != nil {
			return errors.Wrap(err, "sync monitor-stack minio component")
		}

		minioSvc, err := m.svcLister.Services(constants.MonitorStackNamespace).Get(constants.MonitorMinioName)
		if err != nil {
			return errors.Wrapf(err, "get %q service %q", constants.MonitorStackNamespace, constants.MonitorMinioName)
		}

		if err := m.syncThanos(s, &oc.Spec, spec, minioSvc, status); err != nil {
			return errors.Wrapf(err, "sync thanos component")
		}

		if err := m.syncStack(s, &oc.Spec, clustercfg, minioSvc, status); err != nil {
			return errors.Wrapf(err, "sync monitor stack component")
		}

		if err := m.syncBucketPolicy(&oc.Spec.MonitorStack); err != nil {
			return errors.Wrapf(err, "sync monitor")
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (m *monitorStackManager) syncMinio(s *mcclient.ClientSession, ocSpec *v1alpha1.OnecloudClusterSpec, spec *v1alpha1.MonitorStackMinioSpec, status *v1alpha1.MonitorStackMinioStatus) error {
	masterNodes, err := k8sutil.GetReadyMasterNodes(m.nodeLister)
	if err != nil {
		return errors.Wrap(err, "List k8s ready master node")
	}

	if len(masterNodes) >= 3 {
		if spec.Mode == "" {
			spec.Mode = v1alpha1.MinioModeDistributed
		}
	} else {
		if spec.Mode == v1alpha1.MinioModeDistributed {
			return errors.Errorf("Master ready node count %d, but mode is %s", len(masterNodes), spec.Mode)
		}

		if spec.Mode == "" {
			spec.Mode = v1alpha1.MinioModeStandalone
		}
	}

	if spec.Mode == v1alpha1.MinioModeStandalone {
		return m.syncStandaloneMinio(s, ocSpec, spec, status)
	}

	// use distributed mode
	if err := m.syncDistributedMinio(s, ocSpec, spec, status); err != nil {
		return err
	}

	return nil
}

func (m *monitorStackManager) syncStandaloneMinio(
	s *mcclient.ClientSession,
	ocSpec *v1alpha1.OnecloudClusterSpec,
	spec *v1alpha1.MonitorStackMinioSpec,
	status *v1alpha1.MonitorStackMinioStatus,
) error {
	if err := onecloud.SyncMonitorMinio(s, &onecloud.EnableMinioParams{
		Mode:          string(v1alpha1.MinioModeStandalone),
		Replicas:      1,
		DrivesPerNode: 1,
		AccessKey:     spec.AccessKey,
		SecretKey:     spec.SecretKey,
	}, ocSpec); err != nil {
		return errors.Wrap(err, "sync standalone minio")
	}

	// get minio deployment status
	ret, err := m.kubeCli.AppsV1().Deployments(constants.MonitorStackNamespace).Get(context.Background(), constants.MonitorMinioName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "get monitor-minio deployment")
	}

	status.MonitorStackComponentStatus.ImageStatus = getImageStatus(ret)

	status.Replicas = 1
	status.DrivesPerNode = 1

	return nil
}

func (m *monitorStackManager) syncDistributedMinio(
	s *mcclient.ClientSession,
	ocSpec *v1alpha1.OnecloudClusterSpec,
	spec *v1alpha1.MonitorStackMinioSpec,
	status *v1alpha1.MonitorStackMinioStatus,
) error {
	if err := onecloud.SyncMonitorMinio(s, &onecloud.EnableMinioParams{
		Mode:          string(v1alpha1.MinioModeDistributed),
		Replicas:      4,
		DrivesPerNode: 1,
		AccessKey:     spec.AccessKey,
		SecretKey:     spec.SecretKey,
	}, ocSpec); err != nil {
		return errors.Wrap(err, "sync distributed minio")
	}

	// get minio statefulset status
	ssCli := m.kubeCli.AppsV1().StatefulSets(constants.MonitorStackNamespace)
	ss, err := ssCli.Get(context.Background(), constants.MonitorMinioName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "get monitor-minio statefulset")
	}
	status.ImageStatus = getImageStatusByContainer(&ss.Spec.Template.Spec.Containers[0])

	status.Replicas = 4
	status.DrivesPerNode = 1

	return nil
}

func (_ *monitorStackManager) getMinioAuthConfig(spec *v1alpha1.MonitorStackMinioSpec, svc *v1.Service, bucket string) *v1alpha1.ObjectStoreConfig {
	return &v1alpha1.ObjectStoreConfig{
		Bucket:    bucket,
		Endpoint:  fmt.Sprintf("%s.%s:%d", constants.MonitorMinioName, constants.MonitorStackNamespace, svc.Spec.Ports[0].Port),
		AccessKey: spec.AccessKey,
		SecretKey: spec.SecretKey,
	}
}

func (m *monitorStackManager) setDefaultObjectStoreConfig(
	minioSpec *v1alpha1.MonitorStackMinioSpec,
	minioSvc *v1.Service,
	storeSpec *v1alpha1.ObjectStoreConfig,
	defaultBucket string,
) {
	if storeSpec.AccessKey == "" || storeSpec.SecretKey == "" {
		if storeSpec.Bucket == "" {
			storeSpec.Bucket = defaultBucket
		}
		minioConf := m.getMinioAuthConfig(minioSpec, minioSvc, storeSpec.Bucket)
		storeSpec.Endpoint = minioConf.Endpoint
		storeSpec.AccessKey = minioConf.AccessKey
		storeSpec.SecretKey = minioConf.SecretKey
	}
}

func (m *monitorStackManager) syncThanos(
	s *mcclient.ClientSession,
	ocSpec *v1alpha1.OnecloudClusterSpec,
	spec *v1alpha1.MonitorStackSpec,
	minioSvc *v1.Service,
	status *v1alpha1.MonitorStackStatus) error {
	thanosSpec := &spec.Thanos
	storeSpec := &thanosSpec.ObjectStoreConfig

	if spec.Prometheus.Disable == nil || *spec.Prometheus.Disable {
		return nil
	}

	m.setDefaultObjectStoreConfig(&spec.Minio, minioSvc, storeSpec, constants.MonitorBucketThanos)

	if err := onecloud.SyncMonitorThanos(s, ocSpec, thanosSpec); err != nil {
		return errors.Wrap(err, "enable thanos")
	}

	// TODO: fill all thanos service status
	ret, err := m.deployLister.Deployments(constants.MonitorStackNamespace).Get(constants.MonitorThanosQuery)
	if err != nil {
		return errors.Wrap(err, "get thanos-query deployment")
	}
	status.ThanosStatus.ImageStatus = getImageStatus(ret)

	return nil
}

func (m *monitorStackManager) syncStack(
	s *mcclient.ClientSession,
	spec *v1alpha1.OnecloudClusterSpec,
	config *v1alpha1.OnecloudClusterConfig,
	minioSvc *v1.Service,
	status *v1alpha1.MonitorStackStatus) error {

	stackSpec := &spec.MonitorStack
	ns := constants.MonitorStackNamespace

	grafana := &stackSpec.Grafana
	if err := m.validateGrafanaSpec(grafana); err != nil {
		return errors.Wrap(err, "validate grafana")
	}
	lokiStore := &stackSpec.Loki.ObjectStoreConfig
	m.setDefaultObjectStoreConfig(&stackSpec.Minio, minioSvc, lokiStore, constants.MonitorBucketLoki)

	promStore := &stackSpec.Prometheus.ThanosSidecarSpec.ObjectStoreConfig
	m.setDefaultObjectStoreConfig(&stackSpec.Minio, minioSvc, promStore, constants.MonitorBucketThanos)

	if err := onecloud.SyncMonitorStack(s, spec, config); err != nil {
		return errors.Wrap(err, "enable monitor stack component")
	}

	if !grafana.Disable {
		grafanaDeploy, err := m.deployLister.Deployments(ns).Get(constants.MonitorGrafana)
		if err != nil {
			return errors.Wrap(err, "get grafana deployment")
		}
		status.GrafanaStatus.ImageStatus = getImageStatus(grafanaDeploy)
	}

	ssCli := m.kubeCli.AppsV1().StatefulSets(ns)
	if !stackSpec.Loki.Disable {
		lokiSs, err := ssCli.Get(context.Background(), constants.MonitorLoki, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "get loki statefulset")
		}
		status.LokiStatus.ImageStatus = getImageStatusByContainer(&lokiSs.Spec.Template.Spec.Containers[0])
	}

	if stackSpec.Prometheus.Disable != nil && !*stackSpec.Prometheus.Disable {
		promthuesAppName := constants.MonitorPrometheus
		promthues, err := ssCli.Get(context.Background(), promthuesAppName, metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "get %s statefulset", promthuesAppName)
		}
		status.PrometheusStatus.ImageStatus = getImageStatusByContainer(&promthues.Spec.Template.Spec.Containers[0])
	}

	return nil
}

func (m *monitorStackManager) validateGrafanaSpec(spec *v1alpha1.MonitorStackGrafanaSpec) error {
	if spec.AdminUser == "" {
		return errors.Errorf("adminUser is empty")
	}

	if spec.AdminPassword == "" {
		return errors.Errorf("adminPassword is empty")
	}

	return nil
}

func (m *monitorStackManager) newMinioClient(spec *v1alpha1.ObjectStoreConfig) (*minio.Client, error) {
	cli, err := minio.New(spec.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(spec.AccessKey, spec.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, errors.Wrap(err, "new minio client")
	}
	return cli, nil
}

func (m *monitorStackManager) syncBucketPolicy(spec *v1alpha1.MonitorStackSpec) error {
	// set loki bucket policy
	cli, err := m.newMinioClient(&spec.Loki.ObjectStoreConfig)
	if err != nil {
		return errors.Wrap(err, "for loki")
	}
	if err := cli.SetBucketLifecycle(context.Background(), constants.MonitorBucketLoki, &lifecycle.Configuration{
		Rules: []lifecycle.Rule{
			{
				ID: "Removing old log chunks",
				RuleFilter: lifecycle.Filter{
					Prefix: "fake/",
				},
				Expiration: lifecycle.Expiration{
					Days: 7,
				},
				Status: "Enabled",
			},
		},
	}); err != nil {
		return errors.Wrap(err, "set lifecycle policy of 'loki/fake'")
	}
	return nil
}
