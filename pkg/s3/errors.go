package s3

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

var (
	// ErrNilContext identifies an invalid nil context.
	ErrNilContext = errors.New("s3: nil context")
	// ErrNotFound identifies a missing bucket or object.
	ErrNotFound = errors.New("s3: not found")
	// ErrAccessDenied identifies an authorization failure.
	ErrAccessDenied = errors.New("s3: access denied")
	// ErrPreconditionFailed identifies a failed conditional request.
	ErrPreconditionFailed = errors.New("s3: precondition failed")
)

// OperationError records which provider-neutral operation failed while
// retaining the original SDK error for diagnostics.
type OperationError struct {
	Operation string
	Bucket    string
	Key       string
	Kind      error
	Err       error
}

func (err *OperationError) Error() string {
	target := err.Bucket
	if err.Key != "" {
		target += "/" + err.Key
	}
	return fmt.Sprintf("s3: %s %s: %v", err.Operation, target, err.Err)
}

func (err *OperationError) Unwrap() error {
	return err.Err
}

func (err *OperationError) Is(target error) bool {
	return target == err.Kind || errors.Is(err.Err, target)
}

func wrapOperationError(operation string, bucket string, key string, err error) error {
	if err == nil {
		return nil
	}
	return &OperationError{
		Operation: operation,
		Bucket:    bucket,
		Key:       key,
		Kind:      classifyError(err),
		Err:       err,
	}
}

func classifyError(err error) error {
	var responseError *smithyhttp.ResponseError
	if errors.As(err, &responseError) {
		switch responseError.HTTPStatusCode() {
		case http.StatusNotFound:
			return ErrNotFound
		case http.StatusForbidden:
			return ErrAccessDenied
		case http.StatusPreconditionFailed:
			return ErrPreconditionFailed
		}
	}

	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch apiError.ErrorCode() {
		case "NoSuchBucket", "NoSuchKey", "NotFound", "404":
			return ErrNotFound
		case "AccessDenied", "Forbidden", "403":
			return ErrAccessDenied
		case "PreconditionFailed", "ConditionalRequestConflict", "412":
			return ErrPreconditionFailed
		}
	}
	return nil
}

func errorCode(err error) string {
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		return apiError.ErrorCode()
	}
	return ""
}
