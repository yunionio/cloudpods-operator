package dbutil

type IConnection interface {
	IsDatabaseExists(db string) (bool, error)
	IsUserExists(username string) (bool, error)
	CreateUser(username string, password string, database string) error
	UpdateUser(username string, password string, database string) error
	CreateDatabase(db string) error
	Close() error
}
