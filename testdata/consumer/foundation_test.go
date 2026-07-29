package consumer

import (
	"context"
	"net/http"
	"testing"

	"github.com/xiangtao94/golib/pkg/config"
	serviceerrors "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/health"
	"github.com/xiangtao94/golib/pkg/httpclient"
	"github.com/xiangtao94/golib/pkg/lifecycle"
)

type applicationConfig struct {
	Name string `mapstructure:"name"`
}

func TestFoundationCanBeImportedWithoutWorkspace(t *testing.T) {
	loaded, err := config.Load[applicationConfig](config.Options{
		Defaults: map[string]any{"name": "consumer"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "consumer" {
		t.Fatalf("name = %q, want consumer", loaded.Name)
	}

	manager, err := lifecycle.New(lifecycle.Hook{
		Name:   "resource",
		OnStop: func(context.Context) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	if health.New().Ready() {
		t.Fatal("new health gate must start not ready")
	}
	if status := serviceerrors.ErrConflict.HTTPStatus(); status != http.StatusConflict {
		t.Fatalf("conflict status = %d", status)
	}
	if retries := httpclient.DefaultConfig().RetryCount; retries != 0 {
		t.Fatalf("default retries = %d", retries)
	}
}
