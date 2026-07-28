package zlog

import (
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
