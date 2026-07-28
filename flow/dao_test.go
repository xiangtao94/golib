package flow

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type daoModel struct{}

func (daoModel) TableName() string { return "dao_models" }

func TestDBRegistryIsInstanceScoped(t *testing.T) {
	firstDB := &gorm.DB{}
	secondDB := &gorm.DB{}
	first := NewDBRegistry(firstDB, map[string]*gorm.DB{"replica": secondDB})
	second := NewDBRegistry(secondDB, nil)

	require.Same(t, firstDB, first.Default())
	require.Same(t, secondDB, first.Get("replica"))
	require.Same(t, secondDB, second.Default())
	require.Nil(t, second.Get("replica"))
}

func TestDaoReadMasterStateIsPerInstance(t *testing.T) {
	first := NewDao(nil)
	second := NewDao(nil)
	first.SetReadDbMaster(true)

	require.True(t, first.GetReadDbMaster())
	require.False(t, second.GetReadDbMaster())
}

func TestCommonDaoReturnsConfigurationError(t *testing.T) {
	dao := CommonDao[daoModel]{Dao: NewDao(nil)}

	_, err := dao.GetById(1)

	require.ErrorIs(t, err, ErrDatabaseNotConfigured)
}
