package orm

import (
	"testing"

	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestMySQLRequiresSecureTransportByDefault(t *testing.T) {
	_, err := buildMySQLDSN(MysqlConf{Addr: "localhost:3306"})
	require.Error(t, err)
}

func TestMySQLDSNUsesDriverEscapingAndTLS(t *testing.T) {
	dsn, err := buildMySQLDSN(MysqlConf{
		Addr:          "db.example.com:3306",
		User:          "user",
		Password:      "p@ss:word",
		DataBase:      "app",
		TLSConfigName: "true",
	})
	require.NoError(t, err)
	parsed, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "p@ss:word", parsed.Passwd)
	require.Equal(t, "true", parsed.TLSConfig)
}
