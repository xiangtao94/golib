package zlog

import (
	"fmt"
	"github.com/xiangtao94/golib/pkg/env"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 对用户暴露的log配置
type Buffer struct {
	Switch        string        `yaml:"switch"`
	Size          int           `yaml:"size"`
	FlushInterval time.Duration `yaml:"flushInterval"`
}

type LogConfig struct {
	Level     string `yaml:"level"` // 显示的日志等级
	Stdout    bool   `yaml:"stdout"`
	Buffer    Buffer `yaml:"buffer"`
	LogToFile bool   `yaml:"logToFile"`
	Format    string `yaml:"format"`
	LogDir    string `yaml:"logDir"`
}

func (conf LogConfig) SetLogLevel() {
	logConfig.ZapLevel = getLogLevel(conf.Level)
}

func getLogLevel(lv string) (level zapcore.Level) {
	str := strings.ToUpper(lv)
	switch str {
	case "DEBUG":
		level = zap.DebugLevel
	case "INFO":
		level = zap.InfoLevel
	case "WARN":
		level = zap.WarnLevel
	case "ERROR":
		level = zap.ErrorLevel
	case "FATAL":
		level = zap.FatalLevel
	default:
		level = zap.InfoLevel
	}
	return level
}

func (conf LogConfig) SetBuffer() {
	if conf.Buffer.Switch == "false" {
		// 明确关闭buffer
		logConfig.BufferSwitch = false
	} else if conf.Buffer.Switch == "true" {
		// 明确开启buffer
		logConfig.BufferSwitch = true
	} else {
		// 默认buffer设置
		if env.IsDockerPlatform() {
			// 容器环境默认开启
			logConfig.BufferSwitch = true
		} else {
			// 其他环境默认不开启
			logConfig.BufferSwitch = false
		}
	}

	if conf.Buffer.Size != 0 {
		logConfig.BufferSize = conf.Buffer.Size
	}
	if conf.Buffer.FlushInterval != 0 {
		logConfig.BufferFlushInterval = conf.Buffer.FlushInterval
	}
}

func (conf LogConfig) SetLogOutput() {
	if env.IsDockerPlatform() && !conf.LogToFile {
		// 容器环境
		logConfig.Log2File = false
	} else {
		// 开发环境下默认输出到文件
		logConfig.Log2File = true
		logConfig.Path = env.GetLogDirPath()
		// 目录不存在则先创建目录
		if _, err := os.Stat(logConfig.Path); os.IsNotExist(err) {
			err = os.MkdirAll(logConfig.Path, 0777)
			if err != nil {
				panic(fmt.Errorf("log conf err: create log dir '%s' error: %s", logConfig.Path, err))
			}
		}
	}
}

// 全局配置 仅限Init函数进行变更
var logConfig = struct {
	ZapLevel zapcore.Level

	// 以下变量仅对开发环境生效
	Log2File   bool
	Path       string
	ModuleName string
	// 缓冲区
	BufferSwitch        bool
	BufferSize          int
	BufferFlushInterval time.Duration
	LogFormat           string
}{
	ZapLevel: zapcore.InfoLevel,

	Log2File:   true,
	Path:       "./log",
	ModuleName: "xt-demo",

	// 缓冲区，如果不配置默认使用以下配置
	BufferSwitch:        true,
	BufferSize:          256 * 1024, // 256kb
	BufferFlushInterval: 5 * time.Second,
	LogFormat:           "json",
}

func InitLog(conf LogConfig) *zap.SugaredLogger {
	logConfig.ModuleName = env.AppName
	// 全局日志级别
	conf.SetLogLevel()
	// 日志缓冲区设置
	conf.SetBuffer()
	// 日志输出方式
	conf.SetLogOutput()
	// 初始化全局logger
	globalLogger = GetGlobalLogger()
	Info(nil, "Logger initialized")
	return globalLogger
}

func CloseLogger() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
	// 同步缓存的 Logger
	zapCacheLock.Lock()
	for _, logger := range zapLoggerCache {
		if logger != nil {
			_ = logger.Sync()
		}
	}
	zapCacheLock.Unlock()
	if accessLogger != nil {
		_ = accessLogger.Sync()
	}
}
