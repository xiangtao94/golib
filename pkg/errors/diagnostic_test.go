package errors_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	serviceerrors "github.com/xiangtao94/golib/pkg/errors"
)

func wrapDeliveryFailure() error {
	return serviceerrors.ErrInternal.Wrap(errors.New("smtp authentication failed"))
}

type cyclicError struct{}

func (cyclicError) Error() string     { return "cycle" }
func (err cyclicError) Unwrap() error { return err }

func TestDiagnosticIncludesJoinedPrivateCausesAndWrappingSite(t *testing.T) {
	cause := errors.New("redis cleanup failed")
	err := fmt.Errorf("request failed: %w", errors.Join(wrapDeliveryFailure(), serviceerrors.ErrInternal.Wrap(cause)))
	diagnostic := serviceerrors.Diagnostic(err)
	require.Contains(t, diagnostic, "smtp authentication failed")
	require.Contains(t, diagnostic, "redis cleanup failed")
	require.Contains(t, diagnostic, "errors_test.wrapDeliveryFailure")
	require.Contains(t, diagnostic, "diagnostic_test.go:")
	require.ErrorIs(t, err, cause)
	require.Equal(t, "internal server error", serviceerrors.ErrInternal.Wrap(cause).Error())
	require.Equal(t, "internal server error", serviceerrors.Diagnostic(serviceerrors.ErrInternal))
}

func TestDiagnosticHandlesNilAndBoundsCycles(t *testing.T) {
	require.Empty(t, serviceerrors.Diagnostic(nil))
	require.Contains(t, serviceerrors.Diagnostic(cyclicError{}), "traversal limit reached")
	var typedNil *serviceerrors.Error
	require.Empty(t, serviceerrors.Diagnostic(typedNil))
}
