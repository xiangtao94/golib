package agui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/xiangtao94/golib/pkg/middleware"
)

var errEventTooLarge = errors.New("agui: SSE event exceeds configured limit")
var errInvalidEvent = errors.New("agui: invalid SSE event")

func (g *Gateway) forwardStream(ctx *gin.Context, runCtx context.Context, body io.Reader, requestedRunID string) {
	reader := bufio.NewReader(body)
	opened := false
	activeRunID := ""
	for {
		frame, err := readFrame(reader, g.maxEventBytes)
		if err != nil {
			if !opened {
				ctx.AbortWithStatus(statusForUpstreamError(runCtx, err))
				return
			}
			if ctx.Request.Context().Err() == nil {
				writeGatewayRunError(ctx)
			}
			return
		}
		eventType, runID, err := eventIdentity(frame)
		if err != nil {
			if !opened {
				ctx.AbortWithStatus(http.StatusBadGateway)
				return
			}
			if ctx.Request.Context().Err() == nil {
				writeGatewayRunError(ctx)
			}
			return
		}
		if !opened {
			middleware.UploadEventStream(ctx)
			opened = true
		}
		if _, err := ctx.Writer.Write(frame); err != nil {
			return
		}
		ctx.Writer.Flush()
		switch eventType {
		case "RUN_STARTED":
			activeRunID = runID
		case "RUN_FINISHED":
			if runID == requestedRunID {
				return
			}
			if runID == activeRunID {
				activeRunID = ""
			}
		case "RUN_ERROR":
			if activeRunID == "" || activeRunID == requestedRunID {
				return
			}
			activeRunID = ""
		}
	}
}

func readFrame(reader *bufio.Reader, maxBytes int) ([]byte, error) {
	frame := make([]byte, 0, 512)
	for {
		line, err := readLine(reader, maxBytes-len(frame))
		if err != nil {
			return nil, err
		}
		frame = append(frame, line...)
		if bytes.Equal(line, []byte("\n")) || bytes.Equal(line, []byte("\r\n")) {
			return frame, nil
		}
	}
}

func readLine(reader *bufio.Reader, remaining int) ([]byte, error) {
	var line []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > remaining {
			return nil, errEventTooLarge
		}
		line = append(line, part...)
		if err == nil {
			return line, nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, err
		}
	}
}

func eventIdentity(frame []byte) (eventType, runID string, err error) {
	var data []byte
	hasData := false
	for line := range bytes.SplitSeq(frame, []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		hasData = true
		value := bytes.TrimPrefix(line, []byte("data:"))
		value = bytes.TrimPrefix(value, []byte(" "))
		if len(data) > 0 {
			data = append(data, '\n')
		}
		data = append(data, value...)
	}
	if !hasData {
		return "", "", nil // SSE comment or empty frame.
	}
	var event struct {
		Type  string `json:"type"`
		RunID string `json:"runId"`
	}
	if err := json.Unmarshal(data, &event); err != nil || event.Type == "" {
		return "", "", errInvalidEvent
	}
	return event.Type, event.RunID, nil
}

func writeGatewayRunError(ctx *gin.Context) {
	const event = "data: {\"type\":\"RUN_ERROR\",\"message\":\"Agent stream interrupted\",\"code\":\"UPSTREAM_STREAM_ERROR\"}\n\n"
	if _, err := ctx.Writer.WriteString(event); err == nil {
		ctx.Writer.Flush()
	}
}
