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

package onecloud

import (
	"strings"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	k8smod "yunion.io/x/onecloud/pkg/mcclient/modules/k8s"
	"yunion.io/x/onecloud/pkg/mcclient/options/k8s"
)

type ClusterComponentType string

const (
	ClusterComponentTypeMinio        ClusterComponentType = "minio"
	ClusterComponentTypeMonitorMinio ClusterComponentType = "monitorMinio"
	ClusterComponentTypeMonitor      ClusterComponentType = "monitor"
	ClusterComponentTypeThanos       ClusterComponentType = "thanos"
)

type ComponentStatus struct {
	Id      string `json:"id"`
	Created bool   `json:"created"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

type ClusterComponentsStatus struct {
	CephCSI      *ComponentStatus `json:"cephCSI"`
	Monitor      *ComponentStatus `json:"monitor"`
	FluentBit    *ComponentStatus `json:"fluentbit"`
	Thanos       *ComponentStatus `json:"thanos"`
	Minio        *ComponentStatus `json:"minio"`
	MonitorMinio *ComponentStatus `json:"monitorMinio"`
}

func GetSystemClusterComponentsStatus(s *mcclient.ClientSession, id string) (*ClusterComponentsStatus, error) {
	ret, err := k8smod.KubeClusters.GetSpecific(s, id, "components-status", nil)
	if err != nil {
		return nil, errors.Wrapf(err, "Get cluster %s components status", id)
	}
	out := new(ClusterComponentsStatus)
	if err := ret.Unmarshal(out); err != nil {
		return nil, err
	}
	return out, nil
}

func GetSystemCluster(s *mcclient.ClientSession) (jsonutils.JSONObject, error) {
	ret, err := k8smod.KubeClusters.Get(s, K8SSystemClusterName, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Get kubernetes system default cluster")
	}
	return ret, nil
}

func SyncSystemCluster(s *mcclient.ClientSession, id string) error {
	_, err := k8smod.KubeClusters.PerformAction(s, id, "sync", nil)
	return err
}

func syncClusterComponent(
	cType ClusterComponentType,
	s *mcclient.ClientSession,
	getStatus func(*ClusterComponentsStatus) (bool, string),
	onStatusFail func(status, id string) error,
	doEnable func(s *mcclient.ClientSession, id string) error,
	doUpdate func(s *mcclient.ClientSession, id string) error,
) error {
	ret, err := GetSystemCluster(s)
	if err != nil {
		return err
	}
	id, err := ret.GetString("id")
	if err != nil {
		return errors.Wrapf(err, "Get kubernetes system default cluster id from %s", ret)
	}

	isEnabled, err := isClusterComponentEnabled(
		cType, s, id,
		getStatus, onStatusFail,
	)
	if err != nil {
		return errors.Wrapf(err, "Check %q is enabled", cType)
	}
	if isEnabled {
		// update to ensure sync
		if err := doUpdate(s, id); err != nil {
			return errors.Wrapf(err, "Update system default cluster %q component", cType)
		}
		return nil
	}
	if err := SyncSystemCluster(s, id); err != nil {
		return errors.Wrap(err, "Sync system default cluster")
	}

	if err := doEnable(s, id); err != nil {
		return errors.Wrapf(err, "Enable system default cluster %q component", cType)
	}
	return nil
}

func isClusterComponentEnabled(
	name ClusterComponentType,
	s *mcclient.ClientSession, id string,
	getStatus func(*ClusterComponentsStatus) (bool, string),
	onStatusFail func(status, id string) error,
) (bool, error) {
	statuses, err := GetSystemClusterComponentsStatus(s, id)
	if err != nil {
		return false, errors.Wrap(err, "Get cluster components status")
	}

	enabled, status := getStatus(statuses)

	if strings.HasSuffix(status, "_fail") {
		// disable it here
		if err := onStatusFail(status, id); err != nil {
			log.Errorf("Try disable %q when status is %q: %s", name, status, err)
		}
		return false, errors.Errorf("%q deploy status: %s", name, status)
	}
	return enabled, nil
}

func disableClusterComponent(name ClusterComponentType, s *mcclient.ClientSession, systemClusterId string) error {
	params := map[string]string{
		"type": string(name),
	}
	if _, err := k8smod.KubeClusters.PerformAction(s, systemClusterId, "disable-component", jsonutils.Marshal(params)); err != nil {
		return errors.Wrapf(err, "disable %s", name)
	}
	return nil
}

type EnableMinioParams struct {
	Mode          string
	Replicas      int
	DrivesPerNode int
	AccessKey     string
	SecretKey     string
}

type ClusterEnableComponentMinioOpt struct {
	k8s.ClusterComponentOptions
	k8s.ClusterComponentMinioSetting
}

func (o ClusterEnableComponentMinioOpt) Params(ctype ClusterComponentType) (jsonutils.JSONObject, error) {
	params := o.ClusterComponentOptions.Params(string(ctype))
	setting := jsonutils.Marshal(o.ClusterComponentMinioSetting)
	params.Add(setting, string(ctype))
	return params, nil
}

func newClusterComponentOpt(id string) k8s.ClusterComponentOptions {
	return k8s.ClusterComponentOptions{
		IdentOptions: k8s.IdentOptions{
			ID: id,
		},
	}
}

func doEnableClusterComponent(
	cType ClusterComponentType,
	s *mcclient.ClientSession, sId string,
	params jsonutils.JSONObject) error {
	_, err := k8smod.KubeClusters.PerformAction(s, sId, "enable-component", params)
	if err != nil {
		return errors.Wrapf(err, "Enable %s cluster component of system-cluster %s", cType, sId)
	}
	return nil
}

func doUpdateClusterComponent(
	cType ClusterComponentType,
	s *mcclient.ClientSession, sId string,
	params jsonutils.JSONObject) error {
	_, err := k8smod.KubeClusters.PerformAction(s, sId, "update-component", params)
	if err != nil {
		return errors.Wrapf(err, "Update %s cluster component of system-cluster %s", cType, sId)
	}
	return nil
}

func getSyncMinioParams(cType ClusterComponentType, cId string, input *EnableMinioParams) (jsonutils.JSONObject, error) {
	opt := &ClusterEnableComponentMinioOpt{
		ClusterComponentOptions: newClusterComponentOpt(cId),
		ClusterComponentMinioSetting: k8s.ClusterComponentMinioSetting{
			Mode:          input.Mode,
			Replicas:      input.Replicas,
			DrivesPerNode: input.DrivesPerNode,
			AccessKey:     input.AccessKey,
			SecretKey:     input.SecretKey,
			MountPath:     "/export",
			Storage: k8s.ClusterComponentStorage{
				Enabled:   true,
				SizeMB:    1024 * 1024,
				ClassName: v1alpha1.DefaultStorageClass,
			},
		},
	}
	params, err := opt.Params(cType)
	if err != nil {
		return nil, errors.Wrapf(err, "Generate %s component params", cType)
	}
	return params, nil
}

func enableMinio(
	ctype ClusterComponentType,
	s *mcclient.ClientSession,
	systemClusterId string,
	input *EnableMinioParams,
) error {
	params, err := getSyncMinioParams(ctype, systemClusterId, input)
	if err != nil {
		return err
	}
	return doEnableClusterComponent(ctype, s, systemClusterId, params)
}

func updateMinio(
	ctype ClusterComponentType,
	s *mcclient.ClientSession,
	systemClusterId string,
	input *EnableMinioParams,
) error {
	params, err := getSyncMinioParams(ctype, systemClusterId, input)
	if err != nil {
		return err
	}
	return doUpdateClusterComponent(ctype, s, systemClusterId, params)
}

func enableMinioWithParams(
	cType ClusterComponentType,
	s *mcclient.ClientSession,
	params *EnableMinioParams,
	getStatus func(*ClusterComponentsStatus) (bool, string),
) error {
	return syncClusterComponent(
		cType,
		s,
		getStatus,
		func(status, id string) error {
			return disableClusterComponent(cType, s, id)
		},
		func(s *mcclient.ClientSession, id string) error {
			return enableMinio(cType, s, id, params)
		},
		func(s *mcclient.ClientSession, id string) error {
			return updateMinio(cType, s, id, params)
		},
	)
}

func SyncMinio(s *mcclient.ClientSession, input *v1alpha1.Minio) error {
	params := &EnableMinioParams{
		Mode:          string(v1alpha1.MinioModeStandalone),
		Replicas:      1,
		DrivesPerNode: 1,
		AccessKey:     "minioadmin",
		SecretKey:     "yunionminio@admin",
	}

	if input.Mode == v1alpha1.MinioModeDistributed {
		params.Mode = string(v1alpha1.MinioModeDistributed)
		params.Replicas = 4
	}
	return enableMinioWithParams(ClusterComponentTypeMinio, s, params,
		func(status *ClusterComponentsStatus) (bool, string) {
			return status.Minio.Enabled, status.Minio.Status
		},
	)
}

func SyncMonitorMinio(s *mcclient.ClientSession, input *EnableMinioParams) error {
	return enableMinioWithParams(
		ClusterComponentTypeMonitorMinio, s, input,
		func(ccs *ClusterComponentsStatus) (bool, string) {
			if ccs.MonitorMinio == nil {
				return false, ""
			}
			return ccs.MonitorMinio.Enabled, ccs.MonitorMinio.Status
		})
}

func getSyncThanosParams(cId string, input *v1alpha1.MonitorStackThanosSpec) (jsonutils.JSONObject, error) {
	opt := &k8s.ClusterEnableComponentThanosOpt{
		ClusterComponentOptions: newClusterComponentOpt(cId),
		ClusterComponentThanosSetting: k8s.ClusterComponentThanosSetting{
			ObjectStoreConfig: k8s.ObjectStoreConfig{
				Bucket:    input.ObjectStoreConfig.Bucket,
				Endpoint:  input.ObjectStoreConfig.Endpoint,
				AccessKey: input.ObjectStoreConfig.AccessKey,
				SecretKey: input.ObjectStoreConfig.SecretKey,
				Insecure:  true,
			},
		},
	}
	params, err := opt.Params()
	if err != nil {
		return nil, errors.Wrapf(err, "Generate thanos component params")
	}
	return params, nil
}

func enableThanos(s *mcclient.ClientSession, cId string, input *v1alpha1.MonitorStackThanosSpec) error {
	params, err := getSyncThanosParams(cId, input)
	if err != nil {
		return err
	}
	return doEnableClusterComponent(ClusterComponentTypeThanos, s, cId, params)
}

func updateThanos(s *mcclient.ClientSession, cId string, input *v1alpha1.MonitorStackThanosSpec) error {
	params, err := getSyncThanosParams(cId, input)
	if err != nil {
		return err
	}
	return doUpdateClusterComponent(ClusterComponentTypeThanos, s, cId, params)
}

func SyncMonitorThanos(s *mcclient.ClientSession, input *v1alpha1.MonitorStackThanosSpec) error {
	cType := ClusterComponentTypeThanos
	return syncClusterComponent(cType, s,
		func(ccs *ClusterComponentsStatus) (bool, string) {
			return ccs.Thanos.Enabled, ccs.Thanos.Status
		},
		func(status string, id string) error {
			return disableClusterComponent(cType, s, id)
		},
		func(s *mcclient.ClientSession, id string) error {
			return enableThanos(s, id, input)
		},
		func(s *mcclient.ClientSession, id string) error {
			return updateThanos(s, id, input)
		},
	)
}

func getSyncMonitorParams(cId string, spec *v1alpha1.OnecloudClusterSpec) (jsonutils.JSONObject, error) {
	input := spec.MonitorStack
	grafana := input.Grafana
	grafanaOAuth := input.Grafana.OAuth
	loki := input.Loki
	prometheus := input.Prometheus
	thanosSidecar := prometheus.ThanosSidecarSpec

	// disable prometheus components by default
	disableProm := true
	if prometheus.Disable != nil {
		disableProm = *prometheus.Disable
	}

	opt := &k8s.ClusterEnableComponentMonitorOpt{
		ClusterComponentOptions: newClusterComponentOpt(cId),
		ClusterComponentMonitorSetting: k8s.ClusterComponentMonitorSetting{
			Grafana: k8s.ClusterComponentMonitorGrafana{
				Disable:           grafana.Disable,
				AdminUser:         grafana.AdminUser,
				AdminPassword:     grafana.AdminPassword,
				PublicAddress:     spec.LoadBalancerEndpoint,
				DisableSubpath:    grafana.DisableSubpath,
				Subpath:           grafana.Subpath,
				EnforceDomain:     grafana.EnforceDomain,
				EnableThanosQuery: true,
			},
			Loki: k8s.ClusterComponentMonitorLoki{
				Disable: loki.Disable,
				ObjectStoreConfig: k8s.ObjectStoreConfig{
					Bucket:    loki.ObjectStoreConfig.Bucket,
					Endpoint:  loki.ObjectStoreConfig.Endpoint,
					AccessKey: loki.ObjectStoreConfig.AccessKey,
					SecretKey: loki.ObjectStoreConfig.SecretKey,
					Insecure:  true,
				},
			},
			Promtail: k8s.ClusterComponentMonitorPromtail{
				Disable: loki.Disable,
			},
			Prometheus: k8s.ClusterComponentMonitorPrometheus{
				Disable: disableProm,
				Thanos: k8s.MonitorPrometheusThanosSidecar{
					ObjectStoreConfig: k8s.ObjectStoreConfig{
						Bucket:    thanosSidecar.ObjectStoreConfig.Bucket,
						Endpoint:  thanosSidecar.ObjectStoreConfig.Endpoint,
						AccessKey: thanosSidecar.ObjectStoreConfig.AccessKey,
						SecretKey: thanosSidecar.ObjectStoreConfig.SecretKey,
						Insecure:  true,
					},
				},
			},
		},
	}

	if grafana.Host != "" {
		opt.Grafana.Host = grafana.Host
		opt.Grafana.PublicAddress = ""
	}

	if grafanaOAuth.Enabled {
		opt.Grafana.Oauth = k8s.ClusterComponentMonitorGrafanaOAuth{
			Enabled:           true,
			ClientId:          grafanaOAuth.ClientId,
			ClientSecret:      grafanaOAuth.ClientSecret,
			Scopes:            grafanaOAuth.Scopes,
			AuthURL:           grafanaOAuth.AuthURL,
			TokenURL:          grafanaOAuth.TokenURL,
			ApiURL:            grafanaOAuth.APIURL,
			AllowedDomains:    grafanaOAuth.AllowedDomains,
			AllowSignUp:       grafanaOAuth.AllowSignUp,
			RoleAttributePath: grafanaOAuth.RoleAttributePath,
		}
	}

	params, err := opt.Params()
	if err != nil {
		return nil, errors.Wrap(err, "Generate monitor stack params")
	}
	return params, nil
}

func enableMonitorStack(s *mcclient.ClientSession, cId string, spec *v1alpha1.OnecloudClusterSpec) error {
	params, err := getSyncMonitorParams(cId, spec)
	if err != nil {
		return err
	}
	return doEnableClusterComponent(ClusterComponentTypeMonitor, s, cId, params)
}

func updateMonitorStack(s *mcclient.ClientSession, cId string, spec *v1alpha1.OnecloudClusterSpec) error {
	params, err := getSyncMonitorParams(cId, spec)
	if err != nil {
		return err
	}
	return doUpdateClusterComponent(ClusterComponentTypeMonitor, s, cId, params)
}

func SyncMonitorStack(s *mcclient.ClientSession, input *v1alpha1.OnecloudClusterSpec) error {
	cType := ClusterComponentTypeMonitor
	return syncClusterComponent(cType, s,
		func(ccs *ClusterComponentsStatus) (bool, string) {
			return ccs.Monitor.Enabled, ccs.Monitor.Status
		},
		func(status string, id string) error {
			return disableClusterComponent(cType, s, id)
		},
		func(s *mcclient.ClientSession, id string) error {
			return enableMonitorStack(s, id, input)
		},
		func(s *mcclient.ClientSession, id string) error {
			return updateMonitorStack(s, id, input)
		},
	)
}
