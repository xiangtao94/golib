package orm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/logger"

	"github.com/xiangtao94/golib/pkg/zlog"
)

func TestGORMKeepsDefaultWriteTransactionsUnlessExplicitlyDisabled(t *testing.T) {
	safe := gormConfig(MysqlConf{}, newLogger())
	optimized := gormConfig(MysqlConf{SkipDefaultTransaction: true}, newLogger())

	require.False(t, safe.SkipDefaultTransaction)
	require.True(t, optimized.SkipDefaultTransaction)
}

func TestORMLoggerSkipsSQLFormattingWhenDebugIsDisabled(t *testing.T) {
	_, err := zlog.InitLog(zlog.LogConfig{
		Level:  "info",
		Stdout: false,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, zlog.CloseLogger()) })

	formatterCalls := 0
	newLogger().Trace(context.Background(), time.Now(), func() (string, int64) {
		formatterCalls++
		return "SELECT 1", 1
	}, nil)

	require.Zero(t, formatterCalls)
}

func TestORMLoggerSilentModeSkipsErrorFormatting(t *testing.T) {
	formatterCalls := 0
	newLogger().LogMode(logger.Silent).Trace(
		context.Background(),
		time.Now(),
		func() (string, int64) {
			formatterCalls++
			return "SELECT 1", 1
		},
		errors.New("query failed"),
	)

	require.Zero(t, formatterCalls)
}

func TestORMLoggerErrorModeStillFormatsErrors(t *testing.T) {
	_, err := zlog.InitLog(zlog.LogConfig{
		Level:     "error",
		Stdout:    false,
		LogToFile: true,
		LogDir:    t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, zlog.CloseLogger()) })

	formatterCalls := 0
	newLogger().LogMode(logger.Error).Trace(
		context.Background(),
		time.Now(),
		func() (string, int64) {
			formatterCalls++
			return "SELECT 1", 1
		},
		errors.New("query failed"),
	)

	require.Equal(t, 1, formatterCalls)
}
