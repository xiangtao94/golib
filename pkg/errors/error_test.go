package errors

import (
	stderrors "errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorKeepsPublicContractSeparateFromCause(t *testing.T) {
	cause := stderrors.New("mysql password leaked in driver error")
	public := New("USER_NOT_FOUND", "user not found", http.StatusNotFound).
		WithReason("NOT_FOUND").
		Wrap(cause)

	require.Equal(t, "user not found", public.Error())
	require.Equal(t, "USER_NOT_FOUND", public.Code())
	require.Equal(t, "NOT_FOUND", public.Reason())
	require.Equal(t, http.StatusNotFound, public.HTTPStatus())
	require.ErrorIs(t, public, cause)
	require.NotContains(t, public.Error(), "mysql")
}

func TestErrorOptionsReturnImmutableCopies(t *testing.T) {
	input := map[string]any{"field": "email"}
	base := New("INVALID_ARGUMENT", "invalid request", http.StatusBadRequest)
	configured := base.WithRetryable(true).WithDetails(input)
	input["field"] = "changed"
	returned := configured.Details()
	returned["field"] = "mutated"

	require.False(t, base.Retryable())
	require.Empty(t, base.Details())
	require.True(t, configured.Retryable())
	require.Equal(t, map[string]any{"field": "email"}, configured.Details())
}

func TestNewHTTPStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		status []int
		want   int
	}{
		{"omitted", nil, http.StatusInternalServerError},
		{"explicit internal", []int{500}, 500},
		{"bad request", []int{400}, 400},
		{"not found", []int{404}, 404},
		{"upper boundary", []int{599}, 599},
		{"zero", []int{0}, 500},
		{"negative", []int{-1}, 500},
		{"success", []int{200}, 500},
		{"below boundary", []int{399}, 500},
		{"above boundary", []int{600}, 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			public := New(" CUSTOM ", "public message", test.status...)
			require.Equal(t, test.want, public.HTTPStatus())
			require.Equal(t, "CUSTOM", public.Code())
			require.Equal(t, "CUSTOM", public.Reason())
			require.Equal(t, "public message", public.Message())
		})
	}
	require.Equal(t, "INTERNAL", New(" ", "internal error").Code())
	require.Panics(t, func() { New("CUSTOM", "message", 400, 404) })
}

func TestFromReturnsPublicErrorOrSafeInternalFallback(t *testing.T) {
	public := New("CONFLICT", "already exists", http.StatusConflict)

	require.Same(t, public, From(public))
	require.Equal(t, ErrInternal.Code(), From(stderrors.New("private")).Code())
	require.Equal(t, ErrInternal.Message(), From(stderrors.New("private")).Message())
}
