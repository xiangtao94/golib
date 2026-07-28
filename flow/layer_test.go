package flow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type testLayer struct {
	Layer
	dependency string
	created    bool
}

func (layer *testLayer) OnCreate() {
	layer.created = true
}

func TestCreateUsesFactoryAndPreservesDependencies(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "value")

	layer := Create(ctx, func() *testLayer {
		return &testLayer{dependency: "injected"}
	})

	require.Equal(t, "injected", layer.dependency)
	require.True(t, layer.created)
	require.Equal(t, "value", layer.GetCtx().Value(contextKey{}))
}
