package zlog

import (
	"testing"

	"github.com/stretchr/testify/require"
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
