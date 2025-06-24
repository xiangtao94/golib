package zlog

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/xiangtao94/golib/pkg/env"
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

// DefaultLogConfig 返回默认的日志配置
func DefaultLogConfig() LogConfig {
	return LogConfig{
		Level:     "info",
		Stdout:    true,
		LogToFile: !env.IsDockerPlatform(), // 容器环境默认不输出到文件
		Format:    "json",
		LogDir:    "./log",
		Buffer: Buffer{
			Size:          256 * 1024,      // 256KB
			FlushInterval: 5 * time.Second, // 5秒
		},
	}
}

// mergeWithDefault 将用户配置与默认配置合并
func mergeWithDefault(userConf LogConfig) LogConfig {
	defaultConf := DefaultLogConfig()

	// 如果用户没有设置，使用默认值
	if userConf.Level == "" {
		userConf.Level = defaultConf.Level
	}
	if userConf.Format == "" {
		userConf.Format = defaultConf.Format
	}
	if userConf.LogDir == "" {
		userConf.LogDir = defaultConf.LogDir
	}

	// Buffer 配置合并
	if userConf.Buffer.Size == 0 {
		userConf.Buffer.Size = defaultConf.Buffer.Size
	}
	if userConf.Buffer.FlushInterval == 0 {
		userConf.Buffer.FlushInterval = defaultConf.Buffer.FlushInterval
	}

	return userConf
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
	// 使用用户配置的 LogDir
	if conf.LogDir != "" {
		logConfig.Path = conf.LogDir
	} else {
		logConfig.Path = env.GetLogDirPath()
	}

	// 使用用户配置的 Format
	if conf.Format != "" {
		logConfig.LogFormat = conf.Format
	}

	// 判断是否输出到文件
	if env.IsDockerPlatform() && !conf.LogToFile {
		// 容器环境且明确设置不输出到文件
		logConfig.Log2File = false
	} else if conf.LogToFile {
		// 明确设置输出到文件
		logConfig.Log2File = true
		// 目录不存在则先创建目录
		if _, err := os.Stat(logConfig.Path); os.IsNotExist(err) {
			err = os.MkdirAll(logConfig.Path, 0777)
			if err != nil {
				panic(fmt.Errorf("log conf err: create log dir '%s' error: %s", logConfig.Path, err))
			}
		}
	} else {
		// 未明确设置，使用环境判断
		logConfig.Log2File = !env.IsDockerPlatform()
		if logConfig.Log2File {
			// 目录不存在则先创建目录
			if _, err := os.Stat(logConfig.Path); os.IsNotExist(err) {
				err = os.MkdirAll(logConfig.Path, 0777)
				if err != nil {
					panic(fmt.Errorf("log conf err: create log dir '%s' error: %s", logConfig.Path, err))
				}
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

// InitLog 初始化日志，支持传入配置或使用默认配置
func InitLog(conf ...LogConfig) *zap.SugaredLogger {
	var logConf LogConfig
	if len(conf) > 0 {
		// 使用传入的配置，并与默认配置合并
		logConf = mergeWithDefault(conf[0])
	} else {
		// 使用默认配置
		logConf = DefaultLogConfig()
	}

	logConfig.ModuleName = env.AppName
	// 全局日志级别
	logConf.SetLogLevel()
	// 日志缓冲区设置
	logConf.SetBuffer()
	// 日志输出方式
	logConf.SetLogOutput()
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
