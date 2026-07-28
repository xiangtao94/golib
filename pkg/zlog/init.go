package zlog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 对用户暴露的log配置
type Buffer struct {
	Enabled       bool          `yaml:"enabled"`
	Size          int           `yaml:"size"`
	FlushInterval time.Duration `yaml:"flushInterval"`
}

type LogConfig struct {
	AppName   string `yaml:"appName"`
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
		AppName:   "server",
		Level:     "info",
		Stdout:    true,
		LogToFile: false,
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
	if userConf.AppName == "" {
		userConf.AppName = defaultConf.AppName
	}
	userConf.Level = strings.ToLower(userConf.Level)
	userConf.Format = strings.ToLower(userConf.Format)

	// Buffer 配置合并
	if userConf.Buffer.Size == 0 {
		userConf.Buffer.Size = defaultConf.Buffer.Size
	}
	if userConf.Buffer.FlushInterval == 0 {
		userConf.Buffer.FlushInterval = defaultConf.Buffer.FlushInterval
	}

	return userConf
}

func (conf LogConfig) setLogLevel() {
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

func (conf LogConfig) setBuffer() {
	logConfig.BufferSwitch = conf.Buffer.Enabled
	logConfig.BufferSize = conf.Buffer.Size
	logConfig.BufferFlushInterval = conf.Buffer.FlushInterval
}

func validateAndPrepareLogConfig(conf LogConfig) error {
	switch conf.Level {
	case "debug", "info", "warn", "error", "fatal":
	default:
		return fmt.Errorf("log conf: unsupported level %q", conf.Level)
	}
	switch conf.Format {
	case "json", "console":
	default:
		return fmt.Errorf("log conf: unsupported format %q", conf.Format)
	}
	if conf.AppName == "." || conf.AppName == ".." || filepath.Base(conf.AppName) != conf.AppName {
		return fmt.Errorf("log conf: app name %q must not contain a path", conf.AppName)
	}
	if conf.Buffer.Enabled && !conf.LogToFile {
		return errors.New("log conf: buffer requires file output")
	}
	if conf.Buffer.Size <= 0 {
		return errors.New("log conf: buffer size must be positive")
	}
	if conf.Buffer.FlushInterval <= 0 {
		return errors.New("log conf: buffer flush interval must be positive")
	}
	if conf.LogToFile {
		if err := os.MkdirAll(conf.LogDir, 0o755); err != nil {
			return fmt.Errorf("log conf: create log dir %q: %w", conf.LogDir, err)
		}
	}
	return nil
}

func (conf LogConfig) setLogOutput() {
	logConfig.Path = conf.LogDir
	logConfig.LogFormat = conf.Format
	logConfig.Stdout = conf.Stdout
	logConfig.Log2File = conf.LogToFile
}

// 全局配置 仅限Init函数进行变更
var logConfig = struct {
	ZapLevel zapcore.Level
	Stdout   bool

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
	Stdout:   true,

	Log2File:   false,
	Path:       "./log",
	ModuleName: "server",

	BufferSwitch:        false,
	BufferSize:          256 * 1024, // 256kb
	BufferFlushInterval: 5 * time.Second,
	LogFormat:           "json",
}

// InitLog replaces the process logger configuration. It must be called during
// application startup, before request-serving goroutines are launched.
func InitLog(conf LogConfig) (*zap.SugaredLogger, error) {
	logConf := mergeWithDefault(conf)
	if err := validateAndPrepareLogConfig(logConf); err != nil {
		return nil, err
	}

	loggerLifecycleMu.Lock()
	defer loggerLifecycleMu.Unlock()

	_ = closeLoggerLocked()
	resetLoggerLocked()
	logConfig.ModuleName = logConf.AppName
	logConf.setLogLevel()
	logConf.setBuffer()
	logConf.setLogOutput()
	globalLogger = newLoggerWithSkipLocked(1).Sugar()
	globalLogger.Info("Logger initialized")
	return globalLogger, nil
}

func resetLoggerLocked() {
	baseZapCore = nil
	baseAccessCore = nil
	zapLoggerCache = make(map[int]*zap.Logger)
	globalLogger = nil
	accessLogger = nil
}

func closeLoggerLocked() error {
	var closeErrors []error
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
	for _, logger := range zapLoggerCache {
		if logger != nil {
			_ = logger.Sync()
		}
	}
	if accessLogger != nil {
		_ = accessLogger.Sync()
	}
	for _, writer := range bufferedWriters {
		if err := writer.Stop(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	bufferedWriters = nil
	for _, closer := range logClosers {
		if err := closer.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	logClosers = nil
	return errors.Join(closeErrors...)
}

func CloseLogger() error {
	loggerLifecycleMu.Lock()
	defer loggerLifecycleMu.Unlock()
	err := closeLoggerLocked()
	resetLoggerLocked()
	return err
}
