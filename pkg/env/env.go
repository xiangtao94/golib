package env

import (
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
)

const DefaultRootPath = "."

const (
	I18N_CONTEXT = "_i18n"
	I18N_ZH      = "zh"
	I18N_EN      = "en"
)

const (
	APP_NAME = "XT_APP_NAME"
)

var runtimeState = struct {
	sync.RWMutex
	localIP  string
	rootPath string
	isDocker bool
	appName  string
	language string
}{language: I18N_ZH}

func init() {
	runtimeState.localIP = GetInternalIp()
	runtimeState.isDocker = os.Getenv(gin.EnvGinMode) == gin.ReleaseMode
	runtimeState.appName = GetEnv(APP_NAME, "XT")
}

// RootPath 返回应用的根目录
func GetRootPath() string {
	runtimeState.RLock()
	defer runtimeState.RUnlock()
	if runtimeState.rootPath != "" {
		return runtimeState.rootPath
	}
	return DefaultRootPath
}

func GetLanguage() string {
	runtimeState.RLock()
	defer runtimeState.RUnlock()
	return runtimeState.language
}

func SetLanguage(lang string) {
	runtimeState.Lock()
	runtimeState.language = lang
	runtimeState.Unlock()
}

// GetConfDirPath 返回配置文件目录绝对地址
func GetConfDirPath() string {
	return filepath.Join(GetRootPath(), "conf")
}

// LogRootPath 返回log目录的绝对地址
func GetLogDirPath() string {
	return filepath.Join(GetRootPath(), "log")
}

// 判断项目运行平台
func IsDockerPlatform() bool {
	runtimeState.RLock()
	defer runtimeState.RUnlock()
	return runtimeState.isDocker
}

// 手动指定SetAppName
func SetAppName(appName string) {
	runtimeState.Lock()
	runtimeState.appName = appName
	runtimeState.Unlock()
}

func GetAppName() string {
	runtimeState.RLock()
	defer runtimeState.RUnlock()
	return runtimeState.appName
}

// SetRootPath 设置应用的根目录
func SetRootPath(r string) {
	runtimeState.Lock()
	defer runtimeState.Unlock()
	if !runtimeState.isDocker {
		runtimeState.rootPath = r
	}
}

func GetLocalIP() string {
	runtimeState.RLock()
	defer runtimeState.RUnlock()
	return runtimeState.localIP
}

func GetInternalIp() string {
	addr, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addr {
		if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}

	return ""
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
