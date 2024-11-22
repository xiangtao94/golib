package http

import (
	"context"
	"crypto/tls"
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/zlog"
	"net/http"
	"net/http/httptrace"
	"time"
)

type timeTrace struct {
	dnsStartTime,
	dnsDoneTime,
	connectStartTime,
	connectDoneTime,
	tlsHandshakeStartTime,
	tlsHandshakeDoneTime,
	getConnTime,
	gotConnTime,
	gotFirstRespTime,
	finishTime time.Time
}

func beforeHttpStat(ctx *gin.Context, client *HttpClientConf, req *http.Request) *timeTrace {
	if client.HttpStat == false {
		return nil
	}

	var t = &timeTrace{}
	trace := &httptrace.ClientTrace{
		// before get a connection
		GetConn:  func(_ string) { t.getConnTime = time.Now() },
		DNSStart: func(_ httptrace.DNSStartInfo) { t.dnsStartTime = time.Now() },
		DNSDone:  func(_ httptrace.DNSDoneInfo) { t.dnsDoneTime = time.Now() },
		// before a new connection
		ConnectStart: func(_, _ string) { t.connectStartTime = time.Now() },
		// after a new connection
		ConnectDone: func(net, addr string, err error) { t.connectDoneTime = time.Now() },
		// after get a connection
		GotConn:              func(_ httptrace.GotConnInfo) { t.gotConnTime = time.Now() },
		GotFirstResponseByte: func() { t.gotFirstRespTime = time.Now() },
		TLSHandshakeStart:    func() { t.tlsHandshakeStartTime = time.Now() },
		TLSHandshakeDone:     func(_ tls.ConnectionState, _ error) { t.tlsHandshakeDoneTime = time.Now() },
	}
	*req = *req.WithContext(httptrace.WithClientTrace(context.Background(), trace))
	return t
}

func afterHttpStat(ctx *gin.Context, client *HttpClientConf, scheme string, t *timeTrace) {
	if client.HttpStat == false {
		return
	}
	t.finishTime = time.Now() // after read body

	cost := func(d time.Duration) float64 {
		if d < 0 {
			return -1
		}
		return float64(d.Nanoseconds()/1e4) / 100.0
	}

	serverProcessDuration := t.gotFirstRespTime.Sub(t.gotConnTime)
	contentTransDuration := t.finishTime.Sub(t.gotFirstRespTime)
	if t.gotConnTime.IsZero() {
		// 没有拿到连接的情况
		serverProcessDuration = 0
		contentTransDuration = 0
	}

	switch scheme {
	case "https":
		f := []zlog.Field{
			zlog.Float64("dnsLookupCost", cost(t.dnsDoneTime.Sub(t.dnsStartTime))),                       // dns lookup
			zlog.Float64("tcpConnectCost", cost(t.connectDoneTime.Sub(t.connectStartTime))),              // tcp connection
			zlog.Float64("tlsHandshakeCost", cost(t.tlsHandshakeStartTime.Sub(t.tlsHandshakeStartTime))), // tls handshake
			zlog.Float64("serverProcessCost", cost(serverProcessDuration)),                               // server processing
			zlog.Float64("contentTransferCost", cost(contentTransDuration)),                              // content transfer
			zlog.Float64("totalCost", cost(t.finishTime.Sub(t.getConnTime))),                             // total cost
		}
		zlog.InfoLogger(ctx, "time trace", f...)
	case "http":
		f := []zlog.Field{
			zlog.Float64("dnsLookupCost", cost(t.dnsDoneTime.Sub(t.dnsStartTime))),          // dns lookup
			zlog.Float64("tcpConnectCost", cost(t.connectDoneTime.Sub(t.connectStartTime))), // tcp connection
			zlog.Float64("serverProcessCost", cost(serverProcessDuration)),                  // server processing
			zlog.Float64("contentTransferCost", cost(contentTransDuration)),                 // content transfer
			zlog.Float64("totalCost", cost(t.finishTime.Sub(t.getConnTime))),                // total cost
		}
		zlog.InfoLogger(ctx, "time trace", f...)
	}
}
