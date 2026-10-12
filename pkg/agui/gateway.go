// Package agui proxies the AG-UI HTTP/SSE binding through a Gin business service.
// It owns transport behavior, not Agent execution, authentication, or storage.
package agui

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xiangtao94/golib"
	"github.com/xiangtao94/golib/pkg/zlog"
)

const (
	DefaultMaxRequestBytes = 4 << 20
	DefaultMaxEventBytes   = 8 << 20
	DefaultMaxConcurrent   = 128
	DefaultMaxRunDuration  = 30 * time.Minute
)

// HeaderFunc adds server-trusted upstream headers, such as service credentials.
// Incoming browser Authorization and Cookie headers are never copied by default.
type HeaderFunc func(context.Context) (http.Header, error)

type Config struct {
	Endpoint        string
	AllowHTTP       bool
	Client          *http.Client
	Headers         HeaderFunc
	MaxRequestBytes int64
	MaxEventBytes   int
	MaxConcurrent   int
	MaxRunDuration  time.Duration
}

type Gateway struct {
	endpoint        string
	client          *http.Client
	ownedTransport  *http.Transport
	headers         HeaderFunc
	maxRequestBytes int64
	maxEventBytes   int
	maxRunDuration  time.Duration
	slots           chan struct{}
}

func New(config Config) (*Gateway, error) {
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Host == "" ||
		(endpoint.Scheme != "https" && !(config.AllowHTTP && endpoint.Scheme == "http")) ||
		endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errors.New("agui: endpoint must be an absolute HTTPS URL without credentials, query, or fragment (HTTP requires AllowHTTP)")
	}
	if config.MaxRequestBytes < 0 || config.MaxEventBytes < 0 ||
		config.MaxConcurrent < 0 || config.MaxRunDuration < 0 {
		return nil, errors.New("agui: limits cannot be negative")
	}
	if config.MaxRequestBytes == math.MaxInt64 {
		return nil, errors.New("agui: MaxRequestBytes must be less than MaxInt64")
	}
	if config.MaxRequestBytes == 0 {
		config.MaxRequestBytes = DefaultMaxRequestBytes
	}
	if config.MaxEventBytes == 0 {
		config.MaxEventBytes = DefaultMaxEventBytes
	}
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = DefaultMaxConcurrent
	}
	if config.MaxRunDuration == 0 {
		config.MaxRunDuration = DefaultMaxRunDuration
	}
	client := config.Client
	var ownedTransport *http.Transport
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.ResponseHeaderTimeout = 30 * time.Second
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		client = &http.Client{Transport: transport}
		ownedTransport = transport
	}
	if client.Timeout != 0 {
		return nil, errors.New("agui: client timeout must be zero; use MaxRunDuration for streams")
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Gateway{
		endpoint:        endpoint.String(),
		client:          &clientCopy,
		ownedTransport:  ownedTransport,
		headers:         config.Headers,
		maxRequestBytes: config.MaxRequestBytes,
		maxEventBytes:   config.MaxEventBytes,
		maxRunDuration:  config.MaxRunDuration,
		slots:           make(chan struct{}, config.MaxConcurrent),
	}, nil
}

// Close releases idle connections only when the gateway created its own
// transport. An injected HTTP client remains owned by its caller.
func (g *Gateway) Close() {
	if g != nil && g.ownedTransport != nil {
		g.ownedTransport.CloseIdleConnections()
	}
}

// Register installs the AG-UI POST route. Apply service authentication and
// authorization middleware to routes before registering the gateway.
func (g *Gateway) Register(routes gin.IRoutes, path string) {
	routes.POST(path, g.Handle)
}

// Wrap preserves the original response writer so the AG-UI route can clear
// only its own write deadline, even when Gin middleware wraps its writer.
func Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := context.WithValue(request.Context(), originalWriterKey{}, writer)
		handler.ServeHTTP(writer, request.WithContext(ctx))
	})
}

// NewHTTPServer keeps normal golib timeouts for other routes and enables the
// AG-UI route's per-request stream deadline adjustment.
func NewHTTPServer(handler http.Handler, config golib.HTTPServerConfig) *golib.HTTPServer {
	return golib.NewHTTPServer(Wrap(handler), config)
}

type originalWriterKey struct{}

// Handle forwards one AG-UI run request and its SSE response. It requires
// Wrap or NewHTTPServer; without one, no upstream run is started.
func (g *Gateway) Handle(ctx *gin.Context) {
	writer, ok := ctx.Request.Context().Value(originalWriterKey{}).(http.ResponseWriter)
	if !ok {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := http.NewResponseController(writer).SetWriteDeadline(time.Time{}); err != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	select {
	case g.slots <- struct{}{}:
		defer func() { <-g.slots }()
	default:
		ctx.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	mediaType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		ctx.AbortWithStatus(http.StatusUnsupportedMediaType)
		return
	}
	if !acceptsEventStream(ctx.GetHeader("Accept")) {
		ctx.AbortWithStatus(http.StatusNotAcceptable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, g.maxRequestBytes+1))
	if err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if int64(len(body)) > g.maxRequestBytes {
		ctx.AbortWithStatus(http.StatusRequestEntityTooLarge)
		return
	}
	var input struct {
		ThreadID *string         `json:"threadId"`
		RunID    *string         `json:"runId"`
		Messages json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &input); err != nil || input.ThreadID == nil ||
		input.RunID == nil || !bytes.HasPrefix(bytes.TrimSpace(input.Messages), []byte("[")) {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	requestCtx, requestID := zlog.EnsureRequestID(ctx.Request.Context(), ctx.GetHeader(zlog.HeaderRequestID))
	requestCtx, cancel := context.WithTimeout(requestCtx, g.maxRunDuration)
	defer cancel()
	upstream, err := http.NewRequestWithContext(requestCtx, http.MethodPost, g.endpoint, bytes.NewReader(body))
	if err != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if g.headers != nil {
		headers, err := g.headers(requestCtx)
		if err != nil {
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		upstream.Header = headers.Clone()
	}
	if upstream.Header == nil {
		upstream.Header = make(http.Header)
	}
	upstream.Header.Set("Content-Type", "application/json")
	upstream.Header.Set("Accept", "text/event-stream")
	upstream.Header.Set(zlog.HeaderRequestID, requestID)

	response, err := g.client.Do(upstream)
	if err != nil {
		ctx.AbortWithStatus(statusForUpstreamError(requestCtx, err))
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		ctx.AbortWithStatus(response.StatusCode)
		return
	}
	responseType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || responseType != "text/event-stream" {
		ctx.AbortWithStatus(http.StatusBadGateway)
		return
	}
	g.forwardStream(ctx, requestCtx, response.Body, *input.RunID)
}

func acceptsEventStream(value string) bool {
	for part := range strings.SplitSeq(value, ",") {
		mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err == nil && mediaType == "text/event-stream" {
			if quality, ok := params["q"]; ok {
				weight, err := strconv.ParseFloat(quality, 64)
				if err != nil || weight <= 0 || weight > 1 {
					continue
				}
			}
			return true
		}
	}
	return false
}

func statusForUpstreamError(ctx context.Context, err error) int {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}
