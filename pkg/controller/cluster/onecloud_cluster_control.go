package cluster

import (
	"golang.org/x/sync/errgroup"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"

	"yunion.io/x/log"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
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
	if err := components.Etcd().Sync(oc); err != nil {
		return err
	}

	for _, component := range []manager.Manager{
		components.Keystone(),
		components.Region(),
		components.Scheduler(),
		components.KubeServer(),
		components.Glance(),
		components.RegionDNS(),
		components.Yunionagent(),
		components.AnsibleServer(),
		components.APIGateway(),
		components.Web(),
		components.Meter(),
		components.EsxiAgent(),
		components.OvnNorth(),
	} {
		if err := component.Sync(oc); err != nil {
			return err
		}
	}

	var dependComponents = []manager.Manager{
		components.Logger(),
		components.Influxdb(),
		components.Climc(),
		components.AutoUpdate(),
		components.Cloudnet(),
		components.Cloudproxy(),
		components.Cloudevent(),
		components.Devtool(),
		components.Webconsole(),
		components.Yunionconf(),
		components.Monitor(),
		components.S3gateway(),
		components.Notify(),
		components.Host(),
		components.HostDeployer(),
		components.HostImage(),
		components.VpcAgent(),
		components.Baremetal(),
		components.ServiceOperator(),
		components.Itsm(),
		components.Telegraf(),
		components.CloudId(),
		components.Cloudmon(),
		components.Suggestion(),
		components.Repo(),
	}
	var grp errgroup.Group
	for _, component := range dependComponents {
		c := component
		grp.Go(func() error {
			return c.Sync(oc)
		})
	}
	if err := grp.Wait(); err != nil {
		return err
	}

	var addonComponents = []manager.Manager{
		components.MonitorStack(),
	}
	// addon components' error will be ignore, only use log.Warningf
	for _, c := range addonComponents {
		if err := c.Sync(oc); err != nil {
			log.Warningf("Sync addons %v error: %v", c, err)
		}
	}

	return nil
}
