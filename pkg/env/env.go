package env

import (
	"github.com/gin-gonic/gin"
	"net"
	"os"
	"path/filepath"
)

const DefaultRootPath = "."

var (
	// 本地ip
	LocalIP string
	// 根目录
	rootPath string
	// 是否docker运行
	isDocker bool
	// 项目AppName
	AppName = "demo"
	// 国际化默认语言 zh 、en
	DefaultLang = "zh"
)

func init() {
	LocalIP = GetInternalIp()
	isDocker = false
	// 运行环境
	r := os.Getenv(gin.EnvGinMode)
	if r == gin.ReleaseMode {
		isDocker = true
	}
}

// RootPath 返回应用的根目录
func GetRootPath() string {
	if rootPath != "" {
		return rootPath
	} else {
		return DefaultRootPath
	}
}

func GetLanguage() string {
	return DefaultLang
}

func SetLanguage(lang string) {
	DefaultLang = lang
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
	return isDocker
}

// 手动指定SetAppName
func SetAppName(appName string) {
	AppName = appName
}

func GetAppName() string {
	return AppName
}

// SetRootPath 设置应用的根目录
func SetRootPath(r string) {
	if !isDocker {
		rootPath = r
	}
}

func GetInternalIp() string {
	addr, err := net.InterfaceAddrs()
	if err != nil {
		panic(err.Error())
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
