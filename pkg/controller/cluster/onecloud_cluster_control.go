package cluster

import (
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"yunion.io/x/onecloud-operator/pkg/manager"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager/certs"
	"yunion.io/x/onecloud-operator/pkg/manager/component"
	"yunion.io/x/onecloud-operator/pkg/manager/config"
)

// ControlInterface implements the control logic for updating OnecloudClusters and their children Deployments or StatefulSets.
type ControlInterface interface {
	// UpdateOnecloudCluster implements the control logic for resource creation, update, and deletion
	UpdateOnecloudCluster(cluster *v1alpha1.OnecloudCluster) error
}

// NewDefaultOnecloudClusterControl returns a new instance of the default implementation ControlInterface that
// implements for OnecloudClusters.
func NewDefaultOnecloudClusterControl(
	ocControl controller.ClusterControlInterface,
	configManager *config.ConfigManager,
	certsManager *certs.CertsManager,
	componentManager *component.ComponentManager,
	recorder record.EventRecorder,
) ControlInterface {
	return &defaultClusterControl{
		ocControl:            ocControl,
		clusterConfigManager: configManager,
		clusterCertsManager:  certsManager,
		components:           componentManager,
		recorder:             recorder,
	}
}

type defaultClusterControl struct {
	ocControl            controller.ClusterControlInterface
	clusterConfigManager *config.ConfigManager
	clusterCertsManager  *certs.CertsManager
	components           *component.ComponentManager
	recorder             record.EventRecorder
}

// UpdateOnecloudCluster executes the core logic loop  for a onecloud cluster
func (occ *defaultClusterControl) UpdateOnecloudCluster(oc *v1alpha1.OnecloudCluster) error {
	var errs []error
	oldStatus := oc.Status.DeepCopy()

	if err := occ.updateOnecloudCluster(oc); err != nil {
		errs = append(errs, err)
	}
	if apiequality.Semantic.DeepEqual(&oc.Status, oldStatus) {
		return errorutils.NewAggregate(errs)
	}
	if _, err := occ.ocControl.UpdateCluster(oc.DeepCopy(), &oc.Status, oldStatus); err != nil {
		errs = append(errs, err)
	}

	return errorutils.NewAggregate(errs)
}

func (occ *defaultClusterControl) updateOnecloudCluster(oc *v1alpha1.OnecloudCluster) error {
	// syncing global cluster configuration
	if _, err := occ.clusterConfigManager.CreateOrUpdate(oc); err != nil {
		return err
	}

	// syncing cluster certs
	if err := occ.clusterCertsManager.CreateOrUpdate(oc); err != nil {
		return err
	}

	// syncing all PVs managed by operator's reclaim policy to Retain
	/*	if err := occ.reclaimPolicyManager.Sync(oc); err != nil {
		return err
	}*/

	// cleaning all orphan pods which don't have a related PVc managed
	/*	if err := occ.orphanPodsCleaner.Clean(oc); err != nil {
		return err
	}*/

	components := occ.components

	for _, component := range []manager.Manager{
		components.Keystone(),
		components.Logger(),
		components.Climc(),
		components.Influxdb(),
		components.Region(),
		components.Scheduler(),
		components.Glance(),
		components.Webconsole(),
		components.Yunionagent(),
		components.Yunionconf(),
		components.KubeServer(),
		components.AnsibleServer(),
		components.Cloudnet(),
		components.Cloudevent(),
		components.APIGateway(),
		components.Web(),
		components.Notify(),
		components.Host(),
		components.HostDeployer(),
		components.Baremetal(),
	} {
		if err := component.Sync(oc); err != nil {
			return err
		}
	}

	return nil
}
