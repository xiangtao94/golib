# Changelog

## v1.10.1

- Log private error chains (including joined causes), wrapping stacks and request IDs from `web.Handle` while preserving safe public error responses.
- Capture the wrapping call site in `errors.Error.Wrap`; add log-only `errors.Diagnostic` with bounded cause traversal.
- Include the logging stack for ERROR and higher levels in ordinary zlog loggers; access logs are unchanged.

## v1.9.0

- Default JSON responses use numeric `code` equal to the HTTP status and always include `data` (`null` on errors). Business error identifiers move to `error_code`; clients reading string values from `code` must update.
- Add `render.JSONRenderer.SuccessWithStatus` and `web.WithSuccessStatus` for successful responses such as `202 Accepted`.
- Keep the Go `render.Response.Code` string field and custom renderer factory API unchanged. SSE error events keep their existing payload format.
