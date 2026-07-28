package orm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalPageOrderClauseAllowsOnlyMappedFields(t *testing.T) {
	allowed := map[string]string{
		"id":        "id",
		"createdAt": "created_at",
	}

	t.Run("mapped field", func(t *testing.T) {
		page := &NormalPage{SortBy: "createdAt", SortDirection: SortDescending}

		order := page.orderClause(allowed)

		require.Equal(t, "created_at", order.Column.Name)
		require.True(t, order.Desc)
	})

	t.Run("unknown field falls back to id", func(t *testing.T) {
		page := &NormalPage{
			SortBy:        "id desc; DROP TABLE users",
			SortDirection: SortDescending,
		}

		order := page.orderClause(allowed)

		require.Equal(t, "id", order.Column.Name)
		require.True(t, order.Desc)
	})

	t.Run("invalid direction falls back to ascending", func(t *testing.T) {
		page := &NormalPage{SortBy: "id", SortDirection: "desc; DROP TABLE users"}

		order := page.orderClause(allowed)

		require.Equal(t, "id", order.Column.Name)
		require.False(t, order.Desc)
	})
}

func TestGORMKeepsDefaultWriteTransactionsUnlessExplicitlyDisabled(t *testing.T) {
	safe := newGORMConfig(MysqlConf{})
	optimized := newGORMConfig(MysqlConf{SkipDefaultTransaction: true})

	require.False(t, safe.SkipDefaultTransaction)
	require.True(t, optimized.SkipDefaultTransaction)
}
