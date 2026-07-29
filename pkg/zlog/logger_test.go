package zlog

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestAppendLogFileTail(t *testing.T) {
	tests := map[string]string{
		txtLogNormal:    "app.log",
		txtLogWarnFatal: "app.log.wf",
		txtLogAccess:    "app.log.access",
		"unknown":       "app.log",
	}

	for loggerType, expected := range tests {
		t.Run(loggerType, func(t *testing.T) {
			require.Equal(t, expected, appendLogFileTail("app", loggerType))
		})
	}
}

func TestInitLogReconfiguresLoggersCreatedBeforeInitialization(t *testing.T) {
	_ = GetGlobalLogger()

	logger, err := InitLog(LogConfig{
		Level:  "error",
		Stdout: true,
	})
	require.NoError(t, err)

	require.False(t, logger.Desugar().Core().Enabled(zapcore.InfoLevel))
	require.True(t, logger.Desugar().Core().Enabled(zapcore.ErrorLevel))
}

func TestCloseLoggerStopsBufferedWritersAndIsIdempotent(t *testing.T) {
	_, err := InitLog(LogConfig{
		Stdout:    false,
		LogToFile: true,
		LogDir:    t.TempDir(),
		Buffer: Buffer{
			Enabled:       true,
			Size:          1024,
			FlushInterval: time.Hour,
		},
	})
	require.NoError(t, err)
	_ = GetAccessLogger()
	require.NotEmpty(t, bufferedWriters)

	require.NoError(t, CloseLogger())
	require.Empty(t, bufferedWriters)
	require.NoError(t, CloseLogger())
}

func TestInitLogRejectsInvalidConfigWithoutReplacingCurrentLogger(t *testing.T) {
	current, err := InitLog(LogConfig{
		Level:  "error",
		Stdout: true,
	})
	require.NoError(t, err)

	replacement, err := InitLog(LogConfig{
		Format: "xml",
	})

	require.Error(t, err)
	require.Nil(t, replacement)
	require.Same(t, current, GetGlobalLogger())
	require.True(t, current.Desugar().Core().Enabled(zapcore.ErrorLevel))
	require.False(t, current.Desugar().Core().Enabled(zapcore.InfoLevel))
}

func TestDisabledDebugLoggerDoesNotAllocateContextFields(t *testing.T) {
	_, err := InitLog(LogConfig{
		Level:  "error",
		Stdout: false,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, CloseLogger()) })
	ctx := context.Background()
	DebugLogger(ctx, "warm cache")

	allocations := testing.AllocsPerRun(1_000, func() {
		DebugLogger(ctx, "disabled")
	})

	require.Zero(t, allocations)
}

func TestDisabledSugaredAndAccessLoggersDoNotAllocateContextFields(t *testing.T) {
	_, err := InitLog(LogConfig{
		Level:  "error",
		Stdout: false,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, CloseLogger()) })
	ctx := context.Background()
	Debugf(ctx, "warm cache")
	AccessInfo(ctx)

	debugAllocations := testing.AllocsPerRun(1_000, func() {
		Debugf(ctx, "disabled")
	})
	accessAllocations := testing.AllocsPerRun(1_000, func() {
		AccessInfo(ctx)
	})

	require.Zero(t, debugAllocations)
	require.Zero(t, accessAllocations)
}

func TestPanicLoggerStillPanicsWhenPanicLevelIsDisabled(t *testing.T) {
	_, err := InitLog(LogConfig{
		Level:  "fatal",
		Stdout: false,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, CloseLogger()) })

	require.Panics(t, func() {
		PanicLogger(context.Background(), "panic contract")
	})
	require.Panics(t, func() {
		Panic(context.Background(), "panic contract")
	})
}

func TestLoggerCacheOnlyRetainsCommonCallerSkips(t *testing.T) {
	_, err := InitLog(LogConfig{
		Level:  "error",
		Stdout: false,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, CloseLogger()) })

	for skip := 0; skip < 1_000; skip++ {
		_ = NewLoggerWithSkip(skip)
	}

	require.LessOrEqual(t, len(zapLoggerCache), maxCachedCallerSkip+1)
}
