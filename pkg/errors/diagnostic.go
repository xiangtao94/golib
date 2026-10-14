package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// Diagnostic expands private causes and wrapping stacks for server logs only.
// Never use it as a client-facing message. It handles both ordinary wrapping
// and errors.Join, and bounds traversal to tolerate cyclic error chains.
func Diagnostic(err error) string {
	var out strings.Builder
	remaining := 64
	var visit func(error)
	visit = func(current error) {
		if current == nil {
			return
		}
		if remaining == 0 {
			return
		}
		remaining--
		if out.Len() > 0 {
			out.WriteString("\ncaused by: ")
		}
		out.WriteString(current.Error())
		if public, ok := current.(*Error); ok && public != nil && len(public.stack) > 0 {
			out.WriteString("\nwrapped at:")
			frames := runtime.CallersFrames(public.stack)
			for {
				frame, more := frames.Next()
				fmt.Fprintf(&out, "\n%s\n\t%s:%d", frame.Function, frame.File, frame.Line)
				if !more {
					break
				}
			}
		}
		switch wrapped := current.(type) {
		case interface{ Unwrap() []error }:
			for _, child := range wrapped.Unwrap() {
				if remaining == 0 {
					break
				}
				visit(child)
			}
		case interface{ Unwrap() error }:
			visit(wrapped.Unwrap())
		}
	}
	visit(err)
	if remaining == 0 {
		out.WriteString("\n[error chain traversal limit reached]")
	}
	return out.String()
}
