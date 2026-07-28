package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testConfig struct {
	Database struct {
		Host    string        `mapstructure:"host"`
		Timeout time.Duration `mapstructure:"timeout"`
	} `mapstructure:"database"`
	StartsAt time.Time `mapstructure:"starts_at"`
}

func TestLoadReadsEnvironmentOnlyFields(t *testing.T) {
	t.Setenv("CONFIG_TEST_DATABASE_HOST", "db.internal")
	t.Setenv("CONFIG_TEST_DATABASE_TIMEOUT", "3s")
	t.Setenv("CONFIG_TEST_STARTS_AT", "2026-07-28T09:30:00+08:00")

	got, err := Load[testConfig](Options{EnvPrefix: "CONFIG_TEST"}, nil)

	require.NoError(t, err)
	require.Equal(t, "db.internal", got.Database.Host)
	require.Equal(t, 3*time.Second, got.Database.Timeout)
	require.True(
		t,
		got.StartsAt.Equal(
			time.Date(2026, time.July, 28, 9, 30, 0, 0, time.FixedZone("", 8*60*60)),
		),
	)
}

func TestLoadAppliesDefaultsFileAndEnvironmentInOrder(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "app.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(`
database:
  host: file-host
  timeout: 2s
`), 0o600))
	t.Setenv("CONFIG_PRIORITY_DATABASE_HOST", "env-host")

	got, err := Load[testConfig](Options{
		File:      configFile,
		EnvPrefix: "CONFIG_PRIORITY",
		Defaults: map[string]any{
			"database.host":    "default-host",
			"database.timeout": "1s",
		},
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "env-host", got.Database.Host)
	require.Equal(t, 2*time.Second, got.Database.Timeout)
}

func TestLoadRequiresDeclaredFileByDefault(t *testing.T) {
	_, err := Load[testConfig](Options{
		File: filepath.Join(t.TempDir(), "missing.yaml"),
	}, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "read config")
}

func TestLoadAllowsExplicitlyOptionalFile(t *testing.T) {
	got, err := Load[testConfig](Options{
		File:         filepath.Join(t.TempDir(), "missing.yaml"),
		OptionalFile: true,
		Defaults:     map[string]any{"database.host": "default-host"},
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "default-host", got.Database.Host)
}

func TestLoadRejectsUnknownFileKeys(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "app.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(`
database:
  host: db.internal
unknown: true
`), 0o600))

	_, err := Load[testConfig](Options{File: configFile}, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "decode config")
}

func TestLoadRunsValidationAfterDecode(t *testing.T) {
	validationErr := errors.New("database.host is required")

	_, err := Load[testConfig](Options{}, func(got testConfig) error {
		require.Empty(t, got.Database.Host)
		return validationErr
	})

	require.ErrorIs(t, err, validationErr)
	require.Contains(t, err.Error(), "validate config")
}
