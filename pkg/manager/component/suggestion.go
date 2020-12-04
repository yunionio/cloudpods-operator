package component

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"path"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud/pkg/monitor/options"
)

type suggestionManager struct {
	*ComponentManager
}

func newSuggestionManager(man *ComponentManager) manager.Manager {
	return &suggestionManager{man}
}

func (m *suggestionManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	if !IsEnterpriseEdition(oc) {
		return nil
	}
	return syncComponent(m, oc, oc.Spec.Suggestion.Disable)
}

func (m *suggestionManager) getDBConfig(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig {
	return &cfg.Monitor.DB
}

func (m *suggestionManager) getCloudUser(cfg *v1alpha1.OnecloudClusterConfig) *v1alpha1.CloudUser {
	return &cfg.Monitor.CloudUser
}

func (m *suggestionManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl {
	return controller.NewRegisterEndpointComponent(
		man, v1alpha1.SuggestionComponentType,
		constants.ServiceNameSuggestion, constants.ServiceTypeSuggestion,
		constants.SuggestionPort, "")
}

func (m *suggestionManager) getService(oc *v1alpha1.OnecloudCluster) []*corev1.Service {
	return []*corev1.Service{m.newSingleNodePortService(v1alpha1.SuggestionComponentType, oc, constants.SuggestionPort)}
}

func (m *suggestionManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error) {
	opt := &options.Options
	if err := SetOptionsDefault(opt, constants.ServiceTypeSuggestion); err != nil {
		return nil, err
	}
	config := cfg.Monitor
	SetDBOptions(&opt.DBOptions, oc.Spec.Mysql, config.DB)
	SetOptionsServiceTLS(&opt.BaseOptions)
	SetServiceCommonOptions(&opt.CommonOptions, oc, config.ServiceCommonOptions)
	opt.AutoSyncTable = true
	opt.SslCertfile = path.Join(constants.CertDir, constants.ServiceCertName)
	opt.SslKeyfile = path.Join(constants.CertDir, constants.ServiceKeyName)
	opt.Port = constants.SuggestionPort
	return m.newServiceConfigMap(v1alpha1.SuggestionComponentType, oc, opt), nil
}

func (m *suggestionManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error) {
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
	return m.newDefaultDeploymentNoInit(
		v1alpha1.SuggestionComponentType, oc,
		NewVolumeHelper(oc, controller.ComponentConfigMapName(oc, v1alpha1.SuggestionComponentType), v1alpha1.SuggestionComponentType),
		oc.Spec.Suggestion, cf)
}
