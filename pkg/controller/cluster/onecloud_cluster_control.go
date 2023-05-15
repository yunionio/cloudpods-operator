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
	if apiequality.Semantic.DeepEqual(&oc.Status, oldStatus) && !v1alpha1.ClearComponent {
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
		components.Influxdb(),
		components.Telegraf(),
		components.Region(),
		components.Scheduler(),
		components.Web(),
		components.KubeServer(),
		components.Glance(),
		components.RegionDNS(),
		components.Yunionagent(),
		components.AnsibleServer(),
		components.APIGateway(),
		components.Meter(),
		components.EsxiAgent(),
		components.OvnNorth(),
		components.APIMap(),
		components.VpcAgent(),
		components.HostDeployer(),
		components.HostImage(),
		components.Host(),
		components.HostHealth(),
		components.Monitor(),
		components.Cloudmon(),
	} {
		if err := component.Sync(oc); err != nil {
			if !controller.StopServices {
				return errors.Wrap(err, "sync component")
			} else {
				log.Warningf("Stop service error: %v", err)
			}
		}
	}

	var dependComponents = []manager.Manager{
		components.Logger(),
		components.Climc(),
		components.AutoUpdate(),
		components.Cloudnet(),
		components.Cloudproxy(),
		components.Cloudevent(),
		components.Devtool(),
		components.Webconsole(),
		components.Yunionconf(),
		components.S3gateway(),
		components.Notify(),
		components.Baremetal(),
		components.ServiceOperator(),
		components.Itsm(),
		components.CloudId(),
		components.Suggestion(),
		components.Scheduledtask(),
		components.Report(),
		components.Lbgent(),
		components.EChartsSSR(),
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

const (
	mysqlKeySlaveIORunning  = "Slave_IO_Running"
	mysqlKeySlaveSQLRunning = "Slave_SQL_Running"
	mysqlKeyLastError       = "Last_Error"
	mysqlKeyLastIOError     = "Last_IO_Error"
	mysqlKeyMasterHost      = "Master_Host"
	mysqlKeyIP              = "IP"
	mysqlKeyOperatorError   = "Operator_Error"
)

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

	masterHost := status[mysqlKeyMasterHost]
	if masterHost.String == "" {
		return nil
	}

	showSlaveStatus := func(masterHost string) (string, map[string]sql.NullString, error) {
		info := oc.Spec.Mysql.DeepCopy()
		info.Host = masterHost
		conn, err := mysql.NewConnection(info)
		if err != nil {
			return "", nil, errors.Wrapf(err, "mysql.NewConnection of %s", masterHost)
		}
		defer conn.Close()
		sqlConn := conn.(*mysql.Connection)
		status, err := sqlConn.ShowSlaveStatus()
		if err != nil {
			return "", nil, errors.Wrapf(err, "ShowSlaveStatus of %s", masterHost)
		}
		// notify slave status
		slaveIORunning := status[mysqlKeySlaveIORunning]
		slaveSQLRunning := status[mysqlKeySlaveSQLRunning]
		peerHost := status[mysqlKeyMasterHost].String
		if slaveIORunning.String == "Yes" && slaveSQLRunning.String == "Yes" {
			return peerHost, nil, nil
		}
		status[mysqlKeyIP] = sql.NullString{
			String: masterHost,
			Valid:  true,
		}
		return peerHost, status, nil
	}

	injectErr := func(ip string, status map[string]sql.NullString, err error) map[string]sql.NullString {
		if status == nil {
			status = make(map[string]sql.NullString)
		}
		status[mysqlKeyOperatorError] = sql.NullString{
			String: err.Error(),
			Valid:  true,
		}
		status[mysqlKeyIP] = sql.NullString{
			String: ip,
			Valid:  true,
		}
		return status
	}

	peerHost, status1, err := showSlaveStatus(masterHost.String)
	if err != nil {
		status1 = injectErr(masterHost.String, status1, err)
	}

	_, status2, err := showSlaveStatus(peerHost)
	if peerHost == "" {
		sqlHost := oc.Spec.Mysql.Host
		if err != nil {
			log.Errorf("showSlaveStatus of %q: %v, try get status from %q", peerHost, err, sqlHost)
		}
		_, status2, err = showSlaveStatus(sqlHost)
	}
	if err != nil {
		status2 = injectErr(peerHost, status1, err)
	}

	// notify slave status
	return occ.notifyMysqlError(oc, status1, status2)
}

func (occ *defaultClusterControl) notifyMysqlError(oc *v1alpha1.OnecloudCluster, status1, status2 map[string]sql.NullString) error {
	if status1 == nil && status2 == nil {
		return nil
	}
	return occ.components.RunWithSession(oc, func(s *mcclient.ClientSession) error {
		getStatus := func() []map[string]string {
			ret := make([]map[string]string, 0)
			for _, s := range []map[string]sql.NullString{status1, status2} {
				if s == nil {
					continue
				}
				if _, ok := s[mysqlKeyOperatorError]; ok {
					ret = append(ret, map[string]string{
						"ip":             s[mysqlKeyIP].String,
						"operator_error": s[mysqlKeyOperatorError].String,
					})
				} else {
					ret = append(ret, map[string]string{
						"ip":                s[mysqlKeyIP].String,
						"slave_io_running":  s[mysqlKeySlaveIORunning].String,
						"slave_sql_running": s[mysqlKeySlaveSQLRunning].String,
						"last_error":        s[mysqlKeyLastError].String,
						"last_io_error":     s[mysqlKeyLastIOError].String,
					})
				}
			}
			return ret
		}
		params := notify.NotificationManagerEventNotifyInput{
			Event: notify.Event.WithAction(notify.ActionMysqlOutOfSync).WithResourceType(notify.TOPIC_RESOURCE_DBINSTANCE).String(),
			ResourceDetails: jsonutils.Marshal(map[string]interface{}{
				"ip":     oc.Spec.Mysql.Host,
				"status": getStatus(),
			}).(*jsonutils.JSONDict),
		}
		_, err := npk.Notification.PerformClassAction(s, "event-notify", jsonutils.Marshal(params))
		return err
	})
}
