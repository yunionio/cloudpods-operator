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
	"strings"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

const (
	RegionDNSConfigTemplate = `.:53 {
    cache 30

    yunion . {
        sql_connection mysql+pymysql://{{.DBUser}}:{{.DBPassword}}@{{.DBHost}}:{{.DBPort}}/{{.DBName}}?charset=utf8
        dns_domain {{.DNSDomain}}
        region {{.Region}}
        k8s_skip
        fallthrough .
    }

    {{- range .Proxies }}

    proxy {{.From}} {{.To}} {
    }
    {{- end }}

    log {
        class error
    }
}
`
)

type RegionDNSConfig struct {
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     int32
	DBName     string
	DNSDomain  string
	Region     string

	Proxies []RegionDNSProxy
}

type RegionDNSProxy struct {
	From string
	To   string
}

func (c RegionDNSConfig) GetContent() (string, error) {
	return CompileTemplateFromMap(RegionDNSConfigTemplate, c)
}

type regionDNSManager struct {
	*ComponentManager
}

func newRegionDNSManager(man *ComponentManager) manager.Manager {
	return &regionDNSManager{ComponentManager: man}
}

func (m *regionDNSManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *regionDNSManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, oc.Spec.RegionDNS.Disable, "")
}

func (m *regionDNSManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	db := oc.Spec.Mysql
	regionDB := cfg.RegionServer.DB
	spec := oc.Spec.RegionDNS
	cType := v1alpha1.RegionDNSComponentType
	defaultDNSTo := []string{"114.114.114.114", "8.8.8.8"}
	if len(spec.Proxies) == 0 {
		spec.Proxies = append(spec.Proxies, v1alpha1.RegionDNSProxy{
			From: ".",
			To:   defaultDNSTo,
		})
	}
	proxies := make([]RegionDNSProxy, len(spec.Proxies))
	for i, p := range spec.Proxies {
		if len(p.From) == 0 {
			spec.Proxies[i].From = "."
		}
		if len(p.To) == 0 {
			spec.Proxies[i].To = defaultDNSTo
		}
		p = spec.Proxies[i]
		proxies[i] = RegionDNSProxy{
			From: p.From,
			To:   strings.Join(p.To, " "),
		}
	}
	regionSpec := oc.Spec.RegionServer
	config := RegionDNSConfig{
		DBUser:     regionDB.Username,
		DBPassword: regionDB.Password,
		DBHost:     db.Host,
		DBPort:     db.Port,
		DBName:     regionDB.Database,
		DNSDomain:  regionSpec.DNSDomain,
		Region:     oc.GetRegion(),
		Proxies:    proxies,
	}
	content, err := config.GetContent()
	if err != nil {
		return nil, false, err
	}
	oc.Spec.RegionDNS = spec
	return m.newConfigMap(cType, "", oc, content), false, nil
}

func (m *regionDNSManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	// use headless service
	cType := v1alpha1.RegionDNSComponentType
	svcName := controller.NewClusterComponentName(oc.GetName(), cType)
	appLabel := m.getComponentLabel(oc, cType)
	svc := &corev1.Service{
		ObjectMeta: m.getObjectMeta(oc, svcName, appLabel),
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  appLabel,
		},
	}
	return []*corev1.Service{svc}
}

func (m *regionDNSManager) getDaemonSet(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.DaemonSet, error) {
	spec := oc.Spec.RegionDNS
	cType := v1alpha1.RegionDNSComponentType
	configMap := controller.ComponentConfigMapName(oc, cType)
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            cType.String(),
				Image:           spec.Image,
				ImagePullPolicy: spec.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/region-dns", "-conf", "/etc/yunion/region-dns.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	if spec.NodeSelector == nil {
		spec.NodeSelector = make(map[string]string)
	}
	spec.NodeSelector[constants.OnecloudControllerLabelKey] = "enable"
	ds, err := m.newDaemonSet(cType, oc, cfg,
		NewVolumeHelper(oc, configMap, cType),
		spec.DaemonSetSpec, "", nil, cf)
	if err != nil {
		return nil, err
	}
	return ds, nil
}
