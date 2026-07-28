package orm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGORMKeepsDefaultWriteTransactionsUnlessExplicitlyDisabled(t *testing.T) {
	safe := newGORMConfig(MysqlConf{})
	optimized := newGORMConfig(MysqlConf{SkipDefaultTransaction: true})

	require.False(t, safe.SkipDefaultTransaction)
	require.True(t, optimized.SkipDefaultTransaction)
}
