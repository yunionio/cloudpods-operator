package component

import (
	"path"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud/pkg/cloudcommon/options"
	"yunion.io/x/onecloud/pkg/mcclient"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
	"yunion.io/x/onecloud-operator/pkg/util/option"
)

type suggestionManager struct {
	*ComponentManager
}

func newSuggestionManager(man *ComponentManager) manager.ServiceManager {
	return &suggestionManager{man}
}

func (m *suggestionManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
	}
}

func (m *suggestionManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.SuggestionComponentType
}

func (m *suggestionManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Suggestion.Disable || !IsEnterpriseEdition(oc) || !isInProductVersion(m, oc)
}

func (m *suggestionManager) GetServiceName() string {
	return constants.ServiceNameSuggestion
}

func (m *suggestionManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(m, oc, "")
}

func (m *suggestionManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Suggestion.DB
}

func (m *suggestionManager) getDBEngine(oc *v1alpha1.OnecloudCluster) v1alpha1.TDBEngineType {
	return oc.Spec.GetDbEngine(oc.Spec.Suggestion.DbEngine)
}

func (m *suggestionManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Suggestion.CloudUser
}

func (m *suggestionManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.SuggestionComponentType,
		constants.ServiceNameSuggestion, constants.ServiceTypeSuggestion,
		man.GetCluster().Spec.Suggestion.Service.NodePort, "")
}

func (m *suggestionManager) getService(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) []*corev1.Service {
	return m.newSinglePortService(v1alpha1.SuggestionComponentType, oc, oc.Spec.Suggestion.Service.InternalOnly, int32(oc.Spec.Suggestion.Service.NodePort), int32(constants.SuggestionPort), oc.Spec.Suggestion.SlaveReplicas > 0)
}

type suggestionOptions struct {
	options.CommonOptions
	options.DBOptions
}

func (m *suggestionManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	opt := &suggestionOptions{}
	if err := option.SetOptionsDefault(opt, constants.ServiceTypeSuggestion); err != nil {
		return nil, false, err
	}
	config := cfg.Suggestion

	switch oc.Spec.GetDbEngine(oc.Spec.Suggestion.DbEngine) {
	case v1alpha1.DBEngineDameng:
		option.SetDamengOptions(&opt.DBOptions, oc.Spec.Dameng, config.DB)
	case v1alpha1.DBEngineMySQL:
		fallthrough
	default:
		option.SetMysqlOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	}

	option.SetOptionsServiceTLS(&opt.BaseOptions, false)
	option.SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions, cfg.CommonConfig)
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = constants.SuggestionPort
	return m.newServiceConfigMap(v1alpha1.SuggestionComponentType, "", oc, opt), false, nil
}

func (m *suggestionManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cf := func(volMounts []corev1.VolumeMount) []corev1.Container {
		return []corev1.Container{
			{
				Name:            "suggestion",
				Image:           oc.Spec.Suggestion.Image,
				ImagePullPolicy: oc.Spec.Suggestion.ImagePullPolicy,
				Command:         []string{"/opt/yunion/bin/suggestion", "--config", "/etc/yunion/suggestion.conf"},
				VolumeMounts:    volMounts,
			},
		}
	}
	return m.newDefaultDeploymentNoInit(v1alpha1.SuggestionComponentType, "", oc, NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.SuggestionComponentType), v1alpha1.SuggestionComponentType), &oc.Spec.Suggestion.DeploymentSpec, cf)
}

func (m *suggestionManager) supportsReadOnlyService() bool {
	return false
}

func (m *suggestionManager) getReadonlyDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string, deployment *apps.Deployment) *apps.Deployment {
	return nil
}

func (m *suggestionManager) getMcclientSyncFunc(oc *v1alpha1.OnecloudCluster) func(*mcclient.ClientSession) error {
	return func(s *mcclient.ClientSession) error {
		if m.IsDisabled(oc) {
			return onecloud.EnsureDisableService(s, m.GetServiceName())
		} else {
			return onecloud.EnsureEnableService(s, m.GetServiceName(), m.supportsReadOnlyService() && oc.Spec.Suggestion.SlaveReplicas > 0)
		}
	}
}
