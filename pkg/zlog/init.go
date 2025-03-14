package zlog

import (
	"fmt"
	"github.com/xiangtao94/golib/pkg/env"
	"go.uber.org/zap/buffer"
	"os"
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
	Level     string `yaml:"level"`
	Stdout    bool   `yaml:"stdout"`
	Buffer    Buffer `yaml:"buffer"`
	LogToFile bool   `yaml:"logToFile"`
}

func (conf LogConfig) SetLogLevel() {
	logConfig.ZapLevel = getLogLevel(conf.Level)
}

func (conf LogConfig) SetBuffer() {
	if conf.Buffer.Switch == "false" || conf.Buffer.Switch == "0" {
		// 明确关闭buffer
		logConfig.BufferSwitch = false
	} else if conf.Buffer.Switch == "true" || conf.Buffer.Switch == "1" {
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
		logConfig.Stdout = true
	} else {
		// 开发环境下默认输出到文件，支持自定义是否输出到终端
		logConfig.Log2File = true
		logConfig.Stdout = conf.Stdout
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
	Stdout     bool
	Log2File   bool
	Path       string
	ModuleName string
	// 缓冲区
	BufferSwitch        bool
	BufferSize          int
	BufferFlushInterval time.Duration
}{
	ZapLevel: zapcore.InfoLevel,

	Stdout:     false,
	Log2File:   true,
	Path:       "./log",
	ModuleName: "xt-demo",

	// 缓冲区，如果不配置默认使用以下配置
	BufferSwitch:        true,
	BufferSize:          256 * 1024, // 256kb
	BufferFlushInterval: 5 * time.Second,
}

func InitLog(moduleName string, conf LogConfig) *zap.SugaredLogger {
	// 定制日志格式
	if err := RegisterJSONEncoder(moduleName); err != nil {
		println("RegisterJSONEncoder: " + err.Error())
	}
	logConfig.ModuleName = moduleName
	// 全局日志级别
	conf.SetLogLevel()
	// 日志缓冲区设置
	conf.SetBuffer()
	// 日志输出方式
	conf.SetLogOutput()

	// 初始化全局logger
	SugaredLogger = GetLogger()
	return SugaredLogger
}

func RegisterJSONEncoder(moduleName string) error {
	return zap.RegisterEncoder(moduleName, func(cfg zapcore.EncoderConfig) (zapcore.Encoder, error) {
		jsonEncoder := zapcore.NewJSONEncoder(cfg)
		return &defaultEncoder{
			Encoder: jsonEncoder,
		}, nil
	})
}

type defaultEncoder struct {
	zapcore.Encoder
}

func (enc *defaultEncoder) Clone() zapcore.Encoder {
	encoderClone := enc.Encoder.Clone()
	return &defaultEncoder{Encoder: encoderClone}
}

func (enc *defaultEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	ent.Time = time.Now()
	return enc.Encoder.EncodeEntry(ent, fields)
}
