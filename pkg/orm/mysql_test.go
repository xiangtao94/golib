package orm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGORMKeepsDefaultWriteTransactionsUnlessExplicitlyDisabled(t *testing.T) {
	safe := gormConfig(MysqlConf{}, newLogger())
	optimized := gormConfig(MysqlConf{SkipDefaultTransaction: true}, newLogger())

	require.False(t, safe.SkipDefaultTransaction)
	require.True(t, optimized.SkipDefaultTransaction)
}
