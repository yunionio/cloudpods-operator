package component

import (
	"path/filepath"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
)

type baseService struct {
	cType   v1alpha1.ComponentType
	options interface{}
}

func newBaseService(
	cType v1alpha1.ComponentType,
	options interface{},
) *baseService {
	return &baseService{
		cType:   cType,
		options: options,
	}
}

const (
	YUNION_ETC_CONFIG_DIR = "/etc/yunion"
)

func (bs baseService) GetConfigFilePath(targetDir string) string {
	return filepath.Join(targetDir, YUNION_ETC_CONFIG_DIR, string(bs.GetType()+".conf"))
}

func (bs baseService) GetCertsDirPath(targetDir string) string {
	return filepath.Join(filepath.Dir(bs.GetConfigFilePath(targetDir)), "pki")
}

func (bs baseService) GetType() v1alpha1.ComponentType {
	return bs.cType
}

func (bs baseService) GetOptions() interface{} {
	return bs.options
}

func (c baseService) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	return nil, nil
}

func (c baseService) GetDefaultDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return nil
}

func (bs baseService) GetDefaultClickhouseConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return nil
}

func (bs baseService) GetDefaultCloudUser(*v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return nil
}

func (bs baseService) BuildCluster(oc *v1alpha1.OnecloudCluster, opts interface{}) error {
	return nil
}

func (bs baseService) BuildClusterConfigDB(cfg *v1alpha1.OnecloudClusterConfig, dbCfg v1alpha1.DBConfig) error {
	return nil
}

func (bs baseService) BuildClusterConfigCloudUser(cfg *v1alpha1.OnecloudClusterConfig, user v1alpha1.CloudUser) error {
	return nil
}

func (bs baseService) GetPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return nil
}

func (bs baseService) BeforeStart(oc *v1alpha1.OnecloudCluster, targetCfgDir string) error {
	return nil
}
