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

package mysql

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"

	apis "yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/dbutil"
)

var (
	allHosts = []string{"%", "127.0.0.1"}
)

type Connection struct {
	db       *sql.DB
	Host     string
	Port     int
	Username string
	Password string
}

func NewConnection(info *apis.Mysql) (dbutil.IConnection, error) {
	host := info.Host
	port := info.Port
	username := info.Username
	password := info.Password
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/", username, password, host, port))
	if err != nil {
		return nil, errors.Wrap(err, "Connect to database")
	}
	return &Connection{
		db:       db,
		Host:     host,
		Port:     int(port),
		Username: username,
		Password: password,
	}, nil
}

func (conn *Connection) DB() *sql.DB {
	return conn.db
}

func (conn *Connection) CheckHealth() error {
	_, err := conn.db.Exec("SHOW MASTER STATUS")
	return err
}

func (conn *Connection) scanMap(rows *sql.Rows) (map[string]sql.NullString, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	if !rows.Next() {
		err = rows.Err()
		if err != nil {
			return nil, err
		} else {
			return nil, nil
		}
	}

	values := make([]interface{}, len(columns))
	for index := range values {
		values[index] = new(sql.NullString)
	}
	err = rows.Scan(values...)

	if err != nil {
		return nil, err
	}

	result := make(map[string]sql.NullString)

	for index, columnName := range columns {
		value := *values[index].(*sql.NullString)
		if value.Valid {
			result[columnName] = value
		}
	}

	return result, nil
}

func (conn *Connection) ShowSlaveStatus() (map[string]sql.NullString, error) {
	rows, err := conn.db.Query("SHOW SLAVE STATUS")
	if err != nil {
		return nil, errors.Wrap(err, "show slave status")
	}
	return conn.scanMap(rows)
}

func (conn *Connection) IsDatabaseExists(db string) (bool, error) {
	var dbName string
	err := conn.db.
		QueryRow("SELECT `SCHEMA_NAME` AS `database` FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", db).
		Scan(&dbName)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, errors.Wrap(err, "do query")
	}
	if dbName == db {
		return true, nil
	}
	return false, nil
}

func (conn *Connection) IsUserExists(username string) (bool, error) {
	return conn.isUserExists(username, "%")
}

func (conn *Connection) isUserExists(username string, host string) (bool, error) {
	q := fmt.Sprintf("SHOW GRANTS FOR '%s'", username)
	if len(host) != 0 {
		q = fmt.Sprintf("%s@'%s'", q, host)
	}
	_, err := conn.db.Exec(q)
	if err != nil {
		return false, errors.Wrapf(err, "exec %s", q)
	}
	return true, nil
}

func (conn *Connection) DropUserByHosts(username string, hosts []string) error {
	for _, host := range hosts {
		if err := conn.DropUserByHost(username, host); err != nil {
			return errors.Wrapf(err, "drop user %s@%s", username, host)
		}
	}
	return nil
}

func (conn *Connection) DropUser(username string) error {
	return conn.DropUserByHosts(username, allHosts)
}

func (conn *Connection) DropUserByHost(username string, address string) error {
	exists, err := conn.isUserExists(username, address)
	if err != nil {
		return errors.Wrapf(err, "drop user %s@%s check exists", username, address)
	}
	if !exists {
		return nil
	}
	userIdx := fmt.Sprintf("'%s'", username)
	if len(address) > 0 {
		userIdx = fmt.Sprintf("%s@'%s'", userIdx, address)
	}
	q := fmt.Sprintf("DROP USER %s", userIdx)
	_, err = conn.db.Exec(q)
	if err != nil {
		log.Errorf("drop user %s fail %s", username, err)
	}
	return nil
}

func (conn *Connection) Grant(username string, password string, database string, address string) error {
	if address == "" {
		address = "%"
	}
	if database == "" {
		database = "*"
	}
	{
		sql := fmt.Sprintf("CREATE USER '%s'@'%s' IDENTIFIED BY '%s'", username, address, password)
		_, err := conn.db.Exec(sql)
		if err != nil {
			return errors.Wrap(err, sql)
		}
	}
	{
		sql := fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'@'%s'", database, username, address)
		_, err := conn.db.Exec(sql)
		if err != nil {
			return errors.Wrap(err, sql)
		}
	}
	return nil
}

func (conn *Connection) CreateUser(username string, password string, database string) error {
	return conn.UpdateUser(username, password, database)
}

func (conn *Connection) UpdateUser(username string, password string, database string) error {
	if database == "" {
		database = "*"
	}
	addrs := allHosts
	if err := conn.DropUser(username); err != nil {
		return errors.Wrapf(err, "Delete user %s", username)
	}
	for _, addr := range addrs {
		if err := conn.Grant(username, password, database, addr); err != nil {
			return errors.Wrapf(err, "Grant user %s@%s to database %s", username, addr, database)
		}
	}
	return nil
}

func (conn *Connection) CreateDatabase(db string) error {
	_, err := conn.db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", db))
	return err
}

func (conn *Connection) DropDatabase(db string) error {
	_, err := conn.db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", db))
	return err
}

func (conn *Connection) IsGrantPrivUser(user string, host string) (bool, error) {
	grant, err := conn.ShowGrants(user, host)
	if err != nil {
		return false, err
	}
	if strings.Contains(grant, "WITH GRANT OPTION") {
		return true, nil
	}
	return false, nil
}

func (conn *Connection) ShowGrants(user string, host string) (string, error) {
	if host == "" {
		host = "%"
	}
	q := fmt.Sprintf("SHOW GRANTS FOR '%s'@'%s'", user, host)
	var ret string
	if err := conn.db.QueryRow(q).Scan(&ret); err != nil {
		return "", errors.Wrapf(err, "show grants for %s@%s", user, host)
	}
	return ret, nil

}

func (conn *Connection) Close() error {
	return conn.db.Close()
}
