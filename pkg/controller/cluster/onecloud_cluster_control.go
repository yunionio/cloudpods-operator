package cluster

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/apis/notify"
	"yunion.io/x/onecloud/pkg/mcclient"
	npk "yunion.io/x/onecloud/pkg/mcclient/modules/notify"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/manager/certs"
	"yunion.io/x/onecloud-operator/pkg/manager/component"
	"yunion.io/x/onecloud-operator/pkg/manager/config"
	"yunion.io/x/onecloud-operator/pkg/util/mysql"
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
		lastCheckMysqlTime:   time.Now(),
	}
}

type defaultClusterControl struct {
	ocControl            controller.ClusterControlInterface
	clusterConfigManager *config.ConfigManager
	clusterCertsManager  *certs.CertsManager
	components           *component.ComponentManager
	recorder             record.EventRecorder
	lastCheckMysqlTime   time.Time
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
			if !controller.StopServices {
				return err
			} else {
				log.Warningf("Stop service error: %v", err)
			}
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
		components.Scheduledtask(),
		components.Report(),
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

	specJson, err := json.Marshal(oc.Spec)
	if err != nil {
		return errors.Wrap(err, "Marshal oc.Spec")
	}
	oc.Status.SpecChecksum = fmt.Sprintf("%x", sha256.Sum256(specJson))

	if time.Since(occ.lastCheckMysqlTime) > time.Duration(controller.MysqlCheckInterval)*time.Minute {
		if err := occ.checkMysqlSlaveStatus(oc); err != nil {
			log.Warningf("checkMysqlSlaveStatus error: %v", err)
		}
		occ.lastCheckMysqlTime = time.Now()
	}

	return nil
}

func (occ *defaultClusterControl) checkMysqlSlaveStatus(oc *v1alpha1.OnecloudCluster) error {
	dbConf := oc.Spec.Mysql
	if dbConf.Host == "" || dbConf.Username == "" || dbConf.Password == "" {
		return nil
	}
	conn, err := mysql.NewConnection(&oc.Spec.Mysql)
	if err != nil {
		return errors.Wrap(err, "mysql.NewConnection")
	}
	defer conn.Close()
	sqlConn := conn.(*mysql.Connection)
	status, err := sqlConn.ShowSlaveStatus()
	if err != nil {
		return errors.Wrap(err, "ShowSlaveStatus")
	}
	if len(status) == 0 {
		return nil
	}

	// notify slave status
	slaveIORunning := status["Slave_IO_Running"]
	slaveSQLRunning := status["Slave_SQL_Running"]
	if slaveIORunning.String == "Yes" && slaveSQLRunning.String == "Yes" {
		return nil
	}
	return occ.notifyMysqlError(oc, status)
}

func (occ *defaultClusterControl) notifyMysqlError(oc *v1alpha1.OnecloudCluster, status map[string]sql.NullString) error {
	return occ.components.RunWithSession(oc, func(s *mcclient.ClientSession) error {
		params := notify.NotificationManagerEventNotifyInput{
			Event: notify.Event.WithAction(notify.ActionMysqlOutOfSync).WithResourceType(notify.TOPIC_RESOURCE_DBINSTANCE).String(),
			ResourceDetails: jsonutils.Marshal(map[string]interface{}{
				"ip":                oc.Spec.Mysql.Host,
				"slave_io_running":  status["Slave_IO_Running"].String,
				"slave_sql_running": status["Slave_SQL_Running"].String,
				"last_error":        status["Last_Error"].String,
			}).(*jsonutils.JSONDict),
		}
		_, err := npk.Notification.PerformClassAction(s, "event-notify", jsonutils.Marshal(params))
		return err
	})
}
