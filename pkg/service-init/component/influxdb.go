package component

import (
	"path"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
)

func init() {
	RegisterComponent(NewInfluxdb())
}

type influxdb struct {
	*baseService
}

func NewInfluxdb() Component {
	return &influxdb{
		baseService: newBaseService(v1alpha1.InfluxdbComponentType, nil),
	}
}

const (
	InfluxDBConfigTemplate = `
[meta]
  # Where the metadata/raft database is stored
  dir = "/var/lib/influxdb/meta"

  # Automatically create a default retention policy when creating a database.
  retention-autocreate = true

  # If log messages are printed for the meta service
  # logging-enabled = true
  # default-retention-policy-name = "default"
  default-retention-policy-name = "30day_only"

[data]
  # The directory where the TSM storage engine stores TSM files.
  dir = "/var/lib/influxdb/data"

  # The directory where the TSM storage engine stores WAL files.
  wal-dir = "/var/lib/influxdb/wal"

[http]
  https-enabled = true
  https-certificate = "{{.CertPath}}"
  https-private-key = "{{.KeyPath}}"
  bind-address = ":{{.Port}}"

[subscriber]
  insecure-skip-verify = true
`
)

type InfluxdbConfig struct {
	Port     int
	CertPath string
	KeyPath  string
}

func (c InfluxdbConfig) GetContent() (string, error) {
	return CompileTemplateFromMap(InfluxDBConfigTemplate, c)
}

func (r influxdb) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	config := InfluxdbConfig{
		Port:     constants.InfluxdbPort,
		CertPath: path.Join(constants.CertDir, constants.ServiceCertName),
		KeyPath:  path.Join(constants.CertDir, constants.ServiceKeyName),
	}
	return config.GetContent()
}

func (r influxdb) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.InfluxdbComponentType,
		constants.ServiceNameInfluxdb, constants.ServiceTypeInfluxdb,
		constants.InfluxdbPort, "")
}
