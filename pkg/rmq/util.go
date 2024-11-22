package rmq

import (
	"fmt"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/xiangtao94/golib/pkg/zlog"
	"net"
	"net/http"
	"strings"
)

// 规范化 key 并添加前缀生成规范化 header
// 流程：将 key 中的下划线转化为中划线，中划线分割的各段首字母大写，其他字母小写，并添加前缀（如果不存在）
func key2Header(key string) string {
	s := http.CanonicalHeaderKey(strings.ReplaceAll(key, "_", "-"))
	// only append header prefix when it doesn't exist.
	if !strings.HasPrefix(s, HeaderPre) {
		s = HeaderPre + s
	}
	return s
}

// 去掉前缀，返回规范化 key
func header2Key(key string) string {
	s := http.CanonicalHeaderKey(strings.ReplaceAll(key, "_", "-"))
	return strings.TrimPrefix(s, HeaderPre)
}

func fmtHeaders(m *primitive.Message, prefix string) string {
	headerStr := ""
	for key, value := range m.GetProperties() {
		if strings.HasPrefix(key, prefix) {
			headerStr += fmt.Sprintf("%s:%s;", key, value)
		}
	}
	return headerStr
}

func getHostListByDns(nameServers []string) (hostList []string) {
	for _, ns := range nameServers {
		host, port, err := net.SplitHostPort(ns)
		if err != nil {
			logger.Warn("invalid nameserver config",
				fields(zlog.String("ns", ns), zlog.String("err", err.Error()))...)
			continue
		}
		// have to resolve the domain name to ips
		addrs, err := net.LookupHost(host)
		if err != nil {
			logger.Warn("failed to lookup nameserver",
				fields(zlog.String("ns", ns), zlog.String("err", err.Error()))...)
			continue
		}

		for _, addr := range addrs {
			hostList = append(hostList, addr+":"+port)
		}
	}

	return hostList
}
