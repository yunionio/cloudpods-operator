package dbutil

import (
	"fmt"
	"net"
)

// FormatHost formats a host address for database connection strings
// IPv6 addresses need to be enclosed in square brackets
// This is a general purpose function that can be used by all database drivers
func FormatHost(host string) string {
	// Check if this is an IPv6 address
	ip := net.ParseIP(host)
	if ip != nil && ip.To4() == nil {
		// This is an IPv6 address, wrap it in brackets
		return fmt.Sprintf("[%s]", host)
	}
	// IPv4 address or hostname, use as is
	return host
}

type IConnection interface {
	IsDatabaseExists(db string) (bool, error)
	IsUserExists(username string) (bool, error)
	CreateUser(username string, password string, database string) error
	UpdateUser(username string, password string, database string) error
	CreateDatabase(db string) error
	Close() error
}
