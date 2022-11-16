package clickhouse

import (
	"crypto/sha256"
	"database/sql"
	"fmt"

	_ "github.com/ClickHouse/clickhouse-go"

	"yunion.io/x/pkg/errors"

	apis "yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/dbutil"
)

type Connection struct {
	db       *sql.DB
	Host     string
	Port     int
	Username string
	Password string
}

func NewConnection(info *apis.Clickhouse) (dbutil.IConnection, error) {
	host := info.Host
	port := info.Port
	username := info.Username
	password := info.Password

	connStr := fmt.Sprintf("tcp://%s:%d?read_timeout=10&write_timeout=20&username=%s&password=%s", host, port, username, password)
	db, err := sql.Open("clickhouse", connStr)
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

func (conn *Connection) IsDatabaseExists(db string) (bool, error) {
	// always return false, so call create database if not exists
	return false, nil
}

func (conn *Connection) IsUserExists(username string) (bool, error) {
	var count int
	q := fmt.Sprintf("SELECT COUNT(*) FROM system.users WHERE name = '%s'", username)
	if err := conn.db.QueryRow(q).Scan(&count); err != nil {
		return false, errors.Wrapf(err, "check user %s exists", username)
	}
	if count > 0 {
		return true, nil
	}
	return false, nil
}

func (conn *Connection) dropUser(username string) error {
	_, err := conn.db.Exec(fmt.Sprintf("DROP USER IF EXISTS `%s`", username))
	return err
}

func sha256hash(str string) string {
	sum := sha256.Sum256([]byte(str))
	return fmt.Sprintf("%x", sum)
}

func (conn *Connection) CreateUser(username string, password string, database string) error {
	if err := conn.dropUser(username); err != nil {
		return errors.Wrapf(err, "Delete user %s", username)
	}
	passhash := sha256hash(password)
	for _, sql := range []string{
		fmt.Sprintf("CREATE USER %s IDENTIFIED WITH sha256_hash BY '%s' HOST ANY", username, passhash),
		fmt.Sprintf("GRANT ALL ON %s.* TO %s", database, username),
	} {
		_, err := conn.db.Exec(sql)
		if err != nil {
			return errors.Wrapf(err, "exec %s", sql)
		}
	}

	return nil
}

func (conn *Connection) CreateDatabase(db string) error {
	_, err := conn.db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", db))
	return err
}

func (conn *Connection) Close() error {
	return conn.db.Close()
}
