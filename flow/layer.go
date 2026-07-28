package flow

import "context"

// Deprecated: pass context.Context to each business operation and use explicit
// constructors for initialization. Request context should not be stored on a
// reusable object.
type ILayer interface {
	SetCtx(context.Context)
	GetCtx() context.Context
	OnCreate()
}

// Deprecated: use explicit constructors and context-aware methods.
type Layer struct {
	ctx context.Context
}

func (layer *Layer) SetCtx(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	layer.ctx = ctx
}

func (layer *Layer) GetCtx() context.Context {
	if layer.ctx == nil {
		return context.Background()
	}
	return layer.ctx
}

func (layer *Layer) OnCreate() {}

// Factory makes dependencies explicit and avoids constructing zero-value
// service objects through reflection.
// Deprecated: call an explicit constructor instead.
type Factory[T ILayer] func() T

// Deprecated: call an explicit constructor and pass context to each operation.
func Create[T ILayer](ctx context.Context, factory Factory[T]) T {
	if factory == nil {
		panic("flow: nil layer factory")
	}
	layer := factory()
	layer.SetCtx(ctx)
	layer.OnCreate()
	return layer
}
