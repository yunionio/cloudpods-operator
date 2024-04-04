package component

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/clickhouse"
	"yunion.io/x/onecloud-operator/pkg/util/dbutil"
	"yunion.io/x/onecloud-operator/pkg/util/mysql"
)

func getMysqlDBConnectionByCluster(oc *v1alpha1.OnecloudCluster) (dbutil.IConnection, error) {
	return mysql.NewConnection(&oc.Spec.Mysql)
}

func getClickhouseDBConnectionByCluster(oc *v1alpha1.OnecloudCluster) (dbutil.IConnection, error) {
	return clickhouse.NewConnection(&oc.Spec.Clickhouse)
}

func EnsureClusterDBUser(oc *v1alpha1.OnecloudCluster, dbConfig v1alpha1.DBConfig) error {
	return ensureClusterDBUser(oc, dbConfig, "mysql")
}

func EnsureClusterClickhouseUser(oc *v1alpha1.OnecloudCluster, dbConfig v1alpha1.DBConfig) error {
	return ensureClusterDBUser(oc, dbConfig, "clickhouse")
}

func ensureClusterDBUser(oc *v1alpha1.OnecloudCluster, dbConfig v1alpha1.DBConfig, driver string) error {
	dbName := dbConfig.Database
	username := dbConfig.Username
	password := dbConfig.Password
	var connfactory func(oc *v1alpha1.OnecloudCluster) (dbutil.IConnection, error)
	switch driver {
	case "mysql":
		connfactory = getMysqlDBConnectionByCluster
	case "clickhouse":
		connfactory = getClickhouseDBConnectionByCluster
	default:
		return fmt.Errorf("unknown db driver %s", driver)
	}
	conn, err := connfactory(oc)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := EnsureDBUser(conn, dbName, username, password); err != nil {
		return err
	}
	return nil
}

func EnsureDBUser(conn dbutil.IConnection, dbName string, username string, password string) error {
	dbExists, err := conn.IsDatabaseExists(dbName)
	if err != nil {
		return errors.Wrap(err, "check db exists")
	}
	if !dbExists {
		if err := conn.CreateDatabase(dbName); err != nil {
			return errors.Wrapf(err, "create database %q", dbName)
		}
	}
	userExists, err := conn.IsUserExists(username)
	if err != nil {
		return errors.Wrap(err, "check user exists")
	}
	if !userExists {
		if err := conn.CreateUser(username, password, dbName); err != nil {
			return errors.Wrapf(err, "create user %q for database %q", username, dbName)
		}
	} else if controller.SyncUser {
		if err := conn.UpdateUser(username, password, dbName); err != nil {
			return errors.Wrapf(err, "create user %q for database %q", username, dbName)
		}
	}
	return nil
}

func ParseSQLAchemyURL(pySQLSrc string) (*v1alpha1.DBConfig, error) {
	if len(pySQLSrc) == 0 {
		return nil, fmt.Errorf("Empty input")
	}

	r := regexp.MustCompile(`[/@:]+`)
	strs := r.Split(pySQLSrc, -1)
	if len(strs) != 6 {
		return nil, fmt.Errorf("Incorrect mysql connection url: %s", pySQLSrc)
	}
	// user, passwd, host, port, dburl := strs[1], strs[2], strs[3], strs[4], strs[5]
	user, passwd, _, _, dburl := strs[1], strs[2], strs[3], strs[4], strs[5]
	queryPos := strings.IndexByte(dburl, '?')
	if queryPos == 0 {
		return nil, fmt.Errorf("Missing database name")
	}
	var query url.Values
	var err error
	if queryPos > 0 {
		queryStr := dburl[queryPos+1:]
		if len(queryStr) > 0 {
			query, err = url.ParseQuery(queryStr)
			if err != nil {
				return nil, err
			}
		}
		dburl = dburl[:queryPos]
	} else {
		query = url.Values{}
	}
	query.Set("parseTime", "True")
	// ret = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", user, passwd, host, port, dburl, query.Encode())
	return &v1alpha1.DBConfig{
		Database: dburl,
		Username: user,
		Password: passwd,
	}, nil
}
