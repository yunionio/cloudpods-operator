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

package dameng

import (
	"database/sql"
	"fmt"

	_ "gitee.com/chunanyong/dm"

	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/pkg/errors"

	apis "yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/dbutil"
)

var (
	AllHosts = []string{"%", "127.0.0.1"}
)

type Connection struct {
	db       *sql.DB
	Host     string
	Port     int
	Username string
	Password string
}

func NewConnection(info *apis.Dameng) (dbutil.IConnection, error) {
	host := info.Host
	port := info.Port
	username := info.Username
	password := info.Password
	db, err := sql.Open("dm", fmt.Sprintf("dm://%s:%s@%s:%d/%s", username, password, host, port, "SYS"))
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

func (conn *Connection) IsDatabaseExists(db string) (bool, error) {
	// check schema existence
	var dbName string
	query := fmt.Sprintf(`SELECT DISTINCT object_name FROM all_objects WHERE object_type='SCH' AND object_name = '%s';`, db)
	err := conn.db.QueryRow(query).Scan(&dbName)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, errors.Wrapf(err, "do query %s", query)
	}
	if dbName == db {
		return true, nil
	}
	return false, nil
}

func (conn *Connection) IsUserExists(username string) (bool, error) {
	var count int
	q := fmt.Sprintf("SELECT COUNT(*) FROM dba_users WHERE username = '%s'", username)
	if err := conn.db.QueryRow(q).Scan(&count); err != nil {
		return false, errors.Wrapf(err, "check user %s exists: %s", username, q)
	}
	if count > 0 {
		return true, nil
	}
	return false, nil
}

func (conn *Connection) CreateUser(username string, password string, database string) error {
	exist, err := conn.IsUserExists(username)
	if err != nil {
		return errors.Wrap(err, "IsUserExists")
	}
	if exist {
		// ensure change password
		err := conn.alterUserPassword(username, password)
		if err != nil {
			return errors.Wrap(err, "alterUserPassword")
		}
	} else {
		// create user
		err := conn.createUserPassword(username, password)
		if err != nil {
			return errors.Wrap(err, "createUserPassword")
		}
	}
	// ensure user roles
	{
		err := conn.ensureUserRoles(username)
		if err != nil {
			return errors.Wrap(err, "ensureUserRoles")
		}
	}
	{
		// ensure user owners schema
		exist, err := conn.IsDatabaseExists(database)
		if err != nil {
			return errors.Wrap(err, "IsDatabaseExists")
		}
		if !exist {
			err := conn.createSchemaForUser(database, username)
			if err != nil {
				return errors.Wrap(err, "ensureUserRoles")
			}
		} else {
			owns, err := conn.isSchemaOwnerUser(database, username)
			if err != nil {
				return errors.Wrap(err, "isSchemaOwnerUser")
			}
			if !owns {
				return errors.Wrapf(httperrors.ErrConflict, "schema %s not belong to user %s", database, username)
			}
		}
	}

	return nil
}

func (conn *Connection) alterUserPassword(username string, password string) error {
	sql := fmt.Sprintf(`ALTER USER %s IDENTIFIED BY %s`, username, password)
	_, err := conn.db.Exec(sql)
	return errors.Wrapf(err, "exec %s", sql)
}

func (conn *Connection) createUserPassword(username string, password string) error {
	sql := fmt.Sprintf(`CREATE USER %s IDENTIFIED BY %s`, username, password)
	_, err := conn.db.Exec(sql)
	return errors.Wrapf(err, "exec %s", sql)
}

func (conn *Connection) ensureUserRoles(username string) error {
	sql := fmt.Sprintf(`GRANT resource,public TO %s`, username)
	_, err := conn.db.Exec(sql)
	return errors.Wrapf(err, "exec %s", sql)
}

func (conn *Connection) CreateDatabase(db string) error {
	return nil
}

func (conn *Connection) createSchemaForUser(db string, username string) error {
	sql := fmt.Sprintf("CREATE SCHEMA %s AUTHORIZATION %s", db, username)
	_, err := conn.db.Exec(sql)
	return errors.Wrapf(err, "exec %s", sql)
}

func (conn *Connection) isSchemaOwnerUser(db string, username string) (bool, error) {
	var count int
	q := fmt.Sprintf(`SELECT COUNT(*) FROM SYSOBJECTS A, DBA_USERS B WHERE A.PID=B.USER_ID AND A.TYPE$='SCH' AND B.USERNAME='%s' AND A.NAME='%s';`, username, db)
	if err := conn.db.QueryRow(q).Scan(&count); err != nil {
		return false, errors.Wrapf(err, "isSchemaOwnerUser %s %s %s", db, username, q)
	}
	if count > 0 {
		return true, nil
	}
	return false, nil
}

func (conn *Connection) Close() error {
	return conn.db.Close()
}
